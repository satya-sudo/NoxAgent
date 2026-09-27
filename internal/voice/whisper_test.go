package voice

import (
	"context"
	"testing"
)

func TestWhisperRejectsEmptyAudio(t *testing.T) {
	_, err := (&WhisperCLI{}).Transcribe(context.Background(), []byte("short"))
	if err == nil {
		t.Fatal("Transcribe accepted invalid audio")
	}
}
