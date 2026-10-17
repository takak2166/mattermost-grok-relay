# コンテナイメージ方針（Harbor）

**mattermost-grok-relay** のコンテナイメージを私有 Harbor レジストリに公開し、Kubernetes（Rancher Fleet）から pull するための方針です。後続の **CD**（`main` マージ時の push）と **Fleet**（`my-k8s-platform` の配置）Issue から参照してください。

レジストリのホスト名・パスワード・私有ネットワークの IP/DNS などの**実値はこの公開リポに書きません**。Secret 名・変数名・論理リソース名のみを記載します。

## 一覧

| 項目 | 決定内容 |
| --- | --- |
| Harbor プロジェクト / リポジトリ | `${HARBOR_REGISTRY}/library/mattermost-grok-relay` |
| 本番で pin するタグ | Git コミット SHA（`github.sha`、40 文字） |
| CI の push 資格情報 | GitHub Actions **リポジトリ secrets**（下表） |
| クラスタの pull 資格情報 | `imagePullSecrets` → Secret 名 `harbor-registry-secret`（`my-k8s-platform` で SealedSecret 管理） |
| Fleet の新タグ取り込み | **GitOps**: `my-k8s-platform` の `fleet.yaml` 等でタグを PR 更新（Harbor だけでは自動デプロイしない） |

---

## Harbor プロジェクト / リポジトリ名

### 採用案

- **プロジェクト:** `library`
- **リポジトリ:** `mattermost-grok-relay`
- **イメージ参照（論理）:** `${HARBOR_REGISTRY}/library/mattermost-grok-relay:<tag>`

`HARBOR_REGISTRY` は GitHub Actions の secret 等で外側から渡すレジストリホスト名です（`my-k8s-platform` の Harbor 連携と同じ名前を推奨）。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| Harbor プロジェクト `mattermost-grok-relay` を新設 | アプリ単位の RBAC | 運用コスト増。既存のセルフビルドイメージは `library/…` 配下 |
| Docker Hub / GHCR | 設定が簡単 | 私有 Harbor 公開の前提と不一致。外向き egress と資格情報管理が増える |
| **`library/mattermost-grok-relay`（採用）** | `my-k8s-platform` の PostgreSQL 系 self-host イメージと同型 | `library` 共有のため RBAC は robot 単位で切る |

### 要確認

- Harbor **robot アカウント**: CI **push** 用とクラスタ/ノード **pull** 用（同一か別の `robot$…` か）、既存 org 共通 robot の流用可否。
- 初回 push 前に Harbor 側でリポジトリ作成が必要か（Harbor 設定依存）。
- `library/mattermost-grok-relay` で **immutable tag**（または同等）を有効にし、同一 SHA タグの再 push で digest が silently 変わらないようにする。

---

## タグ方針

### 採用案

1. **本番 / Fleet で pin するタグ:** `main` でビルドしたコミットの **フル SHA**（`${{ github.sha }}`）。
2. **PR 時:** Harbor には push せず、CI で build/test のみ（CD 実装までは現行 `ci.yml` と同様）。
3. **semver（任意・将来）:** Git タグ `v*.*.*` をリリースしたとき、`:vX.Y.Z` も同一 digest に付与可能（人が読みやすいロールバックメモ用）。Fleet が参照するのは引き続き **SHA** を推奨。
4. **Fleet では使わない:** `latest` や `main` だけに依存する運用。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| 短縮 SHA | YAML が短い | 衝突・Git 対応の曖昧さ |
| CalVer | 見やすい | Git との 1:1 が弱い |
| **フル SHA（採用）** | 不変・トレース容易 | マニフェストが長い |

### ビルド時バージョン

Dockerfile は `-ldflags` 内で `${VERSION}` を参照しますが、現状 `ARG VERSION` がありません。CD 実装時に `ARG VERSION` を追加し、`--build-arg VERSION=…` が無視されないようにします。値はイメージタグ（SHA または semver）と揃えます。

---

## CI: Harbor への push 資格情報

CD は**このリポ**の `main` 向けに実装予定（別 Issue）。資格情報は Git に置きません。

### 採用する Secret / Variable 名

`my-k8s-platform`（例: `postgresql-image-sync.yml`）と揃えます。

| 名前 | 置き場所 | 用途 |
| --- | --- | --- |
| `HARBOR_REGISTRY` | Actions secret（またはホストのみなら variable） | `docker login` とイメージ名の prefix |
| `HARBOR_USERNAME` | Actions secret | `library/mattermost-grok-relay` への **push** 権限を持つ robot / ユーザー |
| `HARBOR_PASSWORD` | Actions secret | 上記のトークン |
| `HARBOR_INSECURE_SKIP_VERIFY` | Actions variable（任意） | 私有 CA / 自己署名 TLS 時の意図を記録（`my-k8s-infra` の Terraform 変数 `harbor_insecure_skip_verify` と同趣旨）。CD 側で runner/docker/buildkit の TLS 設定に反映する必要があり、変数だけでは検証はスキップされない |

