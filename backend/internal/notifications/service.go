package notifications

import (
	"context"
	"fmt"
	"sync"
)

// Level represents the severity of the notification.
type Level string

const (
	LevelInfo     Level = "Info"
	LevelWarning  Level = "Warning"
	LevelCritical Level = "Critical"
)

// Message is the standard notification payload.
type Message struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Level  Level  `json:"level"`
	Source string `json:"source"`
}

// Dispatcher defines the interface for different notification platforms.
type Dispatcher interface {
	Name() string
	Send(ctx context.Context, msg Message) error
}

// Service manages multiple dispatchers and routes notifications.
type Service struct {
	mu          sync.RWMutex
	dispatchers []Dispatcher
}

func NewService() *Service {
	return &Service{
		dispatchers: make([]Dispatcher, 0),
	}
}

func (s *Service) AddDispatcher(d Dispatcher) {
	if d == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.dispatchers {
		if existing != nil && existing.Name() == d.Name() {
			s.dispatchers[i] = d
			return
		}
	}
	s.dispatchers = append(s.dispatchers, d)
}

func (s *Service) RemoveDispatcher(name string) {
	if name == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := make([]Dispatcher, 0, len(s.dispatchers))
	for _, d := range s.dispatchers {
		if d == nil || d.Name() == name {
			continue
		}
		filtered = append(filtered, d)
	}
	s.dispatchers = filtered
}

func (s *Service) Dispatch(ctx context.Context, msg Message) {
	s.mu.RLock()
	dispatchers := append([]Dispatcher(nil), s.dispatchers...)
	s.mu.RUnlock()
	for _, d := range dispatchers {
		if err := d.Send(ctx, msg); err != nil {
			fmt.Printf("Notification failed for %s: %v\n", d.Name(), err)
		}
	}
}
