#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
deploy_script="$repo_root/scripts/deploy.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

[[ -f "$deploy_script" ]] || fail "scripts/deploy.sh is missing"

# shellcheck source=/dev/null
source "$deploy_script"

[[ "$default_image" == "ghcr.io/aurosk-star/starpay:latest" ]] || fail "default image is not the organization GHCR image"

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT
generated_env="$tmp_dir/.env.production"

generate_env "$repo_root/.env.production.example" "$generated_env"
[[ -f "$generated_env" ]] || fail "environment file was not generated"
[[ $(stat -c '%a' "$generated_env") == "600" ]] || fail "environment file mode is not 600"
if rg -q '^[[:space:]]*[^#].*CHANGE_ME_' "$generated_env"; then
  fail "generated environment still contains placeholders"
fi
validate_env "$generated_env" || fail "generated environment did not validate"

[[ $(read_env_value "$generated_env" APP_ENV) == "production" ]] || fail "APP_ENV is not production"
[[ $(read_env_value "$generated_env" REFRESH_COOKIE_SECURE) == "true" ]] || fail "secure refresh cookie is disabled"
encryption_key=$(read_env_value "$generated_env" APP_SECRET_ENCRYPTION_KEY)
case ${#encryption_key} in
  16 | 24 | 32) ;;
  *) fail "application encryption key has invalid length" ;;
esac

original_jwt=$(read_env_value "$generated_env" JWT_SECRET)
set_env_value "$generated_env" HTTP_PORT 9090
[[ $(read_env_value "$generated_env" HTTP_PORT) == "9090" ]] || fail "HTTP_PORT was not updated"
[[ $(read_env_value "$generated_env" JWT_SECRET) == "$original_jwt" ]] || fail "updating HTTP_PORT changed JWT_SECRET"
set_env_value "$generated_env" HTTP_PORT 8080

fake_bin="$tmp_dir/bin"
mkdir -p "$fake_bin"
cat >"$fake_bin/docker" <<'EOF'
#!/usr/bin/env bash
if [[ -n ${FAKE_DOCKER_LOG:-} ]]; then
  printf 'PAYMENT_GATEWAY_IMAGE=%s %s\n' "${PAYMENT_GATEWAY_IMAGE:-}" "$*" >>"$FAKE_DOCKER_LOG"
fi
if [[ "$*" == *"ps -aq"* && "$*" == *"payment-gateway-api"* ]]; then
  printf '%s' "${FAKE_API_CONTAINER_ID:-}"
elif [[ "$*" == *"compose"* && "$*" == *"ps -q api"* ]]; then
  printf '%s\n' "${FAKE_API_CONTAINER_ID:-api-container}"
elif [[ "$*" == *"inspect"* && "$*" == *"api-container"* ]]; then
  printf '%s\n' "${FAKE_API_IMAGE_ID:-sha256:old}"
elif [[ "$*" == *"ps -q postgres"* ]]; then
  printf 'postgres-container\n'
elif [[ "$*" == *"pg_dump"* ]]; then
  printf 'fake-postgres-custom-archive\n'
elif [[ "$*" == *"pg_restore --list"* ]]; then
  cat >/dev/null
  if [[ ${FAKE_PG_RESTORE_FAIL:-0} == "1" ]]; then
    exit 1
  fi
  printf 'archive contents\n'
fi
EOF
chmod +x "$fake_bin/docker"
cat >"$fake_bin/curl" <<'EOF'
#!/usr/bin/env bash
case ${FAKE_CURL_MODE:-success} in
  success)
    printf '{"code":"ok","data":{"status":"ok"},"error":null,"message":"ok"}\n'
    ;;
  invalid)
    printf '{"code":"ok","data":{"status":"starting"},"error":null,"message":"ok"}\n'
    ;;
  fail-once)
    count=0
    if [[ -f ${FAKE_CURL_COUNT_FILE:?} ]]; then
      count=$(<"$FAKE_CURL_COUNT_FILE")
    fi
    count=$((count + 1))
    printf '%s\n' "$count" >"$FAKE_CURL_COUNT_FILE"
    if ((count == 1)); then
      printf '{"code":"ok","data":{"status":"starting"},"error":null,"message":"ok"}\n'
    else
      printf '{"code":"ok","data":{"status":"ok"},"error":null,"message":"ok"}\n'
    fi
    ;;
  *) exit 1 ;;
esac
EOF
chmod +x "$fake_bin/curl"

