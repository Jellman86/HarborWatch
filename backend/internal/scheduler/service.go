package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Task defines the interface for a background job.
type Task interface {
	Name() string
	Run(ctx context.Context) error
}

// Service manages scheduled tasks using cron.
type Service struct {
	cron     *cron.Cron
	store    *Store
	logger   Logger
	mu       sync.RWMutex
	tasks    map[string]cron.EntryID
	registry map[string]func() Task
}

type Logger interface {
	Log(level, source, message string)
}

var cronSpecParser = cron.NewParser(
	cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

func NewService(store *Store) *Service {
	return &Service{
		cron:     cron.New(cron.WithSeconds()), // Support 6-field cron expressions
		store:    store,
		tasks:    make(map[string]cron.EntryID),
		registry: make(map[string]func() Task),
	}
}

func (s *Service) RegisterTask(name string, factory func() Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registry[name] = factory
}

func (s *Service) SetLogger(logger Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = logger
}

func (s *Service) log(level, message string) {
	log.Printf("%s", message)
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	if logger != nil {
		logger.Log(level, "Scheduler", message)
	}
}

func (s *Service) Start() {
	s.cron.Start()
	s.log("INFO", "Scheduler service started")
}

func (s *Service) Stop() {
	s.cron.Stop()
}

// AddTask registers a task into the system. If it's not in the DB, it's saved with the provided default enabled state.
func (s *Service) AddTask(spec string, task Task, enabled bool) error {
	taskName := task.Name()
	spec = normalizeCronSpec(spec)
	if err := validateCronSpec(spec); err != nil {
		return err
	}

	// 1. Ensure it's in the registry so it can be enabled later
	s.mu.Lock()
	if _, ok := s.registry[taskName]; !ok {
		// If no factory registered yet, use a simple one
		s.registry[taskName] = func() Task { return task }
	}
	s.mu.Unlock()

	// 2. Persist to database if not present
	if s.store != nil {
		ctx := context.Background()
		_ = s.store.SaveSchedule(ctx, ScheduleEntry{
			ID:       taskName,
			CronSpec: spec,
			Enabled:  enabled,
		})

		// Keep cron specs up to date for existing installations that persisted older 5-field specs.
		entry, err := s.store.GetSchedule(ctx, taskName)
		if err == nil {
			if entry.CronSpec != spec {
				log.Printf("Updating cron spec for %s: %q -> %q", taskName, entry.CronSpec, spec)
				_ = s.store.UpdateScheduleSpec(ctx, taskName, spec)
			}
		}

		// 3. Fixup: If enabled requested, but DB has it disabled and it never ran (likely due to previous bug), enable it.
		if enabled {
			entry, err := s.store.GetSchedule(ctx, taskName)
			if err == nil && !entry.Enabled && entry.LastRun == 0 {
				log.Printf("Fixing up disabled system task: %s", taskName)
				_ = s.store.ToggleSchedule(ctx, taskName, true)
			}
		}
	}

	return nil
}

func (s *Service) LoadSchedules(ctx context.Context) error {
	if s.store == nil {
		return nil
	}

	entries, err := s.store.ListSchedules(ctx)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.Enabled {
			continue
		}

		spec := normalizeCronSpec(entry.CronSpec)
		if spec != entry.CronSpec && s.store != nil {
			_ = s.store.UpdateScheduleSpec(ctx, entry.ID, spec)
		}
		if err := validateCronSpec(spec); err != nil {
			log.Printf("Skipping invalid cron spec for task %s: %v", entry.ID, err)
			continue
		}

		s.mu.RLock()
		factory, ok := s.registry[entry.ID]
		s.mu.RUnlock()

		if ok {
			task := factory()
			// We use a simplified internal add that doesn't re-save to DB to avoid loops
			id, err := s.scheduleTask(entry.ID, spec, task)
			if err == nil {
				s.mu.Lock()
				s.tasks[entry.ID] = id
				s.mu.Unlock()
			}
		}
	}

	return nil
}

func normalizeCronSpec(spec string) string {
	fields := strings.Fields(spec)
	switch len(fields) {
	case 5:
		// Existing installs may have persisted 5-field cron specs from pre-`cron.WithSeconds` versions.
		return "0 " + strings.Join(fields, " ")
	case 6:
		return strings.Join(fields, " ")
	default:
		return spec
	}
}

