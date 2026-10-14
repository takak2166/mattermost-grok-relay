package main

import (
	"fmt"
	"os"

	"github.com/takak2166/mattermost-grok-relay/internal/app"
)

func main() {
	fmt.Printf("mattermost-grok-relay %s\n", app.Version)
	if len(os.Args) > 1 && os.Args[1] == "-version" {
		return
	}
	fmt.Fprintln(os.Stderr, "relay logic is not implemented yet; use -version to print the build version")
	os.Exit(1)
}
