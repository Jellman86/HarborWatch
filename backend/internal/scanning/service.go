package scanning

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/jobs"
)

type DiagService interface {
	Log(level, source, message string)
}

type DockerClient interface {
	ListImages(ctx context.Context) ([]gen.ImageSummary, error)
}

type Service struct {
	scanner        Scanner
	malwareScanner MalwareScanner
	docker         DockerClient
	store          *Store
	diag           DiagService
	jobManager     *jobs.Manager

	mu         sync.RWMutex
	jobs       map[string]gen.ScanJobStatus
	jobCancels map[string]context.CancelFunc
}

var ErrDuplicateActiveScan = errors.New("a matching scan is already queued or running")

func NewService(scanner Scanner, malwareScanner MalwareScanner, docker DockerClient, store *Store, diag DiagService, jm *jobs.Manager) *Service {
	return &Service{
		scanner:        scanner,
		malwareScanner: malwareScanner,
		docker:         docker,
		store:          store,
		diag:           diag,
		jobManager:     jm,
		jobs:           map[string]gen.ScanJobStatus{},
		jobCancels:     map[string]context.CancelFunc{},
	}
}

func (s *Service) StartScan(target string) (gen.ScanStartResponse, error) {
	if target == "" {
		return gen.ScanStartResponse{}, errors.New("target is required")
	}

	jobID, err := newJobID()
	if err != nil {
		return gen.ScanStartResponse{}, err
	}

	job := gen.ScanJobStatus{
		JobID:     jobID,
		Target:    target,
		Status:    "queued",
		StartedAt: time.Now().UTC().Unix(),
		Source:    s.scanner.Name(),
	}

	s.mu.Lock()
	s.jobs[jobID] = job
	runCtx, runCancel := context.WithCancel(context.Background())
	s.jobCancels[jobID] = runCancel
	s.mu.Unlock()

	jobTracker := &jobs.Job{
		ID:         jobID,
		Type:       jobs.JobTypeScan,
		Subtype:    "trivy",
		Target:     target,
		TargetName: target,
		Status:     "queued",
		Message:    "Waiting for concurrency slot",
		StartedAt:  time.Now().UTC().Unix(),
		Cancel:     runCancel,
	}
	if existingID, duplicate := s.jobManager.RegisterJobIfNoDuplicate(jobTracker); duplicate {
		s.mu.Lock()
		delete(s.jobs, jobID)
		delete(s.jobCancels, jobID)
		s.mu.Unlock()
		runCancel()
		return gen.ScanStartResponse{}, fmt.Errorf("%w for target %s (job=%s)", ErrDuplicateActiveScan, target, existingID)
	}

	_ = s.store.CreateJob(context.Background(), job, "vulnerability")
	if s.diag != nil {
		s.diag.Log("INFO", "Scanner", fmt.Sprintf("trivy scan queued job=%s target=%s", jobID, target))
	}

	go s.run(jobID, target, runCtx)

	return gen.ScanStartResponse{JobID: jobID, Status: "queued"}, nil
}

func (s *Service) StartMalwareScan(target string) (gen.ScanStartResponse, error) {
	return s.StartMalwareScanPath(target, target, false)
}

func (s *Service) ClamAVSignatureStatus(ctx context.Context) (ClamAVSignatureStatus, error) {
	statusCtx, cancel := context.WithTimeout(ctx, envDuration("HW_CLAMAV_STATUS_TIMEOUT", 10*time.Second))
	defer cancel()
	return ReadClamAVSignatureStatus(statusCtx)
}

func (s *Service) UpdateClamAVSignatures(ctx context.Context) (string, error) {
	jobID := "clamav-sigs-" + strconv.FormatInt(time.Now().Unix(), 10)
	lockID := "clamav-signatures"
	if err := s.jobManager.AcquireSlot(ctx, jobID, lockID); err != nil {
		return "", err
	}
	defer s.jobManager.ReleaseSlot(jobID, lockID)

	updateCtx, cancel := context.WithTimeout(ctx, envDuration("HW_CLAMAV_UPDATE_TIMEOUT", 10*time.Minute))
	defer cancel()
	if s.diag != nil {
		s.diag.Log("INFO", "Scanner", "Starting ClamAV signature update")
	}
	summary, err := UpdateClamAVSignatures(updateCtx)
	if err != nil {
		if s.diag != nil {
			s.diag.Log("ERROR", "Scanner", fmt.Sprintf("ClamAV signature update failed: %v", err))
		}
		return summary, err
	}
	if s.diag != nil {
		s.diag.Log("INFO", "Scanner", fmt.Sprintf("ClamAV signature update completed: %s", summary))
	}
	return summary, nil
}

