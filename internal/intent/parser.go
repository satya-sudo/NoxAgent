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
	Name        string
	Query       string
	Value       int
	Duration    time.Duration
	Hour        int
	Minute      int
	DayOffset   int
	Temperature int
}

var volumePattern = regexp.MustCompile(
	`(?i)(?:set\s+)?(?:the\s+)?volume(?:\s+to)?\s+(\d{1,3})(?:\s*%)?`,
)

var timerPattern = regexp.MustCompile(
	`(?i)^(?:set|start|create)(?:\s+me)?\s+(?:a\s+)?timer\s+for\s+(.+)$`,
)

var alarmPattern = regexp.MustCompile(
	`(?i)^set\s+(?:an\s+)?alarm\s+(?:for|at)\s+(.+)$`,
)

var wizBrightnessPattern = regexp.MustCompile(
	`(?i)^(?:set|make|dim|brighten)\s+(?:the\s+)?(.+?)\s+(?:(?:brightness)\s+)?(?:to\s+)?(\d{1,3})(?:\s*%|\s+percent)?$`,
)

var wizWarmPattern = regexp.MustCompile(
	`(?i)^(?:make|set|turn)\s+(?:the\s+)?(.+?)\s+(?:to\s+)?(warm(?:\s+white)?|cool(?:\s+white)?|daylight|white)(?:\s+color)?(?:\s+(?:and|at)\s+(\d{1,3})(?:\s*%|\s+percent)?)?$`,
)

var wizPercentPattern = regexp.MustCompile(
	`(?i)(\d{1,3})\s*(?:%|percent)`,
)

var youtubeSuffixPattern = regexp.MustCompile(
	`(?i)\s+on\s+youtube(?:\s+music)?(?:[\s?!.,]+music)?[\s?!.,]*$`,
)

