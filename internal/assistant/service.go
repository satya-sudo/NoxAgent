package assistant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"nox/internal/browser"
	"nox/internal/homeassistant"
	"nox/internal/intent"
	"nox/internal/llm"
	"nox/internal/scheduler"
)

type Browser interface {
	Execute(context.Context, browser.Action) (browser.Result, error)
}

type Scheduler interface {
	AddTimer(time.Duration, string) (scheduler.Entry, error)
	AddAlarm(time.Time, string) (scheduler.Entry, error)
	List(scheduler.Kind) []scheduler.Entry
	Cancel(scheduler.Kind, string) (int, error)
}

type Home interface {
	TurnOn(context.Context, string) (homeassistant.Entity, error)
	TurnOff(context.Context, string) (homeassistant.Entity, error)
	ActivateScene(context.Context, string) (homeassistant.Entity, error)
	GetState(context.Context, string) (homeassistant.Entity, error)
}

type Interpreter interface {
	Interpret(context.Context, string) (llm.Decision, error)
}

type interpreterStatus interface {
	Health(context.Context) error
	Model() string
}

type conversationObserver interface {
	Observe(input, reply string)
}

type Dependencies struct {
	Browser     Browser
	Scheduler   Scheduler
	Home        Home
	Interpreter Interpreter
}

type Service struct {
	browser  Browser
	schedule Scheduler
	home     Home
	llm      Interpreter
	now      func() time.Time
}

type Reply struct {
	Intent  string `json:"intent"`
	Message string `json:"message"`
}

func New(dependencies Dependencies) *Service {
	return &Service{
		browser:  dependencies.Browser,
		schedule: dependencies.Scheduler,
		home:     dependencies.Home,
		llm:      dependencies.Interpreter,
		now:      time.Now,
	}
}

type LLMStatus struct {
	Configured bool   `json:"configured"`
	Available  bool   `json:"available"`
	Model      string `json:"model,omitempty"`
}

func (s *Service) LLMStatus(ctx context.Context) LLMStatus {
	if s.llm == nil {
		return LLMStatus{}
	}
	status := LLMStatus{Configured: true}
	checker, ok := s.llm.(interpreterStatus)
	if !ok {
		status.Available = true
		return status
	}
	status.Model = checker.Model()
	status.Available = checker.Health(ctx) == nil
	return status
}

func (s *Service) Handle(ctx context.Context, input string) (Reply, error) {
	reply, err := s.handle(ctx, input)
	if err == nil {
		if observer, ok := s.llm.(conversationObserver); ok {
			observer.Observe(input, reply.Message)
		}
	}
	return reply, err
}

func (s *Service) handle(ctx context.Context, input string) (Reply, error) {
	parsed, err := intent.Parse(input)
	if err != nil {
		if s.llm == nil || !errors.Is(err, intent.ErrUnknown) {
			return Reply{}, err
		}
		decision, interpretErr := s.llm.Interpret(ctx, input)
		if interpretErr != nil {
			return Reply{}, interpretErr
		}
		if decision.Action == "reply" {
			return Reply{Intent: "llm.reply", Message: decision.Reply}, nil
		}
		parsed, err = decisionIntent(decision)
		if err != nil {
			return Reply{}, err
		}
	}

	if parsed.Name == "clock.time" {
		return Reply{
			Intent:  parsed.Name,
			Message: fmt.Sprintf("It is %s.", s.now().Format("3:04 PM")),
		}, nil
	}
	if parsed.Name == "wake.greet" {
		return Reply{Intent: parsed.Name, Message: "Sir."}, nil
	}
	if strings.HasPrefix(parsed.Name, "timer.") || strings.HasPrefix(parsed.Name, "alarm.") {
		return s.handleSchedule(parsed)
	}
	if strings.HasPrefix(parsed.Name, "home.") {
		return s.handleHome(ctx, parsed)
	}

	action := browser.Action{Type: parsed.Name, Query: parsed.Query, Value: parsed.Value}
	result, err := s.browser.Execute(ctx, action)
	if err != nil {
		return Reply{}, fmt.Errorf("%s: %w", parsed.Name, err)
	}
	message := result.Message
	if message == "" {
		message = successMessage(parsed)
	}
	return Reply{Intent: parsed.Name, Message: message}, nil
}

func decisionIntent(decision llm.Decision) (intent.Intent, error) {
	parsed := intent.Intent{
		Name:      decision.Action,
		Query:     strings.TrimSpace(decision.Query),
		Value:     decision.Value,
		Duration:  time.Duration(decision.DurationSeconds) * time.Second,
		Hour:      decision.Hour,
		Minute:    decision.Minute,
		DayOffset: decision.DayOffset,
	}
	needsQuery := map[string]bool{
		"browser.google_search": true,
		"youtube_music.play":    true,
		"timer.cancel":          true,
		"alarm.cancel":          true,
		"home.turn_on":          true,
		"home.turn_off":         true,
		"home.activate_scene":   true,
		"home.state":            true,
	}
	if needsQuery[parsed.Name] && parsed.Query == "" {
		return intent.Intent{}, fmt.Errorf("local LLM omitted a required query for %s", parsed.Name)
	}
	if parsed.Name == "timer.set" && parsed.Duration <= 0 {
		return intent.Intent{}, fmt.Errorf("local LLM returned an invalid timer duration")
	}
	return parsed, nil
}

