# Container image policy (Harbor)

This document defines how **mattermost-grok-relay** container images are published to a private Harbor registry and how they are deployed to Kubernetes. Follow-up work in this repository covers **CD** (push on `main` merge); deploy manifests and cluster pull credentials live in a **separate private repository** (not named here).

Registry hostnames, passwords, private-network addresses, and robot account names **must not** appear in this public repository. This document lists only GitHub Actions secret/variable **names** and decisions that belong in this repo.

## Summary

| Topic | Decision |
| --- | --- |
| Image path (Harbor) | `${HARBOR_REGISTRY}/${HARBOR_REPOSITORY}` — default `HARBOR_REPOSITORY=library/mattermost-grok-relay`; repository is created on **first push**, not pre-provisioned |
| Tags | **Immutable tags** enabled for this image (Harbor configuration, out of band) |
| Production tag | Full Git commit SHA on `main` (`github.sha`, 40 characters) |
| Robot account | **Existing robot account** used for both Harbor **push** (CI) and **pull** (cluster); values only in `HARBOR_USERNAME` / `HARBOR_PASSWORD` |
| CD | **Self-hosted** GitHub Actions runner, **main `push` only** (not on `pull_request`); dedicated runner **label** (defined in the CD implementation issue); **public** repo with **fork workflow approval** |
| Pull at runtime | Dedicated Kubernetes namespace **`mattermost-grok-relay`**; pods declare **`imagePullSecrets`** pointing at a pull credential Secret managed in the private deploy repo |
| Deploy tag updates | After push, **manual pull request** in the private deploy repo to set the workload image tag to the new **`main` SHA** — no automatic deploy pickup; automation deferred |

---

## Harbor project and repository

### Adopted

- **Default repository path:** `library/mattermost-grok-relay` (via `HARBOR_REPOSITORY`).
- **Full reference:** `${HARBOR_REGISTRY}/${HARBOR_REPOSITORY}:<tag>`.
- **First push creates the repository**; do not pre-create it in Harbor.
- **Immutable tags** for this image so a tag cannot be replaced with a different digest (configured in Harbor outside this repo).

`HARBOR_REGISTRY` is **hostname only** (optional port, **no URL path**). Use it for `docker login`. Project/repository path belongs in **`HARBOR_REPOSITORY`**.

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Dedicated Harbor project for this app | Stronger RBAC boundary | Extra Harbor projects and robots |
| Docker Hub / GHCR | Simple public hosting | Conflicts with private Harbor publish path |
| Pre-create Harbor repository | Explicit catalog | Unnecessary if first push creates the repo |
| **`library/mattermost-grok-relay` (adopted)** | Simple default for a single app image | Shares Harbor `library` project with other images |

---

## Robot account (push and pull)

### Adopted

- **Do not** create a new Harbor robot for this app.
- Reuse the **existing robot account** already used for registry **pull** in the cluster, and grant it **push** to `library/mattermost-grok-relay`.
- CI reads **`HARBOR_USERNAME`** and **`HARBOR_PASSWORD`** from GitHub Actions secrets (no robot name or token in this repo).

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Separate push-only robot | Narrower push scope | More credentials to rotate |
| **Reuse existing robot (adopted)** | One identity | Push permission must be added to that account |

---

## Tag strategy

### Adopted

1. **Deploy pin:** full **`main` commit SHA** (`${{ github.sha }}`).
2. **Pull requests:** build/test in CI **without** Harbor push (CD workflow does not run on `pull_request`).
3. **Optional later:** semver Git tags may also tag the same digest; deploy pins should still prefer **SHA**.
4. **Not for deploy pins:** `latest` or `main` alone.

### Build-time version (follow-up issue)

Adding Dockerfile **`ARG VERSION`** and CD `--build-arg VERSION=…` is **deferred** to a follow-up issue. Until then, CD may build without that build-arg.

---

## CI: push (GitHub Actions, this repository)

### Adopted

- **Trigger:** `push` to **`main`** only.
- **Runner:** **self-hosted**, with a **dedicated label** (exact label in the CD issue; it will appear in public workflow YAML). The label must not encode internal hostnames, cluster names, or topology.
- **Fork safety:** require approval for workflows from forks on this public repository.
- **PR CI:** existing tests/build without registry push.

| Name | Storage | Purpose |
| --- | --- | --- |
| `HARBOR_REGISTRY` | Secret or variable | Registry **host** for `docker login` (no path) |
| `HARBOR_USERNAME` | Secret | Robot username for push |
| `HARBOR_PASSWORD` | Secret | Robot token/password |
| `HARBOR_INSECURE_SKIP_VERIFY` | Variable (optional) | Documents TLS verify skip **intent**; CD must configure runner/docker/buildkit accordingly |
| `HARBOR_REPOSITORY` | Variable (optional) | Default `library/mattermost-grok-relay`; if changed, tag/push and deploy manifests must use `${HARBOR_REGISTRY}/${HARBOR_REPOSITORY}:<tag>` |

### Push flow (target CD)

1. Self-hosted runner with CD label; checkout `${{ github.sha }}`.
2. `docker login "${HARBOR_REGISTRY}"`.
3. `docker build` (no `VERSION` build-arg until follow-up issue).
4. Tag and push `${HARBOR_REGISTRY}/${HARBOR_REPOSITORY}:${{ github.sha }}` only.

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| GitHub-hosted runners | No runner ops | Often cannot reach a private registry |
| Push on every PR | Early signal | Wasted pushes; fork risk on public repo |
| **Self-hosted, main-only (adopted)** | Reachability and least exposure on PRs | Runner maintenance |

---

## Kubernetes deploy and pull (private repository)

Application **source and CD** live in **this** public repository. **GitOps manifests** and **pull credential Secrets** are maintained in a **private deploy repository** (details not documented here).

### Adopted (decisions referenced by both repos)

- **Namespace:** `mattermost-grok-relay`.
- **Pull auth:** workload pods set **`imagePullSecrets`** to reference a pull credential Secret created/managed in the private deploy repo (same robot credentials as CI, applied out of band).
- **No automatic deploy sync** when a new image tag appears in Harbor.

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Share another app’s namespace | Fewer namespaces | Coupled lifecycle |
| **Dedicated namespace (adopted)** | Clear boundary for the relay |

---

## Updating the deployed image tag

### Adopted

1. CD pushes `${HARBOR_REGISTRY}/${HARBOR_REPOSITORY}:<sha>`.
2. Open a **manual pull request** in the **private deploy repository** to set the relay workload image tag to that **`<sha>`**.
3. After merge, GitOps sync pulls the new image.
4. **Automation** (bot PR, dispatch) is a **later issue**; first rollouts are manual PRs.

### Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| Auto PR on every `main` push | Faster rollout | Deferred; needs review gates |
| Harbor webhooks without Git | Fast | Silent drift, no audit trail |
| **Manual deploy-repo PR (adopted)** | Auditable GitOps | Extra step after each CD push |

---

## Scope split

| Location | Responsibility |
| --- | --- |
| **This repository (public)** | Source, PR CI, Harbor **push** workflow (CD), this policy document |
| **Private deploy repository** | GitOps manifests, pull credential Secrets, image tag pins for the relay |