func (s *Service) StartMalwareScanPath(targetLabel, scanPath string, cleanup bool) (gen.ScanStartResponse, error) {
	targetLabel = strings.TrimSpace(targetLabel)
	scanPath = strings.TrimSpace(scanPath)
	if targetLabel == "" {
		return gen.ScanStartResponse{}, errors.New("target is required")
	}
	if scanPath == "" {
		return gen.ScanStartResponse{}, errors.New("scan path is required")
	}

	jobID, err := newJobID()
	if err != nil {
		return gen.ScanStartResponse{}, err
	}

	job := gen.ScanJobStatus{
		JobID:     jobID,
		Target:    targetLabel,
		Status:    "queued",
		StartedAt: time.Now().UTC().Unix(),
		Source:    s.malwareScanner.Name(),
	}

	s.mu.Lock()
	s.jobs[jobID] = job
	runCtx, runCancel := context.WithCancel(context.Background())
	s.jobCancels[jobID] = runCancel
	s.mu.Unlock()

	jobTracker := &jobs.Job{
		ID:         jobID,
		Type:       jobs.JobTypeScan,
		Subtype:    "clamav",
		Target:     targetLabel,
		TargetName: targetLabel,
		Status:     "queued",
		Message:    "Waiting for concurrency slot",
		StartedAt:  time.Now().UTC().Unix(),
		Cancel:     runCancel,
	}
	if existingID, duplicate := s.jobManager.RegisterJobIfNoDuplicate(jobTracker); duplicate {
		s.mu.Lock()
		delete(s.jobs, jobID)
		delete(s.jobCancels, jobID)
		s.mu.Unlock()
		runCancel()
		return gen.ScanStartResponse{}, fmt.Errorf("%w for target %s (job=%s)", ErrDuplicateActiveScan, targetLabel, existingID)
	}

	_ = s.store.CreateJob(context.Background(), job, "malware")
	if s.diag != nil {
		s.diag.Log("INFO", "Scanner", fmt.Sprintf("clamav scan queued job=%s target=%s path=%s", jobID, targetLabel, scanPath))
	}

	cleanupPath := ""
	if cleanup {
		cleanupPath = scanPath
	}
	go s.runMalware(jobID, targetLabel, scanPath, cleanupPath, runCtx)

	return gen.ScanStartResponse{JobID: jobID, Status: "queued"}, nil
}