func Parse(input string) (Intent, error) {
	text := normalizeSpeech(input)

	if text == "" {
		return Intent{}, ErrUnknown
	}

	// Handle wake word.
	if remainder, ok := stripWakePrefix(text); ok {
		if strings.TrimSpace(remainder) == "" {
			return Intent{Name: "wake.greet"}, nil
		}

		text = normalizeSpeech(remainder)
		text = stripPolitePrefix(text)
	}

	if text == "" {
		return Intent{}, ErrUnknown
	}

	lower := strings.ToLower(strings.Trim(text, " .?!"))

	// Timer.
	if matches := timerPattern.FindStringSubmatch(lower); len(matches) == 2 {
		duration, ok := parseDuration(matches[1])
		if ok {
			return Intent{
				Name:     "timer.set",
				Duration: duration,
			}, nil
		}

		return Intent{}, ErrUnknown
	}

	// Alarm.
	if matches := alarmPattern.FindStringSubmatch(lower); len(matches) == 2 {
		hour, minute, dayOffset, ok := parseAlarmClock(matches[1])
		if ok {
			return Intent{
				Name:      "alarm.set",
				Hour:      hour,
				Minute:    minute,
				DayOffset: dayOffset,
			}, nil
		}

		return Intent{}, ErrUnknown
	}

	// ============================================================
	// LIGHTS
	// ============================================================

	if value, ok := cutPrefixFold(text, "turn on "); ok {
		if isLightTarget(value) {
			return Intent{
				Name:  "wiz.turn_on",
				Query: "t-beamer",
			}, nil
		}

		return withQuery("home.turn_on", value)
	}

	if value, ok := cutPrefixFold(text, "turn off "); ok {
		if isLightTarget(value) {
			return Intent{
				Name:  "wiz.turn_off",
				Query: "t-beamer",
			}, nil
		}

		return withQuery("home.turn_off", value)
	}

	if isLightOnCommand(lower) {
		return Intent{
			Name:  "wiz.turn_on",
			Query: "t-beamer",
		}, nil
	}

	if isLightOffCommand(lower) {
		return Intent{
			Name:  "wiz.turn_off",
			Query: "t-beamer",
		}, nil
	}

	if parsed, ok := parseFlexibleWiZ(lower); ok {
		return parsed, nil
	}

	if matches := wizWarmPattern.FindStringSubmatch(lower); len(matches) == 4 &&
		isLightTarget(matches[1]) {

		brightness := 0

		if matches[3] != "" {
			brightness, _ = strconv.Atoi(matches[3])

			if brightness < 1 || brightness > 100 {
				return Intent{}, ErrUnknown
			}
		}

		return Intent{
			Name:        "wiz.set",
			Query:       "t-beamer",
			Value:       brightness,
			Temperature: temperatureForColor(matches[2]),
		}, nil
	}

	if matches := wizBrightnessPattern.FindStringSubmatch(lower); len(matches) == 3 &&
		isLightTarget(matches[1]) {

		brightness, _ := strconv.Atoi(matches[2])

		if brightness < 1 || brightness > 100 {
			return Intent{}, ErrUnknown
		}

		return Intent{
			Name:  "wiz.set",
			Query: "t-beamer",
			Value: brightness,
		}, nil
	}

	// ============================================================
	// EMAIL
	// ============================================================

	if isEmailReadCommand(lower) {
		return Intent{Name: "gmail.unread"}, nil
	}

	if isEmailCheckCommand(lower) {
		return Intent{Name: "gmail.unread"}, nil
	}

	if query, ok := extractAfterAnyPrefix(
		text,
		"search gmail for ",
		"search my gmail for ",
		"search my email for ",
		"search email for ",
		"find an email about ",
		"find email about ",
		"find emails about ",
	); ok {
		return withQuery("gmail.search", query)
	}

	// ============================================================
	// GENERAL
	// ============================================================

	switch {
	case isWakeGreeting(lower):
		return Intent{Name: "wake.greet"}, nil

	case isTimeCommand(lower):
		return Intent{Name: "clock.time"}, nil

	case strings.HasPrefix(lower, "search google for "):
		return withQuery(
			"browser.google_search",
			text[len("search google for "):],
		)

	case strings.HasPrefix(lower, "search goofle for "):
		return withQuery(
			"browser.google_search",
			text[len("search goofle for "):],
		)

	case strings.HasPrefix(lower, "search gogle for "):
		return withQuery(
			"browser.google_search",
			text[len("search gogle for "):],
		)

	case strings.HasPrefix(lower, "google "):
		return withQuery(
			"browser.google_search",
			text[len("google "):],
		)

	case strings.HasPrefix(lower, "search for "):
		return withQuery(
			"browser.google_search",
			text[len("search for "):],
		)

	case lower == "pause" ||
		lower == "pause music" ||
		lower == "pause the music" ||
		lower == "pause the song":

		return Intent{Name: "youtube_music.pause"}, nil

	case lower == "resume" ||
		lower == "resume music" ||
		lower == "resume the music" ||
		lower == "continue music" ||
		lower == "continue the music":

		return Intent{Name: "youtube_music.resume"}, nil

	case lower == "next" ||
		lower == "next song" ||
		lower == "next track" ||
		lower == "skip" ||
		lower == "skip song" ||
		lower == "skip track":

		return Intent{Name: "youtube_music.next"}, nil

	case lower == "previous" ||
		lower == "previous song" ||
		lower == "previous track" ||
		lower == "last song" ||
		lower == "last track" ||
		lower == "go back one track":

		return Intent{Name: "youtube_music.previous"}, nil

	case lower == "what song is playing" ||
		lower == "what track is playing" ||
		lower == "what is playing" ||
		lower == "what's playing" ||
		lower == "now playing":

		return Intent{Name: "youtube_music.now_playing"}, nil

	case lower == "list alarms" ||
		lower == "show alarms" ||
		lower == "what alarms are set" ||
		lower == "what alarms do i have":

		return Intent{Name: "alarm.list"}, nil

	case lower == "list timers" ||
		lower == "show timers" ||
		lower == "what timers are set" ||
		lower == "what timers do i have":

		return Intent{Name: "timer.list"}, nil

	case lower == "cancel all alarms" ||
		lower == "delete all alarms" ||
		lower == "clear all alarms":

		return Intent{
			Name:  "alarm.cancel",
			Query: "all",
		}, nil

	case lower == "cancel all timers" ||
		lower == "delete all timers" ||
		lower == "clear all timers":

		return Intent{
			Name:  "timer.cancel",
			Query: "all",
		}, nil
	}

	if value, ok := cutPrefixFold(text, "cancel alarm "); ok {
		return withQuery("alarm.cancel", value)
	}

	if value, ok := cutPrefixFold(text, "cancel timer "); ok {
		return withQuery("timer.cancel", value)
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

	if value, ok := cutPrefixFold(text, "what's the status of "); ok {
		return withQuery("home.state", value)
	}

	if value, ok := cutPrefixFold(text, "status of "); ok {
		return withQuery("home.state", value)
	}

	if matches := volumePattern.FindStringSubmatch(lower); len(matches) == 2 {
		value, _ := strconv.Atoi(matches[1])

		if value >= 0 && value <= 100 {
			return Intent{
				Name:  "youtube_music.volume",
				Value: value,
			}, nil
		}
	}

	if strings.HasPrefix(lower, "play ") {
		query := strings.TrimSpace(
			strings.Trim(text[len("play "):], " .?!"),
		)

		query = youtubeSuffixPattern.ReplaceAllString(query, "")

		return withQuery("youtube_music.play", query)
	}

	return Intent{}, ErrUnknown
}

// ============================================================
// SPEECH NORMALIZATION
// ============================================================

func normalizeSpeech(value string) string {
	value = strings.TrimSpace(value)

	replacer := strings.NewReplacer(
		".", " ",
		",", " ",
		"?", " ",
		"!", " ",
		";", " ",
		":", " ",
		"\"", " ",
		"'", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
	)

	value = replacer.Replace(value)

	return strings.ToLower(
		strings.Join(strings.Fields(value), " "),
	)
}

func stripPolitePrefix(value string) string {
	value = strings.TrimSpace(value)

	for {
		original := value

		for _, prefix := range []string{
			"can you please ",
			"could you please ",
			"would you please ",
			"will you please ",
			"can you ",
			"could you ",
			"would you ",
			"will you ",
			"can it please ",
			"can it ",
			"please ",
			"kindly ",
		} {
			if strings.HasPrefix(
				strings.ToLower(value),
				prefix,
			) {
				value = strings.TrimSpace(value[len(prefix):])
				break
			}
		}

		if value == original {
			return value
		}
	}
}

// ============================================================
// WAKE WORD
// ============================================================

func isWakeGreeting(value string) bool {
	words := strings.Fields(value)

	if len(words) != 2 {
		return false
	}

	greeting :=
		words[0] == "hey" ||
			words[0] == "hay" ||
			words[0] == "hi" ||
			words[0] == "hello"

	name :=
		words[1] == "nox" ||
			words[1] == "knox" ||
			words[1] == "knocks" ||
			words[1] == "nocks"

	return greeting && name
}

func HasWakePrefix(value string) bool {
	value = normalizeSpeech(value)

	_, ok := stripWakePrefix(value)

	return ok
}

func stripWakePrefix(value string) (string, bool) {
	fields := strings.Fields(value)

	if len(fields) < 2 {
		return value, false
	}

	first := strings.ToLower(
		strings.Trim(fields[0], " ,.!?;:"),
	)

	second := strings.ToLower(
		strings.Trim(fields[1], " ,.!?;:"),
	)

	if !isWakeGreeting(first + " " + second) {
		return value, false
	}

	return strings.Join(fields[2:], " "), true
}

// ============================================================
// LIGHTS
// ============================================================

func isLightTarget(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))

	return strings.Contains(value, "light") ||
		strings.Contains(value, "lights") ||
		strings.Contains(value, "t-beamer") ||
		strings.Contains(value, "t beamer") ||
		strings.HasPrefix(value, "wiz ")
}

