package webhook

import (
	"encoding/json"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestThreadIDUsesRootWhenSet(t *testing.T) {
	t.Parallel()
	post := &model.Post{Id: "post-1", RootId: "root-abc"}
	if got := ThreadID(post); got != "root-abc" {
		t.Fatalf("ThreadID = %q, want root-abc", got)
	}
}

func TestThreadIDUsesPostIDWhenRootEmpty(t *testing.T) {
	t.Parallel()
	post := &model.Post{Id: "post-dm-1", RootId: ""}
	if got := ThreadID(post); got != "post-dm-1" {
		t.Fatalf("ThreadID = %q, want post-dm-1", got)
	}
}

func TestBuildEnvelope(t *testing.T) {
	t.Parallel()
	post := &model.Post{
		Id:        "p99",
		ChannelId: "ch1",
		RootId:    "",
		UserId:    "7yjx8ntqhzn8zxmnc5k37wqg84",
		Message:   "hello grok",
	}
	raw, err := MarshalEnvelope(post, "@sender")
	if err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Platform != "mattermost" {
		t.Fatalf("platform = %q", env.Meta.Platform)
	}
	if env.Meta.ThreadID != "p99" {
		t.Fatalf("thread_id = %q", env.Meta.ThreadID)
	}
	if env.Meta.User.ID != post.UserId || env.Meta.User.Username != "@sender" {
		t.Fatalf("user = %+v", env.Meta.User)
	}
	if env.Meta.Text != "hello grok" {
		t.Fatalf("text = %q", env.Meta.Text)
	}
}
