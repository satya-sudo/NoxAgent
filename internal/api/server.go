package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nox/internal/assistant"
	"nox/internal/browser"
	"nox/internal/gmail"
	"nox/internal/intent"
	"nox/internal/voice"
	"nox/internal/wiz"
)

type Server struct {
	assistant     *assistant.Service
	broker        *browser.Broker
	transcriber   voice.Transcriber
	gmail         *gmail.Client
	wiz           *wiz.Client
	actionTimeout time.Duration
	logger        *slog.Logger
}

type Option func(*Server)

func WithTranscriber(transcriber voice.Transcriber) Option {
	return func(server *Server) { server.transcriber = transcriber }
}

func WithGmail(client *gmail.Client) Option {
	return func(server *Server) { server.gmail = client }
}

func WithWiZ(client *wiz.Client) Option {
	return func(server *Server) { server.wiz = client }
}

func New(service *assistant.Service, broker *browser.Broker, actionTimeout time.Duration, logger *slog.Logger, options ...Option) http.Handler {
	s := &Server{assistant: service, broker: broker, actionTimeout: actionTimeout, logger: logger}
	for _, option := range options {
		option(s)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("POST /v1/commands", s.command)
	mux.HandleFunc("GET /v1/browser/status", s.browserStatus)
	mux.HandleFunc("GET /v1/browser/keepalive", s.browserKeepalive)
	mux.HandleFunc("GET /v1/status", s.status)
	mux.HandleFunc("POST /v1/voice/commands", s.voiceCommand)
	mux.HandleFunc("GET /v1/integrations/gmail/status", s.gmailStatus)
	mux.HandleFunc("GET /v1/integrations/gmail/auth/start", s.gmailAuthStart)
	mux.HandleFunc("GET /v1/integrations/gmail/auth/callback", s.gmailAuthCallback)
	mux.HandleFunc("GET /v1/integrations/gmail/messages", s.gmailMessages)
	mux.HandleFunc("GET /v1/integrations/wiz/status", s.wizStatus)
	mux.HandleFunc("POST /v1/integrations/wiz/discover", s.wizDiscover)
	mux.HandleFunc("GET /v1/browser/actions/next", s.nextAction)
	mux.HandleFunc("POST /v1/browser/actions/{id}/complete", s.completeAction)
	mux.Handle("GET /", uiHandler())
	return s.withMiddleware(mux)
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 750*time.Millisecond)
	defer cancel()
	gmailStatus := gmail.Status{}
	if s.gmail != nil {
		gmailStatus = s.gmail.Status()
	}
	wizStatus := map[string]any{"configured": false, "devices": []wiz.Device{}}
	if s.wiz != nil {
		wizStatus = map[string]any{"configured": true, "devices": s.wiz.Devices()}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"browser": s.broker.Status(time.Now()),
		"voice":   map[string]bool{"configured": s.transcriber != nil},
		"llm":     s.assistant.LLMStatus(ctx),
		"gmail":   gmailStatus,
		"wiz":     wizStatus,
	})
}

func (s *Server) wizStatus(w http.ResponseWriter, _ *http.Request) {
	if s.wiz == nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false, "devices": []wiz.Device{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "devices": s.wiz.Devices()})
}

func (s *Server) wizDiscover(w http.ResponseWriter, r *http.Request) {
	if s.wiz == nil {
		writeError(w, http.StatusServiceUnavailable, "WiZ integration is not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	devices, err := s.wiz.Discover(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (s *Server) gmailStatus(w http.ResponseWriter, _ *http.Request) {
	if s.gmail == nil {
		writeJSON(w, http.StatusOK, gmail.Status{})
		return
	}
	writeJSON(w, http.StatusOK, s.gmail.Status())
}

func (s *Server) gmailAuthStart(w http.ResponseWriter, r *http.Request) {
	if s.gmail == nil {
		writeError(w, http.StatusServiceUnavailable, "Gmail integration is not configured")
		return
	}
	url, err := s.gmail.AuthorizationURL()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, url, http.StatusSeeOther)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"authorization_url": url})
}

func (s *Server) gmailAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.gmail == nil {
		writeError(w, http.StatusServiceUnavailable, "Gmail integration is not configured")
		return
	}
	if providerError := r.URL.Query().Get("error"); providerError != "" {
		writeError(w, http.StatusBadRequest, "Gmail authorization was declined: "+providerError)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := s.gmail.CompleteAuthorization(ctx, r.URL.Query().Get("state"), r.URL.Query().Get("code")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "<!doctype html><title>Gmail connected</title><h1>Gmail connected to Nox.</h1><p>You can close this tab.</p>")
}

func (s *Server) gmailMessages(w http.ResponseWriter, r *http.Request) {
	if s.gmail == nil {
		writeError(w, http.StatusServiceUnavailable, "Gmail integration is not configured")
		return
	}
	limit := int64(10)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 || parsed > 50 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 50")
			return
		}
		limit = parsed
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	messages, err := s.gmail.ListMessages(ctx, r.URL.Query().Get("query"), limit)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, gmail.ErrNotConnected) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
}

func (s *Server) voiceCommand(w http.ResponseWriter, r *http.Request) {
	if s.transcriber == nil {
		writeError(w, http.StatusServiceUnavailable, "local speech recognition is not configured")
		return
	}
	audio, err := io.ReadAll(io.LimitReader(r.Body, 25<<20))
	r.Body.Close()
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read audio")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	transcript, err := s.transcriber.Transcribe(ctx, audio)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if r.Header.Get("X-Nox-Require-Wake") == "true" && !intent.HasWakePrefix(transcript) {
		writeJSON(w, http.StatusOK, map[string]any{"transcript": transcript, "ignored": true})
		return
	}
	reply, err := s.assistant.Handle(ctx, transcript)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":      err.Error(),
			"status":     http.StatusUnprocessableEntity,
			"transcript": transcript,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transcript": transcript, "reply": reply})
}

func (s *Server) browserStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.broker.Status(time.Now()))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) command(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(request.Text) == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.actionTimeout)
	defer cancel()
	reply, err := s.assistant.Handle(ctx, request.Text)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, intent.ErrUnknown) {
			status = http.StatusUnprocessableEntity
		} else if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
			err = fmt.Errorf("browser extension did not respond; load the Nox Browser Bridge extension and keep Chromium open")
		}
		s.logger.Warn("command failed", "text", request.Text, "error", err)
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, reply)
}

func (s *Server) nextAction(w http.ResponseWriter, r *http.Request) {
	wait := 20 * time.Second
	if raw := r.URL.Query().Get("wait"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 && parsed <= 30*time.Second {
			wait = parsed
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), wait)
	defer cancel()
	action, err := s.broker.Next(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, action)
}

func (s *Server) completeAction(w http.ResponseWriter, r *http.Request) {
	var result browser.Result
	if err := decodeJSON(r, &result); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.broker.Complete(r.PathValue("id"), result); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error":  message,
		"status": strconv.Itoa(status),
	})
}
