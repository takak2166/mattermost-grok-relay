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

Bot routing is defined in JSON (file or inline env). Add one object under `bots` per Mattermost bot user. The file holds **no secrets** (safe to commit); webhook URLs and tokens are loaded from environment variables or mounted files.

| Field | Description |
| --- | --- |
| `id` | Mattermost bot user identifier (unique) |
| `url_env` | Name of an environment variable holding the Grok inbound webhook URL |
| `url_file` | Path to a file containing the webhook URL (e.g. mounted secret) |
| `token_env` | Name of an environment variable holding the bot token |
| `token_file` | Path to a file containing the bot token (e.g. mounted secret) |

Use exactly one of `url_env` or `url_file`, and exactly one of `token_env` or `token_file`, per bot. Set `RELAY_CONFIG` to a file path, `RELAY_CONFIG_JSON` for inline JSON, or pass `-config path`.

**Required environment variable:** `RELAY_MATTERMOST_URL` — Mattermost site base URL (`https://…`, or `https://…/subpath` when served under a path). The relay uses the official Go client (`Client4` / `WebSocketClient4`) with that URL for REST and WebSocket.

### WebSocket receive and reconnect

- Each bot user in the mapping table gets its own WebSocket connection.
- **Direct messages** (`channel_type` `D`) are accepted unless the post is from the bot itself.
- **Open, private, and group channels** (`O`, `P`, `G`) are accepted only when the posted event’s `mentions` field (JSON-encoded user ID array) includes the bot’s user ID. Message-body matching is not used. For each connection, `mentions` is the set of user IDs the server treats as notification targets for that connection (including @channel expansion, keywords, and thread engagement where applicable), not only visible `@` handles in the message text. In **group messages** (`G`), upstream Mattermost **v10.11.4** public server sources (tag only—not verified on your running server) add every GM member to internal notification targets, while the WebSocket `mentions` payload for a given connection lists **only that connection’s user ID** when that user is a target. With this relay’s current filter, a bot that is a GM member can therefore see posts whose `mentions` include the bot’s user ID even when the message body has no explicit `@bot`. That does **not** mean broad GM intake is an accepted product requirement here.
- On disconnect or ping timeout, the relay reconnects with exponential backoff: **1s** initial delay, multiply by **2** each failure, **60s** cap. Backoff resets only after a connection stayed up for **60 seconds**; brief flaps keep the current backoff level.
- The official `WebSocketClient` (`server/public` **v0.6.0**) may write JSON decode warnings via Mattermost **mlog** (default stderr when the global mlog logger is unset), separate from this relay’s slog output. That call does not log the client’s URL or auth token fields, but `mlog.Err` prints `error.Error()`, which can include JSON parse/type diagnostics that quote parts of a received WebSocket frame.

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

Without `-version`, the container runs the relay: it loads configuration and maintains bot WebSocket connections until it receives a stop signal.
