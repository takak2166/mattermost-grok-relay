# コンテナイメージ方針（Harbor）

**言語:** [English (image-policy.md)](image-policy.md) | [日本語](image-policy.ja.md)

**mattermost-grok-relay** のコンテナイメージを私有 Harbor に公開し、Kubernetes へデプロイする方針です。本リポの後続作業は **CD**（`main` マージ時の push）です。**GitOps マニフェストと pull 用 Secret** は**別の非公開リポジトリ**で管理します（名称・パスは本公開リポに書きません）。

レジストリのホスト名、パスワード、私有アドレス、robot 名などの**実値は公開リポに置きません**。GitHub Actions の secret/variable **名**と、本リポに属する決定事項だけを記載します。

## 一覧

| 項目 | 決定内容 |
| --- | --- |
| イメージパス（Harbor） | `${HARBOR_REGISTRY}/${HARBOR_REPOSITORY}` — 既定 `HARBOR_REPOSITORY=library/mattermost-grok-relay`。**初回 push でリポジトリ作成**（事前作成しない） |
| タグ | 当該イメージで **immutable tag** を有効（Harbor 側設定、リポ外） |
| 本番タグ | `main` の Git コミット **フル SHA**（`github.sha`、40 文字） |
| Robot | **既存の robot アカウント**を Harbor **push（CI）と pull（クラスタ）の両方**に使用。値は `HARBOR_USERNAME` / `HARBOR_PASSWORD` のみ |
| CD | **self-hosted** runner、**`main` への push のみ**（`pull_request` では動かさない）。専用 runner **ラベル**（CD 実装 Issue で確定）。**public** リポで **fork workflow 承認制** |
| 実行時 pull | Namespace **`mattermost-grok-relay`**。Pod は **`imagePullSecrets`** で非公開デプロイリポが管理する pull 用 Secret を参照 |
| デプロイタグ更新 | push 後、非公開デプロイリポで **手動 PR** によりワークロードのイメージタグを新しい **`main` SHA** に更新。**自動取り込みなし**（自動化は後続） |

---

## Harbor プロジェクト / リポジトリ

### 採用案

- **既定パス:** `library/mattermost-grok-relay`（`HARBOR_REPOSITORY`）。
- **参照形式:** `${HARBOR_REGISTRY}/${HARBOR_REPOSITORY}:<tag>`。
- **初回 push でリポジトリ作成**（Harbor 上で事前作成しない）。
- **immutable tag** により同一タグの digest 上書きを不可（Harbor 設定、本リポ外）。

`HARBOR_REGISTRY` は **ホスト名のみ**（任意 port、**パス不可**）。`docker login` に使う。プロジェクト/リポジトリパスは **`HARBOR_REPOSITORY`**。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| アプリ専用 Harbor プロジェクト | RBAC 分離 | プロジェクト・robot 増 |
| Docker Hub / GHCR | 公開ホスティングが簡単 | 私有 Harbor 方針と不一致 |
| 事前にリポジトリ作成 | カタログが明示的 | 初回 push で足りる |
| **`library/mattermost-grok-relay`（採用）** | 単一アプリ向けの単純な既定 | Harbor `library` 共有 |

---

## Robot アカウント（push / pull）

### 採用案

- 本アプリ用の **新規 robot は作らない**。
- クラスタで **pull に既に使っている既存 robot** に、`library/mattermost-grok-relay` への **push** 権限を付与する。
- CI は GitHub Actions の **`HARBOR_USERNAME`** / **`HARBOR_PASSWORD`** を参照（robot 名・トークンは本リポに書かない）。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| push 専用 robot | push 権限を最小化 | 資格情報が増える |
| **既存 robot 流用（採用）** | 資格情報が一つ | push 権限の付与が必要 |

---

## タグ方針

### 採用案

1. **デプロイ pin:** `main` コミットの **フル SHA**（`${{ github.sha }}`）。
2. **PR:** Harbor へは push しない（CD は `pull_request` 非起動）。
3. **将来任意:** semver Git タグで別名タグを付けてもよいが、デプロイ pin は **SHA** 推奨。
4. **デプロイ pin に使わない:** `latest` / `main` のみ。

