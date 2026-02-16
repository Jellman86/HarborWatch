package scheduler

import (
	"context"
	"fmt"
	"log"
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
	mu       sync.RWMutex
	tasks    map[string]cron.EntryID
	registry map[string]func() Task
}

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

func (s *Service) Start() {
	s.cron.Start()
	log.Println("Scheduler service started")
}

func (s *Service) Stop() {
	s.cron.Stop()
}

// AddTask registers a task into the system. If it's not in the DB, it's saved as DISABLED.
func (s *Service) AddTask(spec string, task Task) error {
	taskName := task.Name()

	// 1. Ensure it's in the registry so it can be enabled later
	s.mu.Lock()
	if _, ok := s.registry[taskName]; !ok {
		// If no factory registered yet, use a simple one
		s.registry[taskName] = func() Task { return task }
	}
	s.mu.Unlock()

	// 2. Persist to database if not present (On by default for system tasks)
	if s.store != nil {
		_ = s.store.SaveSchedule(context.Background(), ScheduleEntry{
			ID:       taskName,
			CronSpec: spec,
			Enabled:  true,
		})
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

		s.mu.RLock()
		factory, ok := s.registry[entry.ID]
		s.mu.RUnlock()

		if ok {
			task := factory()
			// We use a simplified internal add that doesn't re-save to DB to avoid loops
			id, err := s.cron.AddFunc(entry.CronSpec, func() {
				log.Printf("Executing scheduled task: %s", entry.ID)
				if err := task.Run(context.Background()); err != nil {
					log.Printf("Error executing task %s: %v", entry.ID, err)
				}
				_ = s.store.UpdateLastRun(context.Background(), entry.ID, time.Now().Unix())
			})
			if err == nil {
				s.mu.Lock()
				s.tasks[entry.ID] = id
				s.mu.Unlock()
			}
		}
	}

	return nil
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

	entries, _ := s.store.ListSchedules(ctx)
	var spec string
	for _, e := range entries {
		if e.ID == name {
			spec = e.CronSpec
			break
		}
	}

	factory, ok := s.registry[name]
	if ok && spec != "" {
		task := factory()
		id, err := s.cron.AddFunc(spec, func() {
			log.Printf("Executing scheduled task: %s", name)
			if err := task.Run(context.Background()); err != nil {
				log.Printf("Error executing task %s: %v", name, err)
			}
			if s.store != nil {
				_ = s.store.UpdateLastRun(context.Background(), name, time.Now().Unix())
			}
		})
		if err != nil {
			return err
		}
		s.tasks[name] = id
	}

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
	log.Printf("Manually triggering task: %s", name)
	go func() {
		if err := task.Run(context.Background()); err != nil {
			log.Printf("Manual task %s failed: %v", name, err)
		}
		if s.store != nil {
			_ = s.store.UpdateLastRun(context.Background(), name, time.Now().Unix())
		}
	}()

	return nil
}
