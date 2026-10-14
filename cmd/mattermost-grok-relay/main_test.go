package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMainVersionFlag(t *testing.T) {
	t.Parallel()

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", ".", "-version")
	cmd.Dir = repoRoot + "/cmd/mattermost-grok-relay"
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go run: %v", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected version output on stdout")
	}
}
