package mattermost

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/takak2166/mattermost-grok-relay/internal/config"
)

// PostHandler is invoked for posts that pass the channel-type filter (DM, group, or
// explicit @mention in O/P) and the configured sender allowlist.
// Downstream relay steps (webhook forwarding) are out of scope for this package.
type PostHandler func(ctx context.Context, botID string, post *model.Post)

// BotListener maintains one WebSocket connection for a single bot user.
type BotListener struct {
	BotID    string
	SiteURL  string
	Identity BotIdentity
	WsOrigin string
	Token    string

	Dial           DialWebSocket
	ResolveID      ResolveIdentity
	Backoff        Backoff
	OnPost         PostHandler
	AllowedSenders config.AllowedUserSet
	Logger         *slog.Logger
	Now            func() time.Time
}

func (l *BotListener) Run(ctx context.Context) error {
	if l.Dial == nil {
		l.Dial = DefaultDialWebSocket
	}
	if l.Logger == nil {
		l.Logger = slog.Default()
	}
	if l.ResolveID == nil {
		l.ResolveID = DefaultResolveIdentity
	}
	backoff := l.Backoff
	if backoff.Initial == 0 {
		backoff = NewBackoff()
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		identity, err := l.ResolveID(ctx, l.SiteURL, l.Token, l.BotID)
		if err != nil {
			logBotListenerWarn(l.Logger, l.BotID, "mattermost resolve bot identity failed", classifyIdentityResolveFail(err))
			if err := backoff.Sleep(ctx); err != nil {
				return err
			}
			continue
		}
		l.Identity = identity
		ws, err := l.Dial(ctx, l.WsOrigin, l.Token)
		if err != nil {
			logBotListenerWarn(l.Logger, l.BotID, "mattermost websocket connect failed", classifyWebSocketDialFail(err))
			if err := backoff.Sleep(ctx); err != nil {
				return err
			}
			continue
		}
		l.Logger.Info("mattermost websocket connected", "bot_id", l.BotID)
		connectedAt := l.now()

		runErr := l.consume(ctx, ws)
		ws.Close()
		backoff.RecordDisconnect(l.now().Sub(connectedAt))
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
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
			return ctx.Err()
		case <-ws.PingTimeoutChannel():
			return nil
		case event, ok := <-ws.EventChannel():
			if !ok {
				return nil
			}
			l.handleEvent(ctx, event)
		}
	}
}

func (l *BotListener) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

func (l *BotListener) handleEvent(ctx context.Context, event *model.WebSocketEvent) {
	channelType, post, mentionIDs, err := ParsePostedEvent(event)
	if err != nil {
		if event.EventType() == model.WebsocketEventPosted {
			logBotListenerWarn(l.Logger, l.BotID, "mattermost posted event invalid", failClassPostedEventInvalid)
		}
		return
	}
	if MentionsFieldMalformed(event.GetData()) {
		l.Logger.Debug("mattermost posted event has malformed mentions field", "bot_id", l.BotID, "channel_type", channelType)
	}
	if !ShouldRelayPost(l.Identity.UserID, l.Identity.Username, channelType, post, mentionIDs) {
		return
	}
	if !l.AllowedSenders.Contains(post.UserId) {
		logRelaySenderNotAllowed(
			l.Logger,
			l.BotID,
			channelType,
			post.UserId,
			senderUsernameFromEvent(event),
			post.ChannelId,
			post.Id,
			post.Message,
		)
		return
	}
	if l.OnPost != nil {
		l.OnPost(ctx, l.BotID, post)
	}
}
