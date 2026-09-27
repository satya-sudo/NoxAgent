package intent

import (
	"errors"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		intent    Intent
		wantError error
	}{
		{name: "time", input: "Nox, what time is it?", intent: Intent{Name: "clock.time"}},
		{name: "wake greeting", input: "Hey Nox", intent: Intent{Name: "wake.greet"}},
		{name: "wake greeting speech alias", input: "Hay Knox", intent: Intent{Name: "wake.greet"}},
		{name: "wake greeting punctuation", input: "Hey, Knox.", intent: Intent{Name: "wake.greet"}},
		{name: "wake prefix command", input: "Hey Nox, play some music on YouTube", intent: Intent{Name: "youtube_music.play", Query: "some music"}},
		{name: "speech preamble", input: "Enough, play some music on YouTube.", intent: Intent{Name: "youtube_music.play", Query: "some music"}},
		{name: "google", input: "search Google for Go context tutorial", intent: Intent{Name: "browser.google_search", Query: "Go context tutorial"}},
		{name: "google typo", input: "search goofle for weather in Delhi", intent: Intent{Name: "browser.google_search", Query: "weather in Delhi"}},
		{name: "play", input: "play Blinding Lights on YouTube Music", intent: Intent{Name: "youtube_music.play", Query: "Blinding Lights"}},
		{name: "pause", input: "pause the music", intent: Intent{Name: "youtube_music.pause"}},
		{name: "next", input: "skip song", intent: Intent{Name: "youtube_music.next"}},
		{name: "next track", input: "next track", intent: Intent{Name: "youtube_music.next"}},
		{name: "previous track", input: "previous track", intent: Intent{Name: "youtube_music.previous"}},
		{name: "volume", input: "set the volume to 42%", intent: Intent{Name: "youtube_music.volume", Value: 42}},
		{name: "timer", input: "set a timer for 1 hour and 30 minutes", intent: Intent{Name: "timer.set", Duration: 90 * time.Minute}},
		{name: "alarm", input: "set an alarm for 7:30 tomorrow morning", intent: Intent{Name: "alarm.set", Hour: 7, Minute: 30, DayOffset: 1}},
		{name: "list alarms", input: "list alarms", intent: Intent{Name: "alarm.list"}},
		{name: "cancel timer", input: "cancel timer abc123", intent: Intent{Name: "timer.cancel", Query: "abc123"}},
		{name: "light on", input: "turn on the living room lights", intent: Intent{Name: "home.turn_on", Query: "the living room lights"}},
		{name: "scene", input: "activate scene movie night", intent: Intent{Name: "home.activate_scene", Query: "movie night"}},
		{name: "sensor", input: "status of bedroom temperature", intent: Intent{Name: "home.state", Query: "bedroom temperature"}},
		{name: "unknown", input: "make me a sandwich", wantError: ErrUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Parse(test.input)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Parse() error = %v, want %v", err, test.wantError)
			}
			if got != test.intent {
				t.Fatalf("Parse() = %#v, want %#v", got, test.intent)
			}
		})
	}
}

func TestParseRejectsInvalidVolume(t *testing.T) {
	_, err := Parse("set volume to 500")
	if !errors.Is(err, ErrUnknown) {
		t.Fatalf("Parse() error = %v, want ErrUnknown", err)
	}
}

func TestHasWakePrefix(t *testing.T) {
	for _, value := range []string{"Hey Nox", "Hey Knox, play music", "Noise, hey Nox"} {
		if !HasWakePrefix(value) {
			t.Fatalf("HasWakePrefix(%q) = false", value)
		}
	}
	if HasWakePrefix("play music") {
		t.Fatal("ordinary command was treated as wake-prefixed")
	}
}
