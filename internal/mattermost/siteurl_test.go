package mattermost

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestNormalizeSiteURLPreservesSubpath(t *testing.T) {
	t.Parallel()
	got, err := NormalizeSiteURL("https://chat.example.invalid/mm/")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://chat.example.invalid/mm"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestWebSocketSiteURLSubpath(t *testing.T) {
	t.Parallel()
	got, err := WebSocketSiteURL("https://chat.example.invalid/mm")
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := "wss://chat.example.invalid/mm"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("got %q", got)
	}
	client := model.NewAPIv4Client("https://chat.example.invalid/mm")
	if client.APIURL != "https://chat.example.invalid/mm/api/v4" {
		t.Fatalf("APIURL = %q", client.APIURL)
	}
	wsURL, err := WebSocketSiteURL(client.URL)
	if err != nil {
		t.Fatal(err)
	}
	if gotConnect := wsURL + model.APIURLSuffix + "/websocket"; gotConnect != "wss://chat.example.invalid/mm/api/v4/websocket" {
		t.Fatalf("connect URL = %q", gotConnect)
	}
}
