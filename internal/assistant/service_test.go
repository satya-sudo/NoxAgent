package assistant

import (
	"context"
	"testing"
	"time"

	"nox/internal/llm"
	"nox/internal/scheduler"
)

type fakeScheduler struct {
	timerDuration time.Duration
	entries       []scheduler.Entry
}

func (f *fakeScheduler) AddTimer(duration time.Duration, _ string) (scheduler.Entry, error) {
	f.timerDuration = duration
	return scheduler.Entry{ID: "abcdef12", Kind: scheduler.KindTimer, DueAt: time.Now().Add(duration)}, nil
}

func (f *fakeScheduler) AddAlarm(dueAt time.Time, _ string) (scheduler.Entry, error) {
	return scheduler.Entry{ID: "abcdef12", Kind: scheduler.KindAlarm, DueAt: dueAt}, nil
}

func (f *fakeScheduler) List(kind scheduler.Kind) []scheduler.Entry {
	var result []scheduler.Entry
	for _, entry := range f.entries {
		if entry.Kind == kind {
			result = append(result, entry)
		}
	}
	return result
}

func (f *fakeScheduler) Cancel(_ scheduler.Kind, _ string) (int, error) {
	return 1, nil
}

type fakeInterpreter struct {
	decision llm.Decision
}

func (f fakeInterpreter) Interpret(context.Context, string) (llm.Decision, error) {
	return f.decision, nil
}

func TestHandleSetsTimerWithoutLLM(t *testing.T) {
	schedules := &fakeScheduler{}
	service := New(Dependencies{Scheduler: schedules})
	reply, err := service.Handle(context.Background(), "set a timer for 5 minutes")
	if err != nil {
		t.Fatal(err)
	}
	if schedules.timerDuration != 5*time.Minute || reply.Intent != "timer.set" {
		t.Fatalf("duration = %s, reply = %#v", schedules.timerDuration, reply)
	}
}

func TestHandleWakeGreeting(t *testing.T) {
	service := New(Dependencies{})
	reply, err := service.Handle(context.Background(), "Hey Nox")
	if err != nil {
		t.Fatal(err)
	}
	if reply.Intent != "wake.greet" || reply.Message != "Sir." {
		t.Fatalf("reply = %#v", reply)
	}
}

func TestHandleUsesLLMForConversation(t *testing.T) {
	service := New(Dependencies{
		Interpreter: fakeInterpreter{decision: llm.Decision{Action: "reply", Reply: "Hello from Nox."}},
	})
	reply, err := service.Handle(context.Background(), "tell me something interesting")
	if err != nil {
		t.Fatal(err)
	}
	if reply.Intent != "llm.reply" || reply.Message != "Hello from Nox." {
		t.Fatalf("reply = %#v", reply)
	}
}

func TestDecisionIntentRequiresSafeArguments(t *testing.T) {
	_, err := decisionIntent(llm.Decision{Action: "home.turn_on"})
	if err == nil {
		t.Fatal("decisionIntent accepted a missing entity query")
	}
}
