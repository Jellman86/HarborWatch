package updates

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/ai"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/jobs"
	"github.com/Jellman86/HarborWatch/backend/internal/notifications"
	"github.com/Jellman86/HarborWatch/backend/internal/portainer"
)

type Request struct {
	ContainerID          string
	TargetImage          string
	ValidateURL          string
	CurrentImage         string
	ContainerName        string
	Labels               map[string]string
	RepositoryURL        string
	ChangelogURL         string
	ReleaseContext       string
	ValidateMode         string
	ValidateTimeoutSec   int
	ValidateIntervalSec  int
	AIValidateLogs       bool
	AIBlockRiskThreshold int
	BypassAI             bool
	SkipHealthCheck      bool

	// Portainer support
	IsPortainerManaged  bool
	PortainerStackID    int
	PortainerEndpointID int
}

type PortainerClient interface {
	GetStack(ctx context.Context, stackID int) (*portainer.Stack, error)
	GetStackFile(ctx context.Context, stackID int) (string, error)
	UpdateStack(ctx context.Context, stackID int, endpointID int, yaml string, env []map[string]string, prune bool, pullImage bool) error
	ListStacks(ctx context.Context) ([]portainer.Stack, error)
}

type DiagService interface {
	Log(level, source, message string)
}

type Service struct {
	store      *Store
	executor   Executor
	ai         *ai.Service
	notif      *notifications.Service
	diag       DiagService
	portainer  PortainerClient
	jobManager *jobs.Manager

	startMu     sync.Mutex
	mu          sync.RWMutex
	subscribers map[string][]chan gen.UpdateStepEvent
}

func NewService(store *Store, executor Executor, aiSvc *ai.Service, notif *notifications.Service, diag DiagService, portainer PortainerClient, jm *jobs.Manager) *Service {
	return &Service{
		store:       store,
		executor:    executor,
		ai:          aiSvc,
		notif:       notif,
		diag:        diag,
		portainer:   portainer,
		jobManager:  jm,
		subscribers: map[string][]chan gen.UpdateStepEvent{},
	}
}

