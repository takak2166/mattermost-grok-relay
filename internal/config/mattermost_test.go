package config

import "testing"

func TestMattermostServerURL(t *testing.T) {
	t.Parallel()
	lookup := func(key string) (string, bool) {
		if key == "RELAY_MATTERMOST_URL" {
			return "https://mattermost.example.invalid/", true
		}
		return "", false
	}
	got, err := MattermostServerURL(lookup)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://mattermost.example.invalid"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMattermostServerURLSubpath(t *testing.T) {
	t.Parallel()
	lookup := func(key string) (string, bool) {
		if key == "RELAY_MATTERMOST_URL" {
			return "https://chat.example.invalid/mm/", true
		}
		return "", false
	}
	got, err := MattermostServerURL(lookup)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://chat.example.invalid/mm" {
		t.Fatalf("got %q", got)
	}
}

func TestMattermostServerURLMissing(t *testing.T) {
	t.Parallel()
	_, err := MattermostServerURL(func(string) (string, bool) { return "", false })
	if err == nil {
		t.Fatal("expected error")
	}
}
