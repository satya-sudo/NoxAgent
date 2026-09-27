//go:build !linux && !darwin

package notify

import (
	"context"
	"log/slog"

	"nox/internal/scheduler"
)

type Desktop struct {
	logger *slog.Logger
}

func NewDesktop(logger *slog.Logger) *Desktop {
	return &Desktop{logger: logger}
}

func (d *Desktop) Notify(_ context.Context, entry scheduler.Entry) error {
	d.logger.Info("desktop notifications are enabled only on Linux", "kind", entry.Kind, "id", entry.ID, "label", entry.Label)
	return nil
}
