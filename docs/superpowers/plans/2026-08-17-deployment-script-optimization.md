# StarPay Deployment Script Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a safe interactive deployment wizard while preserving automation compatibility, and add verified backups, deployment metadata, and automatic API rollback.

**Architecture:** Keep `scripts/deploy.sh` as the single production entrypoint, with small Bash functions grouped by output, configuration, wizard, backup, deployment, rollback, and metadata responsibilities. Extend the shell test harness with isolated fake Docker and curl commands so failure and rollback paths run without touching real services.

**Tech Stack:** Bash 4+, Docker Compose v2, PostgreSQL `pg_dump`/`pg_restore`, curl, flock, existing Dockerfile and Make-based verification.

## Global Constraints

- `scripts/deploy.sh install` and `scripts/deploy.sh update` remain non-interactive and backward compatible.
- Running `scripts/deploy.sh` without arguments enters the Chinese interactive wizard.
- Default bind address remains `127.0.0.1`; secrets are never printed.
- Default backup retention is 7; `--keep-backups N` accepts integers greater than or equal to 1.
- `--local-build` and an explicitly supplied `--image` are mutually exclusive.
- Deployment failure restores only the prior API image; PostgreSQL is never restored automatically.
- No remote bootstrap, automatic Git update, GHCR publishing workflow, or zero-downtime orchestration is added.
- All behavior changes follow red-green TDD.

---

### Task 1: CLI compatibility and interactive wizard

**Files:**
- Modify: `scripts/deploy.sh`
- Modify: `scripts/deploy_test.sh`

**Interfaces:**
- Produces: `detect_deployment_state ENV_FILE COMPOSE_FILE IMAGE`, setting `DEPLOYMENT_STATE` to `install` or `update`.
- Produces: `run_wizard`, setting `mode`, `env_file`, `image`, `build_mode`, `skip_backup`, and `keep_backups` before invoking `deploy`.
- Produces: `set_env_value FILE KEY VALUE`, atomically preserving unrelated lines.
- Extends: `main` so zero arguments call `run_wizard`; explicit modes remain non-interactive.

- [ ] **Step 1: Add failing CLI and configuration tests**

Add assertions to `scripts/deploy_test.sh` that prove:

```bash
[[ $(read_env_value "$generated_env" HTTP_BIND) == "127.0.0.1" ]]
set_env_value "$generated_env" HTTP_PORT 9090
[[ $(read_env_value "$generated_env" HTTP_PORT) == "9090" ]]
[[ $(read_env_value "$generated_env" JWT_SECRET) == "$original_jwt" ]]

if bash "$deploy_script" update --image test:one --local-build >/dev/null 2>&1; then
  fail "conflicting image sources unexpectedly succeeded"
fi
if bash "$deploy_script" update --keep-backups 0 >/dev/null 2>&1; then
  fail "invalid backup retention unexpectedly succeeded"
fi
```

Source the script with a temporary environment file and fake Compose state, call `detect_deployment_state`, and assert missing configuration is `install`, existing configuration is `update`, and a running API without configuration fails.

- [ ] **Step 2: Run the deployment tests and verify RED**

Run: `bash scripts/deploy_test.sh`

Expected: FAIL because `set_env_value`, `detect_deployment_state`, `--local-build`, and `--keep-backups` do not exist.

- [ ] **Step 3: Implement the minimal CLI and wizard functions**

Implement:

```bash
set_env_value() { ... }              # temporary file + chmod 600 + mv
detect_deployment_state() { ... }    # env and Compose container inspection
ask() { ... }                        # reads /dev/tty, with stdin fallback for tests
ask_choice() { ... }                 # validates enumerated choices
confirm() { ... }                    # y/N or Y/n default
run_wizard() { ... }                 # state, port, bind, source, backup, summary
```

Add `--local-build` and `--keep-backups N` parsing, validation, help text, and the explicit-image conflict check. Track whether `--image` was supplied rather than treating the default image as explicit.

- [ ] **Step 4: Run deployment tests and verify GREEN**

