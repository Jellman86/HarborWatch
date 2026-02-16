package notifications

import (
	"context"
	"fmt"
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
	Title   string `json:"title"`
	Body    string `json:"body"`
	Level   Level  `json:"level"`
	Source  string `json:"source"`
}

// Dispatcher defines the interface for different notification platforms.
type Dispatcher interface {
	Name() string
	Send(ctx context.Context, msg Message) error
}

// Service manages multiple dispatchers and routes notifications.
type Service struct {
	dispatchers []Dispatcher
}

func NewService() *Service {
	return &Service{
		dispatchers: make([]Dispatcher, 0),
	}
}

func (s *Service) AddDispatcher(d Dispatcher) {
	s.dispatchers = append(s.dispatchers, d)
}

func (s *Service) Dispatch(ctx context.Context, msg Message) {
	for _, d := range s.dispatchers {
		if err := d.Send(ctx, msg); err != nil {
			fmt.Printf("Notification failed for %s: %v\n", d.Name(), err)
		}
	}
}
