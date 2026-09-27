package config

import (
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Address       string
	ActionTimeout time.Duration
	StatePath     string
	HomeURL       string
	HomeToken     string
	LLMURL        string
	LLMModel      string
	LLMAPIKey     string
	WhisperBin    string
	WhisperModel  string
	WhisperLang   string
}

func Load() Config {
	statePath := "nox-state.json"
	if directory, err := os.UserConfigDir(); err == nil {
		statePath = filepath.Join(directory, "nox", "state.json")
	}
	config := Config{
		Address:       "127.0.0.1:7080",
		ActionTimeout: 15 * time.Second,
		StatePath:     statePath,
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
	config.HomeURL = os.Getenv("NOX_HOME_ASSISTANT_URL")
	config.HomeToken = os.Getenv("NOX_HOME_ASSISTANT_TOKEN")
	config.LLMURL = os.Getenv("NOX_LLM_URL")
	config.LLMModel = os.Getenv("NOX_LLM_MODEL")
	config.LLMAPIKey = os.Getenv("NOX_LLM_API_KEY")
	config.WhisperBin = os.Getenv("NOX_WHISPER_BIN")
	config.WhisperModel = os.Getenv("NOX_WHISPER_MODEL")
	config.WhisperLang = os.Getenv("NOX_WHISPER_LANGUAGE")
	return config
}
