package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nox/internal/api"
	"nox/internal/assistant"
	"nox/internal/browser"
	"nox/internal/config"
	"nox/internal/gmail"
	"nox/internal/homeassistant"
	"nox/internal/llm"
	"nox/internal/notify"
	"nox/internal/scheduler"
	"nox/internal/voice"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	settings := config.Load()
	broker := browser.NewBroker(32)
	store, err := scheduler.OpenStore(settings.StatePath)
	if err != nil {
		logger.Error("could not open scheduler state", "path", settings.StatePath, "error", err)
		os.Exit(1)
	}
	schedules := scheduler.NewService(store, notify.NewDesktop(logger), logger)
	var home assistant.Home
	if settings.HomeURL != "" || settings.HomeToken != "" {
		homeClient, homeErr := homeassistant.New(settings.HomeURL, settings.HomeToken)
		if homeErr != nil {
			logger.Error("invalid Home Assistant configuration", "error", homeErr)
			os.Exit(1)
		}
		home = homeClient
		logger.Info("Home Assistant integration enabled", "url", settings.HomeURL)
	}
	var interpreter assistant.Interpreter
	if settings.LLMURL != "" {
		llmClient, llmErr := llm.New(settings.LLMURL, settings.LLMModel, settings.LLMAPIKey)
		if llmErr != nil {
			logger.Error("invalid local LLM configuration", "error", llmErr)
			os.Exit(1)
		}
		interpreter = llmClient
		logger.Info("local LLM fallback enabled", "url", settings.LLMURL, "model", settings.LLMModel)
	}
	var transcriber voice.Transcriber
	if settings.WhisperBin != "" || settings.WhisperModel != "" {
		if settings.WhisperBin == "" || settings.WhisperModel == "" {
			logger.Error("both NOX_WHISPER_BIN and NOX_WHISPER_MODEL are required")
			os.Exit(1)
		}
		transcriber = &voice.WhisperCLI{Binary: settings.WhisperBin, Model: settings.WhisperModel, Language: settings.WhisperLang}
		logger.Info("local voice transcription enabled", "model", settings.WhisperModel)
	}
	var gmailClient *gmail.Client
	if settings.GmailCredentialsPath != "" {
		gmailClient, err = gmail.New(gmail.Config{
			CredentialsPath: settings.GmailCredentialsPath,
			TokenPath:       settings.GmailTokenPath,
			RedirectURL:     settings.GmailRedirectURL,
		})
		if err != nil {
			logger.Error("invalid Gmail configuration", "error", err)
			os.Exit(1)
		}
		logger.Info("Gmail read-only integration enabled", "redirect_url", settings.GmailRedirectURL)
	}
	service := assistant.New(assistant.Dependencies{
		Browser:     broker,
		Scheduler:   schedules,
		Home:        home,
		Interpreter: interpreter,
		Gmail:       gmailClient,
	})
	options := make([]api.Option, 0, 2)
	if transcriber != nil {
		options = append(options, api.WithTranscriber(transcriber))
	}
	if gmailClient != nil {
		options = append(options, api.WithGmail(gmailClient))
	}
	var handler http.Handler = api.New(service, broker, settings.ActionTimeout, logger, options...)

	server := &http.Server{
		Addr:              settings.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go schedules.Run(ctx)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	logger.Info("Nox is listening", "address", settings.Address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