func isLightOnCommand(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "lights on",
		"light on",
		"turn lights on",
		"turn light on",
		"turn the lights on",
		"turn the light on",
		"switch lights on",
		"switch light on",
		"switch the lights on",
		"switch the light on",
		"put the lights on",
		"put the light on":
		return true
	}

	return false
}

func isLightOffCommand(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "lights off",
		"light off",
		"turn lights off",
		"turn light off",
		"turn the lights off",
		"turn the light off",
		"switch lights off",
		"switch light off",
		"switch the lights off",
		"switch the light off",
		"put the lights off",
		"put the light off":
		return true
	}

	return false
}

func parseFlexibleWiZ(value string) (Intent, bool) {
	value = strings.ToLower(strings.TrimSpace(value))

	if !(strings.HasPrefix(value, "make ") ||
		strings.HasPrefix(value, "set ") ||
		strings.HasPrefix(value, "turn ") ||
		strings.HasPrefix(value, "dim ") ||
		strings.HasPrefix(value, "brighten ")) {
		return Intent{}, false
	}

	if !isLightTarget(value) {
		return Intent{}, false
	}

	brightness := 0

	if matches := wizPercentPattern.FindStringSubmatch(value); len(matches) == 2 {
		brightness, _ = strconv.Atoi(matches[1])

		if brightness < 1 || brightness > 100 {
			return Intent{}, false
		}
	}

	temperature := 0

	switch {
	case strings.Contains(value, "warm"):
		temperature = 2700

	case strings.Contains(value, "cool"):
		temperature = 5000

	case strings.Contains(value, "daylight"):
		temperature = 4200

	case strings.Contains(value, "white"):
		temperature = 4000
	}

	if brightness == 0 {
		if matches := wizBrightnessPattern.FindStringSubmatch(value); len(matches) == 3 {
			brightness, _ = strconv.Atoi(matches[2])

			if brightness < 1 || brightness > 100 {
				return Intent{}, false
			}
		}
	}

	if brightness == 0 && temperature == 0 {
		return Intent{}, false
	}

	return Intent{
		Name:        "wiz.set",
		Query:       "t-beamer",
		Value:       brightness,
		Temperature: temperature,
	}, true
}

