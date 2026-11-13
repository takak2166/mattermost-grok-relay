package webhook

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const forwardTimeout = 10 * time.Second

// DoPost performs the HTTP POST. It is swapped in tests.
type DoPost func(context.Context, *http.Client, *http.Request) (*http.Response, error)

// Client POSTs inbound meta JSON to a Grok webhook once per call.
type Client struct {
	HTTPClient *http.Client
	Logger     *slog.Logger
	DoPost     DoPost
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: forwardTimeout}
}

func (c *Client) doPost(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	if c.DoPost != nil {
		return c.DoPost(ctx, client, req)
	}
	return client.Do(req.WithContext(ctx))
}

// Forward sends one JSON envelope with the configured auth header. It does not retry.
func (c *Client) Forward(ctx context.Context, botID, webhookURL, key string, post *model.Post, senderName string) ForwardResult {
	ctx, cancel := context.WithTimeout(ctx, forwardTimeout)
	defer cancel()

	logger := c.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if post == nil {
		result := ForwardResult{FailClass: failClassWebhookForwardEncode}
		logForwardResult(logger, botID, "", result)
		return result
	}

	body, err := MarshalEnvelope(post, senderName)
	if err != nil {
		result := ForwardResult{FailClass: failClassWebhookForwardEncode}
		logForwardResult(logger, botID, post.Id, result)
		return result
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		result := ForwardResult{FailClass: failClassWebhookForwardBuildReq}
		logForwardResult(logger, botID, post.Id, result)
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	SetRequestAuth(req, key)

	resp, err := c.doPost(ctx, c.httpClient(), req)
	if err != nil {
		result := ForwardResult{FailClass: classifyForwardError(err)}
		logForwardResult(logger, botID, post.Id, result)
		return result
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result := ForwardResult{Success: true}
		logForwardResult(logger, botID, post.Id, result)
		return result
	}
	result := ForwardResult{FailClass: failClassWebhookForwardHTTP}
	logForwardResult(logger, botID, post.Id, result)
	return result
}

func classifyForwardError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return failClassWebhookForwardTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return failClassWebhookForwardTimeout
	}
	return failClassWebhookForwardNetwork
}