func (s *Service) handleHome(ctx context.Context, parsed intent.Intent) (Reply, error) {
	if s.home == nil {
		return Reply{}, fmt.Errorf("Home Assistant is not configured")
	}
	var (
		entity homeassistant.Entity
		err    error
	)
	switch parsed.Name {
	case "home.turn_on":
		entity, err = s.home.TurnOn(ctx, parsed.Query)
	case "home.turn_off":
		entity, err = s.home.TurnOff(ctx, parsed.Query)
	case "home.activate_scene":
		entity, err = s.home.ActivateScene(ctx, parsed.Query)
	case "home.state":
		entity, err = s.home.GetState(ctx, parsed.Query)
	default:
		return Reply{}, intent.ErrUnknown
	}
	if err != nil {
		return Reply{}, err
	}

	var message string
	switch parsed.Name {
	case "home.turn_on":
		message = fmt.Sprintf("Turned on %s.", entity.Name())
	case "home.turn_off":
		message = fmt.Sprintf("Turned off %s.", entity.Name())
	case "home.activate_scene":
		message = fmt.Sprintf("Activated the %s scene.", entity.Name())
	case "home.state":
		message = formatEntityState(entity)
	}
	return Reply{Intent: parsed.Name, Message: message}, nil
}

func formatEntityState(entity homeassistant.Entity) string {
	state := entity.State
	if unit, ok := entity.Attributes["unit_of_measurement"].(string); ok && unit != "" {
		state += " " + unit
	}
	return fmt.Sprintf("%s is %s.", entity.Name(), state)
}

func (s *Service) handleSchedule(parsed intent.Intent) (Reply, error) {
	if s.schedule == nil {
		return Reply{}, fmt.Errorf("scheduler is unavailable")
	}
	switch parsed.Name {
	case "timer.set":
		entry, err := s.schedule.AddTimer(parsed.Duration, "")
		if err != nil {
			return Reply{}, err
		}
		return Reply{Intent: parsed.Name, Message: fmt.Sprintf("Timer %s set for %s.", shortID(entry.ID), formatDuration(parsed.Duration))}, nil
	case "alarm.set":
		now := s.now()
		dueAt := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour, parsed.Minute, 0, 0, now.Location())
		if parsed.DayOffset > 0 {
			dueAt = dueAt.AddDate(0, 0, parsed.DayOffset)
		} else if !dueAt.After(now) {
			dueAt = dueAt.AddDate(0, 0, 1)
		}
		entry, err := s.schedule.AddAlarm(dueAt, "")
		if err != nil {
			return Reply{}, err
		}
		return Reply{Intent: parsed.Name, Message: fmt.Sprintf("Alarm %s set for %s.", shortID(entry.ID), dueAt.Format("Monday at 3:04 PM"))}, nil
	case "timer.list":
		return Reply{Intent: parsed.Name, Message: formatEntries("timer", s.schedule.List(scheduler.KindTimer))}, nil
	case "alarm.list":
		return Reply{Intent: parsed.Name, Message: formatEntries("alarm", s.schedule.List(scheduler.KindAlarm))}, nil
	case "timer.cancel":
		return s.cancelSchedule(parsed, scheduler.KindTimer)
	case "alarm.cancel":
		return s.cancelSchedule(parsed, scheduler.KindAlarm)
	default:
		return Reply{}, intent.ErrUnknown
	}
}

func (s *Service) cancelSchedule(parsed intent.Intent, kind scheduler.Kind) (Reply, error) {
	prefix := parsed.Query
	if prefix == "all" {
		prefix = ""
	}
	count, err := s.schedule.Cancel(kind, prefix)
	if err != nil {
		return Reply{}, err
	}
	return Reply{Intent: parsed.Name, Message: fmt.Sprintf("Cancelled %d %s(s).", count, kind)}, nil
}

func formatEntries(kind string, entries []scheduler.Entry) string {
	if len(entries) == 0 {
		return fmt.Sprintf("There are no active %ss.", kind)
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts = append(parts, fmt.Sprintf("%s at %s", shortID(entry.ID), entry.DueAt.Format("Monday 3:04 PM")))
	}
	return fmt.Sprintf("Active %ss: %s.", kind, strings.Join(parts, "; "))
}

func shortID(id string) string {
	if len(id) > 6 {
		return id[:6]
	}
	return id
}

func formatDuration(duration time.Duration) string {
	if duration%time.Hour == 0 {
		return fmt.Sprintf("%d hour(s)", int(duration/time.Hour))
	}
	if duration%time.Minute == 0 {
		return fmt.Sprintf("%d minute(s)", int(duration/time.Minute))
	}
	return duration.Round(time.Second).String()
}

func successMessage(parsed intent.Intent) string {
	switch parsed.Name {
	case "browser.google_search":
		return fmt.Sprintf("Searching Google for %s.", parsed.Query)
	case "youtube_music.play":
		return fmt.Sprintf("Playing %s on YouTube Music.", parsed.Query)
	case "youtube_music.pause":
		return "Music paused."
	case "youtube_music.resume":
		return "Resuming music."
	case "youtube_music.next":
		return "Skipping to the next song."
	case "youtube_music.previous":
		return "Going back to the previous song."
	case "youtube_music.volume":
		return fmt.Sprintf("Volume set to %d percent.", parsed.Value)
	default:
		return "Done."
	}
}
