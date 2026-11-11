package mattermost

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/url"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	exitClassListenerExited = "listener_exited"

	failClassIdentityResolve    = "identity_resolve_failed"
	failClassIdentityTransport  = "identity_transport_failed"
	failClassIdentityAPI        = "identity_api_failed"
	failClassWebSocketDial      = "websocket_dial_failed"
	failClassWebSocketTransport = "websocket_transport_failed"
	failClassPostedEventInvalid = "posted_event_invalid"
	failClassSenderNotAllowed   = "sender_not_allowed"
)

const logMsgRelaySenderNotAllowed = "relay post sender not allowed"

const relayLogMessagePrefixRunes = 100

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

func classifyIdentityResolveFail(err error) string {
	if err == nil {
		return failClassIdentityResolve
	}
	var appErr *model.AppError
	if errors.As(err, &appErr) {
		return failClassIdentityAPI
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return failClassIdentityTransport
	}
	return failClassIdentityResolve
}

func classifyWebSocketDialFail(err error) string {
	if err == nil {
		return failClassWebSocketDial
	}
	var appErr *model.AppError
	if errors.As(err, &appErr) {
		if inner := appErr.Unwrap(); inner != nil {
			return classifyWebSocketDialFail(inner)
		}
		return failClassWebSocketTransport
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return failClassWebSocketTransport
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return failClassWebSocketTransport
	}
	return failClassWebSocketDial
}

func logBotListenerExit(logger *slog.Logger, botID, exitClass string) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Error("mattermost bot listener stopped", "bot_id", botID, "exit_class", exitClass)
}

func logBotListenerWarn(logger *slog.Logger, botID, event, failClass string) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn(event, "bot_id", botID, "fail_class", failClass)
}

func logRelaySenderNotAllowed(
	logger *slog.Logger,
	botID, channelType, senderUserID, senderUsername, channelID, postID, message string,
) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info(
		logMsgRelaySenderNotAllowed,
		"bot_id", botID,
		"fail_class", failClassSenderNotAllowed,
		"sender_user_id", senderUserID,
		"sender_name", senderUsername,
		"channel_type", channelType,
		"channel_id", channelID,
		"post_id", postID,
		"message_prefix", MessagePrefix(message, relayLogMessagePrefixRunes),
	)
}

func senderUsernameFromEvent(event *model.WebSocketEvent) string {
	if event == nil {
		return ""
	}
	data := event.GetData()
	if data == nil {
		return ""
	}
	name, _ := data["sender_name"].(string)
	return name
}