func (s *Service) StartUpdate(req Request) (gen.UpdateStartResponse, error) {
	req.ValidateMode = normalizeValidateMode(req.ValidateMode)
	if req.ValidateTimeoutSec <= 0 {
		req.ValidateTimeoutSec = 45
	}
	if req.ValidateIntervalSec <= 0 {
		req.ValidateIntervalSec = 2
	}
	if req.ContainerID == "" || req.TargetImage == "" {
		return gen.UpdateStartResponse{}, errors.New("containerId and targetImage are required")
	}
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if (req.ValidateMode == "http" || req.ValidateMode == "both") && strings.TrimSpace(req.ValidateURL) == "" {
		return gen.UpdateStartResponse{}, errors.New("validateUrl is required for http or both validation mode")
	}
	if existing, err := s.store.ListRunsForContainer(context.Background(), req.ContainerID, 1); err == nil {
		if len(existing) > 0 && strings.EqualFold(strings.TrimSpace(existing[0].Status), "running") {
			return gen.UpdateStartResponse{}, fmt.Errorf("container %s already has a running update job (%s)", req.ContainerID, existing[0].JobID)
		}
	} else {
		return gen.UpdateStartResponse{}, fmt.Errorf("check existing update jobs: %w", err)
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
	if err := s.store.CreateRunWithContainerName(context.Background(), run, req.ContainerName); err != nil {
		return gen.UpdateStartResponse{}, err
	}
	go s.execute(jobID, req)
	return gen.UpdateStartResponse{JobID: jobID, Status: "running"}, nil
}

func (s *Service) GetJob(ctx context.Context, jobID string) (*gen.UpdateJobStatus, error) {
	return s.store.GetRun(ctx, jobID)
}

func (s *Service) ListContainerJobs(ctx context.Context, containerID string, limit int) ([]gen.UpdateJobStatus, error) {
	return s.store.ListRunsForContainer(ctx, containerID, limit)
}

func (s *Service) ActiveJobs() []gen.JobProgress {
	return s.jobManager.ActiveJobs()
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

func (s *Service) setRunProgress(jobID string, status string, progress int) {
	s.jobManager.UpdateJob(jobID, progress, status, "")
	_ = s.store.UpdateRunProgress(context.Background(), jobID, status, progress)
}

func (s *Service) execute(jobID string, req Request) {
	// Create cancellable context for the whole update pipeline
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	s.jobManager.RegisterJob(&jobs.Job{
		ID:         jobID,
		Type:       jobs.JobTypeUpdate,
		Target:     req.ContainerID,
		TargetName: req.ContainerName,
		Status:     "queued",
		Message:    "Waiting for concurrency slot",
		StartedAt:  time.Now().UTC().Unix(),
		Cancel:     runCancel,
	})
	defer s.jobManager.FinishJob(jobID)

	// Acquire slot with container lock
	ctx, cancel := context.WithTimeout(runCtx, 20*time.Minute)
	defer cancel()

	if err := s.jobManager.AcquireSlot(ctx, jobID, req.ContainerID); err != nil {
		s.finish(jobID, "failed", err)
		return
	}
	defer s.jobManager.ReleaseSlot(jobID, req.ContainerID)

	if req.IsPortainerManaged && s.portainer != nil {
		s.executePortainer(ctx, jobID, req)
		return
	}
	s.executeLocal(ctx, jobID, req)
}

func (s *Service) executeLocal(ctx context.Context, jobID string, req Request) {
	s.setRunProgress(jobID, "running", 5)
	failed := s.runStep(ctx, jobID, "preflight", func(ctx context.Context) error { return s.executor.Preflight(ctx, req) })
	if failed != nil {
		s.finish(jobID, "failed", failed)
		return
	}

	// AI analysis step...
	if !req.BypassAI && s.ai != nil && s.ai.HasProvider() {
		s.setRunProgress(jobID, "running", 10)
		if err := s.runStep(ctx, jobID, "release_analysis", func(ctx context.Context) error {
			notes := buildAIReleaseContext(req)
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

			threshold := req.AIBlockRiskThreshold
			if threshold < 0 || threshold > 100 {
				threshold = envInt("HW_AI_BLOCK_RISK_THRESHOLD", 80, 0, 100)
			}
			if blocked, reason := shouldBlockForAI(analysis, threshold); blocked {
				if s.notif != nil {
					s.notif.Dispatch(ctx, notifications.Message{
						Title:  "Update Blocked: AI High Risk",
						Body:   fmt.Sprintf("AI analysis detected potential breaking changes for %s and has blocked the automatic rollout.", req.ContainerName),
						Level:  notifications.LevelCritical,
						Source: "Update Engine",
						Fields: map[string]string{
							"Container":  req.ContainerName,
							"New Image":  req.TargetImage,
							"Risk Score": fmt.Sprintf("%d/100", analysis.RiskScore),
							"Reason":     reason,
						},
					})
				}
				return fmt.Errorf("AI blocked update: %s", reason)
			}
			return nil
		}); err != nil {
			s.finish(jobID, "failed", err)
			return
		}
	} else {
		message := "skipped: ai provider not configured"
		if req.BypassAI {
			message = "skipped: bypass requested by user"
		}
		s.emit(jobID, gen.UpdateStepEvent{
			JobID:     jobID,
			Step:      "release_analysis",
			Status:    "completed",
			Message:   message,
			Timestamp: time.Now().UTC().Unix(),
		})
	}

	s.setRunProgress(jobID, "running", 25)
	if err := s.runStep(ctx, jobID, "backup", func(ctx context.Context) error { return s.executor.Backup(ctx, req) }); err != nil {
		s.rollback(jobID, req, err)
		return
	}
	s.setRunProgress(jobID, "running", 35)
	if err := s.runStep(ctx, jobID, "pull", func(ctx context.Context) error { return s.executor.Pull(ctx, req) }); err != nil {
		s.rollback(jobID, req, err)
		return
	}
	s.setRunProgress(jobID, "running", 60)
	if err := s.runStep(ctx, jobID, "recreate", func(ctx context.Context) error { return s.executor.Recreate(ctx, req) }); err != nil {
		s.rollback(jobID, req, err)
		return
	}
	s.setRunProgress(jobID, "running", 80)
	if !req.SkipHealthCheck {
		if err := s.runStep(ctx, jobID, "validate", func(ctx context.Context) error { return s.executor.Validate(ctx, req) }); err != nil {
			s.rollback(jobID, req, err)
			return
		}
	} else {
		s.emit(jobID, gen.UpdateStepEvent{
			JobID:     jobID,
			Step:      "validate",
			Status:    "completed",
			Message:   "skipped: force update requested (no health check)",
			Timestamp: time.Now().UTC().Unix(),
		})
		s.setRunProgress(jobID, "running", 90)
	}
	// On success, cleanup backups
	s.setRunProgress(jobID, "running", 95)
	_ = s.runStep(ctx, jobID, "cleanup", func(ctx context.Context) error { return s.executor.Cleanup(ctx, req) })
	if req.AIValidateLogs && s.ai != nil && s.ai.HasProvider() {
		if err := s.runStep(ctx, jobID, "ai_health_assessment", func(ctx context.Context) error {
			logTail := envInt("HW_AI_HEALTH_LOG_TAIL", 300, 50, 2000)
			logs, err := collectContainerLogsForAI(ctx, req.ContainerID, logTail)
			if err != nil {
				return fmt.Errorf("collect container logs for AI: %w", err)
			}
			assessment, err := s.ai.AnalyzeHealthLogs(ctx, req.ContainerID, logs)
			if err != nil {
				return fmt.Errorf("AI health assessment failed: %w", err)
			}
			if !assessment.Healthy {
				return fmt.Errorf("AI health assessment marked container unhealthy (confidence %d): %s", assessment.Confidence, assessment.Summary)
			}
			return nil
		}); err != nil {
			s.rollback(jobID, req, err)
			return
		}
	}

	s.setRunProgress(jobID, "completed", 100)
	if s.notif != nil {
		s.notif.Dispatch(ctx, notifications.Message{
			Title:  "Update Successful",
			Body:   fmt.Sprintf("Container %s has been successfully updated to the latest image.", req.ContainerName),
			Level:  notifications.LevelInfo,
			Source: "Update Engine",
			Fields: map[string]string{
				"Container": req.ContainerName,
				"New Image": req.TargetImage,
				"Status":    "Ready",
			},
		})
	}
	s.emit(jobID, gen.UpdateStepEvent{JobID: jobID, Step: "success", Status: "completed", Message: "Update pipeline completed", Timestamp: time.Now().UTC().Unix()})
	s.finish(jobID, "completed", nil)
}

func (s *Service) executePortainer(ctx context.Context, jobID string, req Request) {
	s.setRunProgress(jobID, "running", 5)
	if err := s.runStep(ctx, jobID, "preflight", func(ctx context.Context) error {
		if req.PortainerStackID == 0 {
			return errors.New("portainer stack id is required")
		}
		if s.portainer == nil {
			return errors.New("portainer client not initialized")
		}
		return nil
	}); err != nil {
		s.finish(jobID, "failed", err)
		return
	}

	// AI analysis step...
	if !req.BypassAI && s.ai != nil && s.ai.HasProvider() {
		s.setRunProgress(jobID, "running", 10)
		if err := s.runStep(ctx, jobID, "release_analysis", func(ctx context.Context) error {
			notes := buildAIReleaseContext(req)
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

			threshold := req.AIBlockRiskThreshold
			if threshold < 0 || threshold > 100 {
				threshold = envInt("HW_AI_BLOCK_RISK_THRESHOLD", 80, 0, 100)
			}
			if blocked, reason := shouldBlockForAI(analysis, threshold); blocked {
				return fmt.Errorf("AI blocked update: %s", reason)
			}
			return nil
		}); err != nil {
			s.finish(jobID, "failed", err)
			return
		}
	} else {
		message := "skipped: ai provider not configured"
		if req.BypassAI {
			message = "skipped: bypass requested by user"
		}
		s.emit(jobID, gen.UpdateStepEvent{
			JobID:     jobID,
			Step:      "release_analysis",
			Status:    "completed",
			Message:   message,
			Timestamp: time.Now().UTC().Unix(),
		})
	}

	s.setRunProgress(jobID, "running", 20)
	var stackYAML string
	var existingEnv []map[string]string
	if err := s.runStep(ctx, jobID, "fetch_config", func(ctx context.Context) error {
		// Fetch both the file and the stack metadata to preserve Envs
		yaml, err := s.portainer.GetStackFile(ctx, req.PortainerStackID)
		if err != nil {
			return err
		}
		stackYAML = yaml

		stack, err := s.portainer.GetStack(ctx, req.PortainerStackID)
		if err == nil && stack != nil {
			existingEnv = stack.Env
		}
		return nil
	}); err != nil {
		s.finish(jobID, "failed", err)
		return
	}

	s.setRunProgress(jobID, "running", 30)
	if err := s.runStep(ctx, jobID, "portainer_redeploy", func(ctx context.Context) error {
		// Portainer redeploy with PullImage=true handles pull and recreate
		// We pass the existingEnv to ensure manually configured Portainer variables are not lost.
		return s.portainer.UpdateStack(ctx, req.PortainerStackID, req.PortainerEndpointID, stackYAML, existingEnv, true, true)
	}); err != nil {
		s.finish(jobID, "failed", err)
		return
	}

	s.setRunProgress(jobID, "running", 80)
	if !req.SkipHealthCheck {
		if err := s.runStep(ctx, jobID, "validate", func(ctx context.Context) error {
			// Portainer stacks might take a while to come back up, so we wait and validate
			return s.executor.Validate(ctx, req)
		}); err != nil {
			s.finish(jobID, "failed", err) // Rollback for Portainer is manual or via manual stack revert for now
			return
		}
	} else {
		s.emit(jobID, gen.UpdateStepEvent{
			JobID:     jobID,
			Step:      "validate",
			Status:    "completed",
			Message:   "skipped: force update requested (no health check)",
			Timestamp: time.Now().UTC().Unix(),
		})
	}

	if req.AIValidateLogs && s.ai != nil && s.ai.HasProvider() {
		s.setRunProgress(jobID, "running", 90)
		if err := s.runStep(ctx, jobID, "ai_health_assessment", func(ctx context.Context) error {
			logs, _ := collectContainerLogsForAI(ctx, req.ContainerID, 300)
			assessment, err := s.ai.AnalyzeHealthLogs(ctx, req.ContainerID, logs)
			if err == nil && !assessment.Healthy {
				return fmt.Errorf("AI marked unhealthy: %s", assessment.Summary)
			}
			return nil
		}); err != nil {
			s.finish(jobID, "failed", err)
			return
		}
	}

	s.setRunProgress(jobID, "completed", 100)
	s.finish(jobID, "completed", nil)
}

func (s *Service) rollback(jobID string, req Request, cause error) {
	_ = s.runStep(context.Background(), jobID, "rollback", func(ctx context.Context) error { return s.executor.Rollback(ctx, req, cause) })
	s.finish(jobID, "rolled_back", cause)
}

func (s *Service) runStep(ctx context.Context, jobID, step string, fn func(context.Context) error) error {
	msg := "step: " + step
	start := gen.UpdateStepEvent{JobID: jobID, Step: step, Status: "running", Message: "step started", Timestamp: time.Now().UTC().Unix()}
	s.emit(jobID, start)
	s.jobManager.UpdateJob(jobID, -1, "running", msg)
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
				Title:  fmt.Sprintf("Update Job %s", strings.Title(status)),
				Body:   fmt.Sprintf("An update task has failed or was rolled back: %s", msg),
				Level:  notifications.LevelCritical,
				Source: "Update Engine",
				Fields: map[string]string{
					"Job ID": jobID,
					"Error":  msg,
				},
			})
		}
	}
	s.jobManager.UpdateJob(jobID, 100, status, msg)
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

