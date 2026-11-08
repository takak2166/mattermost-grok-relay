package mattermost

import (
	"context"
	"log/slog"
	"sync"

	"github.com/takak2166/mattermost-grok-relay/internal/config"
)

// Hub starts one websocket listener per configured bot.
type Hub struct {
	SiteURL   string
	WsOrigin  string
	Bots      []config.Bot
	Dial      DialWebSocket
	OnPost    PostHandler
	Logger    *slog.Logger
	ResolveID func(siteURL, token, userID string) (BotIdentity, error)
}

func (h *Hub) Run(ctx context.Context) error {
	if h.Dial == nil {
		h.Dial = DefaultDialWebSocket
	}
	if h.Logger == nil {
		h.Logger = slog.Default()
	}
	if h.ResolveID == nil {
		h.ResolveID = ResolveBotIdentity
	}
	wsOrigin, err := WebSocketOrigin(h.SiteURL)
	if err != nil {
		return err
	}
	h.WsOrigin = wsOrigin

	var wg sync.WaitGroup
	for _, bot := range h.Bots {
		wg.Add(1)
		go func(bot config.Bot) {
			defer wg.Done()
			listener := &BotListener{
				BotID:     bot.ID,
				SiteURL:   h.SiteURL,
				WsOrigin:  wsOrigin,
				Token:     bot.Token.Value(),
				Dial:      h.Dial,
				ResolveID: h.ResolveID,
				OnPost:    h.OnPost,
				Logger:    h.Logger,
			}
			_ = listener.Run(ctx)
		}(bot)
	}

	wg.Wait()
	return ctx.Err()
}
