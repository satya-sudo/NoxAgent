package intent

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrUnknown = errors.New("I don't know how to do that yet")

type Intent struct {
	Name      string
	Query     string
	Value     int
	Duration  time.Duration
	Hour      int
	Minute    int
	DayOffset int
}

var volumePattern = regexp.MustCompile(`(?i)(?:set\s+)?(?:the\s+)?volume(?:\s+to)?\s+(\d{1,3})(?:\s*%)?`)
var timerPattern = regexp.MustCompile(`(?i)^(?:set|start)\s+(?:a\s+)?timer\s+for\s+(.+)$`)
var alarmPattern = regexp.MustCompile(`(?i)^set\s+(?:an\s+)?alarm\s+(?:for|at)\s+(.+)$`)

func Parse(input string) (Intent, error) {
	text := strings.TrimSpace(input)
	text = stripSpeechPreamble(text)
	lower := strings.ToLower(strings.Trim(text, " .?!"))
	if lower == "" {
		return Intent{}, ErrUnknown
	}
	if remainder, ok := stripWakePrefix(text); ok {
		if strings.TrimSpace(remainder) == "" {
			return Intent{Name: "wake.greet"}, nil
		}
		text = strings.TrimSpace(remainder)
		lower = strings.ToLower(strings.Trim(text, " .?!"))
	}

	if matches := timerPattern.FindStringSubmatch(lower); len(matches) == 2 {
		duration, ok := parseDuration(matches[1])
		if ok {
			return Intent{Name: "timer.set", Duration: duration}, nil
		}
		return Intent{}, ErrUnknown
	}
	if matches := alarmPattern.FindStringSubmatch(lower); len(matches) == 2 {
		hour, minute, dayOffset, ok := parseAlarmClock(matches[1])
		if ok {
			return Intent{Name: "alarm.set", Hour: hour, Minute: minute, DayOffset: dayOffset}, nil
		}
		return Intent{}, ErrUnknown
	}

	switch {
	case isWakeGreeting(lower):
		return Intent{Name: "wake.greet"}, nil
	case lower == "time" || strings.Contains(lower, "what time") || strings.Contains(lower, "tell me the time"):
		return Intent{Name: "clock.time"}, nil
	case lower == "check my email" || lower == "check email" || lower == "check gmail" ||
		lower == "do i have new email" || lower == "do i have new emails" ||
		lower == "any unread email" || lower == "any unread emails" ||
		lower == "read my unread email" || lower == "read my unread emails":
		return Intent{Name: "gmail.unread"}, nil
	case strings.HasPrefix(lower, "search gmail for "):
		return withQuery("gmail.search", text[len("search gmail for "):])
	case strings.HasPrefix(lower, "search my email for "):
		return withQuery("gmail.search", text[len("search my email for "):])
	case strings.HasPrefix(lower, "search google for "):
		return withQuery("browser.google_search", text[len("search google for "):])
	case strings.HasPrefix(lower, "search goofle for "):
		return withQuery("browser.google_search", text[len("search goofle for "):])
	case strings.HasPrefix(lower, "search gogle for "):
		return withQuery("browser.google_search", text[len("search gogle for "):])
	case strings.HasPrefix(lower, "google "):
		return withQuery("browser.google_search", text[len("google "):])
	case strings.HasPrefix(lower, "search for "):
		return withQuery("browser.google_search", text[len("search for "):])
	case lower == "pause" || lower == "pause music" || lower == "pause the music":
		return Intent{Name: "youtube_music.pause"}, nil
	case lower == "resume" || lower == "resume music" || lower == "resume the music":
		return Intent{Name: "youtube_music.resume"}, nil
	case lower == "next" || lower == "next song" || lower == "next track" ||
		lower == "skip" || lower == "skip song" || lower == "skip track":
		return Intent{Name: "youtube_music.next"}, nil
	case lower == "previous" || lower == "previous song" || lower == "previous track" ||
		lower == "last song" || lower == "last track" || lower == "go back one track":
		return Intent{Name: "youtube_music.previous"}, nil
	case lower == "what song is playing" || lower == "what track is playing" ||
		lower == "what is playing" || lower == "now playing":
		return Intent{Name: "youtube_music.now_playing"}, nil
	case lower == "list alarms" || lower == "show alarms" || lower == "what alarms are set":
		return Intent{Name: "alarm.list"}, nil
	case lower == "list timers" || lower == "show timers" || lower == "what timers are set":
		return Intent{Name: "timer.list"}, nil
	case lower == "cancel all alarms" || lower == "delete all alarms":
		return Intent{Name: "alarm.cancel", Query: "all"}, nil
	case lower == "cancel all timers" || lower == "delete all timers":
		return Intent{Name: "timer.cancel", Query: "all"}, nil
	}
	if value, ok := cutPrefixFold(text, "cancel alarm "); ok {
		return withQuery("alarm.cancel", value)
	}
	if value, ok := cutPrefixFold(text, "cancel timer "); ok {
		return withQuery("timer.cancel", value)
	}
	if value, ok := cutPrefixFold(text, "turn on "); ok {
		return withQuery("home.turn_on", value)
	}
	if value, ok := cutPrefixFold(text, "turn off "); ok {
		return withQuery("home.turn_off", value)
	}
	if value, ok := cutPrefixFold(text, "activate scene "); ok {
		return withQuery("home.activate_scene", value)
	}
	if value, ok := cutPrefixFold(text, "set scene "); ok {
		return withQuery("home.activate_scene", value)
	}
	if value, ok := cutPrefixFold(text, "what is the status of "); ok {
		return withQuery("home.state", value)
	}
	if value, ok := cutPrefixFold(text, "status of "); ok {
		return withQuery("home.state", value)
	}

	if matches := volumePattern.FindStringSubmatch(lower); len(matches) == 2 {
		value, _ := strconv.Atoi(matches[1])
		if value <= 100 {
			return Intent{Name: "youtube_music.volume", Value: value}, nil
		}
	}

	if strings.HasPrefix(lower, "play ") {
		query := strings.TrimSpace(strings.Trim(text[len("play "):], " .?!"))
		query = trimSuffixFold(query, " on youtube music")
		query = trimSuffixFold(query, " on youtube")
		return withQuery("youtube_music.play", query)
	}

	return Intent{}, ErrUnknown
}