func (s *Service) run(jobID, target string, runCtx context.Context) {
	s.setJobProgressWithMessage(jobID, 0, "Waiting for concurrency slot")
	if err := s.jobManager.AcquireSlot(runCtx, jobID, "image:"+target); err != nil {
		s.setJobCancelled(jobID, "scan cancelled: "+err.Error())
		return
	}
	defer s.jobManager.ReleaseSlot(jobID, "image:"+target)
	// FinishJob is called at the very end of the function to ensure all final logs/persistence are captured in UI.
	defer s.jobManager.FinishJob(jobID)

	ctx, cancel := context.WithTimeout(runCtx, envDuration("HW_TRIVY_SCAN_TIMEOUT", 15*time.Minute))
	defer cancel()
	if err := ctx.Err(); err != nil {
		s.setJobCancelled(jobID, "scan cancelled before execution")
		return
	}
	s.setJobProgressWithMessage(jobID, 5, "Initializing scan engine")
	if s.diag != nil {
		s.diag.Log("INFO", "Scanner", fmt.Sprintf("trivy scan started job=%s target=%s", jobID, target))
	}

	s.setJobProgressWithMessage(jobID, 20, "Scanning vulnerabilities")

	// Estimated progress advances within scan phase while the scanner runs.
	stopTrickle := make(chan struct{})
	go func() {
		s.runBoundedEstimatedProgress(jobID, 20, 75, envDuration("HW_TRIVY_SCAN_TIMEOUT", 15*time.Minute), 5*time.Second, stopTrickle)
	}()

	result, err := s.scanner.Scan(ctx, target)
	close(stopTrickle) // Stop the trickler

	if err != nil {
		if errors.Is(err, context.Canceled) && runCtx.Err() == context.Canceled {
			s.setJobCancelled(jobID, "scan cancelled by user")
			return
		}
		s.setJobFailed(jobID, err)
		return
	}
	if runCtx.Err() == context.Canceled {
		s.setJobCancelled(jobID, "scan cancelled by user")
		return
	}

	s.setJobProgressWithMessage(jobID, 82, "Persisting results")
	if err := s.store.SaveResult(ctx, result); err != nil {
		s.setJobFailed(jobID, err)
		return
	}

	if !s.setJobCompleted(jobID) {
		return
	}
	if s.diag != nil {
		total := result.Critical + result.High + result.Medium + result.Low + result.Unknown
		s.diag.Log("INFO", "Scanner", fmt.Sprintf("trivy scan completed job=%s target=%s total=%d critical=%d high=%d medium=%d low=%d unknown=%d", jobID, result.Target, total, result.Critical, result.High, result.Medium, result.Low, result.Unknown))
	}
}

func (s *Service) runMalware(jobID, targetLabel, scanPath, cleanupPath string, runCtx context.Context) {
	if cleanupPath != "" {
		defer func() { _ = os.RemoveAll(cleanupPath) }()
	}

	containerID := ""
	if strings.HasPrefix(targetLabel, "container:") {
		containerID = strings.TrimPrefix(targetLabel, "container:")
	}

	s.setJobProgressWithMessage(jobID, 0, "Waiting for concurrency slot")
	lockID := "malware:" + targetLabel
	if containerID != "" {
		lockID = containerID
	}

	if err := s.jobManager.AcquireSlot(runCtx, jobID, lockID); err != nil {
		s.setJobCancelled(jobID, "scan cancelled: "+err.Error())
		return
	}
	defer s.jobManager.ReleaseSlot(jobID, lockID)
	defer s.jobManager.FinishJob(jobID)

	ctx, cancel := context.WithTimeout(runCtx, envDuration("HW_CLAMAV_SCAN_TIMEOUT", 15*time.Minute))
	defer cancel()
	if err := ctx.Err(); err != nil {
		s.setJobCancelled(jobID, "scan cancelled before execution")
		return
	}
	s.setJobProgressWithMessage(jobID, 5, "Initializing scan engine")
	if s.diag != nil {
		s.diag.Log("INFO", "Scanner", fmt.Sprintf("clamav scan started job=%s target=%s path=%s", jobID, targetLabel, scanPath))
	}

	s.setJobProgressWithMessage(jobID, 25, "Scanning filesystem")

	// Estimated progress advances within scan phase while the scanner runs.
	stopTrickle := make(chan struct{})
	go func() {
		s.runBoundedEstimatedProgress(jobID, 25, 85, envDuration("HW_CLAMAV_SCAN_TIMEOUT", 15*time.Minute), 5*time.Second, stopTrickle)
	}()

	result, err := s.malwareScanner.ScanPath(ctx, scanPath)
	close(stopTrickle) // Stop the trickler

	if err != nil {
		if errors.Is(err, context.Canceled) && runCtx.Err() == context.Canceled {
			s.setJobCancelled(jobID, "scan cancelled by user")
			return
		}
		s.setJobFailed(jobID, err)
		return
	}
	if runCtx.Err() == context.Canceled {
		s.setJobCancelled(jobID, "scan cancelled by user")
		return
	}
	result.Target = targetLabel

	s.setJobProgressWithMessage(jobID, 88, "Persisting results")
	if err := s.store.SaveMalwareResult(ctx, result); err != nil {
		s.setJobFailed(jobID, err)
		return
	}

	if !s.setJobCompleted(jobID) {
		return
	}
	if s.diag != nil {
		s.diag.Log("INFO", "Scanner", fmt.Sprintf("clamav scan completed job=%s target=%s infected=%t threats=%d", jobID, targetLabel, result.Infected, len(result.FoundThreats)))
	}
}