Run: `bash scripts/deploy_test.sh`

Expected: `deploy script tests passed`.

- [ ] **Step 5: Commit Task 1**

```bash
git add scripts/deploy.sh scripts/deploy_test.sh
git commit -m "Add interactive production deployment wizard"
```

### Task 2: Verified PostgreSQL backups and retention

**Files:**
- Modify: `scripts/deploy.sh`
- Modify: `scripts/deploy_test.sh`

**Interfaces:**
- Changes: `backup_database ENV_FILE COMPOSE_FILE IMAGE KEEP_BACKUPS` prints the final backup path only on success.
- Produces: `validate_database_backup COMPOSE_ARGS...`, reading the archive from stdin via container `pg_restore --list`.
- Produces: `prune_database_backups BACKUP_DIR KEEP_BACKUPS`, deleting only excess `payment-gateway-*.dump` files.

- [ ] **Step 1: Add failing backup validation and retention tests**

Use a temporary backup directory override and a fake Compose executable that records invocations. Assert:

```bash
backup_path=$(backup_database "$generated_env" "$repo_root/docker-compose.prod.yml" test:image 2)
[[ -s "$backup_path" ]] || fail "verified backup was not created"
[[ $(stat -c '%a' "$backup_path") == "600" ]] || fail "backup mode is not 600"
[[ $(find "$test_backup_dir" -maxdepth 1 -name 'payment-gateway-*.dump' | wc -l) -eq 2 ]]
rg -q 'pg_restore --list' "$fake_docker_log" || fail "backup archive was not verified"
```

Configure the fake `pg_restore --list` call to fail and assert no final `.dump` file is created and `backup_database` returns nonzero.

- [ ] **Step 2: Run the deployment tests and verify RED**

Run: `bash scripts/deploy_test.sh`

Expected: FAIL because backup validation, injectable backup directory, and retention do not exist.

- [ ] **Step 3: Implement verified backups and safe pruning**

Add `DEPLOY_BACKUP_DIR` as a test/deployment override with the repository `backups/` directory as default. Validate the temporary archive using:

```bash
"${compose[@]}" exec -T postgres pg_restore --list <"$temporary" >/dev/null
```

Only after validation, move and chmod the archive. Sort the exact `payment-gateway-*.dump` matches by timestamp/name, resolve each path inside the configured backup directory, and remove only entries exceeding `KEEP_BACKUPS`.

- [ ] **Step 4: Run deployment tests and verify GREEN**

Run: `bash scripts/deploy_test.sh`

Expected: `deploy script tests passed`, including simulated corrupt-backup failure.

- [ ] **Step 5: Commit Task 2**

```bash
git add scripts/deploy.sh scripts/deploy_test.sh
git commit -m "Verify and retain production database backups"
```

### Task 3: Health validation, deployment metadata, and rollback

**Files:**
- Modify: `scripts/deploy.sh`
- Modify: `scripts/deploy_test.sh`

**Interfaces:**
- Changes: `wait_for_health` requires an HTTP success response containing `"status":"ok"`.
- Produces: `current_api_image_id ENV_FILE COMPOSE_FILE IMAGE`, returning the inspected image ID or an empty string.
- Produces: `rollback_api ENV_FILE COMPOSE_FILE OLD_IMAGE_ID`, recreating only `api` and rerunning health checks.
- Produces: `write_deployment_record MODE BUILD_MODE IMAGE IMAGE_ID BACKUP_PATH` at `${DEPLOY_STATE_FILE:-$repo_root/.tmp/last-deployment}`.

- [ ] **Step 1: Add failing health, record, and rollback tests**

Create fake Docker/curl programs in a temporary PATH. Simulate a current API image `sha256:old`, a failed new health response, then a successful old health response. Assert the command log contains:

```text
PAYMENT_GATEWAY_IMAGE=sha256:old
up -d --no-deps --force-recreate --no-build api
```

Assert the deployment returns nonzero after successful rollback. For a successful deployment, inspect the state file and assert it contains the image ID and Git commit but does not contain generated PostgreSQL, Redis, JWT, or encryption secrets.

