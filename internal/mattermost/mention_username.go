package mattermost

import "strings"

// messageMentionsUsername reports whether message contains an at-mention of username
// using Mattermost username character rules ([a-z0-9._-]) and boundary checks so
// @bot does not match inside @botlonger. WebSocket data.mentions alone cannot tell
// whether the user typed an explicit @-handle (e.g. @channel still lists notify targets).
func messageMentionsUsername(message, username string) bool {
	if username == "" || message == "" {
		return false
	}
	needle := "@" + username
	searchFrom := 0
	for {
		i := strings.Index(message[searchFrom:], needle)
		if i < 0 {
			return false
		}
		start := searchFrom + i
		end := start + len(needle)
		if start > 0 && isMentionWordChar(rune(message[start-1])) {
			searchFrom = start + 1
			continue
		}
		if end < len(message) && isUsernameChar(rune(message[end])) {
			searchFrom = start + 1
			continue
		}
		return true
	}
}

// isMentionWordChar matches the \B boundary before @channel/@here in server/public post.go.
func isMentionWordChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
}

func isUsernameChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_'
}
