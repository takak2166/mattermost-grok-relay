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
)

// Run loads websocket listeners for all bots and blocks until shutdown.
func Run(ctx context.Context, siteURL string, cfg config.Config) error {
	logger := slog.Default()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	hub := &mattermost.Hub{
		SiteURL: siteURL,
		Bots:    cfg.Bots,
		OnPost: func(ctx context.Context, botID string, post *model.Post) {
			logger.Debug("accepted post", "bot_id", botID, "post_id", post.Id, "channel_id", post.ChannelId)
		},
		Logger: logger,
	}
	return hub.Run(ctx)
}
