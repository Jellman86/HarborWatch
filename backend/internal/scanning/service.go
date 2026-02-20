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
)

type DiagService interface {
	Log(level, source, message string)
}

type Service struct {
	scanner        Scanner
	malwareScanner MalwareScanner
	store          *Store
	diag           DiagService

	mu         sync.RWMutex
	jobs       map[string]gen.ScanJobStatus
	jobCancels map[string]context.CancelFunc

	// Concurrency guards to avoid spawning too many heavy scanners at once.
	trivySem  chan struct{}
	clamavSem chan struct{}
}

func NewService(scanner Scanner, malwareScanner MalwareScanner, store *Store, diag DiagService) *Service {
	trivyConcurrency := envInt("HW_TRIVY_MAX_CONCURRENCY", 2, 1, 16)
	clamavConcurrency := envInt("HW_CLAMAV_MAX_CONCURRENCY", 1, 1, 8)
	return &Service{
		scanner:        scanner,
		malwareScanner: malwareScanner,
		store:          store,
		diag:           diag,
		jobs:           map[string]gen.ScanJobStatus{},
		jobCancels:     map[string]context.CancelFunc{},
		trivySem:       make(chan struct{}, trivyConcurrency),
		clamavSem:      make(chan struct{}, clamavConcurrency),
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
		Status:    "running",
		StartedAt: time.Now().UTC().Unix(),
		Source:    s.scanner.Name(),
	}

	s.mu.Lock()
	s.jobs[jobID] = job
	runCtx, runCancel := context.WithCancel(context.Background())
	s.jobCancels[jobID] = runCancel
	s.mu.Unlock()

	_ = s.store.CreateJob(context.Background(), job, "vulnerability")
	if s.diag != nil {
		s.diag.Log("INFO", "Scanner", fmt.Sprintf("trivy scan queued job=%s target=%s", jobID, target))
	}

	go s.run(jobID, target, runCtx)

	return gen.ScanStartResponse{JobID: jobID, Status: "running"}, nil
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
	s.clamavSem <- struct{}{}
	defer func() { <-s.clamavSem }()

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

	_ = s.store.CreateJob(context.Background(), job, "malware")
	if s.diag != nil {
		s.diag.Log("INFO", "Scanner", fmt.Sprintf("clamav scan queued job=%s target=%s path=%s", jobID, targetLabel, scanPath))
	}

	cleanupPath := ""
	if cleanup {
		cleanupPath = scanPath
	}
	go s.runMalware(jobID, targetLabel, scanPath, cleanupPath, runCtx)

	return gen.ScanStartResponse{JobID: jobID, Status: "running"}, nil
}

func (s *Service) run(jobID, target string, runCtx context.Context) {
	if !acquireScanSlot(runCtx, s.trivySem) {
		s.setJobCancelled(jobID, "scan cancelled before execution")
		return
	}
	defer func() { <-s.trivySem }()

	ctx, cancel := context.WithTimeout(runCtx, envDuration("HW_TRIVY_SCAN_TIMEOUT", 15*time.Minute))
	defer cancel()
	if err := ctx.Err(); err != nil {
		s.setJobCancelled(jobID, "scan cancelled before execution")
		return
	}
	if s.diag != nil {
		s.diag.Log("INFO", "Scanner", fmt.Sprintf("trivy scan started job=%s target=%s", jobID, target))
	}

	s.setJobProgressWithMessage(jobID, 10, "Initializing scan engine")
	result, err := s.scanner.Scan(ctx, target)
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

	s.setJobProgressWithMessage(jobID, 80, "Persisting results")
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

	if !acquireScanSlot(runCtx, s.clamavSem) {
		s.setJobCancelled(jobID, "scan cancelled before execution")
		return
	}
	defer func() { <-s.clamavSem }()

	ctx, cancel := context.WithTimeout(runCtx, envDuration("HW_CLAMAV_SCAN_TIMEOUT", 15*time.Minute))
	defer cancel()
	if err := ctx.Err(); err != nil {
		s.setJobCancelled(jobID, "scan cancelled before execution")
		return
	}
	s.setJobProgress(jobID, 5)
	if s.diag != nil {
		s.diag.Log("INFO", "Scanner", fmt.Sprintf("clamav scan started job=%s target=%s path=%s", jobID, targetLabel, scanPath))
	}

	s.setJobProgressWithMessage(jobID, 20, "Scanning filesystem")
	result, err := s.malwareScanner.ScanPath(ctx, scanPath)
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

	s.setJobProgressWithMessage(jobID, 85, "Persisting results")
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
	s.setJobProgressWithMessage(jobID, progress, "")
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
	_ = s.store.UpdateJobProgress(context.Background(), jobID, job.Status, progress)
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]gen.JobProgress, 0, len(s.jobs))
	for _, job := range s.jobs {
		if job.Status == "running" || job.Status == "queued" {
			msg := "Processing..."
			if job.Status == "queued" {
				msg = "Waiting for slot..."
			} else if job.Progress >= 80 {
				msg = "Finalizing results..."
			} else if job.Source == "trivy" {
				msg = "Scanning vulnerabilities..."
			} else if job.Source == "clamav" {
				msg = "Scanning malware..."
			}

			out = append(out, gen.JobProgress{
				ID:        job.JobID,
				Type:      "scan:" + job.Source,
				Target:    job.Target,
				Status:    job.Status,
				Message:   msg,
				Progress:  job.Progress,
				StartedAt: job.StartedAt,
			})
		}
	}
	return out
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