missing_env="$tmp_dir/.env.missing"
PATH="$fake_bin:$PATH" FAKE_API_CONTAINER_ID="" \
  detect_deployment_state "$missing_env" "$repo_root/docker-compose.prod.yml" "test:image"
[[ "$DEPLOYMENT_STATE" == "install" ]] || fail "missing deployment was not detected as install"

PATH="$fake_bin:$PATH" FAKE_API_CONTAINER_ID="" \
  detect_deployment_state "$generated_env" "$repo_root/docker-compose.prod.yml" "test:image"
[[ "$DEPLOYMENT_STATE" == "update" ]] || fail "existing environment was not detected as update"

if PATH="$fake_bin:$PATH" FAKE_API_CONTAINER_ID="api-container" \
  detect_deployment_state "$missing_env" "$repo_root/docker-compose.prod.yml" "test:image" >/dev/null 2>&1; then
  fail "running API without an environment file unexpectedly passed deployment detection"
fi

test_backup_dir="$tmp_dir/backups"
mkdir -p "$test_backup_dir"
printf 'old-1\n' >"$test_backup_dir/payment-gateway-20260101-000001.dump"
printf 'old-2\n' >"$test_backup_dir/payment-gateway-20260101-000002.dump"
printf 'old-3\n' >"$test_backup_dir/payment-gateway-20260101-000003.dump"
printf 'keep me\n' >"$test_backup_dir/unrelated.dump"
prune_database_backups "$test_backup_dir" 2
[[ $(find "$test_backup_dir" -maxdepth 1 -name 'payment-gateway-*.dump' | wc -l) -eq 2 ]] \
  || fail "backup retention did not keep exactly two script backups"
[[ -f "$test_backup_dir/unrelated.dump" ]] || fail "backup retention deleted an unrelated file"

fake_docker_log="$tmp_dir/docker.log"
backup_path=$(
  PATH="$fake_bin:$PATH" \
    FAKE_DOCKER_LOG="$fake_docker_log" \
    DEPLOY_BACKUP_DIR="$test_backup_dir" \
    backup_database "$generated_env" "$repo_root/docker-compose.prod.yml" test:image 2 2>/dev/null
)
[[ -s "$backup_path" ]] || fail "verified backup was not created"
[[ $(stat -c '%a' "$backup_path") == "600" ]] || fail "backup mode is not 600"
[[ $(find "$test_backup_dir" -maxdepth 1 -name 'payment-gateway-*.dump' | wc -l) -eq 2 ]] \
  || fail "backup retention was not applied after backup"
rg -q 'pg_restore --list' "$fake_docker_log" || fail "backup archive was not verified"

corrupt_backup_dir="$tmp_dir/corrupt-backups"
mkdir -p "$corrupt_backup_dir"
if PATH="$fake_bin:$PATH" \
  FAKE_DOCKER_LOG="$fake_docker_log" \
  FAKE_PG_RESTORE_FAIL=1 \
  DEPLOY_BACKUP_DIR="$corrupt_backup_dir" \
  backup_database "$generated_env" "$repo_root/docker-compose.prod.yml" test:image 2 >/dev/null 2>&1; then
  fail "corrupt backup unexpectedly succeeded validation"
fi
if find "$corrupt_backup_dir" -maxdepth 1 -name 'payment-gateway-*.dump' | rg -q .; then
  fail "corrupt backup was promoted to a final dump"
fi

current_image=$(
  PATH="$fake_bin:$PATH" \
    FAKE_API_CONTAINER_ID="api-container" \
    FAKE_API_IMAGE_ID="sha256:old" \
    current_api_image_id "$generated_env" "$repo_root/docker-compose.prod.yml" test:image
)
[[ "$current_image" == "sha256:old" ]] || fail "current API image ID was not detected"

if PATH="$fake_bin:$PATH" \
  FAKE_CURL_MODE=invalid \
  DEPLOY_HEALTH_ATTEMPTS=1 \
  DEPLOY_HEALTH_INTERVAL=0 \
  wait_for_health "$generated_env" "$repo_root/docker-compose.prod.yml" test:image >/dev/null 2>&1; then
  fail "invalid health response unexpectedly passed"
fi

rollback_log="$tmp_dir/rollback-docker.log"
curl_count_file="$tmp_dir/curl-count"
if PATH="$fake_bin:$PATH" \
  FAKE_DOCKER_LOG="$rollback_log" \
  FAKE_API_CONTAINER_ID="api-container" \
  FAKE_API_IMAGE_ID="sha256:old" \
  FAKE_CURL_MODE=fail-once \
  FAKE_CURL_COUNT_FILE="$curl_count_file" \
  DEPLOY_HEALTH_ATTEMPTS=1 \
  DEPLOY_HEALTH_INTERVAL=0 \
  deploy update "$generated_env" "$repo_root/docker-compose.prod.yml" test:new true image 2 >/dev/null 2>&1; then
  fail "failed deployment unexpectedly succeeded after rollback"
