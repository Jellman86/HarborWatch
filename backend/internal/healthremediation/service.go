package healthremediation

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/diag"
	"github.com/Jellman86/HarborWatch/backend/internal/dockerengine"
	"github.com/Jellman86/HarborWatch/backend/internal/jobs"
	"github.com/Jellman86/HarborWatch/backend/internal/rules"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
)

type Service struct {
	db            *sql.DB
	dockerClient  *dockerengine.Client
	rulesStore    *rules.Store
	settingsStore *settings.Store
	diagService   *diag.Service
	jobManager    *jobs.Manager

	mu             sync.Mutex
	restartHistory map[string]*restartStats // containerID -> stats
}

type restartStats struct {
	Attempts      int
	LastAttemptAt time.Time
	WindowStart   time.Time
}

func NewService(db *sql.DB, docker *dockerengine.Client, rs *rules.Store, ss *settings.Store, ds *diag.Service, jm *jobs.Manager) *Service {
	return &Service{
		db:             db,
		dockerClient:   docker,
		rulesStore:     rs,
		settingsStore:  ss,
		diagService:    ds,
		jobManager:     jm,
		restartHistory: make(map[string]*restartStats),
	}
}

func (s *Service) Init(ctx context.Context) error {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name='remediation_runs' LIMIT 1`).Scan(&exists)
	if err == sql.ErrNoRows {
		return fmt.Errorf("remediation_runs table missing; run schema migrations before health remediation init")
	}
	if err != nil {
		return fmt.Errorf("verify remediation_runs table: %w", err)
	}
	return nil
}

func (s *Service) Start(ctx context.Context) {
	s.diagService.Log("INFO", "HealthRemediation", "Unhealthy Auto-Remediation service starting")
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := s.listenEvents(ctx); err != nil {
				if ctx.Err() == nil {
					s.diagService.Log("ERROR", "HealthRemediation", fmt.Sprintf("Event listener failed, retrying in 10s: %v", err))
					time.Sleep(10 * time.Second)
				}
			}
		}
	}
}

func (s *Service) listenEvents(ctx context.Context) error {
	stream, err := s.dockerClient.OpenEventStream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		raw := scanner.Bytes()
		var rawEvent struct {
			Type   string `json:"Type"`
			Action string `json:"Action"`
			Status string `json:"status"`
			ID     string `json:"id"`
			Actor  struct {
				ID         string            `json:"ID"`
				Attributes map[string]string `json:"Attributes"`
			} `json:"Actor"`
		}

		if err := json.Unmarshal(raw, &rawEvent); err != nil {
			continue
		}

		// Re-check settings periodically
		st, _ := s.settingsStore.Get(ctx)
		if !st.UnhealthyAutoRemediationEnabled {
			continue
		}

		id := rawEvent.ID
		if id == "" {
			id = rawEvent.Actor.ID
		}
		action := rawEvent.Action
		if action == "" {
			action = rawEvent.Status
		}

		if rawEvent.Type == "container" && strings.HasPrefix(action, "health_status:") {
			if strings.Contains(action, "unhealthy") {
				containerName := rawEvent.Actor.Attributes["name"]
				if containerName == "" {
					containerName = id[:12]
				}
				go s.handleUnhealthy(ctx, id, containerName)
			}
		}
	}

	return scanner.Err()
}

func (s *Service) handleUnhealthy(ctx context.Context, containerID, containerName string) {
	// Re-check global setting inside goroutine
	st, _ := s.settingsStore.Get(ctx)
	if !st.UnhealthyAutoRemediationEnabled {
		return
	}

	rules, err := s.rulesStore.Get(ctx, containerID, containerName)
	if err != nil {
		return
	}

	// Check if container is opted in
	if !rules.RestartOnUnhealthy {
		return
	}

	s.mu.Lock()
	stats, ok := s.restartHistory[containerID]
	if !ok {
		stats = &restartStats{WindowStart: time.Now()}
		s.restartHistory[containerID] = stats
	}
	s.mu.Unlock()

	// 1. Cooldown Check
	cooldown := time.Duration(rules.UnhealthyRestartCooldownSec) * time.Second
	if cooldown == 0 {
		cooldown = time.Duration(st.UnhealthyRestartCooldownSecDefault) * time.Second
	}
	if time.Since(stats.LastAttemptAt) < cooldown {
		s.diagService.Log("INFO", "HealthRemediation", fmt.Sprintf("Suppressed restart for %s: cooldown active (%s remaining)", containerName, (cooldown-time.Since(stats.LastAttemptAt)).Round(time.Second)))
		return
	}

	// 2. Max Restarts Check (Window: 1 hour)
	if time.Since(stats.WindowStart) > 1*time.Hour {
		stats.Attempts = 0
		stats.WindowStart = time.Now()
	}

	if stats.Attempts >= st.MaxRestartsPerWindow {
		s.diagService.Log("WARN", "HealthRemediation", fmt.Sprintf("Suppressed restart for %s: max attempts (%d) reached for this hour window", containerName, st.MaxRestartsPerWindow))
		return
	}

	// 3. Busy Check (JobManager)
	jobID := fmt.Sprintf("remediation-%s-%d", containerID, time.Now().Unix())
	if err := s.jobManager.AcquireSlot(ctx, jobID, containerID); err != nil {
		// Suppress logs for "busy" if it's already a remediation job?
		// Actually AcquireSlot returns error if busy.
		return
	}
	defer s.jobManager.ReleaseSlot(jobID, containerID)

	// 4. Register Job for visibility
	job := &jobs.Job{
		ID:         jobID,
		Type:       jobs.JobTypeRemediation,
		Subtype:    "unhealthy-restart",
		Target:     containerID,
		TargetName: containerName,
		Status:     "running",
		Message:    "Restarting unhealthy container",
		StartedAt:  time.Now().Unix(),
	}
	s.jobManager.RegisterJob(job)
	defer s.jobManager.FinishJob(jobID)

	// 5. Persist run to DB
	_, _ = s.db.ExecContext(ctx, `
INSERT INTO remediation_runs (id, container_id, container_name, type, reason, status, started_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, jobID, containerID, containerName, "Restart", "docker_health_status_unhealthy", "running", job.StartedAt)

	// 6. Perform Restart
	s.diagService.Log("INFO", "HealthRemediation", fmt.Sprintf("Auto-restarting unhealthy container: %s", containerName))

	stats.Attempts++
	stats.LastAttemptAt = time.Now()

	status := "completed"
	errMsg := ""
	if err := s.dockerClient.RestartContainer(ctx, containerID); err != nil {
		s.diagService.Log("ERROR", "HealthRemediation", fmt.Sprintf("Failed to restart %s: %v", containerName, err))
		s.jobManager.UpdateJob(jobID, 0, "failed", err.Error())
		status = "failed"
		errMsg = err.Error()
	} else {
		s.diagService.Log("INFO", "HealthRemediation", fmt.Sprintf("Successfully restarted %s (Attempt %d/%d)", containerName, stats.Attempts, st.MaxRestartsPerWindow))
		s.jobManager.UpdateJob(jobID, 100, "completed", "Restart successful")
	}

	_, _ = s.db.ExecContext(ctx, `
UPDATE remediation_runs SET status = ?, error = ?, completed_at = ? WHERE id = ?
`, status, errMsg, time.Now().Unix(), jobID)
}

func (s *Service) PruneRuns(ctx context.Context, olderThan int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM remediation_runs WHERE started_at < ?", olderThan)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
