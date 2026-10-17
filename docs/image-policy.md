# Container image policy (Harbor)

This document defines how **mattermost-grok-relay** container images are published to the private Harbor registry and how Kubernetes (Rancher Fleet) pulls them. It is the reference for follow-up work: **CD** (push on `main` merge) and **Fleet** (deploy manifests in `my-k8s-platform`).

Operational values (registry hostname, passwords, private-network IP/DNS, robot account names) **must not** appear in this public repository. Only secret **names**, variable **names**, and logical resource names are documented here.

## Summary

| Topic | Decision |
| --- | --- |
| Harbor project / repository | `${HARBOR_REGISTRY}/library/mattermost-grok-relay` (created on **first push**, not pre-provisioned) |
| Harbor tags | **Immutable tags** enabled on this repository (Harbor UI configuration, out of band) |
| Primary tag (production) | Git commit SHA on `main` (`github.sha`, full 40 characters) |
| Harbor identity | **Existing cluster robot account** for both **push** (CI) and **pull** (cluster); credentials via `HARBOR_USERNAME` / `HARBOR_PASSWORD` secrets only |
| CD runner | **Self-hosted** runner with a **dedicated label** (label name fixed in the CD implementation issue); repository stays **public** |
| CI push credentials | GitHub Actions **repository secrets** (see below) |
| Cluster pull credentials | Namespace `mattermost-grok-relay`, Secret `harbor-registry-secret` (new SealedSecret in `my-k8s-platform`, not reused from other namespaces) |
| Fleet tag updates | **Manual PR** in `my-k8s-platform` to bump the image tag to the new `main` SHA after push—**no** automatic Fleet pickup; automation is a later issue |

---

## Harbor project and repository name

### Adopted

- **Project:** `library` (Harbor project name)
- **Repository:** `mattermost-grok-relay`
- **Full image reference (logical):** `${HARBOR_REGISTRY}/library/mattermost-grok-relay:<tag>`
- **Repository lifecycle:** Do **not** create the Harbor repository ahead of time; the **first successful CI push** creates it (Harbor default behavior).
- **Tag immutability:** Enable **immutable tags** on `library/mattermost-grok-relay` in Harbor so a tag cannot be overwritten with a different digest (configured in Harbor outside this repo).

`HARBOR_REGISTRY` is the registry host (and optional port/path prefix) configured out-of-band—for example a GitHub Actions secret with the same name used in `my-k8s-platform`.

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Dedicated Harbor project `mattermost-grok-relay` | Clear RBAC boundary per app | New project + robot accounts to operate; unlike existing self-built images under `library/` |
| Push to Docker Hub / GHCR | No private registry setup | Conflicts with agreed private Harbor publish path; extra egress and credential surface |
| Pre-create Harbor repository | Explicit governance | Extra manual step; not required for first push |
| **`library/mattermost-grok-relay` (adopted)** | Matches existing pattern for self-built images (e.g. Bitnami PostgreSQL mirrors under `library/…` in `my-k8s-platform`) | Shares `library` with other images |

---

## Harbor robot account (push and pull)

### Adopted

- Use the **existing cluster robot account** already used for Harbor **pull** on the cluster—**do not** create a new robot for this app.
- The same credentials are passed to CI via GitHub Actions secrets **`HARBOR_USERNAME`** and **`HARBOR_PASSWORD`** (robot name and token values are **not** documented in this public repo).
- Ensure Harbor RBAC allows that robot to **push** to `library/mattermost-grok-relay` as well as **pull**.

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Separate CI push robot | Narrower push scope | Extra rotation and Harbor configuration |
| **Reuse cluster robot (adopted)** | One identity to rotate; aligns with node/`registries.yaml` and pull secrets | Push permission must be added to an existing account |

---

## Tag strategy

### Adopted

1. **Production / Fleet pin (required):** `<full-git-commit-sha>` from the commit built on `main` (GitHub Actions: `${{ github.sha }}`).
2. **Pull request CI:** build and test **without** pushing to Harbor (current `ci.yml`; CD push workflow does not run on `pull_request`).
3. **Semver (optional, later):** when release tags `v*.*.*` are introduced, also push `:vX.Y.Z` pointing at the same digest as the SHA tag for human-readable rollback notes. Fleet should still pin **SHA** for reproducibility.
4. **Not used for Fleet:** floating tags such as `latest` or `main` as the sole deploy reference.

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Short SHA (7–12 chars) | Shorter manifests | Collision risk; harder to correlate with Git without normalization |
| CalVer / date tags | Readable ordering | Decoupled from Git; weaker traceability for a single-repo service |
| **`github.sha` full (adopted)** | Immutable, 1:1 with Git; standard for GitOps | Long strings in YAML |

### Build-time version label (follow-up issue)

Adding **`ARG VERSION`** to the Dockerfile and wiring CD `--build-arg VERSION=${{ github.sha }}` is **out of scope for this policy** and will be handled in a **follow-up issue**. Until then, CD builds use the Dockerfile as-is (binary version may remain `0.0.0-dev` until that work lands).

---

## CI: credentials, runner, and push (GitHub Actions)

Harbor **push** runs in **this repository** (CD issue). Credentials are **not** committed; they live in GitHub.

### Adopted: workflow triggers and runner

- **When push runs:** only on **`push` to `main`** (after merge). The Harbor push workflow **must not** run on `pull_request`.
- **Runner:** **self-hosted** runner(s) that can reach the private Harbor registry (same class as other home infra workflows).
- **Runner label:** jobs use a **dedicated label** reserved for this repo’s CD; the **exact label string** is defined in the **CD implementation issue** (not in this public doc).
- **Public repo safety:** enable **approval for fork PR workflows** (or equivalent) so untrusted forks cannot consume self-hosted runners or secrets.
- **Pull request CI:** existing `ci.yml` continues to validate build/test on PRs without Harbor push.

