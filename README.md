# mattermost-grok-relay

**Languages:** [English](README.md) | [日本語 (README.ja.md)](README.ja.md)

Go implementation of a relay service between Mattermost and Grok Bot.

This repository is a greenfield Go project. Business logic will land in follow-up work.

## Layering (scaffold)

- `cmd/` holds entrypoints only; logic lives under `internal/`.
- Dependencies flow `cmd → internal` only (minimal cross-`internal` coupling).
- Logging will use the standard library `log/slog` once relay logic is implemented.
- Boundary packages for external APIs (Mattermost, Grok, and similar) will be added under `internal/` as features arrive.

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

## Container images (Harbor)

Registry hostnames and credentials are **not** stored in this repository. See [docs/image-policy.md](docs/image-policy.md) (Japanese: [docs/image-policy.ja.md](docs/image-policy.ja.md)) for Harbor image naming, tag strategy, CI push settings, and how deploy tags are updated (deploy manifests live in a separate private repository).
