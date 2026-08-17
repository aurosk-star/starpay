# Dual-Registry Container Image Publishing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build each StarPay container image once and publish the same multi-platform result to GHCR and Docker Hub with traceable tags and supply-chain attestations.

**Architecture:** A dedicated GitHub Actions workflow builds Pull Requests without credentials or pushes, then publishes default-branch and release-tag builds to both registries. A shell contract test locks down triggers, permissions, registries, tags, platforms, secret references, and action pinning; the existing CI deployment job runs that test.

**Tech Stack:** GitHub Actions, Docker Buildx/BuildKit, Docker metadata action, GHCR, Docker Hub, Bash

## Global Constraints

- GHCR image is `ghcr.io/aurosk-star/starpay`; Docker Hub image is `docker.io/zxabugx/payment-gateway`.
- The production deployment default remains `ghcr.io/aurosk-star/starpay:latest`.
- Build `linux/amd64` and `linux/arm64` from the existing root `Dockerfile`.
- Pull Requests never receive registry credentials and never push images.
- All third-party GitHub Actions use a full 40-character commit SHA.
- Docker Hub credentials exist only as `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` repository secrets.
- Version tags are built only from matching Git tags; current `main` must not be labelled `v0.2.0`.
- GitHub and CodeUP `main` must be synchronized after merge.

---

### Task 1: Add the image-workflow contract and implementation

**Files:**
- Create: `scripts/container_workflow_test.sh`
- Create: `.github/workflows/publish-image.yml`
- Modify: `.github/workflows/ci.yml`
- Modify: `Dockerfile`

**Interfaces:**
- Consumes: root `Dockerfile`; repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`; built-in `github.token`.
- Produces: PR-only build validation and dual-registry image publication from `main`, `v*.*.*`, and manual runs.

- [ ] **Step 1: Write the failing contract test**

Create `scripts/container_workflow_test.sh`:

```bash
#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
workflow="$repo_root/.github/workflows/publish-image.yml"
ci_workflow="$repo_root/.github/workflows/ci.yml"
dockerfile="$repo_root/Dockerfile"

fail() {
  printf '[container-workflow-test] FAIL: %s\n' "$*" >&2
  exit 1
}

require_text() {
  local file=$1
  local text=$2
  grep -Fq -- "$text" "$file" || fail "$file is missing: $text"
}

[[ -f "$workflow" ]] || fail "missing $workflow"

require_text "$workflow" "pull_request:"
require_text "$workflow" "branches: [main]"
require_text "$workflow" 'tags: ["v*.*.*"]'
require_text "$workflow" "workflow_dispatch:"
require_text "$workflow" "packages: write"
require_text "$workflow" "attestations: write"
require_text "$workflow" "id-token: write"
require_text "$workflow" "ghcr.io/aurosk-star/starpay"
require_text "$workflow" "docker.io/zxabugx/payment-gateway"
require_text "$workflow" 'username: ${{ secrets.DOCKERHUB_USERNAME }}'
require_text "$workflow" 'password: ${{ secrets.DOCKERHUB_TOKEN }}'
require_text "$workflow" "type=raw,value=latest,enable={{is_default_branch}}"
require_text "$workflow" "type=ref,event=tag"
require_text "$workflow" 'type=semver,pattern={{version}}'
require_text "$workflow" 'type=semver,pattern={{major}}.{{minor}}'
require_text "$workflow" "type=sha,prefix=sha-"
require_text "$workflow" "platforms: linux/amd64,linux/arm64"
require_text "$workflow" "provenance: mode=max"
require_text "$workflow" "sbom: true"
require_text "$workflow" "push: \${{ github.event_name != 'pull_request' }}"
require_text "$workflow" "name: Make GHCR package public"
require_text "$workflow" 'GH_TOKEN: ${{ github.token }}'
require_text "$workflow" "/orgs/aurosk-star/packages/container/starpay"
require_text "$workflow" "visibility=public"
require_text "$ci_workflow" "bash scripts/container_workflow_test.sh"
require_text "$dockerfile" 'FROM --platform=$BUILDPLATFORM oven/bun:'
require_text "$dockerfile" 'FROM --platform=$BUILDPLATFORM golang:'
require_text "$dockerfile" 'ARG TARGETOS'
require_text "$dockerfile" 'ARG TARGETARCH'
require_text "$dockerfile" 'CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build'
require_text "$dockerfile" 'USER 65532:65532'
if grep -Eq '^[[:space:]]*RUN[[:space:]]+(addgroup|adduser)' "$dockerfile"; then
  fail "final image user creation must not require target-platform execution"
fi

while IFS= read -r action; do
  [[ "$action" =~ @[0-9a-f]{40}([[:space:]]|$) ]] \
    || fail "action is not pinned to a full commit SHA: $action"
done < <(sed -nE 's/^[[:space:]]*-[[:space:]]*uses:[[:space:]]*(.+)$/\1/p' "$workflow")

if grep -REn 'dckr_pat_[A-Za-z0-9_-]+' "$repo_root/.github"; then
  fail "Docker Hub token literal found in .github"
fi

