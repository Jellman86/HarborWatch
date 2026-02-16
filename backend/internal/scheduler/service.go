package scheduler

import (
	"context"
	"log"
	"sync"

	"github.com/robfig/cron/v3"
)

// Task defines the interface for a background job.
type Task interface {
	Name() string
	Run(ctx context.Context) error
}

// Service manages scheduled tasks using cron.
type Service struct {
	cron  *cron.Cron
	mu    sync.RWMutex
	tasks map[string]cron.EntryID
}

func NewService() *Service {
	return &Service{
		cron:  cron.New(cron.WithSeconds()), // Support 6-field cron expressions
		tasks: make(map[string]cron.EntryID),
	}
}

func (s *Service) Start() {
	s.cron.Start()
	log.Println("Scheduler service started")
}

func (s *Service) Stop() {
	s.cron.Stop()
}

// AddTask schedules a new task. If a task with the same name exists, it is replaced.
func (s *Service) AddTask(spec string, task Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing if present
	if id, ok := s.tasks[task.Name()]; ok {
		s.cron.Remove(id)
	}

	id, err := s.cron.AddFunc(spec, func() {
		log.Printf("Executing scheduled task: %s", task.Name())
		if err := task.Run(context.Background()); err != nil {
			log.Printf("Error executing task %s: %v", task.Name(), err)
		}
	})
	if err != nil {
		return err
	}

	s.tasks[task.Name()] = id
	return nil
}

func (s *Service) RemoveTask(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id, ok := s.tasks[name]; ok {
		s.cron.Remove(id)
		delete(s.tasks, name)
	}
}
