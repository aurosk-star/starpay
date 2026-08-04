# Open Source Readiness Design

## Objective

Prepare the payment gateway for its first public source release under the
Apache License 2.0. The release must have a clean security baseline, automated
continuous integration, clear contribution and vulnerability-reporting paths,
and no knowingly redistributed third-party material without compatible terms.

The public gateway repository will be separate from the existing public
`zmoyi/starpay-go` SDK repository. The gateway uses Apache-2.0; the SDK keeps
its existing MIT license.

## Current Baseline

The repository currently has a clean working tree and passes backend tests,
`go vet`, SDK tests, frontend tests, TypeScript checks, frontend lint and build,
and deployment-script tests.

The open-source blockers found during the readiness audit are:

- The repository root has no license or community health files.
- The repository has no GitHub Actions workflows.
- `govulncheck` reports nine reachable vulnerabilities from the Go toolchain
  and dependency graph.
- `bun audit` reports sixteen frontend dependency vulnerabilities, including
  seven high-severity findings.
- The repository tracks 111 third-party files under `.agents/skills/` without
  accompanying license files.
- Public-facing documentation still contains a private Codeup clone URL, a
  personal container namespace, public-SDK `GOPRIVATE` guidance, and a generic
  Rsbuild frontend README.
- Two tests contain test-only PEM private-key fixtures that are expected to
  trigger generic secret scanners.

## Chosen Approach

Use attack-surface reduction before dependency upgrades. Remove tooling that
does not participate in application runtime or reproducible builds, upgrade
the remaining direct and transitive dependencies to fixed versions, and use
automated security gates to prevent regressions.

Do not suppress a real vulnerability merely to obtain a green check. A scanner
exception is allowed only for a verified non-secret test fixture or a proven
false positive. Every exception must be exact-path scoped, include a reason in
version control, and continue to leave production credentials and reachable
vulnerabilities unignored.

## Licensing and Third-Party Material

Add the unmodified Apache License 2.0 text as the root `LICENSE`. Add `NOTICE`
identifying the project as Starpay and the copyright holder as "Starpay
contributors". Add `THIRD_PARTY_NOTICES.md` containing dependencies or bundled
assets whose licenses require attribution.

Do not add SPDX headers to generated Ent files or mechanically modify every
source file. Repository-level licensing is sufficient, avoids generated-code
churn, and preserves upstream notices where present.

Remove the vendored `.agents/skills/` directory and `skills-lock.json` from the
public source tree. They are development-environment material rather than
payment-gateway source. Keep `AGENTS.md` because it documents repository-local
engineering rules and does not bundle third-party skill implementations.

Retain `docs/superpowers/specs/` and `docs/superpowers/plans/` because they are
project-specific engineering records. Before release, scan them for private
URLs, credentials, personal data, and obsolete operational instructions.

## Backend Vulnerability Remediation

Set the supported toolchain to Go 1.26.5 by adding `toolchain go1.26.5` to the
root and SDK modules. Pin the Docker builder image to `golang:1.26.5` instead
of using `golang:latest`.

Upgrade `golang.org/x/text` to at least v0.39.0 and
`github.com/quic-go/quic-go` to at least v0.59.1. Prefer upgrading their direct
parents when that yields the fixed transitive version. Run `go mod tidy` after
the upgrades and retain only dependencies required by the build and tools.

The backend security gate is `govulncheck ./...` with zero reachable
vulnerabilities. Findings in imported but unreachable symbols remain visible
in logs and must be reviewed, but the release is blocked by any reachable
finding.

## Frontend Vulnerability Remediation

Remove the `shadcn` CLI package from `web/package.json`. The generated shadcn/ui
components are source files and neither runtime nor normal builds require the
CLI. When maintainers need to generate another component, documentation will
use an explicitly pinned one-off CLI version that has passed an audit at that
time.

Upgrade Tailwind/PostCSS and other remaining direct dependencies to versions
whose transitive graphs contain no known advisories. Regenerate `web/bun.lock`
with the repository's Bun version. Prefer upstream fixed versions; do not use
an override when a fixed parent release exists. A temporary override is
acceptable only when it selects a semver-compatible security fix, includes an
explanatory comment supported by the package format, and has a tracked removal
condition.

The initial public-release gate is a zero-finding `bun audit`, including
development dependencies. This protects both the shipped frontend and the
maintainer build environment.

