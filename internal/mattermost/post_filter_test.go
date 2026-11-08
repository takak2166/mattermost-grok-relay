package mattermost

import (
	"encoding/json"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestShouldRelayPost(t *testing.T) {
	t.Parallel()
	botID := "bot-user-id"
	botUser := "relaybot"
	makePost := func(msg, user string) *model.Post {
		return &model.Post{UserId: user, Message: msg}
	}

	tests := []struct {
		name        string
		channelType string
		post        *model.Post
		want        bool
	}{
		{"dm", string(model.ChannelTypeDirect), makePost("hello", "other"), true},
		{"self dm", string(model.ChannelTypeDirect), makePost("hi", botID), false},
		{"channel mention", string(model.ChannelTypeOpen), makePost("hey @relaybot", "other"), true},
		{"channel no mention", string(model.ChannelTypeOpen), makePost("hey there", "other"), false},
		{"group channel ignored", string(model.ChannelTypeGroup), makePost("@relaybot", "other"), false},
		{"deleted", string(model.ChannelTypeOpen), &model.Post{UserId: "other", Message: "@relaybot", DeleteAt: 1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ShouldRelayPost(botID, botUser, tt.channelType, tt.post)
			if got != tt.want {
				t.Fatalf("ShouldRelayPost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePostedEvent(t *testing.T) {
	t.Parallel()
	post := &model.Post{Id: "post-id", UserId: "user-1", Message: "hello"}
	raw, err := json.Marshal(post)
	if err != nil {
		t.Fatal(err)
	}
	ev := model.NewWebSocketEvent(model.WebsocketEventPosted, "", "", "", nil, "")
	ev.Add("channel_type", string(model.ChannelTypeOpen))
	ev.Add("post", string(raw))

	ct, parsed, err := ParsePostedEvent(ev)
	if err != nil {
		t.Fatal(err)
	}
	if ct != string(model.ChannelTypeOpen) {
		t.Fatalf("channel type = %q", ct)
	}
	if parsed.Id != post.Id {
		t.Fatalf("post id = %q", parsed.Id)
	}
}
