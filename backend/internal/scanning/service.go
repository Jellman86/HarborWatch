package scanning

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

type Service struct {
	scanner        Scanner
	malwareScanner MalwareScanner
	store          *Store

	mu   sync.RWMutex
	jobs map[string]gen.ScanJobStatus
}

func NewService(scanner Scanner, malwareScanner MalwareScanner, store *Store) *Service {
	return &Service{
		scanner:        scanner,
		malwareScanner: malwareScanner,
		store:          store,
		jobs:           map[string]gen.ScanJobStatus{},
	}
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

	return NewService(NewTrivyScanner(), NewClamAVScanner(), store), nil
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
		Source:    s.malwareScanner.Name(),
	}

	s.mu.Lock()
	s.jobs[jobID] = job
	s.mu.Unlock()

	_ = s.store.CreateJob(context.Background(), job, "malware")

	go s.runMalware(jobID, target)

	return gen.ScanStartResponse{JobID: jobID, Status: "running"}, nil
}

func (s *Service) run(jobID, target string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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

func (s *Service) runMalware(jobID, target string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	result, err := s.malwareScanner.ScanPath(ctx, target)
	if err != nil {
		s.setJobFailed(jobID, err)
		return
	}

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

func (s *Service) MalwareSummaries(ctx context.Context, target string) ([]gen.MalwareScanSummary, error) {
	return s.store.MalwareSummaries(ctx, target)
}

func newJobID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("create job id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
