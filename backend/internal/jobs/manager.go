package jobs

import (
	"context"
	"fmt"
	"sync"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

// JobType defines the category of work being performed.
type JobType string

const (
	JobTypeUpdate   JobType = "update"
	JobTypeScan     JobType = "scan"
	JobTypeRedeploy JobType = "redeploy"
)

// Job represents a queued or running task.
type Job struct {
	ID           string
	Type         JobType
	Subtype      string // e.g. "trivy", "clamav"
	Target       string // Usually ContainerID or Image name
	TargetName   string // Friendly name
	Progress     int
	ProgressMode string // measured | estimated
	Status       string
	Message      string
	StartedAt    int64
	Cancel       context.CancelFunc
}

// Manager coordinates heavy operations with global concurrency limits and per-container locking.
type Manager struct {
	mu             sync.RWMutex
	jobs           map[string]*Job
	containerLocks map[string]string // containerID -> jobID

	maxConcurrency int
	inUse          int
	slotChanged    chan struct{}
}

func NewManager(maxConcurrency int) *Manager {
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}
	return &Manager{
		jobs:           make(map[string]*Job),
		containerLocks: make(map[string]string),
		maxConcurrency: maxConcurrency,
		slotChanged:    make(chan struct{}, 1),
	}
}

func (m *Manager) SetMaxConcurrency(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if n <= 0 {
		n = 1
	}
	if n == m.maxConcurrency {
		return
	}

	// Adjusting a semaphore at runtime is tricky in Go.
	m.maxConcurrency = n
	m.notifySlotChangeLocked()
}

func (m *Manager) AcquireSlot(ctx context.Context, jobID, containerID string) error {
	for {
		m.mu.Lock()
		if owner, exists := m.containerLocks[containerID]; exists && owner != jobID {
			m.mu.Unlock()
			return fmt.Errorf("container %s is already busy with another job", containerID)
		}
		m.containerLocks[containerID] = jobID
		if m.inUse < m.maxConcurrency {
			m.inUse++
			m.mu.Unlock()
			return nil
		}
		ch := m.slotChanged
		m.mu.Unlock()

		select {
		case <-ch:
		case <-ctx.Done():
			m.mu.Lock()
			if m.containerLocks[containerID] == jobID {
				delete(m.containerLocks, containerID)
			}
			m.mu.Unlock()
			return ctx.Err()
		}
	}
}

func (m *Manager) ReleaseSlot(jobID, containerID string) {
	m.mu.Lock()
	if m.containerLocks[containerID] == jobID {
		delete(m.containerLocks, containerID)
	}
	if m.inUse > 0 {
		m.inUse--
	}
	m.notifySlotChangeLocked()
	m.mu.Unlock()
}

func (m *Manager) RegisterJob(job *Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job.ProgressMode == "" {
		job.ProgressMode = "measured"
	}
	m.jobs[job.ID] = job
}

func (m *Manager) UpdateJob(id string, progress int, status, message string) {
	m.UpdateJobWithMode(id, progress, status, message, "")
}

func (m *Manager) UpdateJobWithMode(id string, progress int, status, message, progressMode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[id]; ok {
		j.Progress = progress
		if status != "" {
			j.Status = status
		}
		if message != "" {
			j.Message = message
		}
		if progressMode != "" {
			j.ProgressMode = progressMode
		}
		if j.ProgressMode == "" {
			j.ProgressMode = "measured"
		}
	}
}

func (m *Manager) FinishJob(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, id)
}

func (m *Manager) CancelAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, j := range m.jobs {
		if j.Cancel != nil {
			j.Cancel()
		}
		delete(m.jobs, id)
	}
	// Also clear container locks
	m.containerLocks = make(map[string]string)
	m.inUse = 0
	m.notifySlotChangeLocked()
}

func (m *Manager) ActiveJobs() []gen.JobProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]gen.JobProgress, 0, len(m.jobs))
	for _, j := range m.jobs {
		typeName := string(j.Type)
		if j.Subtype != "" {
			typeName += ":" + j.Subtype
		}
		out = append(out, gen.JobProgress{
			ID:           j.ID,
			Type:         typeName,
			Target:       j.TargetName,
			Status:       j.Status,
			Message:      j.Message,
			Progress:     j.Progress,
			ProgressMode: j.ProgressMode,
			StartedAt:    j.StartedAt,
		})
	}
	return out
}

func (m *Manager) notifySlotChangeLocked() {
	select {
	case m.slotChanged <- struct{}{}:
	default:
	}
}
