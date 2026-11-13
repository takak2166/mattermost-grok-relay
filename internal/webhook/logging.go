package webhook

import "log/slog"

const (
	logMsgWebhookForwardSucceeded = "grok webhook forward succeeded"
	logMsgWebhookForwardFailed    = "grok webhook forward failed"

	failClassWebhookForwardHTTP       = "webhook_forward_http_error"
	failClassWebhookForwardNetwork    = "webhook_forward_network_error"
	failClassWebhookForwardTimeout    = "webhook_forward_timeout"
	failClassWebhookForwardEncode     = "webhook_forward_encode_error"
	failClassWebhookForwardBuildReq   = "webhook_forward_build_request_error"
	failClassWebhookForwardUnknownBot = "webhook_forward_unknown_bot"
)

// FailClassUnknownBot is returned when no bot entry matches the listener id.
const FailClassUnknownBot = failClassWebhookForwardUnknownBot

func logForwardResult(logger *slog.Logger, botID, postID string, result ForwardResult) {
	if logger == nil {
		logger = slog.Default()
	}
	if result.Success {
		logger.Info(logMsgWebhookForwardSucceeded, "bot_id", botID, "post_id", postID)
		return
	}
	failClass := result.FailClass
	if failClass == "" {
		failClass = failClassWebhookForwardNetwork
	}
	logger.Warn(logMsgWebhookForwardFailed, "bot_id", botID, "post_id", postID, "fail_class", failClass)
}