// Speech recognizers commonly render the name "Nox" as the homophones
// "Knox" or "knocks". Keep this deliberately narrow so an unrelated phrase
// cannot wake the assistant.
func isWakeGreeting(value string) bool {
	words := strings.FieldsFunc(value, func(r rune) bool {
		return r < 'a' || r > 'z'
	})
	if len(words) != 2 {
		return false
	}
	greeting := words[0] == "hey" || words[0] == "hay" || words[0] == "hi"
	name := words[1] == "nox" || words[1] == "knox" || words[1] == "knocks"
	return greeting && name
}

func HasWakePrefix(value string) bool {
	value = strings.TrimSpace(value)
	if _, ok := stripWakePrefix(value); ok {
		return true
	}
	_, ok := stripWakePrefix(stripSpeechPreamble(value))
	return ok
}

func stripWakePrefix(value string) (string, bool) {
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return value, false
	}
	first := strings.ToLower(strings.Trim(fields[0], " ,.!?;:"))
	second := strings.ToLower(strings.Trim(fields[1], " ,.!?;:"))
	if !isWakeGreeting(first + " " + second) {
		return value, false
	}
	return strings.Join(fields[2:], " "), true
}

// A short phrase before punctuation is usually conversational filler or a
// Whisper artifact. If the suffix is clearly a registered command, use it.
func stripSpeechPreamble(value string) string {
	for index, r := range value {
		if r != ',' && r != ';' && r != ':' {
			continue
		}
		candidate := strings.TrimSpace(value[index+1:])
		lower := strings.ToLower(candidate)
		for _, prefix := range []string{
			"hey ", "hay ", "hi ",
			"play ", "search ", "google ", "set ", "start ", "turn ",
			"activate ", "pause", "resume", "next", "previous", "skip",
			"list ", "show ", "cancel ", "what ", "time", "status ",
			"check ", "read my ",
		} {
			if strings.HasPrefix(lower, prefix) {
				return candidate
			}
		}
	}
	return value
}

func parseDuration(value string) (time.Duration, bool) {
	value = strings.ReplaceAll(strings.ToLower(value), " and ", " ")
	fields := strings.Fields(value)
	if len(fields) == 0 || len(fields)%2 != 0 {
		return 0, false
	}
	var total time.Duration
	for index := 0; index < len(fields); index += 2 {
		amount, err := strconv.Atoi(fields[index])
		if err != nil || amount <= 0 {
			return 0, false
		}
		var unit time.Duration
		switch strings.TrimSuffix(fields[index+1], "s") {
		case "second", "sec":
			unit = time.Second
		case "minute", "min":
			unit = time.Minute
		case "hour", "hr":
			unit = time.Hour
		default:
			return 0, false
		}
		total += time.Duration(amount) * unit
	}
	return total, total > 0
}

func parseAlarmClock(value string) (hour, minute, dayOffset int, ok bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(value, "tomorrow") {
		dayOffset = 1
		value = strings.ReplaceAll(value, "tomorrow", "")
	}
	isMorning := strings.Contains(value, "morning")
	isEvening := strings.Contains(value, "evening") || strings.Contains(value, "tonight") || strings.Contains(value, "night")
	for _, noise := range []string{"in the morning", "morning", "in the evening", "evening", "tonight", "at night", "night", " at "} {
		value = strings.ReplaceAll(value, noise, " ")
	}
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if isMorning && !strings.Contains(value, "am") && !strings.Contains(value, "pm") {
		value += " am"
	}
	if isEvening && !strings.Contains(value, "am") && !strings.Contains(value, "pm") {
		value += " pm"
	}

	for _, layout := range []string{"3:04 pm", "3 pm", "15:04", "15"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.Hour(), parsed.Minute(), dayOffset, true
		}
	}
	return 0, 0, 0, false
}

func cutPrefixFold(value, prefix string) (string, bool) {
	if len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix) {
		return strings.TrimSpace(value[len(prefix):]), true
	}
	return "", false
}

func withQuery(name, query string) (Intent, error) {
	query = strings.TrimSpace(strings.Trim(query, " .?!"))
	if query == "" {
		return Intent{}, ErrUnknown
	}
	return Intent{Name: name, Query: query}, nil
}

func trimSuffixFold(value, suffix string) string {
	if len(value) >= len(suffix) && strings.EqualFold(value[len(value)-len(suffix):], suffix) {
		return strings.TrimSpace(value[:len(value)-len(suffix)])
	}
	return value
}