fi
rg -q 'PAYMENT_GATEWAY_IMAGE=sha256:old .*up -d --no-deps --force-recreate --no-build api' "$rollback_log" \
  || fail "failed deployment did not recreate API with the previous image"

deployment_state="$tmp_dir/last-deployment"
DEPLOY_STATE_FILE="$deployment_state" \
  write_deployment_record update image test:new sha256:new "$backup_path"
[[ $(stat -c '%a' "$deployment_state") == "600" ]] || fail "deployment record mode is not 600"
rg -q '^image_id=sha256:new$' "$deployment_state" || fail "deployment record is missing image ID"
rg -q '^git_commit=[0-9a-f]+$' "$deployment_state" || fail "deployment record is missing Git commit"
for secret in \
  "$(read_env_value "$generated_env" POSTGRES_PASSWORD)" \
  "$(read_env_value "$generated_env" REDIS_PASSWORD)" \
  "$(read_env_value "$generated_env" JWT_SECRET)" \
  "$(read_env_value "$generated_env" APP_SECRET_ENCRYPTION_KEY)"; do
  if rg -F -q "$secret" "$deployment_state"; then
    fail "deployment record contains a production secret"
  fi
done

invalid_env="$tmp_dir/.env.invalid"
cp "$generated_env" "$invalid_env"
printf '\nJWT_SECRET=CHANGE_ME_INVALID\n' >>"$invalid_env"
if validate_env "$invalid_env" >/dev/null 2>&1; then
  fail "placeholder environment unexpectedly validated"
fi

weak_jwt_env="$tmp_dir/.env.weak-jwt"
cp "$generated_env" "$weak_jwt_env"
printf '\nJWT_SECRET=local-development-secret-change-me\n' >>"$weak_jwt_env"
if validate_env "$weak_jwt_env" >/dev/null 2>&1; then
  fail "development JWT secret unexpectedly validated"
fi

weak_encryption_env="$tmp_dir/.env.weak-encryption"
cp "$generated_env" "$weak_encryption_env"
printf '\nAPP_SECRET_ENCRYPTION_KEY=development-only-key-32-bytes!!!\n' >>"$weak_encryption_env"
if validate_env "$weak_encryption_env" >/dev/null 2>&1; then
  fail "development application encryption key unexpectedly validated"
fi

compose_output=$(
  PAYMENT_GATEWAY_ENV_FILE="$generated_env" \
    PAYMENT_GATEWAY_IMAGE="ghcr.io/aurosk-star/starpay:test" \
    docker compose --env-file "$generated_env" -f "$repo_root/docker-compose.prod.yml" config
)
[[ "$compose_output" == *"image: ghcr.io/aurosk-star/starpay:test"* ]] || fail "Compose did not use PAYMENT_GATEWAY_IMAGE"
[[ "$compose_output" == *"APP_NAME: payment-gateway"* ]] || fail "Compose did not load PAYMENT_GATEWAY_ENV_FILE"

help_output=$(bash "$deploy_script" --help)
[[ "$help_output" == *"install"* && "$help_output" == *"update"* ]] || fail "help does not describe install and update"
[[ "$help_output" == *"--local-build"* && "$help_output" == *"--keep-backups"* ]] || fail "help does not describe new deployment options"
if bash "$deploy_script" invalid-mode >/dev/null 2>&1; then
  fail "invalid deployment mode unexpectedly succeeded"
fi

conflict_output=$(bash "$deploy_script" update --image test:one --local-build 2>&1 || true)
[[ "$conflict_output" == *"--local-build cannot be used with --image"* ]] \
  || fail "conflicting deployment sources were not rejected clearly"

retention_output=$(bash "$deploy_script" update --keep-backups 0 2>&1 || true)
[[ "$retention_output" == *"--keep-backups must be an integer greater than or equal to 1"* ]] \
  || fail "invalid backup retention was not rejected clearly"

wizard_output=$(printf '8080\n1\n1\n7\nn\n' | DEPLOY_INPUT_FD=/dev/stdin bash "$deploy_script" 2>&1 || true)
[[ "$wizard_output" == *"部署已取消"* ]] || fail "zero-argument invocation did not enter the deployment wizard"

printf 'deploy script tests passed\n'
