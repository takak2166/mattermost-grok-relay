package mattermost

import (
	"context"
	"fmt"
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, len(h.Bots))
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
			if err := listener.Run(ctx); err != nil && err != context.Canceled {
				errCh <- fmt.Errorf("bot %q: %w", bot.ID, err)
			}
		}(bot)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		<-done
		return ctx.Err()
	case err := <-errCh:
		return err
	case <-done:
		return nil
	}
}
