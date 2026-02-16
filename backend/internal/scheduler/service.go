package scheduler

import (
	"context"
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

// AddTask schedules a new task and persists it to the database.
func (s *Service) AddTask(spec string, task Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Remove existing if present
	if id, ok := s.tasks[task.Name()]; ok {
		s.cron.Remove(id)
	}

	// 2. Schedule the function
	taskName := task.Name()
	id, err := s.cron.AddFunc(spec, func() {
		log.Printf("Executing scheduled task: %s", taskName)
		ctx := context.Background()
		if err := task.Run(ctx); err != nil {
			log.Printf("Error executing task %s: %v", taskName, err)
		}
		if s.store != nil {
			_ = s.store.UpdateLastRun(ctx, taskName, time.Now().Unix())
		}
	})
	if err != nil {
		return err
	}

	s.tasks[taskName] = id

	// 3. Persist to database
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