func validateCronSpec(spec string) error {
	normalized := normalizeCronSpec(spec)
	if strings.TrimSpace(normalized) == "" {
		return errors.New("cron spec is required")
	}
	if _, err := cronSpecParser.Parse(normalized); err != nil {
		return fmt.Errorf("invalid cron spec: %w", err)
	}
	return nil
}

func (s *Service) scheduleTask(name, spec string, task Task) (cron.EntryID, error) {
	return s.cron.AddFunc(spec, func() {
		s.log("INFO", fmt.Sprintf("Executing scheduled task: %s", name))
		if err := task.Run(context.Background()); err != nil {
			s.log("ERROR", fmt.Sprintf("Error executing task %s: %v", name, err))
		} else {
			s.log("INFO", fmt.Sprintf("Scheduled task completed: %s", name))
		}
		if s.store != nil {
			_ = s.store.UpdateLastRun(context.Background(), name, time.Now().Unix())
		}
	})
}

func (s *Service) RemoveTask(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id, ok := s.tasks[name]; ok {
		s.cron.Remove(id)
		delete(s.tasks, name)
	}

	if s.store != nil {
		_ = s.store.DeleteSchedule(context.Background(), name)
	}
}

func (s *Service) ListSchedules(ctx context.Context) ([]ScheduleEntry, error) {
	if s.store == nil {
		return []ScheduleEntry{}, nil
	}
	return s.store.ListSchedules(ctx)
}

func (s *Service) ToggleTask(ctx context.Context, name string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Update database
	if s.store != nil {
		if err := s.store.ToggleSchedule(ctx, name, enabled); err != nil {
			return err
		}
	}

	// 2. Update runtime cron state
	if !enabled {
		if id, ok := s.tasks[name]; ok {
			s.cron.Remove(id)
			delete(s.tasks, name)
		}
		return nil
	}

	// If enabling, we need to find the spec and factory
	if _, ok := s.tasks[name]; ok {
		return nil // Already running
	}
	if s.store == nil {
		return errors.New("scheduler store unavailable")
	}

	entries, err := s.store.ListSchedules(ctx)
	if err != nil {
		return err
	}
	var spec string
	for _, e := range entries {
		if e.ID == name {
			spec = normalizeCronSpec(e.CronSpec)
			if spec != e.CronSpec {
				_ = s.store.UpdateScheduleSpec(ctx, e.ID, spec)
			}
			break
		}
	}
	if spec == "" {
		return fmt.Errorf("task %s schedule not found", name)
	}
	if err := validateCronSpec(spec); err != nil {
		return err
	}

	factory, ok := s.registry[name]
	if !ok {
		return fmt.Errorf("task %s not found in registry", name)
	}
	task := factory()
	id, err := s.scheduleTask(name, spec, task)
	if err != nil {
		return err
	}
	s.tasks[name] = id

	return nil
}

func (s *Service) UpdateTaskSchedule(ctx context.Context, name string, spec string) error {
	if s.store == nil {
		return errors.New("scheduler store unavailable")
	}

	spec = normalizeCronSpec(spec)
	if err := validateCronSpec(spec); err != nil {
		return err
	}

	entry, err := s.store.GetSchedule(ctx, name)
	if err != nil {
		return err
	}
	if entry.CronSpec == spec {
		return nil
	}

	if err := s.store.UpdateScheduleSpec(ctx, name, spec); err != nil {
		return err
	}
	if !entry.Enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if id, ok := s.tasks[name]; ok {
		s.cron.Remove(id)
		delete(s.tasks, name)
	}

	factory, ok := s.registry[name]
	if !ok {
		return fmt.Errorf("task %s not found in registry", name)
	}

	task := factory()
	id, err := s.scheduleTask(name, spec, task)
	if err != nil {
		return err
	}
	s.tasks[name] = id

	return nil
}

func (s *Service) RunTask(ctx context.Context, name string) error {
	s.mu.RLock()
	factory, ok := s.registry[name]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("task %s not found in registry", name)
	}

	task := factory()
	s.log("INFO", fmt.Sprintf("Manually triggering task: %s", name))
	go func() {
		if err := task.Run(context.Background()); err != nil {
			s.log("ERROR", fmt.Sprintf("Manual task %s failed: %v", name, err))
		} else {
			s.log("INFO", fmt.Sprintf("Manual task completed: %s", name))
		}
		if s.store != nil {
			_ = s.store.UpdateLastRun(context.Background(), name, time.Now().Unix())
		}
	}()

	return nil
}