func buildAIReleaseContext(req Request) string {
	var b strings.Builder
	b.WriteString("Container update context:\n")
	b.WriteString(fmt.Sprintf("- container_id: %s\n", strings.TrimSpace(req.ContainerID)))
	if name := strings.TrimSpace(req.ContainerName); name != "" {
		b.WriteString(fmt.Sprintf("- container_name: %s\n", name))
	}
	if cur := strings.TrimSpace(req.CurrentImage); cur != "" {
		b.WriteString(fmt.Sprintf("- current_image: %s\n", cur))
	}
	b.WriteString(fmt.Sprintf("- target_image: %s\n", strings.TrimSpace(req.TargetImage)))
	b.WriteString(fmt.Sprintf("- current_tag: %s\n", extractImageTag(req.CurrentImage)))
	b.WriteString(fmt.Sprintf("- target_tag: %s\n", extractImageTag(req.TargetImage)))
	if repo := strings.TrimSpace(req.RepositoryURL); repo != "" {
		b.WriteString(fmt.Sprintf("- source_repository: %s\n", repo))
	}
	if changelog := strings.TrimSpace(req.ChangelogURL); changelog != "" {
		b.WriteString(fmt.Sprintf("- changelog_url: %s\n", changelog))
	}
	if len(req.Labels) > 0 {
		for _, k := range []string{"org.opencontainers.image.source", "org.label-schema.vcs-url", "com.docker.compose.project", "com.docker.compose.service"} {
			if v := strings.TrimSpace(req.Labels[k]); v != "" {
				b.WriteString(fmt.Sprintf("- label_%s: %s\n", strings.ReplaceAll(k, ".", "_"), v))
			}
		}
	}
	if rc := strings.TrimSpace(req.ReleaseContext); rc != "" {
		b.WriteString("\nRepository intelligence:\n")
		b.WriteString(rc)
		b.WriteString("\n")
	}
	b.WriteString("\nEvaluate upgrade risk and breaking changes for this deployment context.")
	return b.String()
}

