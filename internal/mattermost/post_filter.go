package mattermost

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// ParsePostedEvent extracts channel type and post from a posted websocket event.
func ParsePostedEvent(event *model.WebSocketEvent) (channelType string, post *model.Post, err error) {
	if event == nil {
		return "", nil, fmt.Errorf("mattermost: nil event")
	}
	if event.EventType() != model.WebsocketEventPosted {
		return "", nil, fmt.Errorf("mattermost: not a posted event")
	}
	data := event.GetData()
	ct, _ := data["channel_type"].(string)
	rawPost, ok := data["post"].(string)
	if !ok || rawPost == "" {
		return "", nil, fmt.Errorf("mattermost: posted event missing post payload")
	}
	post = &model.Post{}
	if err := json.Unmarshal([]byte(rawPost), post); err != nil {
		return "", nil, fmt.Errorf("mattermost: decode post: %w", err)
	}
	return ct, post, nil
}

// ShouldRelayPost reports whether a post should be picked up for this bot (mention or DM).
func ShouldRelayPost(botUserID, botUsername, channelType string, post *model.Post) bool {
	if post == nil || post.DeleteAt != 0 {
		return false
	}
	if post.UserId == botUserID {
		return false
	}
	if channelType == string(model.ChannelTypeDirect) {
		return true
	}
	if channelType == string(model.ChannelTypeOpen) || channelType == string(model.ChannelTypePrivate) {
		return messageMentionsUsername(post.Message, botUsername)
	}
	return false
}

func messageMentionsUsername(message, username string) bool {
	username = strings.TrimSpace(username)
	if username == "" {
		return false
	}
	pattern := regexp.MustCompile(`(?i)@` + regexp.QuoteMeta(username) + `\b`)
	return pattern.MatchString(message)
}