func temperatureForColor(value string) int {
	switch {
	case strings.HasPrefix(value, "warm"):
		return 2700

	case strings.HasPrefix(value, "cool"):
		return 5000

	case value == "daylight":
		return 4200

	case value == "white":
		return 4000
	}

	return 0
}

// ============================================================
// EMAIL
// ============================================================

func isEmailTarget(value string) bool {
	return strings.Contains(value, "email") ||
		strings.Contains(value, "emails") ||
		strings.Contains(value, "mail") ||
		strings.Contains(value, "gmail")
}

func isLatestEmail(value string) bool {
	return strings.Contains(value, "latest") ||
		strings.Contains(value, "most recent") ||
		strings.Contains(value, "recent") ||
		strings.Contains(value, "newest") ||
		strings.Contains(value, "new email") ||
		strings.Contains(value, "new emails")
}

func isEmailReadVerb(value string) bool {
	return strings.Contains(value, "read") ||
		strings.Contains(value, "show") ||
		strings.Contains(value, "tell me") ||
		strings.Contains(value, "what does") ||
		strings.Contains(value, "what did") ||
		strings.Contains(value, "open")
}

func isEmailReadCommand(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))

	if !isEmailTarget(value) {
		return false
	}

	if isLatestEmail(value) && isEmailReadVerb(value) {
		return true
	}

	switch value {
	case "read my email",
		"read my emails",
		"read my mail",
		"read my mails",
		"show my email",
		"show my emails",
		"show my mail",
		"show my mails",
		"read email",
		"read emails",
		"read mail",
		"read mails",
		"show email",
		"show emails",
		"show mail",
		"show mails":
		return true
	}

	return false
}