printf '[container-workflow-test] PASS\n'
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
bash scripts/container_workflow_test.sh
```

Expected: exit 1 because the publishing workflow and native cross-compilation declarations do not exist yet.

- [ ] **Step 3: Add the CI invocation and minimal publishing workflow**

Add this step to the `deployment` job in `.github/workflows/ci.yml` after the deployment-script test:

```yaml
      - run: bash scripts/container_workflow_test.sh
```

Update the Dockerfile builders to execute on the Runner's native platform and cross-compile Go for the requested target:

```dockerfile
FROM --platform=$BUILDPLATFORM oven/bun:1.3.14 AS web-builder

WORKDIR /src/web

COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

COPY web/ ./
RUN bun run build

FROM --platform=$BUILDPLATFORM golang:1.26.6-alpine3.23 AS go-builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web-builder /src/web/dist ./internal/platform/webui/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags webui -trimpath -ldflags="-s -w" -o /out/payment-gateway ./cmd/server

FROM alpine:3.24

WORKDIR /app
COPY --from=go-builder /out/payment-gateway /app/payment-gateway
COPY LICENSE NOTICE THIRD_PARTY_NOTICES.md /licenses/

USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/app/payment-gateway"]
```

Create `.github/workflows/publish-image.yml`:

```yaml
name: Publish container image

