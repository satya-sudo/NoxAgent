package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"
)

type Notifier interface {
	Notify(context.Context, Entry) error
}

type Service struct {
	store    *Store
	notifier Notifier
	logger   *slog.Logger
	wake     chan struct{}
	now      func() time.Time
}

func NewService(store *Store, notifier Notifier, logger *slog.Logger) *Service {
	return &Service{
		store:    store,
		notifier: notifier,
		logger:   logger,
		wake:     make(chan struct{}, 1),
		now:      time.Now,
	}
}

func (s *Service) AddTimer(duration time.Duration, label string) (Entry, error) {
	if duration <= 0 {
		return Entry{}, fmt.Errorf("timer duration must be positive")
	}
	return s.add(KindTimer, s.now().Add(duration), label)
}

func (s *Service) AddAlarm(dueAt time.Time, label string) (Entry, error) {
	if !dueAt.After(s.now()) {
		return Entry{}, fmt.Errorf("alarm time must be in the future")
	}
	return s.add(KindAlarm, dueAt, label)
}

func (s *Service) add(kind Kind, dueAt time.Time, label string) (Entry, error) {
	id, err := newID()
	if err != nil {
		return Entry{}, err
	}
	entry := Entry{
		ID:        id,
		Kind:      kind,
		Label:     label,
		DueAt:     dueAt,
		CreatedAt: s.now(),
		State:     StateScheduled,
	}
	if err := s.store.Add(entry); err != nil {
		return Entry{}, err
	}
	s.signalWake()
	return entry, nil
}

func (s *Service) List(kind Kind) []Entry {
	return s.store.List(kind)
}

func (s *Service) Cancel(kind Kind, idPrefix string) (int, error) {
	count, err := s.store.Cancel(kind, idPrefix)
	if err == nil {
		s.signalWake()
	}
	return count, err
}

func (s *Service) Run(ctx context.Context) {
	for {
		next, exists := s.store.NextDue()
		if !exists {
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
				continue
			}
		}

		wait := time.Until(next)
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.wake:
			timer.Stop()
			continue
		case <-timer.C:
			s.fireDue(ctx)
		}
	}
}

func (s *Service) fireDue(ctx context.Context) {
	entries, err := s.store.TakeDue(s.now())
	if err != nil {
		s.logger.Error("failed to update due schedules", "error", err)
		return
	}
	for _, entry := range entries {
		s.logger.Info("schedule fired", "kind", entry.Kind, "id", entry.ID, "label", entry.Label)
		if err := s.notifier.Notify(ctx, entry); err != nil {
			s.logger.Error("notification failed", "kind", entry.Kind, "id", entry.ID, "error", err)
		}
	}
}

func (s *Service) signalWake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func newID() (string, error) {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("create schedule id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}
