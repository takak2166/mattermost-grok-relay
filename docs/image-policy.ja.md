# コンテナイメージ方針（Harbor）

**mattermost-grok-relay** のコンテナイメージを私有 Harbor レジストリに公開し、Kubernetes（Rancher Fleet）から pull するための方針です。後続の **CD**（`main` マージ時の push）と **Fleet**（`my-k8s-platform` の配置）Issue から参照してください。

レジストリのホスト名・パスワード・私有ネットワークの IP/DNS・robot アカウント名などの**実値はこの公開リポに書きません**。Secret 名・変数名・論理リソース名のみを記載します。

## 一覧

| 項目 | 決定内容 |
| --- | --- |
| Harbor プロジェクト / リポジトリ | `${HARBOR_REGISTRY}/library/mattermost-grok-relay`（**初回 push で作成**、事前作成しない） |
| Harbor タグ | イメージ向け **immutable tag**（Harbor プロジェクト `library` のルールで `mattermost-grok-relay` を対象。リポ外設定。初回 push 前後どちらでも可） |
| 本番で pin するタグ | `main` の Git コミット SHA（`github.sha`、40 文字） |
| Harbor 認証 | **既存のクラスタ用 robot アカウント**を **push（CI）と pull（クラスタ）の両方**に使用。資格情報は `HARBOR_USERNAME` / `HARBOR_PASSWORD` の Secret のみ |
| CD runner | **self-hosted**。専用**ラベル**付き（ラベル名は **CD 実装 Issue** で確定）。リポジトリは **public** のまま |
| CI の push 資格情報 | GitHub Actions **リポジトリ secrets**（下表） |
| クラスタの pull 資格情報 | Namespace `mattermost-grok-relay`、Secret `harbor-registry-secret`（`my-k8s-platform` に **新規** SealedSecret。他 Namespace から流用不可） |
| Fleet の新タグ取り込み | push 後 **`my-k8s-platform` で手動 PR** によりイメージタグを `main` の SHA に更新。**自動取り込みは使わない**。自動化は後続 Issue |

---

## Harbor プロジェクト / リポジトリ名

### 採用案

- **プロジェクト:** `library`
- **リポジトリ:** `mattermost-grok-relay`
- **イメージ参照（論理）:** `${HARBOR_REGISTRY}/library/mattermost-grok-relay:<tag>`
- **リポジトリ作成:** Harbor 上で**事前作成しない**。**初回の CI push 成功時**に作成される前提。
- **タグ immutability:** Harbor プロジェクト `library` 上のルールで `mattermost-grok-relay` を対象に **immutable tag** を有効にする（リポジトリ未作成でもプロジェクトルールは設定可能）。同一タグの digest 上書きは不可。Harbor UI の手順やホスト名は本リポに書かない。

`HARBOR_REGISTRY` は GitHub Actions の secret 等で外側から渡すレジストリホスト名（任意で port/path プレフィックスを含む）です。`my-k8s-platform` と同じ secret 名を使います。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| Harbor プロジェクト `mattermost-grok-relay` を新設 | アプリ単位の RBAC | 運用コスト増。既存 self-build は `library/…` |
| Docker Hub / GHCR | 設定が簡単 | 私有 Harbor 前提と不一致。外向き egress と資格情報管理が増える |
| 事前に Harbor リポジトリを作る | 明示的 | 手作業が増える。初回 push で足りる |
| **`library/mattermost-grok-relay`（採用）** | `my-k8s-platform` の Bitnami PostgreSQL ミラー等と同型（`library/…`） | `library` 共有 |

---

## Harbor robot アカウント（push / pull）

### 採用案

- **新規 robot は作らない**。**既存のクラスタ用 robot アカウント**（Harbor pull に既に使っているもの）を CI の **push** にも使う。
- 資格情報は GitHub Actions の **`HARBOR_USERNAME`** / **`HARBOR_PASSWORD`** で渡す（robot 名・トークンの実値は公開リポに書かない）。
- Harbor RBAC で、当該 robot に `library/mattermost-grok-relay` への **push** 権限があることを確認する。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| CI 専用 push robot | push 権限を最小化 | ローテーションと Harbor 設定が増える |
| **既存クラスタ robot を流用（採用）** | 身分が一つ。ノード設定・pull secret と整合 | push 権限の付与が必要 |

