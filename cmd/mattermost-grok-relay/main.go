package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/takak2166/mattermost-grok-relay/internal/app"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mattermost-grok-relay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	if *showVersion {
		if _, err := fmt.Fprintf(stdout, "mattermost-grok-relay %s\n", app.Version); err != nil {
			return 1
		}
		return 0
	}
	if fs.NArg() > 0 {
		if _, err := fmt.Fprintf(stderr, "unknown arguments: %v\n", fs.Args()); err != nil {
			return 1
		}
		return 1
	}
	if _, err := fmt.Fprintln(stderr, "relay logic is not implemented yet; use -version to print the build version"); err != nil {
		return 1
	}
	return 1
}
