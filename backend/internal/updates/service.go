package updates

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/ai"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/notifications"
)

type Request struct {
	ContainerID string
	TargetImage string
	ValidateURL string
}

type DiagService interface {
	Log(level, source, message string)
}

type Service struct {
	store    *Store
	executor Executor
	ai       *ai.Service
	notif    *notifications.Service
	diag     DiagService

	mu          sync.RWMutex
	subscribers map[string][]chan gen.UpdateStepEvent
}

func NewService(store *Store, executor Executor, aiSvc *ai.Service, notif *notifications.Service, diag DiagService) *Service {
	return &Service{
		store:       store,
		executor:    executor,
		ai:          aiSvc,
		notif:       notif,
		diag:        diag,
		subscribers: map[string][]chan gen.UpdateStepEvent{},
	}
}

func (s *Service) StartUpdate(req Request) (gen.UpdateStartResponse, error) {
	if req.ContainerID == "" || req.TargetImage == "" || req.ValidateURL == "" {
		return gen.UpdateStartResponse{}, errors.New("containerId, targetImage and validateUrl are required")
	}
	jobID, err := newID()
	if err != nil {
		return gen.UpdateStartResponse{}, err
	}
	now := time.Now().UTC().Unix()
	run := gen.UpdateJobStatus{
		JobID:       jobID,
		ContainerID: req.ContainerID,
		TargetImage: req.TargetImage,
		ValidateURL: req.ValidateURL,
		Status:      "running",
		CreatedAt:   now,
		UpdatedAt:   now,
		Steps:       []gen.UpdateStepEvent{},
	}
	if err := s.store.CreateRun(context.Background(), run); err != nil {
		return gen.UpdateStartResponse{}, err
	}
	go s.execute(jobID, req)
	return gen.UpdateStartResponse{JobID: jobID, Status: "running"}, nil
}

func (s *Service) GetJob(ctx context.Context, jobID string) (*gen.UpdateJobStatus, error) {
	return s.store.GetRun(ctx, jobID)
}

func (s *Service) Subscribe(jobID string) (<-chan gen.UpdateStepEvent, func()) {
	ch := make(chan gen.UpdateStepEvent, 8)
	s.mu.Lock()
	s.subscribers[jobID] = append(s.subscribers[jobID], ch)
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		list := s.subscribers[jobID]
		out := make([]chan gen.UpdateStepEvent, 0, len(list))
		for _, c := range list {
			if c != ch {
				out = append(out, c)
			}
		}
		s.subscribers[jobID] = out
		close(ch)
	}
}

func (s *Service) execute(jobID string, req Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	failed := s.runStep(ctx, jobID, "preflight", func(ctx context.Context) error { return s.executor.Preflight(ctx, req) })
	if failed != nil {
		s.finish(jobID, "failed", failed)
		return
	}

	// NEW: AI Release Analysis Step
	if s.ai != nil && s.ai.HasProvider() {
		if err := s.runStep(ctx, jobID, "release_analysis", func(ctx context.Context) error {
			// In a real implementation, we'd fetch the actual release notes here.
			// For now, we simulate with a placeholder.
			notes := "Placeholder release notes for " + req.TargetImage
			analysis, err := s.ai.AnalyzeReleaseNotes(ctx, notes)
			if err != nil {
				return err
			}
			summary := &gen.AIAnalysisSummary{
				RiskScore:       analysis.RiskScore,
				RiskLevel:       string(analysis.RiskLevel),
				Summary:         analysis.Summary,
				BreakingChanges: analysis.BreakingChanges,
			}
			_ = s.store.SaveAIAnalysis(ctx, jobID, summary)

			// Policy enforcement: Pause if high risk
			if analysis.RiskScore >= 80 {
				if s.notif != nil {
					s.notif.Dispatch(ctx, notifications.Message{
						Title:  "Update Paused: High Risk Detected",
						Body:   fmt.Sprintf("AI detected high risk (%d) for %s: %s", analysis.RiskScore, req.TargetImage, analysis.Summary),
						Level:  notifications.LevelCritical,
						Source: "Update Engine",
					})
				}
				return fmt.Errorf("AI detected high risk (%d): %s", analysis.RiskScore, analysis.Summary)
			}
			return nil
		}); err != nil {
			s.finish(jobID, "failed", err)
			return
		}
	}

	if err := s.runStep(ctx, jobID, "backup", func(ctx context.Context) error { return s.executor.Backup(ctx, req) }); err != nil {
		s.rollback(jobID, req, err)
		return
	}
	if err := s.runStep(ctx, jobID, "pull", func(ctx context.Context) error { return s.executor.Pull(ctx, req) }); err != nil {
		s.rollback(jobID, req, err)
		return
	}
	if err := s.runStep(ctx, jobID, "recreate", func(ctx context.Context) error { return s.executor.Recreate(ctx, req) }); err != nil {
		s.rollback(jobID, req, err)
		return
	}
	if err := s.runStep(ctx, jobID, "validate", func(ctx context.Context) error { return s.executor.Validate(ctx, req) }); err != nil {
		s.rollback(jobID, req, err)
		return
	}

	s.emit(jobID, gen.UpdateStepEvent{JobID: jobID, Step: "success", Status: "completed", Message: "Update pipeline completed", Timestamp: time.Now().UTC().Unix()})
	s.finish(jobID, "completed", nil)
}

func (s *Service) rollback(jobID string, req Request, cause error) {
	_ = s.runStep(context.Background(), jobID, "rollback", func(ctx context.Context) error { return s.executor.Rollback(ctx, req, cause) })
	s.finish(jobID, "rolled_back", cause)
}

func (s *Service) runStep(ctx context.Context, jobID, step string, fn func(context.Context) error) error {
	start := gen.UpdateStepEvent{JobID: jobID, Step: step, Status: "running", Message: "step started", Timestamp: time.Now().UTC().Unix()}
	s.emit(jobID, start)
	if err := fn(ctx); err != nil {
		s.emit(jobID, gen.UpdateStepEvent{JobID: jobID, Step: step, Status: "failed", Message: err.Error(), Timestamp: time.Now().UTC().Unix()})
		return err
	}
	s.emit(jobID, gen.UpdateStepEvent{JobID: jobID, Step: step, Status: "completed", Message: "step completed", Timestamp: time.Now().UTC().Unix()})
	return nil
}

func (s *Service) finish(jobID, status string, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
		if s.diag != nil {
			s.diag.Log("ERROR", "UpdateEngine", fmt.Sprintf("Job %s %s: %s", jobID, status, msg))
		}
		// Send notification for failures
		if (status == "failed" || status == "rolled_back") && s.notif != nil {
			s.notif.Dispatch(context.Background(), notifications.Message{
				Title:  fmt.Sprintf("Update Job %s: %s", status, jobID),
				Body:   fmt.Sprintf("Error: %s", msg),
				Level:  notifications.LevelCritical,
				Source: "Update Engine",
			})
		}
	}
	_ = s.store.UpdateRunStatus(context.Background(), jobID, status, msg, time.Now().UTC().Unix())
}

func (s *Service) emit(jobID string, e gen.UpdateStepEvent) {
	_ = s.store.AddStep(context.Background(), e)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ch := range s.subscribers[jobID] {
		select {
		case ch <- e:
		default:
		}
	}
}

func newID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
