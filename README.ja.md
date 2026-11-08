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
- **DM**（`channel_type` `D`）は、Bot 自身の投稿以外を受け付けます。
- **オープン・プライベート・グループ**（`O` / `P` / `G`）は、posted イベントの `mentions`（ユーザー ID の JSON 配列）に Bot のユーザー ID が含まれる場合のみ受け付けます。本文の `@` 文字列は見ません。接続ごとの `mentions` は、その接続ユーザーに対してサーバーが通知対象とするユーザー ID の集合です（@channel 展開、キーワード、スレッド参加者などを含み、本文に見える @ だけではありません）。**グループメッセージ**（`G`）では、公開ソースの Mattermost サーバー **v10.11.4**（タグ参照のみ。稼働中サーバーのバージョンは未検証）において、内部の通知対象に GM メンバー全員が入り得る一方、WebSocket の `mentions` ペイロードは、その接続ユーザーが対象のとき **その接続ユーザーの ID のみ**を載せる実装です。現行フィルタでは、GM メンバーである Bot は本文に明示的 `@bot` がなくても `mentions` に自身のユーザー ID が含まれる投稿を受け付け得ます。ここでは GM を広く受け入れることが製品要件として確定したわけではありません。
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
