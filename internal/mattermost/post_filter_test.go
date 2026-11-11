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
		mentions    []string
		want        bool
	}{
		{"dm", string(model.ChannelTypeDirect), makePost("hello", "other"), nil, true},
		{"self dm", string(model.ChannelTypeDirect), makePost("hi", botID), nil, false},
		{"group without mention field", string(model.ChannelTypeGroup), makePost("gm text", "other"), nil, true},
		{"group with only other mention", string(model.ChannelTypeGroup), makePost("gm", "other"), []string{"other"}, true},
		{"open explicit mention and id", string(model.ChannelTypeOpen), makePost("hey @relaybot", "other"), []string{botID}, true},
		{"open explicit mention different case", string(model.ChannelTypeOpen), makePost("hey @RelayBot", "other"), []string{botID}, true},
		{"open mention id only channel broadcast", string(model.ChannelTypeOpen), makePost("@channel everyone", "other"), []string{botID}, false},
		{"open username only no mentions field", string(model.ChannelTypeOpen), makePost("hey @relaybot", "other"), nil, false},
		{"open no username with mention id", string(model.ChannelTypeOpen), makePost("hey", "other"), []string{botID}, false},
		{"private explicit mention", string(model.ChannelTypePrivate), makePost("p @relaybot ok", "other"), []string{botID}, true},
		{"prefix username false", string(model.ChannelTypeOpen), makePost("@relaybotx", "other"), []string{botID}, false},
		{"deleted", string(model.ChannelTypeOpen), &model.Post{UserId: "other", DeleteAt: 1, Message: "@relaybot"}, []string{botID}, false},
		{"system type", string(model.ChannelTypeOpen), &model.Post{UserId: "other", Type: model.PostTypeJoinChannel, Message: "x"}, []string{botID}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ShouldRelayPost(botID, botUser, tt.channelType, tt.post, tt.mentions)
			if got != tt.want {
				t.Fatalf("ShouldRelayPost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseMentionUserIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data map[string]any
		want []string
	}{
		{"json string", map[string]any{"mentions": `["u1","u2"]`}, []string{"u1", "u2"}},
		{"empty string", map[string]any{"mentions": ""}, nil},
		{"malformed", map[string]any{"mentions": "not-json"}, nil},
		{"missing", map[string]any{}, nil},
		{"slice", map[string]any{"mentions": []any{"a", "b"}}, []string{"a", "b"}},
		{"string slice", map[string]any{"mentions": []string{"a", "b"}}, []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ParseMentionUserIDs(tt.data)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v want %v", got, tt.want)
				}
			}
		})
	}
}

func TestMentionsFieldMalformed(t *testing.T) {
	t.Parallel()
	if !MentionsFieldMalformed(map[string]any{"mentions": "not-json"}) {
		t.Fatal("expected malformed")
	}
	if MentionsFieldMalformed(map[string]any{"mentions": `["u1"]`}) {
		t.Fatal("valid json should not be malformed")
	}
	if MentionsFieldMalformed(map[string]any{}) {
		t.Fatal("missing field is not malformed")
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
	ev.Add("mentions", `["bot-user-id"]`)

	ct, parsed, mentions, err := ParsePostedEvent(ev)
	if err != nil {
		t.Fatal(err)
	}
	if ct != string(model.ChannelTypeOpen) {
		t.Fatalf("channel type = %q", ct)
	}
	if parsed.Id != post.Id {
		t.Fatalf("post id = %q", parsed.Id)
	}
	if len(mentions) != 1 || mentions[0] != "bot-user-id" {
		t.Fatalf("mentions = %v", mentions)
	}
}
