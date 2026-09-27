//go:build linux

package notify

import (
	"context"
	"fmt"
	"log/slog"
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
	path, err := exec.LookPath("notify-send")
	if err != nil {
		d.logger.Warn("notify-send is unavailable; alarm was logged only", "kind", entry.Kind, "id", entry.ID)
		return nil
	}
	return exec.CommandContext(ctx, path, "--urgency=critical", title, message).Run()
}
