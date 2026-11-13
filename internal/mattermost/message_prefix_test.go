package mattermost

import (
	"strings"
	"testing"
)

func TestMessagePrefix(t *testing.T) {
	t.Parallel()
	const max = 100
	ascii := strings.Repeat("a", 120)
	got := MessagePrefix(ascii, max)
	if len([]rune(got)) != max+1 {
		t.Fatalf("runes = %d, want %d", len([]rune(got)), max+1)
	}
	if got[len(got)-3:] != "…" {
		t.Fatalf("suffix = %q", got[len(got)-3:])
	}

	multibyte := ""
	for range 101 {
		multibyte += "あ"
	}
	got = MessagePrefix(multibyte, max)
	if len([]rune(got)) != max+1 {
		t.Fatalf("multibyte runes = %d, want %d", len([]rune(got)), max+1)
	}

	short := "hello"
	if MessagePrefix(short, max) != short {
		t.Fatalf("short message changed: %q", MessagePrefix(short, max))
	}

	exact := strings.Repeat("b", max)
	if MessagePrefix(exact, max) != exact {
		t.Fatalf("exact length should not truncate")
	}
	if MessagePrefix("x", 0) != "" {
		t.Fatal("zero max should return empty")
	}
}
