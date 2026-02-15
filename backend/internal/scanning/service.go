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
	scanner Scanner
	store   *Store

	mu   sync.RWMutex
	jobs map[string]gen.ScanJobStatus
}

func NewService(scanner Scanner, store *Store) *Service {
	return &Service{
		scanner: scanner,
		store:   store,
		jobs:    map[string]gen.ScanJobStatus{},
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

	return NewService(NewTrivyScanner(), store), nil
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

	go s.run(jobID, target)

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

	s.mu.Lock()
	job := s.jobs[jobID]
	job.Status = "completed"
	job.CompletedAt = time.Now().UTC().Unix()
	s.jobs[jobID] = job
	s.mu.Unlock()
}

func (s *Service) setJobFailed(jobID string, err error) {
	s.mu.Lock()
	job := s.jobs[jobID]
	job.Status = "failed"
	job.Error = err.Error()
	job.CompletedAt = time.Now().UTC().Unix()
	s.jobs[jobID] = job
	s.mu.Unlock()
}

func (s *Service) Job(jobID string) (gen.ScanJobStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobID]
	return job, ok
}

func (s *Service) LatestSummary(ctx context.Context) (*gen.ScanSummary, error) {
	return s.store.LatestSummary(ctx)
}

func newJobID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("create job id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
