//go:build darwin

package notify

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"nox/internal/scheduler"
)

type Desktop struct {
	logger *slog.Logger
}

func NewDesktop(logger *slog.Logger) *Desktop {
	return &Desktop{logger: logger}
}

func (d *Desktop) Notify(ctx context.Context, entry scheduler.Entry) error {
	title := "Nox timer"
	if entry.Kind == scheduler.KindAlarm {
		title = "Nox alarm"
	}
	message := entry.Label
	if message == "" {
		message = fmt.Sprintf("Your %s is due.", entry.Kind)
	}
	if path, err := exec.LookPath("osascript"); err == nil {
		script := `on run argv
display notification (item 2 of argv) with title (item 1 of argv)
end run`
		if output, notifyErr := exec.CommandContext(ctx, path, "-e", script, "--", title, message).CombinedOutput(); notifyErr != nil {
			d.logger.Warn("macOS notification failed", "error", notifyErr, "output", string(output))
		}
	}
	sound := "/System/Library/Sounds/Glass.aiff"
	if entry.Kind == scheduler.KindAlarm {
		sound = "/System/Library/Sounds/Sosumi.aiff"
	}
	if _, err := os.Stat(sound); err != nil {
		return nil
	}
	path, err := exec.LookPath("afplay")
	if err != nil {
		return nil
	}
	return exec.CommandContext(ctx, path, sound).Run()
}
