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

**必須環境変数:** `RELAY_MATTERMOST_URL` — Mattermost サイトのベース URL（`https://…`。サブパス配下の場合は `https://…/subpath`）。公式 Go クライアント（`Client4` / `WebSocketClient4`）が REST と WebSocket の両方にこの URL を使います。

### WebSocket 受信と再接続

- 対応表の Bot ユーザーごとに独立した WebSocket 接続を張ります。
- **DM**（`D`）と **グループメッセージ**（`G`）は、Bot 自身の投稿以外を受け付けます（削除済み・非 default のシステム投稿種別は従来どおり除外）。
- **オープン・プライベート**（`O` / `P`）は、本文に `@<Bot ユーザー名>` が明示されていること（ユーザー名文字と境界により部分一致を防ぐ。**ユーザー名の比較は大文字小文字を区別しません**）と、posted イベントの `mentions` に Bot のユーザー ID が含まれることの **両方**が必要です。`@channel` / `@here` / `@all` だけの投稿は、`mentions` に Bot が載っていても除外します（`mentions` はサーバー通知対象であり、本文の明示 @ を代替できないため）。
- 各 WebSocket 接続の前に、Mattermost REST（`GetMe`）で Bot ユーザーを解決します。呼び出し元の **context** を伝播し、HTTP クライアントの既定タイムアウトは **30 秒**です。
- WebSocket 接続は Mattermost `server/public` **v0.6.0**（`NewWebSocketClient4WithDialer`）と gorilla/websocket **v1.5.3** のカスタム Dialer（TCP／ハンドシェイクとも **30 秒**上限）を使います。停止・キャンセル時は **リスナーは進行中のダイアル完了を待たず** `ctx.Err()` で戻ります。ダイアル goroutine は SDK／gorilla の上限までバックグラウンドで続く場合があります。キャンセル後に成功した接続はダイアル側が `Close` します。
- **終了コード:** 正常な停止（`context.Canceled`、例: SIGTERM）は **0**。**全 Bot リスナーが終了した**のにプロセス用 context がまだ有効な場合は **1** で終了し、標準エラーに `relay stopped` を出します。
- 切断や ping タイムアウト後は指数バックオフで再接続します（初回 **1 秒**、失敗ごとに **2 倍**、上限 **60 秒**）。バックオフのリセットは、接続が **60 秒** 以上維持されたときだけ行います（短時間の切断ではリセットしません）。
- 公式 `WebSocketClient`（`server/public` **v0.6.0**）は、受信フレームの JSON デコード失敗時に Mattermost の **mlog** で警告を出す場合があります（グローバル mlog 未設定時は標準エラー出力）。relay の slog とは別経路です。この呼び出しはクライアントの URL や認証トークン用フィールドをログに載せませんが、`mlog.Err` は `error.Error()` をそのまま出すため、受信フレーム由来の JSON 診断文字列が含まれる可能性があります。

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

`-version` 以外では設定を読み込み、Bot ごとの WebSocket を維持する常駐 relay として動作します（停止シグナルで終了）。
