package scheduler

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

type recordingNotifier struct {
	entries chan Entry
}

func (n *recordingNotifier) Notify(_ context.Context, entry Entry) error {
	n.entries <- entry
	return nil
}

func TestTimerPersistsAndFires(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	notifier := &recordingNotifier{entries: make(chan Entry, 1)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewService(store, notifier, logger)

	entry, err := service.AddTimer(30*time.Millisecond, "test timer")
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	entries := reopened.List(KindTimer)
	if len(entries) != 1 || entries[0].ID != entry.ID {
		t.Fatalf("persisted entries = %#v", entries)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go service.Run(ctx)
	select {
	case fired := <-notifier.entries:
		if fired.ID != entry.ID {
			t.Fatalf("fired id = %s, want %s", fired.ID, entry.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timer did not fire")
	}
	if active := store.List(KindTimer); len(active) != 0 {
		t.Fatalf("active timers after firing = %#v", active)
	}
}

func TestCancelByShortID(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewService(store, &recordingNotifier{entries: make(chan Entry, 1)}, logger)
	entry, err := service.AddTimer(time.Hour, "")
	if err != nil {
		t.Fatal(err)
	}
	count, err := service.Cancel(KindTimer, entry.ID[:6])
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(service.List(KindTimer)) != 0 {
		t.Fatalf("cancelled = %d, active = %#v", count, service.List(KindTimer))
	}
}
