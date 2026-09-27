package browser

import (
	"context"
	"testing"
	"time"
)

func TestBrokerRoundTrip(t *testing.T) {
	broker := NewBroker(1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	type response struct {
		result Result
		err    error
	}
	responses := make(chan response, 1)
	go func() {
		result, err := broker.Execute(ctx, Action{Type: "youtube_music.pause"})
		responses <- response{result: result, err: err}
	}()

	action, err := broker.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if action.ID == "" || action.Type != "youtube_music.pause" {
		t.Fatalf("unexpected action: %#v", action)
	}
	if err := broker.Complete(action.ID, Result{OK: true, Message: "Music paused."}); err != nil {
		t.Fatal(err)
	}

	completed := <-responses
	if completed.err != nil {
		t.Fatal(completed.err)
	}
	if completed.result.Message != "Music paused." {
		t.Fatalf("unexpected result: %#v", completed.result)
	}
}

func TestBrokerSkipsActionAfterCallerTimesOut(t *testing.T) {
	broker := NewBroker(2)
	commandCtx, cancelCommand := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelCommand()
	go func() {
		_, _ = broker.Execute(commandCtx, Action{Type: "browser.google_search"})
	}()
	<-commandCtx.Done()

	nextCtx, cancelNext := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancelNext()
	_, err := broker.Next(nextCtx)
	if err == nil {
		t.Fatal("Next returned a stale browser action")
	}
}

func TestBrokerStatusTracksExtensionPoll(t *testing.T) {
	broker := NewBroker(1)
	if broker.Status(time.Now()).Connected {
		t.Fatal("new broker unexpectedly reports a connection")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = broker.Next(ctx)
	if !broker.Status(time.Now()).Connected {
		t.Fatal("broker did not record extension poll")
	}
}