---

## タグ方針

### 採用案

1. **本番 / Fleet で pin するタグ:** `main` でビルドしたコミットの **フル SHA**（`${{ github.sha }}`）。
2. **PR 時:** Harbor には push しない（`pull_request` では push workflow を動かさない。検証は現行 `ci.yml` の build/test）。
3. **semver（任意・将来）:** Git タグ `v*.*.*` をリリースしたとき、`:vX.Y.Z` も同一 digest に付与可能（人が読みやすいロールバックメモ用）。Fleet が参照するのは引き続き **SHA**。
4. **Fleet では使わない:** `latest` や `main` だけに依存する運用。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| 短縮 SHA | YAML が短い | 衝突・Git 対応の曖昧さ |
| CalVer | 見やすい | Git との 1:1 が弱い |
| **フル SHA（採用）** | 不変・トレース容易 | マニフェストが長い |

### ビルド時バージョン（後続 Issue）

Dockerfile への **`ARG VERSION`** 追加と、CD の `--build-arg VERSION=${{ github.sha }}` 連携は**本 Issue の範囲外**で**後続 Issue**で行う。それまでは Dockerfile 現状のまま build し、バイナリ `-version` は `0.0.0-dev` のままになり得る。

---

## CI: 資格情報・runner・push（GitHub Actions）

Harbor **push** は**このリポ**で行う（CD Issue）。資格情報は Git に置きません。

### 採用案: トリガーと runner

- **push workflow が動く条件:** **`main` への `push` のみ**（マージ後）。**`pull_request` では動かさない**。
- **runner:** 私有 Harbor に到達できる **self-hosted** runner（自宅インフラの他 workflow と同種）。
- **ラベル:** 本リポ CD 専用の **runner ラベル**を付ける。**ラベル文字列は CD 実装 Issue で決める**（CD workflow の公開 YAML の `runs-on:` に現れる）。
- **public リポの安全策:** **fork 由来 workflow は承認制**（または同等）とし、信頼できない fork が self-hosted runner や secrets を使えないようにする。
- **PR CI:** 既存 `ci.yml` で build/test のみ（Harbor push なし）。

### 採用する Secret / Variable 名

`my-k8s-platform`（例: `postgresql-image-sync.yml`）と揃えます。

| 名前 | 置き場所 | 用途 |
| --- | --- | --- |
| `HARBOR_REGISTRY` | Actions secret（またはホストのみなら variable） | `docker login` とイメージ名の prefix |
| `HARBOR_USERNAME` | Actions secret | **既存クラスタ用 robot** のユーザー名（push + pull） |
| `HARBOR_PASSWORD` | Actions secret | 上記 robot のトークン |
| `HARBOR_INSECURE_SKIP_VERIFY` | Actions variable（任意） | 私有 CA / 自己署名 TLS 時の意図を記録（`my-k8s-infra` の Terraform 変数 `harbor_insecure_skip_verify` と同趣旨）。CD で runner/docker/buildkit の TLS 設定に反映する必要があり、変数だけでは検証はスキップされない |

任意の **repository variable**:

| 名前 | 用途 |
| --- | --- |
| `HARBOR_REPOSITORY` | 既定 `library/mattermost-grok-relay`。リポジトリパスを変えるときだけ上書き |

### push 手順（CD の想定）

1. トリガー: **self-hosted**（CD ラベル付き）で **`main` への push**。
2. `${{ github.sha }}` を checkout。
3. 上記 secret で `docker login`。
4. `docker build`（Dockerfile `ARG VERSION` 対応までは build-arg なし）。
5. `${HARBOR_REGISTRY}/library/mattermost-grok-relay:${{ github.sha }}` で tag。
6. そのタグのみ push（Harbor の immutable tag により同一タグの上書きは不可）。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| GitHub-hosted runner | セットアップが簡単 | 私有レジストリに届かないことが多い |
| PR でも push | 早期検証 | 負荷と fork リスク |
| **self-hosted + main のみ push（採用）** | 到達性と secrets 露出のバランス | runner 運用が必要。CD 実装後はラベルが公開 YAML に載る |
| Harbor OIDC | 長期パスワード不要 | 既存 repos に未整備 |