func extractImageTag(image string) string {
	raw := strings.TrimSpace(image)
	if raw == "" {
		return "unknown"
	}
	withoutDigest := strings.SplitN(raw, "@", 2)[0]
	lastSlash := strings.LastIndex(withoutDigest, "/")
	lastColon := strings.LastIndex(withoutDigest, ":")
	if lastColon > lastSlash {
		tag := strings.TrimSpace(withoutDigest[lastColon+1:])
		if tag != "" {
			return tag
		}
	}
	return "latest"
}

func collectContainerLogsForAI(ctx context.Context, containerID string, tail int) (string, error) {
	if strings.TrimSpace(containerID) == "" {
		return "", errors.New("containerId is required")
	}
	if tail <= 0 {
		tail = 300
	}
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", strconv.Itoa(tail), containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker logs failed: %w (%s)", err, truncate(string(out), 300))
	}
	return string(out), nil
}

func envInt(key string, fallback, minValue, maxValue int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if parsed < minValue {
		return minValue
	}
	if parsed > maxValue {
		return maxValue
	}
	return parsed
}

func shouldBlockForAI(analysis ai.AnalysisResult, riskThreshold int) (bool, string) {
	reasons := make([]string, 0, 3)
	if analysis.ActionRequired {
		reasons = append(reasons, "action_required=true")
	}
	if len(analysis.BreakingChanges) > 0 {
		max := len(analysis.BreakingChanges)
		if max > 2 {
			max = 2
		}
		reasons = append(reasons, "breaking changes: "+strings.Join(analysis.BreakingChanges[:max], "; "))
	}
	if analysis.RiskScore >= riskThreshold {
		reasons = append(reasons, fmt.Sprintf("risk_score=%d (threshold=%d)", analysis.RiskScore, riskThreshold))
	}
	if len(reasons) == 0 {
		return false, ""
	}
	return true, strings.Join(reasons, " | ")
}
