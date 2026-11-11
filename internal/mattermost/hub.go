package mattermost

import (
	"context"
	"log/slog"
	"sync"

	"github.com/takak2166/mattermost-grok-relay/internal/config"
)

type botListenerRunFunc func(context.Context, *BotListener) error

// Hub starts one websocket listener per configured bot.
type Hub struct {
	SiteURL   string
	WsOrigin  string
	Bots      []config.Bot
	Dial      DialWebSocket
	OnPost    PostHandler
	Logger    *slog.Logger
	ResolveID ResolveIdentity

	// runListener is used in tests to inject listener exit behavior.
	runListener botListenerRunFunc
}

func (h *Hub) Run(ctx context.Context) error {
	if h.Dial == nil {
		h.Dial = DefaultDialWebSocket
	}
	if h.Logger == nil {
		h.Logger = slog.Default()
	}
	if h.ResolveID == nil {
		h.ResolveID = DefaultResolveIdentity
	}
	wsOrigin, err := WebSocketSiteURL(h.SiteURL)
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
			run := h.runListener
			if run == nil {
				run = func(ctx context.Context, l *BotListener) error {
					return l.Run(ctx)
				}
			}
			if err := run(ctx, listener); err != nil {
				if ok, exitClass := botListenerExitClass(err); ok {
					logBotListenerExit(h.Logger, bot.ID, exitClass)
				}
			}
		}(bot)
	}

	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrAllListenersStopped
}
