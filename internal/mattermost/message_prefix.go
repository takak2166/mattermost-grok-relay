package mattermost

// MessagePrefix returns the first maxRunes runes of message for structured logging.
// When truncated, the prefix ends with "…" so operators can see clipping occurred.
func MessagePrefix(message string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(message)
	if len(runes) <= maxRunes {
		return message
	}
	return string(runes[:maxRunes]) + "…"
}
