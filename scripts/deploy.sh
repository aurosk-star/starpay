#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/.." && pwd)
default_env_file="$repo_root/.env.production"
default_compose_file="$repo_root/docker-compose.prod.yml"
default_image="ghcr.io/aurosk-star/starpay:latest"

log() {
  printf '[deploy] %s\n' "$*"
}

fail() {
  printf '[deploy] ERROR: %s\n' "$*" >&2
  return 1
}

usage() {
  cat <<'EOF'
Usage:
  scripts/deploy.sh install [options]
  scripts/deploy.sh update [options]

Modes:
  install         First deployment. Generates .env.production when absent.
  update          Backup PostgreSQL, pull the API image, and recreate services.

Options:
  --env-file PATH Use a production environment file at PATH.
  --image IMAGE   Deploy IMAGE (default: ghcr.io/aurosk-star/starpay:latest).
  --skip-backup   Skip the PostgreSQL backup during update.
  -h, --help      Show this help.
EOF
}

read_env_value() {
  local env_file=$1
  local key=$2
  local value
  value=$(awk -v wanted="$key" '
    /^[[:space:]]*#/ { next }
    {
      line = $0
      sub(/^[[:space:]]*/, "", line)
      split(line, parts, "=")
      if (parts[1] == wanted) {
        sub(/^[^=]*=/, "", line)
        found = line
      }
    }
    END { print found }
  ' "$env_file")
  value=${value%$'\r'}
  if [[ "$value" == \"*\" && "$value" == *\" ]]; then
    value=${value:1:${#value}-2}
  elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
    value=${value:1:${#value}-2}
  fi
  printf '%s' "$value"
}

generate_env() {
  local template=$1
  local target=$2
  [[ -f "$template" ]] || fail "environment template not found: $template" || return 1
  command -v openssl >/dev/null 2>&1 || fail "openssl is required to generate secrets" || return 1

  local postgres_password redis_password jwt_secret encryption_key
  postgres_password=$(openssl rand -hex 24)
  redis_password=$(openssl rand -hex 24)
  jwt_secret=$(openssl rand -hex 32)
  encryption_key=$(openssl rand -hex 16)

  mkdir -p "$(dirname "$target")"
  local temporary="$target.tmp.$$"
  umask 077
  awk \
    -v postgres_password="$postgres_password" \
    -v redis_password="$redis_password" \
    -v jwt_secret="$jwt_secret" \
    -v encryption_key="$encryption_key" '
      /^POSTGRES_PASSWORD=/ { print "POSTGRES_PASSWORD=" postgres_password; next }
      /^REDIS_PASSWORD=/ { print "REDIS_PASSWORD=" redis_password; next }
      /^JWT_SECRET=/ { print "JWT_SECRET=" jwt_secret; next }
      /^APP_SECRET_ENCRYPTION_KEY=/ { print "APP_SECRET_ENCRYPTION_KEY=" encryption_key; next }
      { print }
    ' "$template" >"$temporary"
  chmod 600 "$temporary"
  mv "$temporary" "$target"
}

validate_env() {
  local env_file=$1
  [[ -f "$env_file" ]] || fail "environment file not found: $env_file" || return 1
  if awk '/^[[:space:]]*#/ { next } /CHANGE_ME_/ { found = 1 } END { exit !found }' "$env_file"; then
    fail "replace all CHANGE_ME values in $env_file"
    return 1
  fi
  if [[ $(read_env_value "$env_file" JWT_SECRET) == "local-development-secret-change-me" ]]; then
    fail "JWT_SECRET must not use the development default"
    return 1
  fi
  if [[ $(read_env_value "$env_file" APP_SECRET_ENCRYPTION_KEY) == "development-only-key-32-bytes!!!" ]]; then
    fail "APP_SECRET_ENCRYPTION_KEY must not use the development default"
    return 1
  fi

  local required key value
  required=(APP_ENV POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD REDIS_PASSWORD JWT_SECRET APP_SECRET_ENCRYPTION_KEY)
  for key in "${required[@]}"; do
    value=$(read_env_value "$env_file" "$key")
    if [[ -z "$value" ]]; then
      fail "$key is required in $env_file"
      return 1
    fi
  done
  if [[ $(read_env_value "$env_file" APP_ENV) != "production" ]]; then
    fail "APP_ENV must be production"
    return 1
  fi
  if [[ $(read_env_value "$env_file" REFRESH_COOKIE_SECURE) != "true" ]]; then
    fail "REFRESH_COOKIE_SECURE must be true"
    return 1
  fi

  value=$(read_env_value "$env_file" APP_SECRET_ENCRYPTION_KEY)
  case ${#value} in
    16 | 24 | 32) ;;
    *)
      fail "APP_SECRET_ENCRYPTION_KEY must be 16, 24, or 32 bytes"
      return 1
      ;;
  esac
}

require_commands() {
  local command_name
  for command_name in docker curl flock; do
    command -v "$command_name" >/dev/null 2>&1 || fail "$command_name is required" || return 1
  done
  docker info >/dev/null 2>&1 || fail "Docker daemon is unavailable or permission is denied" || return 1
  docker compose version >/dev/null 2>&1 || fail "Docker Compose v2 is required" || return 1
}

backup_database() {
  local env_file=$1
  local compose_file=$2
  local image=$3
  local backup_dir="$repo_root/backups"
  local timestamp backup_file temporary
  timestamp=$(date '+%Y%m%d-%H%M%S')
  backup_file="$backup_dir/payment-gateway-$timestamp.dump"
  temporary="$backup_file.tmp"
  mkdir -p "$backup_dir"

  local -a compose=(docker compose --env-file "$env_file" -f "$compose_file")
  export PAYMENT_GATEWAY_ENV_FILE="$env_file"
  export PAYMENT_GATEWAY_IMAGE="$image"
  if [[ -z $("${compose[@]}" ps -q postgres) ]]; then
    fail "PostgreSQL is not running; use install for the first deployment"
    return 1
  fi
  log "backing up PostgreSQL to $backup_file"
  if ! "${compose[@]}" exec -T postgres sh -c 'exec pg_dump -U "$POSTGRES_USER" -Fc "$POSTGRES_DB"' >"$temporary"; then
    rm -f "$temporary"
    fail "PostgreSQL backup failed"
    return 1
  fi
  if [[ ! -s "$temporary" ]]; then
    rm -f "$temporary"
    fail "PostgreSQL backup is empty"
    return 1
  fi
  mv "$temporary" "$backup_file"
  chmod 600 "$backup_file"
}

wait_for_health() {
  local env_file=$1
  local compose_file=$2
  local image=$3
  local port health_url
  port=$(read_env_value "$env_file" HTTP_PORT)
  [[ -n "$port" ]] || port=8080
  health_url="http://127.0.0.1:$port/healthz"
  local -a compose=(docker compose --env-file "$env_file" -f "$compose_file")
  export PAYMENT_GATEWAY_ENV_FILE="$env_file"
  export PAYMENT_GATEWAY_IMAGE="$image"

  log "waiting for $health_url"
  local attempt
  for attempt in $(seq 1 90); do
    if curl --fail --silent --show-error --max-time 3 "$health_url" >/dev/null 2>&1; then
      log "health check passed"
      return 0
    fi
    sleep 1
  done
  "${compose[@]}" logs --tail=100 api >&2 || true
  fail "health check failed: $health_url"
}

deploy() {
  local mode=$1
  local env_file=$2
  local compose_file=$3
  local image=$4
  local skip_backup=$5
  local template="$repo_root/.env.production.example"

  if [[ "$mode" == "install" && ! -f "$env_file" ]]; then
    log "generating $env_file"
    generate_env "$template" "$env_file"
  fi
  validate_env "$env_file"
  require_commands

  mkdir -p "$repo_root/.tmp"
  exec 9>"$repo_root/.tmp/deploy.lock"
  flock -n 9 || fail "another deployment is already running" || return 1

  local -a compose=(docker compose --env-file "$env_file" -f "$compose_file")
  export PAYMENT_GATEWAY_ENV_FILE="$env_file"
  export PAYMENT_GATEWAY_IMAGE="$image"
  "${compose[@]}" config --quiet

  if [[ "$mode" == "update" && "$skip_backup" != "true" ]]; then
    backup_database "$env_file" "$compose_file" "$image"
  fi

  if [[ "$mode" == "install" ]]; then
    log "pulling application, PostgreSQL, and Redis images"
    "${compose[@]}" pull api postgres redis
  else
    log "pulling application image only"
    "${compose[@]}" pull api
  fi

  log "starting production services with $image"
  "${compose[@]}" up -d --no-build --remove-orphans
  wait_for_health "$env_file" "$compose_file" "$image"
  "${compose[@]}" ps

  local port
  port=$(read_env_value "$env_file" HTTP_PORT)
  [[ -n "$port" ]] || port=8080
  log "deployment complete: http://127.0.0.1:$port"
  log "configure Nginx or another TLS proxy to forward the public domain to this address"
}

main() {
  if [[ ${1:-} == "--help" || ${1:-} == "-h" ]]; then
    usage
    return 0
  fi
  local mode=${1:-}
  if [[ "$mode" != "install" && "$mode" != "update" ]]; then
    usage >&2
    return 2
  fi
  shift

  local env_file="$default_env_file"
  local compose_file="$default_compose_file"
  local image="$default_image"
  local skip_backup="false"
  while (($# > 0)); do
    case "$1" in
      --env-file)
        [[ $# -ge 2 ]] || fail "--env-file requires a path" || return 2
        env_file=$2
        shift 2
        ;;
      --image)
        [[ $# -ge 2 ]] || fail "--image requires a value" || return 2
        image=$2
        shift 2
        ;;
      --skip-backup)
        skip_backup="true"
        shift
        ;;
      -h | --help)
        usage
        return 0
        ;;
      *)
        fail "unknown option: $1"
        return 2
        ;;
    esac
  done

  if [[ "$env_file" != /* ]]; then
    env_file="$PWD/$env_file"
  fi
  deploy "$mode" "$env_file" "$compose_file" "$image" "$skip_backup"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
