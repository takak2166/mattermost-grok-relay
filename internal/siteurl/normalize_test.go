package siteurl

import "testing"

func TestNormalizePreservesSubpath(t *testing.T) {
	t.Parallel()
	got, err := Normalize("https://fake.example.invalid/mm/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://fake.example.invalid/mm" {
		t.Fatalf("got %q", got)
	}
}