- [ ] **Step 2: Run the deployment tests and verify RED**

Run: `bash scripts/deploy_test.sh`

Expected: FAIL because rollback and deployment records are not implemented and health only checks HTTP status.

- [ ] **Step 3: Implement explicit deploy failure handling and rollback**

Before pull/build, capture the old image ID. Wrap Compose start and health validation in `if` conditions. On update failure with a nonempty old ID, invoke `rollback_api`; always return the original deployment failure status. On failed first install, stop only `api` and print status/log commands.

Write deployment metadata atomically with mode `600`. Use a fixed key list and never serialize the environment file.

- [ ] **Step 4: Run deployment tests and verify GREEN**

Run: `bash scripts/deploy_test.sh`

Expected: `deploy script tests passed`, including the rollback command and secret-exclusion assertions.

- [ ] **Step 5: Commit Task 3**

```bash
git add scripts/deploy.sh scripts/deploy_test.sh
git commit -m "Add automatic API deployment rollback"
```

### Task 4: Compose health check and production documentation

**Files:**
- Modify: `docker-compose.prod.yml`
- Modify: `scripts/deploy_test.sh`
- Modify: `docs/PRODUCTION_DEPLOYMENT.md`

**Interfaces:**
- Adds: Compose API health check for `http://127.0.0.1:8080/healthz`.
- Documents: interactive use, non-interactive compatibility, local builds, backup retention, records, and rollback boundaries.

- [ ] **Step 1: Add a failing Compose health-check assertion**

Extend the rendered Compose assertions:

```bash
[[ "$compose_output" == *"http://127.0.0.1:8080/healthz"* ]] \
  || fail "Compose API healthcheck is missing"
```

- [ ] **Step 2: Run the deployment tests and verify RED**

Run: `bash scripts/deploy_test.sh`

Expected: FAIL with `Compose API healthcheck is missing`.

- [ ] **Step 3: Add the health check and update documentation**

Add a BusyBox `wget` API health check with interval 10 seconds, timeout 3 seconds, 10 retries, and 20-second start period. Update the production guide with exact examples:

```bash
./scripts/deploy.sh
./scripts/deploy.sh update --image ghcr.io/aurosk-star/starpay:v0.2.0 --keep-backups 7
./scripts/deploy.sh update --local-build --keep-backups 7
```

Explain that rollback is API-only and database restore remains manual.

- [ ] **Step 4: Run deployment tests and verify GREEN**

Run: `bash scripts/deploy_test.sh`

Expected: `deploy script tests passed`.

- [ ] **Step 5: Commit Task 4**

```bash
git add docker-compose.prod.yml scripts/deploy_test.sh docs/PRODUCTION_DEPLOYMENT.md
git commit -m "Document resilient production deployment workflow"
```

### Task 5: Full verification and handoff

**Files:**
- Verify only: all changed files

**Interfaces:**
- Consumes: all behavior from Tasks 1-4.
- Produces: evidence suitable for PR review and merge.

- [ ] **Step 1: Run shell syntax and whitespace checks**

```bash
bash -n scripts/deploy.sh scripts/deploy_test.sh
git diff --check github/main...HEAD
```

Expected: both commands exit 0 with no output.

- [ ] **Step 2: Run deployment-specific tests**

Run: `bash scripts/deploy_test.sh`

Expected: `deploy script tests passed`.

- [ ] **Step 3: Run the complete project verification**

Run: `make verify`

Expected: backend and SDK tests, frontend tests/typecheck/build/lint, deployment tests, govulncheck, audits, and vet all exit 0.

- [ ] **Step 4: Build the production container**

Run: `docker build -t starpay:deploy-test .`

Expected: image build exits 0 and produces `starpay:deploy-test`.

- [ ] **Step 5: Review final scope and history**

```bash
git status --short
git log --oneline github/main..HEAD
git diff --stat github/main...HEAD
```

Expected: clean worktree, design/plan plus four focused implementation commits, and changes limited to the deployment script, tests, production Compose, and production deployment documentation.
