# mattermost-grok-relay

**Languages:** [English](README.md) | [日本語 (README.ja.md)](README.ja.md)

Go implementation of a relay service between Mattermost and Grok Bot.

This repository is a greenfield Go project. Business logic will land in follow-up work.

## Layering (scaffold)

- `cmd/` holds entrypoints only; logic lives under `internal/`.
- Dependencies flow `cmd → internal` only (minimal cross-`internal` coupling).
- Logging will use the standard library `log/slog` once relay logic is implemented.
- Boundary packages for external APIs (Mattermost, Grok, and similar) will be added under `internal/` as features arrive.

## Configuration

Bot routing is defined in JSON (file or inline env). Add one object under `bots` per Mattermost bot user; tokens are **not** stored in the file.

| Field | Description |
| --- | --- |
| `id` | Mattermost bot user identifier (unique) |
| `webhook_url` | Grok Bot inbound webhook URL for this bot |
| `token_env` | Name of an environment variable holding the bot token |
| `token_file` | Path to a file containing the bot token (e.g. mounted secret) |

Use exactly one of `token_env` or `token_file` per bot. Set `RELAY_CONFIG` to a file path, `RELAY_CONFIG_JSON` for inline JSON, or pass `-config path`.

See `internal/config/testdata/example.json` for a sample.

## Development

```bash
go test ./...
go run ./cmd/mattermost-grok-relay -version
```

## Docker

```bash
docker build -t mattermost-grok-relay:local .
docker run --rm mattermost-grok-relay:local -version
```

The image **prints the version with `-version` and exits**; it is not a long-running process yet. Always-on WebSocket relay logic and the default container command will change when that work lands.
