package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/takak2166/mattermost-grok-relay/internal/app"
	"github.com/takak2166/mattermost-grok-relay/internal/config"
	"github.com/takak2166/mattermost-grok-relay/internal/relay"
)

var startRelay = relay.Run

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr, os.LookupEnv))
}

func run(args []string, stdout, stderr io.Writer, lookupEnv func(string) (string, bool)) int {
	fs := flag.NewFlagSet("mattermost-grok-relay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version and exit")
	configPath := fs.String("config", "", "path to relay JSON config (overrides RELAY_CONFIG)")
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

	opts := config.LoadOptions{LookupEnv: lookupEnv}
	opts.ApplySources(*configPath)

	cfg, err := config.Load(opts)
	if err != nil {
		if _, werr := fmt.Fprintf(stderr, "config load failed: %s\n", err); werr != nil {
			return 1
		}
		return 1
	}

	siteURL, err := config.MattermostServerURL(lookupEnv)
	if err != nil {
		if _, werr := fmt.Fprintf(stderr, "config load failed: %s\n", err); werr != nil {
			return 1
		}
		return 1
	}

	allowed, err := config.AllowedUserIDs(lookupEnv)
	if err != nil {
		if _, werr := fmt.Fprintf(stderr, "config load failed: %s\n", err); werr != nil {
			return 1
		}
		return 1
	}

	if err := startRelay(context.Background(), siteURL, cfg, allowed); err != nil {
		if errors.Is(err, context.Canceled) {
			return 0
		}
		if _, werr := fmt.Fprintf(stderr, "relay stopped: %s\n", err); werr != nil {
			return 1
		}
		return 1
	}
	return 0
}
