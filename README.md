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
| `key_env` | Name of an environment variable holding the Grok inbound webhook auth key |
| `key_file` | Path to a file containing the webhook auth key (e.g. mounted secret) |

Use exactly one of `url_env` or `url_file`, exactly one of `token_env` or `token_file`, and exactly one of `key_env` or `key_file`, per bot. Set `RELAY_CONFIG` to a file path, `RELAY_CONFIG_JSON` for inline JSON, or pass `-config path`.

**Required environment variables**

| Variable | Description |
| --- | --- |
| `RELAY_MATTERMOST_URL` | Mattermost site base URL (`https://…`, or `https://…/subpath` when served under a path). The relay uses the official Go client (`Client4` / `WebSocketClient4`) with this URL for REST and WebSocket. |
| `RELAY_ALLOWED_USER_IDS` | Comma-separated Mattermost user IDs allowed to trigger relay handling for **all** bots (whitespace around IDs is trimmed; empty comma-separated segments are ignored). Each ID must **exactly match** Mattermost’s 26-character alphanumeric user ID (case-sensitive). **Fail closed:** if this variable is unset, empty, or parses to no IDs, the process exits at startup with a configuration error and does not run in an “allow everyone” mode. Example: `7yjx8ntqhzn8zxmnc5k37wqg84,kq3m8nwp2hx7zc5b4j9f6v1drt`. |

### Sender allowlist (after channel filter)

Posts that pass the DM/group/channel filters above are checked against `RELAY_ALLOWED_USER_IDS`. Allowed senders are forwarded to that bot’s Grok inbound webhook (see below). Disallowed senders are not forwarded; the relay writes one structured `slog` info record with fixed message `relay post sender not allowed`, `fail_class=sender_not_allowed`, and fields: `bot_id`, sender user ID (`sender_user_id`), sender display name from the posted event (`sender_name`, when present — Mattermost typically sends `@username`), channel type (`D` / `G` / `O` / `P`), `channel_id`, `post_id`, and a `message_prefix` of the post body (first 100 runes, with `…` when truncated). Tokens, secrets, and URLs are not logged. For **denied** senders only, the first 100 runes of post text are **intentionally** recorded in `message_prefix` for operational audit; restrict access to log storage accordingly.

### Grok webhook forward (allowed senders)

For each allowed post, the relay POSTs JSON `{"meta":{...}}` to the bot’s configured webhook URL:

| `meta` field | Value |
| --- | --- |
| `platform` | `"mattermost"` |
| `channel_id` | Mattermost channel ID |
| `thread_id` | `root_id` when set, otherwise `post_id` (same rule for DMs) |
| `post_id` | Mattermost post ID |
| `user.id` | Sender user ID |
| `user.username` | Posted event `sender_name` when present |
| `text` | Full post message body |

Outbound HTTP uses a **10 second** timeout, **no retries**, and treats HTTP 2xx as success. Non-2xx responses and network errors are failures; the relay logs one fixed `slog` line per attempt (`grok webhook forward succeeded` or `grok webhook forward failed` with `fail_class`) and does **not** log the URL, auth key, or message body. The attempt result is available to later error-reply logic via `webhook.ForwardResult`.

**Webhook auth (provisional):** The header name/format for the inbound key is not finalized. The relay currently sets `Authorization: Bearer <key>` via `webhook.SetRequestAuth` so only that function needs to change when the format is decided.

### WebSocket receive and reconnect

- Each bot user in the mapping table gets its own WebSocket connection.
- **Direct messages** (`D`) and **group messages** (`G`) are accepted unless the post is from the bot itself (deleted and non-default system post types are skipped, same as before).
- **Open and private channels** (`O`, `P`) require both: the message body contains an explicit `@<bot username>` with username-boundary checks (Mattermost-style `[a-z0-9._-]`, no prefix false-positives; the username match is **case-insensitive**), and the posted event’s `mentions` field includes the bot’s user ID. Posts that only use `@channel`, `@here`, or `@all` are excluded even when `mentions` lists the bot, because `mentions` reflects server notification targets and cannot prove an explicit `@bot` in the text.
- Before each WebSocket session, the relay resolves the bot user via Mattermost REST (`GetMe`) using the caller context and a **30 second** default HTTP client timeout.
- WebSocket connect uses Mattermost `server/public` **v0.6.0** (`NewWebSocketClient4WithDialer`) with a custom gorilla/websocket **v1.5.3** dialer (**30 second** TCP connect and handshake timeout on the dial path). On shutdown or cancel, the **listener returns without waiting** for an in-flight dial; the dial may continue in the background until those SDK/gorilla limits are reached. A connection that finishes after cancel is closed by the dial goroutine.
- **Process exit:** graceful shutdown (`context.Canceled`, for example after SIGTERM) exits **0**; if every bot listener stops while the top-level context is still active, the process exits **1** and prints `relay stopped` on stderr.
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
