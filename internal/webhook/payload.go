package webhook

import (
	"encoding/json"

	"github.com/mattermost/mattermost/server/public/model"
)

// Meta describes where an inbound Mattermost post came from.
type Meta struct {
	Platform  string   `json:"platform"`
	ChannelID string   `json:"channel_id"`
	ThreadID  string   `json:"thread_id"`
	PostID    string   `json:"post_id"`
	User      UserMeta `json:"user"`
	Text      string   `json:"text"`
}

// UserMeta identifies the post author for downstream routing.
type UserMeta struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// Envelope is the JSON body POSTed to the Grok inbound webhook.
type Envelope struct {
	Meta Meta `json:"meta"`
}

// ThreadID returns root_id when set, otherwise post_id (including DMs).
func ThreadID(post *model.Post) string {
	if post == nil {
		return ""
	}
	if post.RootId != "" {
		return post.RootId
	}
	return post.Id
}

// BuildEnvelope constructs the outbound webhook payload.
func BuildEnvelope(post *model.Post, senderName string) Envelope {
	return Envelope{
		Meta: Meta{
			Platform:  "mattermost",
			ChannelID: post.ChannelId,
			ThreadID:  ThreadID(post),
			PostID:    post.Id,
			User: UserMeta{
				ID:       post.UserId,
				Username: senderName,
			},
			Text: post.Message,
		},
	}
}

// MarshalEnvelope JSON-encodes the envelope.
func MarshalEnvelope(post *model.Post, senderName string) ([]byte, error) {
	return json.Marshal(BuildEnvelope(post, senderName))
}
