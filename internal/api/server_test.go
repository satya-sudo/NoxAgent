package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nox/internal/assistant"
	"nox/internal/browser"
)

func TestBrowserCommandRoundTrip(t *testing.T) {
	broker := browser.NewBroker(2)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(assistant.New(assistant.Dependencies{Browser: broker}), broker, time.Second, logger)

	type commandResponse struct {
		status int
		body   []byte
	}
	response := make(chan commandResponse, 1)
	go func() {
		body := bytes.NewBufferString(`{"text":"play Numb on YouTube Music"}`)
		request := httptest.NewRequest(http.MethodPost, "/v1/commands", body)
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		response <- commandResponse{status: recorder.Code, body: recorder.Body.Bytes()}
	}()

	request := httptest.NewRequest(http.MethodGet, "/v1/browser/actions/next?wait=1s", nil).WithContext(context.Background())
	next := httptest.NewRecorder()
	handler.ServeHTTP(next, request)
	if next.Code != http.StatusOK {
		t.Fatalf("next action status = %d", next.Code)
	}
	var action browser.Action
	if err := json.NewDecoder(next.Body).Decode(&action); err != nil {
		t.Fatal(err)
	}
	if action.Type != "youtube_music.play" || action.Query != "Numb" {
		t.Fatalf("unexpected action: %#v", action)
	}

	completeBody := bytes.NewBufferString(`{"ok":true,"message":"Playing Numb on YouTube Music."}`)
	completeRequest := httptest.NewRequest(http.MethodPost, "/v1/browser/actions/"+action.ID+"/complete", completeBody)
	completeRequest.Header.Set("Content-Type", "application/json")
	complete := httptest.NewRecorder()
	handler.ServeHTTP(complete, completeRequest)
	if complete.Code != http.StatusNoContent {
		t.Fatalf("complete status = %d", complete.Code)
	}

	command := <-response
	if command.status != http.StatusOK {
		t.Fatalf("command status = %d, body = %s", command.status, command.body)
	}
}

type fixedTranscriber string

func (f fixedTranscriber) Transcribe(context.Context, []byte) (string, error) {
	return string(f), nil
}

func TestHealth(t *testing.T) {
	broker := browser.NewBroker(1)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	New(assistant.New(assistant.Dependencies{Browser: broker}), broker, time.Second, logger).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("health status = %d", recorder.Code)
	}
}

func TestDashboardIsEmbedded(t *testing.T) {
	broker := browser.NewBroker(1)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	New(assistant.New(assistant.Dependencies{Browser: broker}), broker, time.Second, logger).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !bytes.Contains(recorder.Body.Bytes(), []byte("Nox Command Center")) {
		t.Fatalf("dashboard status = %d", recorder.Code)
	}
}

func TestHandsFreeVoiceIgnoresSpeechWithoutWakeWord(t *testing.T) {
	broker := browser.NewBroker(1)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	request := httptest.NewRequest(http.MethodPost, "/v1/voice/commands", bytes.NewReader(make([]byte, 44)))
	request.Header.Set("X-Nox-Require-Wake", "true")
	recorder := httptest.NewRecorder()
	New(assistant.New(assistant.Dependencies{Browser: broker}), broker, time.Second, logger, fixedTranscriber("play music")).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !bytes.Contains(recorder.Body.Bytes(), []byte(`"ignored":true`)) {
		t.Fatalf("voice response status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