func (s *Service) setJobProgress(jobID string, progress int) {
	s.setJobProgressWithMessageAndMode(jobID, progress, "", "measured")
}

func (s *Service) setJobProgressEstimated(jobID string, progress int) {
	s.setJobProgressWithMessageAndMode(jobID, progress, "", "estimated")
}

func (s *Service) setJobMessage(jobID string, message string) {
	s.mu.Lock()
	job, ok := s.jobs[jobID]
	if !ok || (job.Status != "running" && job.Status != "queued") {
		s.mu.Unlock()
		return
	}
	// We don't store message in the transient 'jobs' map if it's not in gen.ScanJobStatus,
	// but we added it to gen.JobProgress.
	// Wait, I should check if gen.ScanJobStatus has Message.
	// API spec didn't have Message in ScanJobStatus, only JobProgress.
	// Let's add it to ScanJobStatus too for consistency.
	s.mu.Unlock()
}

func (s *Service) setJobProgressWithMessage(jobID string, progress int, message string) {
	s.setJobProgressWithMessageAndMode(jobID, progress, message, "measured")
}

func (s *Service) setJobProgressWithMessageAndMode(jobID string, progress int, message, progressMode string) {
	s.mu.Lock()
	job, ok := s.jobs[jobID]
	if !ok || (job.Status != "running" && job.Status != "queued") {
		s.mu.Unlock()
		return
	}
	job.Progress = progress
	if job.Status == "queued" && (progress > 0 || message != "") {
		job.Status = "running"
	}
	s.jobs[jobID] = job
	s.mu.Unlock()
	s.jobManager.UpdateJobWithMode(jobID, progress, job.Status, message, progressMode)
	_ = s.store.UpdateJobProgress(context.Background(), jobID, job.Status, progress)
}

func (s *Service) runBoundedEstimatedProgress(jobID string, start, cap int, budget, interval time.Duration, stop <-chan struct{}) {
	if cap <= start {
		return
	}
	if budget <= 0 {
		budget = 15 * time.Minute
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	startTime := time.Now()
	last := start
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			elapsed := time.Since(startTime)
			frac := float64(elapsed) / float64(budget)
			if frac < 0 {
				frac = 0
			}
			if frac > 1 {
				frac = 1
			}
			next := start + int(frac*float64(cap-start))
			if next > cap {
				next = cap
			}
			if next > last {
				last = next
				s.setJobProgressEstimated(jobID, next)
			}
		}
	}
}

func (s *Service) setJobFailed(jobID string, err error) {
	if !s.finishActiveJob(jobID, "failed", err.Error()) {
		return
	}

	if s.diag != nil {
		s.diag.Log("ERROR", "Scanner", fmt.Sprintf("Job %s failed: %v", jobID, err))
	}
}

func (s *Service) setJobCancelled(jobID, reason string) {
	if !s.finishActiveJob(jobID, "cancelled", reason) {
		return
	}
	if s.diag != nil {
		s.diag.Log("WARN", "Scanner", fmt.Sprintf("Job %s cancelled: %s", jobID, reason))
	}
}

func (s *Service) setJobCompleted(jobID string) bool {
	return s.finishActiveJob(jobID, "completed", "")
}

func (s *Service) finishActiveJob(jobID, status, errMsg string) bool {
	now := time.Now().UTC().Unix()
	s.mu.Lock()
	job, ok := s.jobs[jobID]
	if !ok {
		s.mu.Unlock()
		return false
	}
	if job.Status != "running" && job.Status != "queued" {
		s.mu.Unlock()
		return false
	}
	job.Status = status
	job.Error = strings.TrimSpace(errMsg)
	job.CompletedAt = now
	if status == "completed" {
		job.Progress = 100
	}
	s.jobs[jobID] = job
	delete(s.jobCancels, jobID)
	s.mu.Unlock()

	_ = s.store.UpdateJob(context.Background(), jobID, status, strings.TrimSpace(errMsg), now)
	return true
}

