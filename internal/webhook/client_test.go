package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestForwardSuccess(t *testing.T) {
	t.Parallel()
	var gotAuth string
	var gotBody Envelope
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	client := &Client{Logger: logger}

	post := &model.Post{
		Id:        "post-1",
		ChannelId: "ch-1",
		UserId:    "7yjx8ntqhzn8zxmnc5k37wqg84",
		Message:   "hi",
	}
	result := client.Forward(context.Background(), "bot-a", srv.URL, "dummy-webhook-key", post, "@user")
	if !result.Success || result.FailClass != "" {
		t.Fatalf("result = %+v", result)
	}
	if gotAuth != "Bearer dummy-webhook-key" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotBody.Meta.Platform != "mattermost" || gotBody.Meta.Text != "hi" {
		t.Fatalf("body = %+v", gotBody.Meta)
	}
	out := buf.String()
	for _, forbidden := range []string{"dummy-webhook-key", "hi", srv.URL, "http://"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("log leaked %q: %s", forbidden, out)
		}
	}
	if !strings.Contains(out, logMsgWebhookForwardSucceeded) {
		t.Fatalf("log = %s", out)
	}
}

func TestForwardHTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	client := &Client{}
	post := &model.Post{Id: "p1", ChannelId: "c1", UserId: "u1", Message: "x"}
	result := client.Forward(context.Background(), "bot-a", srv.URL, "key", post, "")
	if result.Success || result.FailClass != failClassWebhookForwardHTTP {
		t.Fatalf("result = %+v", result)
	}
}

func TestSetRequestAuthCentralized(t *testing.T) {
	t.Parallel()
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:1/", nil)
	if err != nil {
		t.Fatal(err)
	}
	SetRequestAuth(req, "secret-key-value")
	if req.Header.Get("Authorization") != "Bearer secret-key-value" {
		t.Fatalf("header = %q", req.Header.Get("Authorization"))
	}
}

func TestForwardNetworkUsesInjectedDoPost(t *testing.T) {
	t.Parallel()
	client := &Client{
		DoPost: func(context.Context, *http.Client, *http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}
	post := &model.Post{Id: "p1", ChannelId: "c1", UserId: "u1", Message: "m"}
	result := client.Forward(context.Background(), "bot-a", "http://127.0.0.1:1/", "key", post, "")
	if result.Success || result.FailClass != failClassWebhookForwardNetwork {
		t.Fatalf("result = %+v", result)
	}
}
