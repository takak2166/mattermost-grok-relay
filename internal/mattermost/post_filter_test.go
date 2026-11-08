package mattermost

import (
	"encoding/json"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestShouldRelayPost(t *testing.T) {
	t.Parallel()
	botID := "bot-user-id"
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
		{"open with mention id", string(model.ChannelTypeOpen), makePost("hey", "other"), []string{botID}, true},
		{"open message only no mentions field", string(model.ChannelTypeOpen), makePost("hey @someone", "other"), nil, false},
		{"private with mention", string(model.ChannelTypePrivate), makePost("x", "other"), []string{"other", botID}, true},
		{"group with mention", string(model.ChannelTypeGroup), makePost("x", "other"), []string{botID}, true},
		{"group without mention", string(model.ChannelTypeGroup), makePost("x", "other"), []string{"other"}, false},
		{"deleted", string(model.ChannelTypeOpen), &model.Post{UserId: "other", DeleteAt: 1}, []string{botID}, false},
		{"system type", string(model.ChannelTypeOpen), &model.Post{UserId: "other", Type: model.PostTypeJoinChannel, Message: "x"}, []string{botID}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ShouldRelayPost(botID, tt.channelType, tt.post, tt.mentions)
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
