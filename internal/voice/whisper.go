package voice

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Transcriber interface {
	Transcribe(context.Context, []byte) (string, error)
}

type WhisperCLI struct {
	Binary   string
	Model    string
	Language string
}

func (w *WhisperCLI) Transcribe(ctx context.Context, wav []byte) (string, error) {
	if len(wav) < 44 {
		return "", fmt.Errorf("audio recording is empty")
	}
	directory, err := os.MkdirTemp("", "nox-voice-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(directory)
	input := filepath.Join(directory, "command.wav")
	output := filepath.Join(directory, "transcript")
	if err := os.WriteFile(input, wav, 0o600); err != nil {
		return "", err
	}
	language := w.Language
	if language == "" {
		language = "en"
	}
	command := exec.CommandContext(ctx, w.Binary,
		"-m", w.Model,
		"-f", input,
		"--output-txt",
		"-of", output,
		"-nt",
		"-np",
		"-sns",
		"-l", language,
		"--prompt", "Nox voice assistant. Hey Nox. Google Search. YouTube Music. Play music. Pause music. Next track. Set a timer. Set an alarm.",
	)
	if combined, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf("Whisper transcription failed: %w: %s", err, strings.TrimSpace(string(combined)))
	}
	text, err := os.ReadFile(output + ".txt")
	if err != nil {
		return "", fmt.Errorf("read Whisper transcript: %w", err)
	}
	transcript := strings.TrimSpace(string(text))
	if transcript == "" {
		return "", fmt.Errorf("no speech was detected")
	}
	return transcript, nil
}
