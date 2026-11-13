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
| `key_env` | Grok inbound webhook 認証キーを入れた環境変数名 |
| `key_file` | webhook 認証キーが書かれたファイルパス（Secret マウント等） |

各 Bot では `url_env` / `url_file`、`token_env` / `token_file`、`key_env` / `key_file` それぞれ**ちょうど一方**を指定します。`RELAY_CONFIG`（ファイルパス）、`RELAY_CONFIG_JSON`（JSON 文字列）、または `-config` で読み込みます。

**必須環境変数**

| 変数 | 説明 |
| --- | --- |
| `RELAY_MATTERMOST_URL` | Mattermost サイトのベース URL（`https://…`。サブパス配下の場合は `https://…/subpath`）。公式 Go クライアント（`Client4` / `WebSocketClient4`）が REST と WebSocket の両方にこの URL を使います。 |
| `RELAY_ALLOWED_USER_IDS` | 中継の後段処理を許可する Mattermost ユーザー ID のカンマ区切り一覧（**全 Bot 共通**）。前後の空白は除去し、空要素は無視します。ID は Mattermost の **26 文字の英数字**と**完全一致**（大文字小文字を区別）で比較します。**フェイルクローズ:** 未設定・空文字・パース結果が ID ゼロ件のときは起動時に設定エラーで終了し、誰も通さない状態で動かしません。例: `7yjx8ntqhzn8zxmnc5k37wqg84,kq3m8nwp2hx7zc5b4j9f6v1drt`。 |

### 送信者許可（チャンネル種別フィルタ通過後）

上記の DM / グループ / チャンネル向けフィルタを通過した投稿について、送信者のユーザー ID が `RELAY_ALLOWED_USER_IDS` に含まれる場合のみ、その Bot の Grok inbound webhook へ JSON を POST します（下記）。許可外の送信者は転送せず、固定メッセージ `relay post sender not allowed`、`fail_class=sender_not_allowed` の `slog` info を 1 件だけ記録します。項目は `bot_id`、送信者ユーザー ID（`sender_user_id`）、投稿イベントの表示名（`sender_name`、取れる範囲。Mattermost では通常 `@username` 形式）、チャンネル種別（`D` / `G` / `O` / `P`）、`channel_id`、`post_id`、本文先頭 100 ルーンの `message_prefix`（切り詰め時は `…` 付き）です。トークン・秘密・URL はログに含めません。許可外送信者に限り、運用監査のため投稿本文先頭（最大 100 ルーン）を `message_prefix` として**意図的に**記録します。ログ基盤のアクセスは運用側で制限してください。

### Grok webhook 転送（許可された送信者）

許可された投稿ごとに、Bot 設定の webhook URL へ `{"meta":{...}}` を POST します。

| `meta` フィールド | 内容 |
| --- | --- |
| `platform` | `"mattermost"` |
| `channel_id` | Mattermost チャンネル ID |
| `thread_id` | `root_id` があればそれ、なければ `post_id`（DM も同じ） |
| `post_id` | Mattermost 投稿 ID |
| `user.id` | 送信者ユーザー ID |
| `user.username` | 投稿イベントの `sender_name`（取れる範囲） |
| `text` | 投稿本文全体 |

HTTP は **10 秒**タイムアウト、**再送なし**。2xx を成功とし、それ以外のステータスとネットワークエラーは失敗です。試行ごとに固定の `slog` 1 行（`grok webhook forward succeeded` / `grok webhook forward failed` と `fail_class`）を出し、URL・認証キー・本文はログに含めません。結果は `webhook.ForwardResult` として後続のエラー返信処理から参照できます。

**Webhook 認証（暫定）:** inbound キーのヘッダー形式は未確定です。現状は `webhook.SetRequestAuth` で `Authorization: Bearer <key>` を付与しており、形式が決まったらこの関数だけ差し替えます。

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
