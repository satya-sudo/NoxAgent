package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestInterpretIncludesObservedConversation(t *testing.T) {
	client, err := New("http://127.0.0.1:8080", "local", "")
	if err != nil {
		t.Fatal(err)
	}
	client.Observe("play Numb", "Playing Numb on YouTube Music.")
	client.httpClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Messages []message `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Messages) != 4 || payload.Messages[1].Content != "play Numb" || payload.Messages[3].Content != "play another song by them" {
			t.Fatalf("messages = %#v", payload.Messages)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"choices":[{"message":{"content":"{\"action\":\"youtube_music.play\",\"query\":\"another song by Linkin Park\"}"}}]}`,
			)),
		}, nil
	})
	if _, err := client.Interpret(context.Background(), "play another song by them"); err != nil {
		t.Fatal(err)
	}
}

func TestInterpretValidatesToolDecision(t *testing.T) {
	client, err := New("http://127.0.0.1:8080", "local", "")
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"choices":[{"message":{"content":"{\"action\":\"home.turn_on\",\"query\":\"desk lamp\"}"}}]}`,
			)),
		}, nil
	})
	decision, err := client.Interpret(context.Background(), "could you illuminate my desk")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != "home.turn_on" || decision.Query != "desk lamp" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestValidateRejectsUnknownAction(t *testing.T) {
	err := validate(Decision{Action: "system.shell", Query: "rm"})
	if err == nil {
		t.Fatal("validate accepted an unknown action")
	}
}