### ビルド時バージョン（後続 Issue）

Dockerfile **`ARG VERSION`** と CD の build-arg 連携は**後続 Issue**。それまでは build-arg なし build 可。

---

## CI: push（GitHub Actions、本リポ）

### 採用案

- **トリガー:** **`main` への push** のみ。
- **runner:** **self-hosted**、CD 専用**ラベル**（CD Issue で確定。公開 workflow YAML に現れる）。ラベルに内部ホスト名・クラスタ名・トポロジを含めない。
- **fork 対策:** 本 public リポで fork 由来 workflow の**承認制**。
- **PR CI:** 既存の test/build のみ（レジストリ push なし）。

| 名前 | 置き場所 | 用途 |
| --- | --- | --- |
| `HARBOR_REGISTRY` | secret または variable | `docker login` 用**ホスト**（パス不可） |
| `HARBOR_USERNAME` | secret | push 用 robot ユーザー名 |
| `HARBOR_PASSWORD` | secret | robot トークン |
| `HARBOR_INSECURE_SKIP_VERIFY` | variable（任意） | TLS 検証スキップの**意図**を記録。CD で runner/docker/buildkit 側に反映。TLS 検証を無効にすると中間者攻撃のリスクが高まるため、信頼できる自宅ネットワーク上の**一時的**な措置に留め、正しい証明書に移行したら外す。 |
| `HARBOR_REPOSITORY` | variable（任意） | 既定 `library/mattermost-grok-relay`。変更時は tag/push とデプロイ manifest 双方で `${HARBOR_REGISTRY}/${HARBOR_REPOSITORY}:<tag>` |

### push 手順（CD 想定）

1. CD ラベル付き self-hosted で `${{ github.sha }}` を checkout。
2. `echo "${HARBOR_PASSWORD}" | docker login "${HARBOR_REGISTRY}" -u "${HARBOR_USERNAME}" --password-stdin`。
3. `docker build`（`VERSION` build-arg は後続 Issue まで省略可）。
4. `${HARBOR_REGISTRY}/${HARBOR_REPOSITORY}:${{ github.sha }}` のみ tag & push。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| GitHub-hosted | runner 運用が少ない | 私有レジストリに届きにくい |
| PR でも push | 早期検知 | 無駄 push・fork リスク |
| **self-hosted・main のみ（採用）** | 到達性と PR 時の露出最小 | runner 運用 |

---

## Kubernetes デプロイと pull（非公開リポ）

**ソースと CD** は**本公開リポ**。**GitOps マニフェストと pull 用 Secret** は**非公開デプロイリポ**（詳細は本リポに記載しない）。

### 採用案（両リポで共有する決定）

- **Namespace:** `mattermost-grok-relay`。
- **Pull:** Pod に **`imagePullSecrets`** を設定し、非公開リポで管理する pull 用 Secret を参照（CI と同じ robot 資格情報を、リポ外で適用）。
- Harbor に新タグが増えても**デプロイは自動追従しない**。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| 他アプリ Namespace と共有 | Namespace 数が少ない | ライフサイクル結合 |
| **専用 Namespace（採用）** | relay の境界が明確 | 専用 Namespace が一つ増える |

---

## デプロイ側イメージタグの更新

### 採用案

1. CD が `${HARBOR_REGISTRY}/${HARBOR_REPOSITORY}:<sha>` を push。
2. **非公開デプロイリポ**で **手動 PR** し、relay ワークロードのイメージタグを **`<sha>`** に更新。
3. マージ後 GitOps が同期しクラスタが pull。
4. bot PR 等の**自動化は後続 Issue**（最初は手動 PR）。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| `main` 毎の自動 PR | 反映が速い | 後回し。レビューゲートが必要 |
| Git を経由しない Harbor 連動 | 速い | 監査・再現性が弱い |
| **手動デプロイ PR（採用）** | 監査可能な GitOps | CD 後に手作業 |

---

## 責務の分担

| 場所 | 役割 |
| --- | --- |
| **本リポ（公開）** | ソース、PR 用 CI、Harbor **push**（CD）、本方針 |
| **非公開デプロイリポ** | GitOps manifest、pull 用 Secret、relay のイメージタグ pin |
