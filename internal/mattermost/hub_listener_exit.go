package mattermost

import (
	"context"
	"errors"
	"log/slog"
)

const (
	exitClassListenerExited = "listener_exited"
)

// botListenerExitClass reports whether Hub should emit a structured log and which
// fixed exit_class to use. The original error is never passed to the logger.
func botListenerExitClass(err error) (log bool, exitClass string) {
	if err == nil {
		return false, ""
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false, ""
	}
	return true, exitClassListenerExited
}

func logBotListenerExit(logger *slog.Logger, botID, exitClass string) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Error("mattermost bot listener stopped", "bot_id", botID, "exit_class", exitClass)
}
