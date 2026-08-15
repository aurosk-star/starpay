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
if bash "$deploy_script" invalid-mode >/dev/null 2>&1; then
  fail "invalid deployment mode unexpectedly succeeded"
fi

printf 'deploy script tests passed\n'
