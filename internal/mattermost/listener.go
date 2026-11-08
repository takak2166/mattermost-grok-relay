package mattermost

import (
	"context"
	"log/slog"

	"github.com/mattermost/mattermost/server/public/model"
)

// PostHandler is invoked for posts that pass the mention/DM filter.
// Downstream relay steps (webhook forwarding) are out of scope for this package.
type PostHandler func(ctx context.Context, botID string, post *model.Post)

// BotListener maintains one WebSocket connection for a single bot user.
type BotListener struct {
	BotID    string
	Identity BotIdentity
	WsOrigin string
	Token    string

	Dial    DialWebSocket
	Backoff Backoff
	OnPost  PostHandler
	Logger  *slog.Logger
}

func (l *BotListener) Run(ctx context.Context) error {
	if l.Dial == nil {
		l.Dial = DefaultDialWebSocket
	}
	if l.Logger == nil {
		l.Logger = slog.Default()
	}
	backoff := l.Backoff
	if backoff.Initial == 0 {
		backoff = NewBackoff()
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		ws, err := l.Dial(l.WsOrigin, l.Token)
		if err != nil {
			l.Logger.Warn("mattermost websocket connect failed", "bot_id", l.BotID, "error", err)
			if err := backoff.Sleep(ctx); err != nil {
				return err
			}
			continue
		}
		backoff.Reset()
		l.Logger.Info("mattermost websocket connected", "bot_id", l.BotID)

		runErr := l.consume(ctx, ws)
		ws.Close()
		if runErr != nil && runErr != context.Canceled {
			return runErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		l.Logger.Warn("mattermost websocket disconnected, reconnecting", "bot_id", l.BotID)
		if err := backoff.Sleep(ctx); err != nil {
			return err
		}
	}
}

func (l *BotListener) consume(ctx context.Context, ws WebSocket) error {
	ws.Listen()
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case _, ok := <-ws.PingTimeoutChannel():
			if !ok {
				return nil
			}
			return nil
		case event, ok := <-ws.EventChannel():
			if !ok {
				return nil
			}
			l.handleEvent(ctx, event)
		}
	}
}

func (l *BotListener) handleEvent(ctx context.Context, event *model.WebSocketEvent) {
	channelType, post, err := ParsePostedEvent(event)
	if err != nil {
		return
	}
	if !ShouldRelayPost(l.Identity.UserID, l.Identity.Username, channelType, post) {
		return
	}
	if l.OnPost != nil {
		l.OnPost(ctx, l.BotID, post)
	}
}