---

## Kubernetes: Namespace と pull 資格情報

マニフェストは **`my-k8s-platform`** 側（アプリコードは本リポ、Fleet overlay は `my-k8s-platform`）。

### 採用案

- **Namespace:** 専用 **`mattermost-grok-relay`**（Mattermost 本体と同じ Namespace には置かない）。
- **Secret 名:** `harbor-registry-secret`（`kubernetes.io/dockerconfigjson`）。
- **Pod:** `imagePullSecrets: [{ name: harbor-registry-secret }]`。
- **Git 上の置き場所:** `my-k8s-platform` の **`apps/mattermost-grok-relay/overlays/dev/`** に **新規 SealedSecret**。
- **SealedSecret:** Sealed Secrets は **Namespace 固定（strict）** のため、PostgreSQL 等の **SealedSecret ファイルは流用できない**。Namespace `mattermost-grok-relay` 向けに **`harbor-registry-secret` を作り直す**（中身は**既存クラスタ用 robot** と同じ資格情報を seal 時に使う。実値は公開リポに載せない）。

`my-k8s-infra` の `registries.yaml`（ノード containerd）は pull を補助し得ますが、Fleet 管理ワークロードでは **imagePullSecrets を明示**します。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| `mattermost` Namespace に同居 | Namespace が少ない | ライフサイクルが結合 |
| 他 Namespace の SealedSecret をコピー | 早い | **strict では無効**（暗号文が Namespace に紐付け） |
| **専用 Namespace + 新規 SealedSecret（採用）** | 分離と Sealed Secrets の正しい使い方 | Fleet Issue で seal 作業が必要 |

---

## Fleet: 新しいタグの取り込み

### 採用案

**手動 GitOps のみ**—Harbor や本リポ CD から Fleet を**自動では更新しない**。

1. CD が `${HARBOR_REGISTRY}/library/mattermost-grok-relay:<sha>` を push したあと、**手動で** `my-k8s-platform` に PR を出し、`apps/mattermost-grok-relay/overlays/dev/` 配下で **`fleet.yaml` が参照する manifest / Helm values のイメージタグ**を、その **`main` コミット SHA** に更新する。
2. PR が `main` にマージされると Fleet が Git を同期し、クラスタが新イメージを pull する。
3. **bot PR や dispatch による自動化**は**後続 Issue**に回す。最初の数回は**手動 PR**。

**採用しない:** Harbor webhook、first-party イメージ向け Renovate 自動 bump、`latest` だけの更新。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| `main` 毎の bot PR | 反映が速い | 今回の判断では後回し。ガード・レビュー・自動化設計が必要 |
| `fleet-comparepatch-suggest` のみ | drift 補正 | Harbor 新タグ検知には使わない |
| **手動 tag bump PR（採用）** | 監査可能・GitOps と一致 | CD のあと手作業（自動化前） |

---

## 関連リポジトリ

| リポジトリ | 役割 |
| --- | --- |
| `mattermost-grok-relay`（本リポ） | ソース、CI、Harbor **push**（CD） |
| `my-k8s-platform` | `apps/mattermost-grok-relay/overlays/dev/`、SealedSecret、**イメージタグ pin** |
| `my-k8s-infra` | ノード `registries.yaml`、Terraform 用 `HARBOR_*` |

---

## 更新履歴

- Harbor 公開/pull/Fleet タグ方針の初版（ドキュメントのみ）。
- 決定反映: クラスタ robot 共用、immutable tag、初回 push でリポ作成、self-hosted・main のみ CD、Namespace `mattermost-grok-relay`、SealedSecret 新規、Fleet は手動 PR、Dockerfile `ARG VERSION` は後続 Issue。
