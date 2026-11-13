package relay

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/takak2166/mattermost-grok-relay/internal/config"
	"github.com/takak2166/mattermost-grok-relay/internal/webhook"
)

// Forwarder posts allowed Mattermost events to per-bot Grok webhooks.
type Forwarder struct {
	bots   map[string]config.Bot
	client *webhook.Client
}

// NewForwarder indexes bots by config id.
func NewForwarder(cfg config.Config, client *webhook.Client) *Forwarder {
	bots := make(map[string]config.Bot, len(cfg.Bots))
	for _, b := range cfg.Bots {
		bots[b.ID] = b
	}
	if client == nil {
		client = &webhook.Client{}
	}
	return &Forwarder{bots: bots, client: client}
}

// HandlePost forwards one post and returns the attempt result for downstream error handling.
func (f *Forwarder) HandlePost(ctx context.Context, botID string, post *model.Post, senderName string) webhook.ForwardResult {
	bot, ok := f.bots[botID]
	if !ok {
		return webhook.ForwardResult{FailClass: webhook.FailClassUnknownBot}
	}
	return f.client.Forward(
		ctx,
		botID,
		bot.WebhookURL.Value(),
		bot.WebhookKey.Value(),
		post,
		senderName,
	)
}
