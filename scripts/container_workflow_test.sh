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

if grep -Fq '/orgs/aurosk-star/packages/container/starpay' "$workflow"; then
  fail "GITHUB_TOKEN cannot manage organization package visibility"
fi

printf '[container-workflow-test] PASS\n'