func isEmailCheckCommand(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))

	if !isEmailTarget(value) {
		return false
	}

	switch value {
	case "check my email",
		"check my emails",
		"check my mail",
		"check my mails",
		"check email",
		"check emails",
		"check mail",
		"check mails",
		"check gmail",
		"check my gmail",
		"do i have new email",
		"do i have new emails",
		"do i have any new email",
		"do i have any new emails",
		"any new email",
		"any new emails",
		"any new mail",
		"any new mails",
		"any unread email",
		"any unread emails",
		"any unread mail",
		"any unread mails",
		"read my unread email",
		"read my unread emails",
		"read my unread mail",
		"read my unread mails",
		"anything new in gmail",
		"anything new in my email",
		"anything new in my emails":
		return true
	}

	return false
}

// ============================================================
// TIME
// ============================================================

func isTimeCommand(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "time",
		"what time is it",
		"what is the time",
		"what's the time",
		"tell me the time",
		"can you tell me the time",
		"what time is it now":
		return true
	}

	return false
}

// ============================================================
// DURATION
// ============================================================

func parseDuration(value string) (time.Duration, bool) {
	value = strings.ReplaceAll(
		strings.ToLower(value),
		" and ",
		" ",
	)

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

// ============================================================
// ALARM
// ============================================================

func parseAlarmClock(
	value string,
) (hour, minute, dayOffset int, ok bool) {

	value = strings.ToLower(strings.TrimSpace(value))

	if strings.Contains(value, "tomorrow") {
		dayOffset = 1
		value = strings.ReplaceAll(value, "tomorrow", "")
	}

	isMorning := strings.Contains(value, "morning")

	isEvening :=
		strings.Contains(value, "evening") ||
			strings.Contains(value, "tonight") ||
			strings.Contains(value, "night")

	for _, noise := range []string{
		"in the morning",
		"morning",
		"in the evening",
		"evening",
		"tonight",
		"at night",
		"night",
		" at ",
	} {
		value = strings.ReplaceAll(value, noise, " ")
	}

	value = strings.TrimSpace(
		strings.Join(strings.Fields(value), " "),
	)

	if isMorning &&
		!strings.Contains(value, "am") &&
		!strings.Contains(value, "pm") {
		value += " am"
	}

	if isEvening &&
		!strings.Contains(value, "am") &&
		!strings.Contains(value, "pm") {
		value += " pm"
	}

	for _, layout := range []string{
		"3:04 pm",
		"3 pm",
		"15:04",
		"15",
	} {
		parsed, err := time.Parse(layout, value)

		if err == nil {
			return parsed.Hour(),
				parsed.Minute(),
				dayOffset,
				true
		}
	}

	return 0, 0, 0, false
}

// ============================================================
// STRING HELPERS
// ============================================================

func cutPrefixFold(value, prefix string) (string, bool) {
	if len(value) >= len(prefix) &&
		strings.EqualFold(value[:len(prefix)], prefix) {
		return strings.TrimSpace(value[len(prefix):]), true
	}

	return "", false
}

func extractAfterAnyPrefix(
	value string,
	prefixes ...string,
) (string, bool) {

	lower := strings.ToLower(value)

	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			result := strings.TrimSpace(value[len(prefix):])

			if result != "" {
				return result, true
			}
		}
	}

	return "", false
}

func withQuery(name, query string) (Intent, error) {
	query = strings.TrimSpace(
		strings.Trim(query, " .?!"),
	)

	if query == "" {
		return Intent{}, ErrUnknown
	}

	return Intent{
		Name:  name,
		Query: query,
	}, nil
}
