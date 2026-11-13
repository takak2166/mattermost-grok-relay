package relay

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/takak2166/mattermost-grok-relay/internal/config"
	"github.com/takak2166/mattermost-grok-relay/internal/mattermost"
	"github.com/takak2166/mattermost-grok-relay/internal/webhook"
)

// Run loads websocket listeners for all bots and blocks until shutdown.
func Run(ctx context.Context, siteURL string, cfg config.Config, allowed config.AllowedUserSet) error {
	logger := slog.Default()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	forwarder := NewForwarder(cfg, &webhook.Client{Logger: logger})

	hub := &mattermost.Hub{
		SiteURL:        siteURL,
		Bots:           cfg.Bots,
		AllowedSenders: allowed,
		OnPost: func(ctx context.Context, botID string, post *model.Post, senderName string) {
			go func(p *model.Post, name string) {
				_ = forwarder.HandlePost(ctx, botID, p, name)
			}(post, senderName)
		},
		Logger: logger,
	}
	return hub.Run(ctx)
}
