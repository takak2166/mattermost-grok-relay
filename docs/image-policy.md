# Container image policy (Harbor)

This document defines how **mattermost-grok-relay** container images are published to the private Harbor registry and how Kubernetes (Rancher Fleet) pulls them. It is the reference for follow-up work: **CD** (push on `main` merge) and **Fleet** (deploy manifests in `my-k8s-platform`).

Operational values (registry hostname, passwords, private-network IP/DNS) **must not** appear in this public repository. Only secret **names**, variable **names**, and logical resource names are documented here.

## Summary

| Topic | Decision |
| --- | --- |
| Harbor project / repository | `${HARBOR_REGISTRY}/library/mattermost-grok-relay` |
| Primary tag (production) | Git commit SHA (`github.sha`, full 40 characters) |
| CI push credentials | GitHub Actions **repository secrets** (see below) |
| Cluster pull credentials | `imagePullSecrets` → Kubernetes Secret `harbor-registry-secret` (SealedSecret in `my-k8s-platform`) |
| Fleet tag updates | **GitOps**: explicit tag bump in `fleet.yaml` via PR in `my-k8s-platform` (no auto-deploy from Harbor alone) |

---

## Harbor project and repository name

### Adopted

- **Project:** `library` (Harbor project name)
- **Repository:** `mattermost-grok-relay`
- **Full image reference (logical):** `${HARBOR_REGISTRY}/library/mattermost-grok-relay:<tag>`

`HARBOR_REGISTRY` is the registry host (and optional port/path prefix) configured out-of-band—for example a GitHub Actions secret with the same name used in `my-k8s-platform`.

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Dedicated Harbor project `mattermost-grok-relay` | Clear RBAC boundary per app | New project + robot accounts to operate; unlike existing self-built images under `library/` |
| Push to Docker Hub / GHCR | No private registry setup | Conflicts with agreed private Harbor publish path; extra egress and credential surface |
| **`library/mattermost-grok-relay` (adopted)** | Matches existing pattern for self-built images (e.g. Bitnami PostgreSQL mirrors under `library/…` in `my-k8s-platform`) | Shares `library` with other images; RBAC is per-repository robot account |

### Open points

- Confirm Harbor **robot accounts**: CI **push** robot vs cluster/node **pull** robot (same or separate `robot$…` identities) and whether to reuse an org-wide robot.
- Confirm whether a **new Harbor repository** must be created manually before the first CI push, or is created on first push (Harbor setting-dependent).
- Enable Harbor **immutable tags** (or equivalent) on `library/mattermost-grok-relay` so a re-pushed SHA cannot change digest silently.

---

## Tag strategy

### Adopted

1. **Production / Fleet pin (required):** `<full-git-commit-sha>` from the commit built on `main` (GitHub Actions: `${{ github.sha }}`).
2. **Pull request validation (CI only, optional):** build and test locally in CI **without** pushing to Harbor (current `ci.yml` behavior until CD lands).
3. **Semver (optional, later):** when release tags `v*.*.*` are introduced, also push `:vX.Y.Z` pointing at the same digest as the SHA tag for human-readable rollback notes. Fleet should still pin **SHA** for reproducibility.
4. **Not used for Fleet:** floating tags such as `latest` or `main` as the sole deploy reference.

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Short SHA (7–12 chars) | Shorter manifests | Collision risk; harder to correlate with Git without normalization |
| CalVer / date tags | Readable ordering | Decoupled from Git; weaker traceability for a single-repo service |
| **`github.sha` full (adopted)** | Immutable, 1:1 with Git; standard for GitOps | Long strings in YAML |

### Build-time version label

The Dockerfile **references** `${VERSION}` in `-ldflags` but does not yet declare `ARG VERSION`; CD work must add that line so `--build-arg VERSION=…` is not ignored. CD should set `VERSION` to the same value as the image tag (the Git SHA or semver tag used for that build).

---

## CI: credentials and push (GitHub Actions)

Publishing runs in **this repository** on `main` (follow-up CD issue). Credentials are **not** committed; they live in GitHub.

### Adopted secret / variable names

Align with `my-k8s-platform` Harbor workflows where possible:

| Name | Storage | Purpose |
| --- | --- | --- |
| `HARBOR_REGISTRY` | Actions **secret** (or **variable** if non-sensitive host only) | Registry hostname for `docker login` and image name prefix |
| `HARBOR_USERNAME` | Actions **secret** | Harbor robot or user with **push** to `library/mattermost-grok-relay` |
| `HARBOR_PASSWORD` | Actions **secret** | Token/password for `HARBOR_USERNAME` |
| `HARBOR_INSECURE_SKIP_VERIFY` | Actions **variable** (optional) | Records intent when the registry uses a private CA/self-signed TLS (same idea as Terraform `harbor_insecure_skip_verify` in `my-k8s-infra`). The CD workflow must map this to runner/docker/buildkit TLS settings—the variable alone does not skip verification. |