### Adopted: secret / variable names

Align with `my-k8s-platform` Harbor workflows where possible:

| Name | Storage | Purpose |
| --- | --- | --- |
| `HARBOR_REGISTRY` | Actions **secret** (or **variable** if non-sensitive host only) | Registry hostname for `docker login` and image name prefix |
| `HARBOR_USERNAME` | Actions **secret** | Existing **cluster robot account** username (push + pull) |
| `HARBOR_PASSWORD` | Actions **secret** | Token/password for that robot |
| `HARBOR_INSECURE_SKIP_VERIFY` | Actions **variable** (optional) | Records intent when the registry uses a private CA/self-signed TLS (same idea as Terraform `harbor_insecure_skip_verify` in `my-k8s-infra`). The CD workflow must map this to runner/docker/buildkit TLS settings—the variable alone does not skip verification. |

Optional **repository variable** (non-secret):

| Name | Purpose |
| --- | --- |
| `HARBOR_REPOSITORY` | Default `library/mattermost-grok-relay`; override only if repository path changes |

### Push flow (target CD behavior)

1. Trigger: **`push` to `main`** on a self-hosted runner with the CD label.
2. Checkout at `${{ github.sha }}`.
3. `docker login` using the secrets above.
4. `docker build` (no `VERSION` build-arg until the follow-up Dockerfile issue).
5. Tag: `${HARBOR_REGISTRY}/library/mattermost-grok-relay:${{ github.sha }}`.
6. `docker push` that tag only (immutable tags in Harbor prevent silent overwrites).

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| GitHub-hosted runners | Simple setup | Often cannot reach a private home registry |
| Push on every PR | Early integration | Leaks build load; fork approval still needed on public repo |
| **Self-hosted + main-only push (adopted)** | Reliable registry access; minimal secret exposure on PRs | Runner ops and label discipline required |
| OIDC to Harbor | No long-lived password in GitHub | Not documented in existing repos yet |

---

## Kubernetes: namespace and pull credentials

Runtime manifests live in **`my-k8s-platform`**, not in this repo (app code in this repository; Fleet overlays in `my-k8s-platform`).

### Adopted

- **Namespace:** dedicated **`mattermost-grok-relay`** (not shared with the Mattermost stack namespace).
- **Secret name:** `harbor-registry-secret` (type `kubernetes.io/dockerconfigjson`).
- **Pod spec:** `imagePullSecrets: [{ name: harbor-registry-secret }]`.
- **Git storage:** a **new SealedSecret** under `apps/mattermost-grok-relay/overlays/dev/` in `my-k8s-platform`.
- **SealedSecret scope:** Sealed Secrets are **namespace-scoped (strict)**—do **not** reuse the SealedSecret blob from another namespace (e.g. PostgreSQL). Create **`harbor-registry-secret` again** for namespace `mattermost-grok-relay` using the **same existing cluster robot credentials** (`HARBOR_USERNAME` / `HARBOR_PASSWORD` values at seal time; not stored in this repo).

Node-level **containerd** auth via `registries.yaml` (Terraform template in `my-k8s-infra`) complements pull secrets for some setups; Fleet workloads should still declare `imagePullSecrets` for explicit, namespace-scoped behavior.

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Deploy in `mattermost` namespace | Fewer namespaces | Couples lifecycle with Mattermost instance |
| Reuse another namespace’s SealedSecret file | Copy-paste speed | **Invalid** under strict Sealed Secrets—ciphertext is namespace-bound |
| **Dedicated namespace + new SealedSecret (adopted)** | Clear isolation; correct Sealed Secrets usage | One-time seal step in Fleet issue |

---

## Fleet: picking up new image tags

### Adopted

**Manual GitOps only**—no automatic Fleet pickup from Harbor or from this repo’s CD.

1. After CD pushes `${HARBOR_REGISTRY}/library/mattermost-grok-relay:<sha>`, open a **manual PR** in `my-k8s-platform` that updates the relay overlay under `apps/mattermost-grok-relay/overlays/dev/` so the **image tag** (in the Deployment/manifest or Helm values referenced by `fleet.yaml`) equals that **`main` commit SHA**.
2. Fleet syncs from Git when the PR merges to `main`; clusters pull the new digest.
3. **Automation** (bot PR, dispatch workflow, etc.) is explicitly **deferred to a later issue**—the first rollouts use **hand-opened PRs**.

**Not adopted:** Harbor webhooks, Renovate-driven first-party image bumps, or floating `latest` without a Git change.

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Auto-update on every `main` push (bot PR) | Faster rollout | Deferred by decision; needs guards and review |
| `fleet-comparepatch-suggest` only | Reconciles cluster drift | Does not detect “new image in Harbor” |
| **Manual tag-bump PR (adopted)** | Clear audit trail; matches GitOps | Extra step after each CD push until automation exists |

---

## Related repositories

| Repository | Role |
| --- | --- |
| `mattermost-grok-relay` (this repo) | Source, CI build/test, Harbor **push** (CD) |
| `my-k8s-platform` | Fleet overlays under `apps/mattermost-grok-relay/overlays/dev/`, SealedSecrets, **image tag** pins |
| `my-k8s-infra` | Node `registries.yaml`, Harbor domain variables for Terraform (`HARBOR_*` in infra workflows) |

---

## Document history

- Initial policy for Harbor publish/pull and Fleet tagging (documentation-only).
- Decisions confirmed: shared cluster robot, immutable tags, first-push repo creation, self-hosted main-only CD, namespace `mattermost-grok-relay`, new SealedSecret, manual Fleet tag PRs, Dockerfile `ARG VERSION` deferred.
