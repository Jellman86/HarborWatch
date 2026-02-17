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

	mu   sync.RWMutex
	jobs map[string]gen.ScanJobStatus

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
	s.mu.Unlock()

	_ = s.store.CreateJob(context.Background(), job, "vulnerability")

	go s.run(jobID, target)

	return gen.ScanStartResponse{JobID: jobID, Status: "running"}, nil
}

func (s *Service) StartMalwareScan(target string) (gen.ScanStartResponse, error) {
	return s.StartMalwareScanPath(target, target, false)
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
		Status:    "running",
		StartedAt: time.Now().UTC().Unix(),
		Source:    s.malwareScanner.Name(),
	}

	s.mu.Lock()
	s.jobs[jobID] = job
	s.mu.Unlock()

	_ = s.store.CreateJob(context.Background(), job, "malware")

	cleanupPath := ""
	if cleanup {
		cleanupPath = scanPath
	}
	go s.runMalware(jobID, targetLabel, scanPath, cleanupPath)

	return gen.ScanStartResponse{JobID: jobID, Status: "running"}, nil
}

func (s *Service) run(jobID, target string) {
	s.trivySem <- struct{}{}
	defer func() { <-s.trivySem }()

	ctx, cancel := context.WithTimeout(context.Background(), envDuration("HW_TRIVY_SCAN_TIMEOUT", 15*time.Minute))
	defer cancel()

	result, err := s.scanner.Scan(ctx, target)
	if err != nil {
		s.setJobFailed(jobID, err)
		return
	}

	if err := s.store.SaveResult(ctx, result); err != nil {
		s.setJobFailed(jobID, err)
		return
	}

	now := time.Now().UTC().Unix()
	s.mu.Lock()
	job := s.jobs[jobID]
	job.Status = "completed"
	job.CompletedAt = now
	s.jobs[jobID] = job
	s.mu.Unlock()

	_ = s.store.UpdateJob(context.Background(), jobID, "completed", "", now)
}

func (s *Service) runMalware(jobID, targetLabel, scanPath, cleanupPath string) {
	if cleanupPath != "" {
		defer func() { _ = os.RemoveAll(cleanupPath) }()
	}

	s.clamavSem <- struct{}{}
	defer func() { <-s.clamavSem }()

	ctx, cancel := context.WithTimeout(context.Background(), envDuration("HW_CLAMAV_SCAN_TIMEOUT", 15*time.Minute))
	defer cancel()

	result, err := s.malwareScanner.ScanPath(ctx, scanPath)
	if err != nil {
		s.setJobFailed(jobID, err)
		return
	}
	result.Target = targetLabel

	if err := s.store.SaveMalwareResult(ctx, result); err != nil {
		s.setJobFailed(jobID, err)
		return
	}

	now := time.Now().UTC().Unix()
	s.mu.Lock()
	job := s.jobs[jobID]
	job.Status = "completed"
	job.CompletedAt = now
	s.jobs[jobID] = job
	s.mu.Unlock()

	_ = s.store.UpdateJob(context.Background(), jobID, "completed", "", now)
}

func (s *Service) setJobFailed(jobID string, err error) {
	now := time.Now().UTC().Unix()
	s.mu.Lock()
	job := s.jobs[jobID]
	job.Status = "failed"
	job.Error = err.Error()
	job.CompletedAt = now
	s.jobs[jobID] = job
	s.mu.Unlock()

	if s.diag != nil {
		s.diag.Log("ERROR", "Scanner", fmt.Sprintf("Job %s failed: %v", jobID, err))
	}

	_ = s.store.UpdateJob(context.Background(), jobID, "failed", err.Error(), now)
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