## Secrets and Configuration Safety

Keep `.env` and `.env.production` excluded from both Git and Docker build
contexts. Replace the fixed application-secret encryption key in
`.env.production.example` with a non-runnable replacement marker. Production
startup must reject the documented development JWT secret, the documented
development application-encryption key, and unreplaced `CHANGE_ME` values.

Retain the PEM fixtures only if the payment-provider tests require parseable
keys. Mark them as test-only and add exact file-and-rule exceptions to the
secret scanner. The exceptions must not match other private keys or other
paths.

Run gitleaks against the full Git history before the first public push. Any
real credential found in any commit is rotated first and removed from history
second. If the existing author email should not become public, rewrite it to a
GitHub noreply address before the first gateway push.

## Continuous Integration Design

Create `.github/workflows/ci.yml` for pull requests and pushes to `main`. It
runs independent backend, SDK, frontend, and repository-safety jobs with
least-privilege read-only permissions:

- Backend: Go 1.26.5, module download, `go test ./...`, `go vet
  ./...`, and `govulncheck ./...`.
- SDK: `go test -count=1 ./...` and `go vet ./...` from `sdk/go`.
- Frontend: Bun 1.3.13, frozen install, Node tests, TypeScript check,
  oxlint, production build, and `bun audit`.
- Deployment tooling: `bash scripts/deploy_test.sh`.
- Repository safety: full-history gitleaks scan with only the reviewed test-key
  exceptions.

Create `.github/workflows/security.yml` for weekly scheduled scans, manual
dispatch, pushes to `main`, and pull requests where the scanner supports them.
It runs CodeQL for Go and JavaScript/TypeScript, builds the application image,
scans it with Trivy, and produces an SPDX or CycloneDX SBOM. High and critical
container findings block the workflow. Scanner action versions are pinned to
immutable commit SHAs, with Dependabot responsible for proposing updates.

Create `.github/dependabot.yml` with weekly update groups for Go modules, the
frontend package ecosystem, GitHub Actions, and Docker base images. PostgreSQL
and Redis remain on `latest` in Compose by existing project decision; the
public deployment guide explicitly documents that policy and its operational
trade-off.

CI workflows do not publish images, create releases, or mutate repository
settings. Release automation and GitHub branch-protection configuration are
separate follow-up work after the open-source readiness changes merge.

## Community and Public Documentation

Add:

- `SECURITY.md` with supported versions, a private reporting route, expected
  acknowledgement and remediation timelines, and a request not to open public
  issues for unpatched vulnerabilities.
- `CONTRIBUTING.md` with local setup, required verification, architecture
  boundaries, commit guidance, and a Developer Certificate of Origin sign-off
  requirement.
- `CODE_OF_CONDUCT.md` using Contributor Covenant 2.1.
- `.github/PULL_REQUEST_TEMPLATE.md` and focused bug, feature, and security
  issue configuration.
- A root changelog describing the first public release baseline.

Update the root README with the Apache-2.0 license, build/security status
badges, maturity statement, supported Go/Bun versions, five-minute quickstart,
screenshots or a visual verification note, security warning, and links to the
security and contribution policies.

Replace the generic `web/README.md` with frontend-specific development and
verification instructions. Replace the private Codeup URL in the production
deployment guide with `https://github.com/zmoyi/starpay`, remove `GOPRIVATE`
instructions for the public SDK, and replace personal container namespace
defaults with `ghcr.io/zmoyi/starpay` while retaining the existing environment
variable override.

## Verification and Release Gate

The readiness change is complete only when all of the following are true from
a clean checkout:

```text
go test ./...                          passes
go vet ./...                           passes
govulncheck ./...                      reports 0 reachable vulnerabilities
cd sdk/go && go test -count=1 ./...    passes
cd sdk/go && go vet ./...              passes
cd web && node --test test/*.test.mts  passes
cd web && bun run typecheck            passes
cd web && bun run lint                 passes
cd web && bun run build                passes
cd web && bun audit                    reports 0 vulnerabilities
bash scripts/deploy_test.sh             passes
gitleaks full-history scan              reports 0 unallowlisted secrets
Trivy application-image scan           reports 0 high or critical findings
git status --short                      produces no output
```

The same commands must pass in GitHub Actions. The repository is not made
public and no release is tagged as part of this implementation; those are
explicit owner actions after review of the clean CI results and the final Git
history.
