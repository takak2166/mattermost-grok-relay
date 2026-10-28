# mattermost-grok-relay

**言語:** [English (README.md)](README.md) | [日本語](README.ja.md)

Mattermost と Grok Bot をつなぐ中継サービス（relay）の Go 実装リポジトリです。

本リポジトリは新規 Go プロジェクトとして一から構築します。ビジネスロジックは後続 Issue で追加します。

## レイヤ方針（土台）

- `cmd/` はエントリポイントのみ。ロジックは `internal/` に置く。
- 依存の向きは `cmd → internal` のみ（`internal` 同士は必要最小限）。
- ログは標準ライブラリの `log/slog` を使う方針（relay 本体実装時から適用）。
- Mattermost / Grok など外部 API 向けの境界パッケージは、機能追加時に `internal/` 配下で切る。

## 設定

Bot の対応表は JSON（ファイルまたは環境変数）で定義します。`bots` 配列に Mattermost Bot ユーザーごとに 1 要素追加するだけで Bot を増やせます。ファイル本体に秘密情報は書かず、git 管理できます。

| フィールド | 説明 |
| --- | --- |
| `id` | Mattermost Bot ユーザー識別子（重複不可） |
| `url_env` | Grok inbound webhook URL を入れた環境変数名 |
| `url_file` | webhook URL が書かれたファイルパス（Secret マウント等） |
| `token_env` | Bot トークンを入れた環境変数名 |
| `token_file` | Bot トークンが書かれたファイルパス（Secret マウント等） |

各 Bot では `url_env` / `url_file` のどちらか一方、`token_env` / `token_file` のどちらか一方を指定します。`RELAY_CONFIG`（ファイルパス）、`RELAY_CONFIG_JSON`（JSON 文字列）、または `-config` で読み込みます。

サンプル: `internal/config/testdata/example.json`

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
