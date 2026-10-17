# mattermost-grok-relay

**言語:** [English (README.md)](README.md) | [日本語](README.ja.md)

Mattermost と Grok Bot をつなぐ中継サービス（relay）の Go 実装リポジトリです。

本リポジトリは新規 Go プロジェクトとして一から構築します。ビジネスロジックは後続 Issue で追加します。

## レイヤ方針（土台）

- `cmd/` はエントリポイントのみ。ロジックは `internal/` に置く。
- 依存の向きは `cmd → internal` のみ（`internal` 同士は必要最小限）。
- ログは標準ライブラリの `log/slog` を使う方針（relay 本体実装時から適用）。
- Mattermost / Grok など外部 API 向けの境界パッケージは、機能追加時に `internal/` 配下で切る。

## 開発

```bash
go test ./...
go run ./cmd/mattermost-grok-relay -version
```

## Docker

```bash
docker build -t mattermost-grok-relay:local .
docker run --rm mattermost-grok-relay:local -version
```

現状のイメージは **`-version` でバージョン表示して終了するだけ** です（常駐プロセスにはなりません）。relay の常時接続ロジックは後続で入れ、デフォルト CMD もそのとき更新します。

## コンテナイメージ（Harbor）

レジストリのホスト名や資格情報の実値は本リポに置きません。Harbor のプロジェクト/リポジトリ名、タグ方針、CI の push 用 Secret、k8s の pull 用 Secret、Fleet のタグ更新の流れは [docs/image-policy.ja.md](docs/image-policy.ja.md)（英語: [docs/image-policy.md](docs/image-policy.md)）を参照してください。
