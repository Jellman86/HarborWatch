package updates

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

type Request struct {
	ContainerID string
	TargetImage string
	ValidateURL string
}

type Service struct {
	store    *Store
	executor Executor

	mu          sync.RWMutex
	subscribers map[string][]chan gen.UpdateStepEvent
}

func NewService(store *Store, executor Executor) *Service {
	return &Service{store: store, executor: executor, subscribers: map[string][]chan gen.UpdateStepEvent{}}
}

func NewServiceFromEnv() (*Service, error) {
	dbPath := os.Getenv("HARBORWATCH_DB_PATH")
	if dbPath == "" {
		dbPath = "/tmp/harborwatch.db"
	}
	store, err := OpenStore(dbPath)
	if err != nil {
		return nil, err
	}
	if err := store.Init(context.Background()); err != nil {
		return nil, err
	}
	return NewService(store, NewCommandExecutor()), nil
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