on:
  pull_request:
    paths:
      - .github/workflows/publish-image.yml
      - Dockerfile
      - go.mod
      - go.sum
      - cmd/**
      - ent/**
      - internal/**
      - web/**
  push:
    branches: [main]
    tags: ["v*.*.*"]
  workflow_dispatch:

permissions:
  contents: read
  packages: write
  attestations: write
  id-token: write

concurrency:
  group: publish-image-${{ github.ref }}
  cancel-in-progress: true

jobs:
  image:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1

      - uses: docker/setup-qemu-action@c7c53464625b32c7a7e944ae62b3e17d2b600130 # v3

      - uses: docker/setup-buildx-action@8d2750c68a42422c14e847fe6c8ac0403b4cbd6f # v3

      - name: Log in to GHCR
        if: github.event_name != 'pull_request'
        uses: docker/login-action@c94ce9fb468520275223c153574b00df6fe4bcc9 # v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ github.token }}

      - name: Log in to Docker Hub
        if: github.event_name != 'pull_request'
        uses: docker/login-action@c94ce9fb468520275223c153574b00df6fe4bcc9 # v3
        with:
          registry: docker.io
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Generate image metadata
        id: meta
        uses: docker/metadata-action@c299e40c65443455700f0fdfc63efafe5b349051 # v5
        with:
          images: |
            ghcr.io/aurosk-star/starpay
            docker.io/zxabugx/payment-gateway
          flavor: |
            latest=false
          tags: |
            type=raw,value=latest,enable={{is_default_branch}}
            type=ref,event=tag
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=sha,prefix=sha-

      - name: Build and publish image
        id: build
        uses: docker/build-push-action@10e90e3645eae34f1e60eeb005ba3a3d33f178e8 # v6
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: ${{ github.event_name != 'pull_request' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          provenance: mode=max
          sbom: true

      - name: Make GHCR package public
        if: github.event_name != 'pull_request'
        env:
          GH_TOKEN: ${{ github.token }}
        run: >-
          gh api --method PATCH
          /orgs/aurosk-star/packages/container/starpay
          -f visibility=public

      - name: Attest GHCR image
        if: github.event_name != 'pull_request'
        uses: actions/attest-build-provenance@8beda2b7ed98355c0e97c0a63bec38ae472e66c4 # v4
        with:
          subject-name: ghcr.io/aurosk-star/starpay
          subject-digest: ${{ steps.build.outputs.digest }}
          push-to-registry: true

      - name: Verify published manifests
        if: github.event_name != 'pull_request'
        env:
          IMAGE_DIGEST: ${{ steps.build.outputs.digest }}
        run: |
          docker buildx imagetools inspect "ghcr.io/aurosk-star/starpay@${IMAGE_DIGEST}"
          docker buildx imagetools inspect "docker.io/zxabugx/payment-gateway@${IMAGE_DIGEST}"
```

- [ ] **Step 4: Run the focused tests and verify GREEN**

Run:

```bash
bash -n scripts/container_workflow_test.sh
bash scripts/container_workflow_test.sh
bash scripts/deploy_test.sh
docker buildx build --platform linux/amd64,linux/arm64 --output type=cacheonly .
```

Expected: all commands exit 0 and both test scripts print `PASS`.

- [ ] **Step 5: Validate YAML and repository checks**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/repo" -w /repo \
  rhysd/actionlint:1.7.11 .github/workflows/*.yml
git diff --check
make verify
```

Expected: actionlint and `git diff --check` produce no errors; `make verify` exits 0.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/publish-image.yml .github/workflows/ci.yml Dockerfile scripts/container_workflow_test.sh
git commit -m "Publish images to GHCR and Docker Hub"
```

### Task 2: Configure publication credentials

**Files:** None. GitHub repository settings only.

**Interfaces:**
- Consumes: Docker Hub username and a read/write personal access token supplied out of band.
- Produces: Actions secrets readable only by non-PR publishing runs.

- [ ] **Step 1: Set the username secret**

Run:

```bash
gh secret set DOCKERHUB_USERNAME --repo aurosk-star/starpay --body zxabugx
```

Expected: command exits 0 without printing a secret value.

- [ ] **Step 2: Set the token secret without putting it in a command argument**

Run interactively:

```bash
gh secret set DOCKERHUB_TOKEN --repo aurosk-star/starpay
```

Paste the token at the hidden prompt and submit it. Never place it in shell history, an environment file, a plan, or a commit.

- [ ] **Step 3: Verify secret names and timestamps**

Run:

```bash
gh secret list --repo aurosk-star/starpay
```

Expected: `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` are listed; values remain unreadable.

### Task 3: Review and merge through the protected branch

**Files:** None beyond Task 1 commits.

**Interfaces:**
- Consumes: feature branch `codex/publish-container-images` and repository branch protection.
- Produces: reviewed `main` commit on GitHub and matching CodeUP `main`.

- [ ] **Step 1: Push the feature branch and open a Pull Request**

```bash
git push -u github codex/publish-container-images
gh pr create --repo aurosk-star/starpay --base main --head codex/publish-container-images \
  --title "Publish images to GHCR and Docker Hub" \
  --body $'## 摘要\n\n- 同一次多平台构建发布到 GHCR 与 Docker Hub\n- main 发布 latest/sha 标签，v*.*.* 发布语义版本标签\n- PR 只构建，不登录或推送\n- 生成 provenance 与 SBOM\n\n## 配置\n\n新增 DOCKERHUB_USERNAME 和 DOCKERHUB_TOKEN repository secrets。部署默认镜像保持 ghcr.io/aurosk-star/starpay:latest。\n\n## 验证\n\n- bash scripts/container_workflow_test.sh\n- bash scripts/deploy_test.sh\n- actionlint\n- make verify'
```

- [ ] **Step 2: Wait for all required and image-build checks**

```bash
gh pr checks --repo aurosk-star/starpay --watch --interval 20
```

Expected: CI, Security, and Publish container image checks complete successfully. The PR image check builds but does not log in or push.

- [ ] **Step 3: Merge the Pull Request**

```bash
gh pr merge --repo aurosk-star/starpay --squash --delete-branch
git switch main
git pull --ff-only github main
```

Expected: the protected branch accepts the merge and local `main` matches `github/main`.

- [ ] **Step 4: Synchronize CodeUP after publication changes are merged**

```bash
git push origin main
```

Expected: `git rev-parse main`, `git rev-parse github/main`, and `git rev-parse origin/main` print the same commit.

### Task 4: Publish and verify current `main`

**Files:** None. Registry and GitHub Actions state only.

**Interfaces:**
- Consumes: merged publishing workflow and configured repository secrets.
- Produces: public, pullable multi-platform `latest` and SHA images in both registries.

- [ ] **Step 1: Monitor the automatic `main` publication**

```bash
publish_run_id=$(gh run list --repo aurosk-star/starpay --workflow publish-image.yml \
  --branch main --limit 1 --json databaseId --jq '.[0].databaseId')
if [[ -z "$publish_run_id" ]]; then
  gh workflow run publish-image.yml --repo aurosk-star/starpay --ref main
  publish_run_id=$(gh run list --repo aurosk-star/starpay --workflow publish-image.yml \
    --branch main --limit 1 --json databaseId --jq '.[0].databaseId')
fi
gh run watch --repo aurosk-star/starpay "$publish_run_id" --interval 20 --exit-status
```

Expected: build, push, attestation, and manifest verification succeed.

- [ ] **Step 2: Verify the workflow made the GHCR package public**

```bash
docker buildx imagetools inspect ghcr.io/aurosk-star/starpay:latest
```

Expected: anonymous inspection succeeds. The publishing workflow's `Make GHCR package public` step uses the repository-scoped `GITHUB_TOKEN`; no personal GitHub package token is required.

- [ ] **Step 3: Verify public manifests and platforms**

```bash
docker buildx imagetools inspect ghcr.io/aurosk-star/starpay:latest
docker buildx imagetools inspect zxabugx/payment-gateway:latest
```

Expected: both commands succeed anonymously and list `linux/amd64` and `linux/arm64` plus attestation manifests.

- [ ] **Step 4: Verify SHA tags and deployment default**

```bash
short_sha=$(git rev-parse --short=7 main)
docker buildx imagetools inspect "ghcr.io/aurosk-star/starpay:sha-${short_sha}"
docker buildx imagetools inspect "zxabugx/payment-gateway:sha-${short_sha}"
bash scripts/deploy_test.sh
```

Expected: both SHA tags exist and deployment tests confirm the default remains `ghcr.io/aurosk-star/starpay:latest`.

- [ ] **Step 5: Perform final repository and registry audit**

```bash
git status --short --branch
gh pr list --repo aurosk-star/starpay --state open
gh run list --repo aurosk-star/starpay --limit 10
```

Expected: clean synchronized `main`, no open task PR, and successful latest CI, Security, and Publish container image runs.
