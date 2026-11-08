package mattermost

import (
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
)

// ParsePostedEvent extracts channel type, post, and mention user IDs from a posted event.
// Mention IDs come from data.mentions (JSON-encoded string array per Mattermost websocket API).
func ParsePostedEvent(event *model.WebSocketEvent) (channelType string, post *model.Post, mentionIDs []string, err error) {
	if event == nil {
		return "", nil, nil, fmt.Errorf("mattermost: nil event")
	}
	if event.EventType() != model.WebsocketEventPosted {
		return "", nil, nil, fmt.Errorf("mattermost: not a posted event")
	}
	data := event.GetData()
	ct, _ := data["channel_type"].(string)
	rawPost, ok := data["post"].(string)
	if !ok || rawPost == "" {
		return "", nil, nil, fmt.Errorf("mattermost: posted event missing post payload")
	}
	post = &model.Post{}
	if err := json.Unmarshal([]byte(rawPost), post); err != nil {
		return "", nil, nil, fmt.Errorf("mattermost: decode post: %w", err)
	}
	mentionIDs = ParseMentionUserIDs(data)
	return ct, post, mentionIDs, nil
}

// ParseMentionUserIDs reads data.mentions from a posted websocket event.
// Server sends a JSON-encoded array of user IDs as a string; missing or malformed data is treated as no mentions.
func ParseMentionUserIDs(data map[string]any) []string {
	if data == nil {
		return nil
	}
	raw, ok := data["mentions"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil
		}
		var ids []string
		if err := json.Unmarshal([]byte(v), &ids); err != nil {
			return nil
		}
		return ids
	case []string:
		return v
	case []any:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				ids = append(ids, s)
			}
		}
		if len(ids) == 0 {
			return nil
		}
		return ids
	default:
		return nil
	}
}

// MentionsFieldMalformed reports whether data.mentions is present but could not be parsed.
func MentionsFieldMalformed(data map[string]any) bool {
	if data == nil {
		return false
	}
	raw, ok := data["mentions"]
	if !ok || raw == nil {
		return false
	}
	switch v := raw.(type) {
	case string:
		if v == "" {
			return false
		}
		var ids []string
		return json.Unmarshal([]byte(v), &ids) != nil
	case []string, []any:
		return false
	default:
		return true
	}
}

// ShouldRelayPost reports whether a post should be picked up for this bot (mention or DM).
func ShouldRelayPost(botUserID, channelType string, post *model.Post, mentionIDs []string) bool {
	if post == nil || post.DeleteAt != 0 {
		return false
	}
	if post.UserId == botUserID {
		return false
	}
	if post.Type != "" && post.Type != model.PostTypeDefault {
		return false
	}
	if channelType == string(model.ChannelTypeDirect) {
		return true
	}
	switch channelType {
	case string(model.ChannelTypeOpen),
		string(model.ChannelTypePrivate),
		string(model.ChannelTypeGroup):
		return mentionIncludesBot(mentionIDs, botUserID)
	default:
		return false
	}
}

func mentionIncludesBot(mentionIDs []string, botUserID string) bool {
	for _, id := range mentionIDs {
		if id == botUserID {
			return true
		}
	}
	return false
}