func (s *Service) Job(ctx context.Context, jobID string) (gen.ScanJobStatus, error) {
	s.mu.RLock()
	job, ok := s.jobs[jobID]
	s.mu.RUnlock()
	if ok {
		return job, nil
	}

	dbJob, err := s.store.GetJob(ctx, jobID)
	if err != nil {
		return gen.ScanJobStatus{}, err
	}
	if dbJob == nil {
		return gen.ScanJobStatus{}, errors.New("job not found")
	}
	return *dbJob, nil
}

func (s *Service) ActiveJobs() []gen.JobProgress {
	return s.jobManager.ActiveJobs()
}

func (s *Service) ListJobs(ctx context.Context, scanType, targetPrefix string, limit int) ([]gen.ScanJobStatus, error) {
	return s.store.ListJobs(ctx, scanType, targetPrefix, limit)
}

func (s *Service) CancelJob(ctx context.Context, jobID string) (gen.ScanJobStatus, error) {
	s.mu.Lock()
	job, ok := s.jobs[jobID]
	if !ok {
		s.mu.Unlock()
		dbJob, err := s.store.GetJob(ctx, jobID)
		if err != nil {
			return gen.ScanJobStatus{}, err
		}
		if dbJob == nil {
			return gen.ScanJobStatus{}, errors.New("job not found")
		}
		if dbJob.Status == "running" {
			return *dbJob, errors.New("job is not cancellable (scan service restart)")
		}
		return *dbJob, nil
	}

	if job.Status != "running" && job.Status != "queued" {
		s.mu.Unlock()
		return job, nil
	}

	cancel := s.jobCancels[jobID]
	delete(s.jobCancels, jobID)
	now := time.Now().UTC().Unix()
	job.Status = "cancelled"
	job.Error = "cancelled by user"
	job.CompletedAt = now
	s.jobs[jobID] = job
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if s.diag != nil {
		s.diag.Log("WARN", "Scanner", fmt.Sprintf("Job %s cancel requested by user", jobID))
	}
	_ = s.store.UpdateJob(context.Background(), jobID, "cancelled", "cancelled by user", now)
	return job, nil
}

func (s *Service) LatestSummary(ctx context.Context) (*gen.ScanSummary, error) {
	return s.store.LatestSummary(ctx)
}

func (s *Service) LatestSummaryForTarget(ctx context.Context, target string) (*gen.ScanSummary, error) {
	return s.store.LatestSummaryForTarget(ctx, target)
}

func (s *Service) LatestDetailsForTarget(ctx context.Context, target string) (*gen.TrivyScanDetails, error) {
	return s.store.LatestDetailsForTarget(ctx, target)
}

func (s *Service) MalwareSummaries(ctx context.Context, target string) ([]gen.MalwareScanSummary, error) {
	return s.store.MalwareSummaries(ctx, target)
}

func (s *Service) MalwareSummariesForContainer(ctx context.Context, containerID string) ([]gen.MalwareScanSummary, error) {
	return s.store.MalwareSummariesByPrefix(ctx, "container:"+containerID)
}

func (s *Service) MalwareDetails(ctx context.Context, target, prefix string, limit int) ([]gen.MalwareScanDetail, error) {
	return s.store.MalwareDetails(ctx, target, prefix, limit)
}

func (s *Service) MalwareDetailsForContainer(ctx context.Context, containerID string, limit int) ([]gen.MalwareScanDetail, error) {
	return s.store.MalwareDetails(ctx, "", "container:"+containerID, limit)
}

func (s *Service) ListImages(ctx context.Context) ([]gen.ImageSummary, error) {
	if s.docker == nil {
		return nil, errors.New("docker client not available in scanning service")
	}
	return s.docker.ListImages(ctx)
}

func newJobID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("create job id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func envInt(key string, fallback, minValue, maxValue int) int {
	raw := os.Getenv(key)
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

func acquireScanSlot(ctx context.Context, sem chan struct{}) bool {
	select {
	case sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}