任意の **repository variable**:

| 名前 | 用途 |
| --- | --- |
| `HARBOR_REPOSITORY` | 既定 `library/mattermost-grok-relay`。リポジトリパスを変えるときだけ上書き |

### push 手順（CD の想定）

1. `${{ github.sha }}` を checkout。
2. 上記 secret で `docker login`。
3. `--build-arg VERSION=${{ github.sha }}` で build。
4. `${HARBOR_REGISTRY}/library/mattermost-grok-relay:${{ github.sha }}` で tag。
5. そのタグのみを push する（無関係な既存タグは上書きしない）。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| Org secrets のみ | ローテーションが一箇所 | 公開リポの可視性・継承が不明確 |
| Harbor OIDC | 長期パスワード不要 | 既存 repos に未整備 |
| **リポ secrets + 共通名（採用）** | 他リポと名前が一致 | robot の push 権限設定が必要 |

### 要確認

- push 用 robot アカウント名と Harbor ロール。
- CI runner が **GitHub-hosted** か **self-hosted** か（私有レジストリへの到達性に影響）。

---

## Kubernetes: pull 資格情報

マニフェストは **`my-k8s-platform`** 側（アプリコードは本リポ、Fleet overlay は `my-k8s-platform`）。

### 採用案

- **Secret 名:** `harbor-registry-secret`（`kubernetes.io/dockerconfigjson`）。
- **Pod:** `imagePullSecrets: [{ name: harbor-registry-secret }]`。
- **Git:** overlay 配下の **SealedSecret**（`apps/postgresql/overlays/dev/harbor-registry-secret-sealed.yaml` と同型）。
- **中身:** レジストリ pull 用 robot のユーザー/パスワード（実値は SealedSecret 生成時のみ。公開リポには載せない）。

`my-k8s-infra` の `registries.yaml`（ノード containerd 設定）は pull を補助し得ますが、Fleet 管理ワークロードでは **imagePullSecrets を明示**する方針とします。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| External Secrets | 集中管理 | 現行 overlay の主パターンではない |
| クラスタ共通 Secret 1 つ | オブジェクトが少ない | 名前空間分離が弱い |
| **名前空間ごと SealedSecret（採用）** | 既存 Mattermost/PostgreSQL と一致 | ローテ時に再 seal |

### 要確認

- relay Deployment の **Namespace**（専用 `mattermost-grok-relay` か Mattermost 近傍か）。
- 既存 SealedSecret を流用するか、Namespace 用に再生成するか。

---

## Fleet: 新しいタグの取り込み

### 採用案

**GitOps（タグ更新 PR）**

1. CD が `${HARBOR_REGISTRY}/library/mattermost-grok-relay:<sha>` を push。
2. `my-k8s-platform` で relay overlay（想定: `apps/mattermost-grok-relay/overlays/dev/` 配下）の **`fleet.yaml` が参照する Deployment/マニフェストまたは Helm values 内のイメージタグ**を `<sha>` に更新する PR を作成・マージ（タグは必ずしも `fleet.yaml` 本体のフィールドではない）。
3. Fleet が Git を同期し、クラスタが新イメージを pull。

PostgreSQL の Harbor 連携と同様、**Harbor にタグが増えただけでは Fleet は動かさない**。Renovate は主に upstream Helm 向けで、本リポの first-party イメージの主更新経路にはしない（将来ルール追加は可）。

### 検討した他案

| 案 | メリット | デメリット |
| --- | --- | --- |
| `main` 毎の bot PR | 反映が速い | レビュー・ガードが必要 |
| `fleet-comparepatch-suggest` のみ | クラスタ drift 補正 | 新イメージ検知用途ではない |
| **手動/PR で tag bump（採用）** | 監査可能・GitOps と一致 | CD のあと 1 ステップ必要 |

### 要確認

- CD 成功後に tag bump PR を半自動化する Action を入れるか。

---

## 関連リポジトリ

| リポジトリ | 役割 |
| --- | --- |
| `mattermost-grok-relay`（本リポ） | ソース、CI、将来 Harbor **push** |
| `my-k8s-platform` | Fleet、SealedSecret、**イメージタグ pin** |
| `my-k8s-infra` | ノード `registries.yaml`、Terraform 用 `HARBOR_*` |

---

## 更新履歴

- Harbor 公開/pull/Fleet タグ方針の初版（本リポはドキュメントのみ）。
