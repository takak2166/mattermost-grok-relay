package mattermost

import "testing"

func TestMessageMentionsUsernameBoundaries(t *testing.T) {
	t.Parallel()
	const bot = "relaybot"

	tests := []struct {
		msg  string
		want bool
	}{
		{"hi @relaybot", true},
		{"@relaybot: hello", true},
		{"(@relaybot)", true},
		{"@relaybot_user", false},
		{"x@relaybot", false},
		{"foo.@relaybot", true},
		{"foo-@relaybot", true},
		{"@relaybotlong", false},
		{"@channel", false},
		{"@channel @relaybot", true},
		{"", false},
	}
	for _, tt := range tests {
		got := messageMentionsUsername(tt.msg, bot)
		if got != tt.want {
			t.Fatalf("messageMentionsUsername(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}
