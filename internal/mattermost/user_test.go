package mattermost

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestResolveBotIdentityRespectsContextCancel(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/users/me" {
			http.NotFound(w, r)
			return
		}
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := ResolveBotIdentity(ctx, srv.URL, "fake-token-value", "bot-user-id", nil)
		done <- err
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancel, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("resolve did not return after cancel")
	}
	close(block)
}

func TestResolveBotIdentityUsesInjectedHTTPClient(t *testing.T) {
	t.Parallel()
	const wantID = "bot-user-id"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/users/me" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(model.User{Id: wantID, Username: "relaybot"})
	}))
	t.Cleanup(srv.Close)

	client := &http.Client{Timeout: time.Second}
	id, err := ResolveBotIdentity(context.Background(), srv.URL, "fake-token", wantID, client)
	if err != nil {
		t.Fatal(err)
	}
	if id.UserID != wantID || id.Username != "relaybot" {
		t.Fatalf("identity = %+v", id)
	}
}