Optional **repository variable** (non-secret):

| Name | Purpose |
| --- | --- |
| `HARBOR_REPOSITORY` | Default `library/mattermost-grok-relay`; override only if repository path changes |

### Push flow (target CD behavior)

1. Checkout at `${{ github.sha }}`.
2. `docker login` using the secrets above.
3. `docker build` with `--build-arg VERSION=${{ github.sha }}`.
4. Tag: `${HARBOR_REGISTRY}/library/mattermost-grok-relay:${{ github.sha }}`.
5. `docker push` that tag only (no overwrite of unrelated tags).

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Org-level secrets only | One place to rotate | This public repo may not inherit org secrets; repo-scoped is explicit |
| OIDC to Harbor | No long-lived password in GitHub | Harbor/OIDC setup not documented in existing repos yet |
| **Repo secrets + shared names (adopted)** | Same names as `postgresql-image-sync.yml`; easy cross-repo mental model | Must ensure robot has push on this repository |

### Open points

- Which Harbor **robot account** receives push permission (`robot$…` name and project role).
- Whether CI runs on **GitHub-hosted** or **self-hosted** runners (self-hosted can reach a private registry without extra tunneling; affects CD workflow design only).

---

## Kubernetes: pull credentials

Runtime manifests live in **`my-k8s-platform`**, not in this repo (app code in this repository; Fleet overlays in `my-k8s-platform`).

### Adopted

- **Secret name:** `harbor-registry-secret` (type `kubernetes.io/dockerconfigjson`).
- **Pod spec:** `imagePullSecrets: [{ name: harbor-registry-secret }]`.
- **Git storage:** **SealedSecret** in the app overlay (same pattern as `apps/postgresql/overlays/dev/harbor-registry-secret-sealed.yaml`).
- **Credentials:** Same registry identity as other private images (robot with **pull** on `library/mattermost-grok-relay`), username/password **not** stored in `mattermost-grok-relay`.

Node-level **containerd** auth via `registries.yaml` (Terraform template in `my-k8s-infra`) complements pull secrets for some setups; Fleet workloads should still declare `imagePullSecrets` for explicit, namespace-scoped behavior.

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| External Secrets Operator | Central secret store | Not the default pattern in current `my-k8s-platform` overlays reviewed |
| One global Secret in `kube-public` | Single object | Weak namespace isolation; not used elsewhere |
| **SealedSecret per namespace (adopted)** | Matches PostgreSQL / Mattermost secrets; GitOps-safe | Re-seal when rotating registry password |

### Open points

- **Namespace** for the relay Deployment (dedicated `mattermost-grok-relay` vs shared with Mattermost stack)—decide in the Fleet layout issue.
- Whether to **reuse** the existing `harbor-registry-secret` SealedSecret blob in another namespace or generate a namespace-specific SealedSecret with the same logical credentials.

---

## Fleet: picking up new image tags

### Adopted

**Manual GitOps (tag update PR)** in `my-k8s-platform`:

1. After CD pushes `${HARBOR_REGISTRY}/library/mattermost-grok-relay:<sha>`, open a PR that updates the relay overlay (expected under `apps/mattermost-grok-relay/overlays/dev/`) **image tag** in the Deployment/manifest or Helm values referenced by `fleet.yaml`—not necessarily a field inside `fleet.yaml` itself—to that SHA.
2. Fleet syncs from Git; clusters pull the new digest when the PR merges to `main`.
3. **Renovate** in `my-k8s-platform` remains focused on upstream Helm charts; it is **not** the primary mechanism for this first-party image (unless a dedicated Renovate rule is added later).

**Not adopted:** automatic Fleet sync from Harbor webhooks or floating `latest` without a Git change—avoids silent drift and matches existing PostgreSQL Harbor flow (image sync script + explicit `fleet.yaml` / version files).

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Auto-update on every `main` push (bot PR) | Faster rollout | Extra automation; must gate on CI green and review |
| `fleet-comparepatch-suggest` only | Reconciles cluster drift | Solves Modified bundles, not “new image exists in Harbor” |
| **Human/PR tag bump (adopted)** | Clear audit trail; consistent with GitOps | Requires a deliberate step after CD |

### Open points

- Optional follow-up: GitHub Action in `my-k8s-platform` or dispatch from this repo’s CD to open a tag-bump PR (still Git-backed, but semi-automated).

---

## Related repositories

| Repository | Role |
| --- | --- |
| `mattermost-grok-relay` (this repo) | Source, CI build/test, future Harbor **push** |
| `my-k8s-platform` | Fleet manifests, SealedSecrets, **image tag** pins |
| `my-k8s-infra` | Node `registries.yaml`, Harbor domain variables for Terraform (`HARBOR_*` in infra workflows) |

---

## Document history

- Initial policy for Harbor publish/pull and Fleet tagging (documentation-only change in this repo).
