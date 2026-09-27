package scheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound  = errors.New("scheduled item not found")
	ErrAmbiguous = errors.New("scheduled item id is ambiguous")
)

type Kind string

const (
	KindAlarm Kind = "alarm"
	KindTimer Kind = "timer"
)

type State string

const (
	StateScheduled State = "scheduled"
	StateFired     State = "fired"
	StateCancelled State = "cancelled"
)

type Entry struct {
	ID        string    `json:"id"`
	Kind      Kind      `json:"kind"`
	Label     string    `json:"label,omitempty"`
	DueAt     time.Time `json:"due_at"`
	CreatedAt time.Time `json:"created_at"`
	State     State     `json:"state"`
}

type stateFile struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

type Store struct {
	path    string
	mu      sync.Mutex
	entries []Entry
}

func OpenStore(path string) (*Store, error) {
	store := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read scheduler state: %w", err)
	}
	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode scheduler state: %w", err)
	}
	if state.Version != 1 {
		return nil, fmt.Errorf("unsupported scheduler state version %d", state.Version)
	}
	store.entries = state.Entries
	return store, nil
}

func (s *Store) Add(entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	return s.saveLocked()
}

func (s *Store) List(kind Kind) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.State == StateScheduled && (kind == "" || entry.Kind == kind) {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].DueAt.Before(entries[j].DueAt) })
	return entries
}

func (s *Store) NextDue() (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var next time.Time
	for _, entry := range s.entries {
		if entry.State != StateScheduled {
			continue
		}
		if next.IsZero() || entry.DueAt.Before(next) {
			next = entry.DueAt
		}
	}
	return next, !next.IsZero()
}

func (s *Store) TakeDue(now time.Time) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var due []Entry
	for index := range s.entries {
		entry := &s.entries[index]
		if entry.State == StateScheduled && !entry.DueAt.After(now) {
			entry.State = StateFired
			due = append(due, *entry)
		}
	}
	if len(due) == 0 {
		return nil, nil
	}
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return due, nil
}

func (s *Store) Cancel(kind Kind, idPrefix string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var matches []int
	for index, entry := range s.entries {
		if entry.State != StateScheduled || (kind != "" && entry.Kind != kind) {
			continue
		}
		if idPrefix == "" || strings.HasPrefix(entry.ID, idPrefix) {
			matches = append(matches, index)
		}
	}
	if len(matches) == 0 {
		return 0, ErrNotFound
	}
	if idPrefix != "" && len(matches) > 1 {
		return 0, ErrAmbiguous
	}
	for _, index := range matches {
		s.entries[index].State = StateCancelled
	}
	if err := s.saveLocked(); err != nil {
		return 0, err
	}
	return len(matches), nil
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create scheduler state directory: %w", err)
	}
	data, err := json.MarshalIndent(stateFile{Version: 1, Entries: s.entries}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode scheduler state: %w", err)
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write scheduler state: %w", err)
	}
	if err := os.Rename(temporary, s.path); err != nil {
		return fmt.Errorf("replace scheduler state: %w", err)
	}
	return nil
}
