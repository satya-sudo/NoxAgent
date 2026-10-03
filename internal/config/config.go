package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Address              string
	ActionTimeout        time.Duration
	StatePath            string
	GmailCredentialsPath string
	GmailTokenPath       string
	GmailRedirectURL     string
	HomeURL              string
	HomeToken            string
	LLMURL               string
	LLMModel             string
	LLMAPIKey            string
	WhisperBin           string
	WhisperModel         string
	WhisperLang          string
	WiZEnabled           bool
	WiZLights            string
	WiZBroadcast         string
}

func Load() Config {
	statePath := "nox-state.json"
	gmailTokenPath := "gmail-token.json"
	if directory, err := os.UserConfigDir(); err == nil {
		statePath = filepath.Join(directory, "nox", "state.json")
		gmailTokenPath = filepath.Join(directory, "nox", "gmail-token.json")
	}
	config := Config{
		Address:          "127.0.0.1:7080",
		ActionTimeout:    15 * time.Second,
		StatePath:        statePath,
		GmailTokenPath:   gmailTokenPath,
		GmailRedirectURL: "http://127.0.0.1:7080/v1/integrations/gmail/auth/callback",
	}
	if value := os.Getenv("NOX_ADDRESS"); value != "" {
		config.Address = value
	}
	if value := os.Getenv("NOX_ACTION_TIMEOUT"); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			config.ActionTimeout = parsed
		}
	}
	if value := os.Getenv("NOX_STATE_PATH"); value != "" {
		config.StatePath = value
	}
	config.GmailCredentialsPath = os.Getenv("NOX_GMAIL_CREDENTIALS_PATH")
	if value := os.Getenv("NOX_GMAIL_TOKEN_PATH"); value != "" {
		config.GmailTokenPath = value
	}
	if value := os.Getenv("NOX_GMAIL_REDIRECT_URL"); value != "" {
		config.GmailRedirectURL = value
	}
	config.HomeURL = os.Getenv("NOX_HOME_ASSISTANT_URL")
	config.HomeToken = os.Getenv("NOX_HOME_ASSISTANT_TOKEN")
	config.LLMURL = os.Getenv("NOX_LLM_URL")
	config.LLMModel = os.Getenv("NOX_LLM_MODEL")
	config.LLMAPIKey = os.Getenv("NOX_LLM_API_KEY")
	config.WhisperBin = os.Getenv("NOX_WHISPER_BIN")
	config.WhisperModel = os.Getenv("NOX_WHISPER_MODEL")
	config.WhisperLang = os.Getenv("NOX_WHISPER_LANGUAGE")
	config.WiZEnabled = strings.EqualFold(os.Getenv("NOX_WIZ_ENABLED"), "true")
	config.WiZLights = os.Getenv("NOX_WIZ_LIGHTS")
	config.WiZBroadcast = os.Getenv("NOX_WIZ_BROADCAST")
	if config.WiZLights != "" {
		config.WiZEnabled = true
	}
	return config
}
