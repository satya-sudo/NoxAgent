package browser

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrActionNotFound = errors.New("browser action not found")

type Action struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Query string `json:"query,omitempty"`
	Value int    `json:"value,omitempty"`
}

type Result struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type completion struct {
	result Result
	err    error
}

type pendingAction struct {
	ctx  context.Context
	done chan completion
}

type Broker struct {
	queue chan Action

	mu       sync.Mutex
	pending  map[string]pendingAction
	lastPoll time.Time
}

func NewBroker(capacity int) *Broker {
	if capacity < 1 {
		capacity = 16
	}
	return &Broker{
		queue:   make(chan Action, capacity),
		pending: make(map[string]pendingAction),
	}
}

func (b *Broker) Execute(ctx context.Context, action Action) (Result, error) {
	if action.ID == "" {
		id, err := newID()
		if err != nil {
			return Result{}, fmt.Errorf("create action id: %w", err)
		}
		action.ID = id
	}

	done := make(chan completion, 1)
	b.mu.Lock()
	b.pending[action.ID] = pendingAction{ctx: ctx, done: done}
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		delete(b.pending, action.ID)
		b.mu.Unlock()
	}()

	select {
	case b.queue <- action:
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}

	select {
	case completed := <-done:
		return completed.result, completed.err
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}

func (b *Broker) Next(ctx context.Context) (Action, error) {
	b.Touch()

	for {
		select {
		case action := <-b.queue:
			b.mu.Lock()
			pending, active := b.pending[action.ID]
			b.mu.Unlock()
			if !active || pending.ctx.Err() != nil {
				continue
			}
			return action, nil
		case <-ctx.Done():
			return Action{}, ctx.Err()
		}
	}
}

func (b *Broker) Touch() {
	b.mu.Lock()
	b.lastPoll = time.Now()
	b.mu.Unlock()
}

type Status struct {
	Connected bool       `json:"connected"`
	LastPoll  *time.Time `json:"last_poll,omitempty"`
}

func (b *Broker) Status(now time.Time) Status {
	b.mu.Lock()
	lastPoll := b.lastPoll
	b.mu.Unlock()
	if lastPoll.IsZero() {
		return Status{}
	}
	return Status{
		Connected: now.Sub(lastPoll) <= 35*time.Second,
		LastPoll:  &lastPoll,
	}
}

func (b *Broker) Complete(id string, result Result) error {
	b.mu.Lock()
	pending, ok := b.pending[id]
	b.mu.Unlock()
	if !ok {
		return ErrActionNotFound
	}

	completed := completion{result: result}
	if !result.OK {
		completed.err = errors.New(result.Message)
	}
	select {
	case pending.done <- completed:
		return nil
	default:
		return ErrActionNotFound
	}
}

func newID() (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
