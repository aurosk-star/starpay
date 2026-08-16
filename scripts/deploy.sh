#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/.." && pwd)
default_env_file="$repo_root/.env.production"
default_compose_file="$repo_root/docker-compose.prod.yml"
default_image="ghcr.io/aurosk-star/starpay:latest"
default_keep_backups=7

if [[ -t 1 && -z ${NO_COLOR:-} ]]; then
  color_cyan=$'\033[0;36m'
  color_green=$'\033[0;32m'
  color_yellow=$'\033[1;33m'
  color_red=$'\033[0;31m'
  color_bold=$'\033[1m'
  color_reset=$'\033[0m'
else
  color_cyan=""
  color_green=""
  color_yellow=""
  color_red=""
  color_bold=""
  color_reset=""
fi

log() {
  printf '%s[deploy]%s %s\n' "$color_cyan" "$color_reset" "$*"
}

success() {
  printf '%s[deploy] OK:%s %s\n' "$color_green" "$color_reset" "$*"
}

warn() {
  printf '%s[deploy] WARN:%s %s\n' "$color_yellow" "$color_reset" "$*" >&2
}

fail() {
  printf '%s[deploy] ERROR:%s %s\n' "$color_red" "$color_reset" "$*" >&2
  return 1
}

usage() {
  cat <<'EOF'
Usage:
  scripts/deploy.sh
  scripts/deploy.sh install [options]
  scripts/deploy.sh update [options]

Modes:
  no arguments    Interactive deployment wizard.
  install         First deployment. Generates .env.production when absent.
  update          Backup PostgreSQL, pull the API image, and recreate services.

Options:
  --env-file PATH Use a production environment file at PATH.
  --image IMAGE   Deploy IMAGE (default: ghcr.io/aurosk-star/starpay:latest).
  --local-build   Build the API image from the current source tree.
  --skip-backup   Skip the PostgreSQL backup during update.
  --keep-backups N
                  Keep the newest N script-created backups (default: 7).
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

set_env_value() {
  local env_file=$1
  local key=$2
  local value=$3
  [[ -f "$env_file" ]] || fail "environment file not found: $env_file" || return 1

  local temporary
  temporary=$(mktemp "${env_file}.tmp.XXXXXX")
  if ! awk -v wanted="$key" -v replacement="$value" '
    BEGIN { updated = 0 }
    {
      line = $0
      sub(/^[[:space:]]*/, "", line)
      sub(/^export[[:space:]]+/, "", line)
      split(line, parts, "=")
      if (parts[1] == wanted) {
        if (!updated) {
          print wanted "=" replacement
          updated = 1
        }
        next
      }
      print
    }
    END {
      if (!updated) {
        print wanted "=" replacement
      }
    }
  ' "$env_file" >"$temporary"; then
    rm -f -- "$temporary"
    fail "failed to update $key in $env_file"
    return 1
  fi
  chmod 600 "$temporary"
  mv "$temporary" "$env_file"
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

input_file() {
  if [[ -n ${DEPLOY_INPUT_FD:-} ]]; then
    printf '%s' "$DEPLOY_INPUT_FD"
  elif [[ -t 0 && -r /dev/tty ]]; then
    printf '/dev/tty'
  else
    printf '/dev/stdin'
  fi
}

ask() {
  local prompt=$1
  local default=$2
  local variable_name=$3
  local input=""
  if [[ -n "$default" ]]; then
    printf '%s%s%s [%s]: ' "$color_bold" "$prompt" "$color_reset" "$default"
  else
    printf '%s%s%s: ' "$color_bold" "$prompt" "$color_reset"
  fi
  IFS= read -r input <"$(input_file)" || true
  printf -v "$variable_name" '%s' "${input:-$default}"
}

ask_choice() {
  local prompt=$1
  local default=$2
  local allowed=$3
  local variable_name=$4
  local answer
  while true; do
    ask "$prompt" "$default" answer
    if [[ " $allowed " == *" $answer "* ]]; then
      printf -v "$variable_name" '%s' "$answer"
      return 0
    fi
    warn "无效选择：$answer（可选：$allowed）"
  done
}

confirm() {
  local prompt=$1
  local default=${2:-n}
  local answer
  ask "$prompt" "$default" answer
  [[ "$answer" == "y" || "$answer" == "Y" || "$answer" == "yes" || "$answer" == "YES" ]]
}

detect_deployment_state() {
  local env_file=$1
  local compose_file=$2
  local image=$3
  local container_id=""
  container_id=$(docker ps -aq --filter 'name=^/payment-gateway-api$' 2>/dev/null || true)

  if [[ -f "$env_file" ]]; then
    DEPLOYMENT_STATE="update"
    DEPLOYMENT_REASON="检测到已有生产环境文件"
    return 0
  fi
  if [[ -n "$container_id" ]]; then
    fail "API container exists but environment file is missing: $env_file"
    return 1
  fi
  DEPLOYMENT_STATE="install"
  DEPLOYMENT_REASON="未检测到生产环境文件或已有 API 容器"
}

print_wizard_banner() {
  printf '\n%s%sStarPay 生产部署向导%s\n' "$color_bold" "$color_cyan" "$color_reset"
  printf '%s\n\n' '────────────────────────'
}

run_wizard() {
  local env_file="$default_env_file"
  local compose_file="$default_compose_file"
  local image="$default_image"
  local build_mode="image"
  local skip_backup="false"
  local keep_backups=$default_keep_backups
  local mode port bind_choice source_choice backup_choice

  print_wizard_banner
  detect_deployment_state "$env_file" "$compose_file" "$image"
  mode="$DEPLOYMENT_STATE"
  success "$DEPLOYMENT_REASON"

  local default_port default_bind
  default_port=$(read_env_value "$env_file" HTTP_PORT 2>/dev/null || true)
  [[ -n "$default_port" ]] || default_port=8080
  default_bind=$(read_env_value "$env_file" HTTP_BIND 2>/dev/null || true)
  [[ -n "$default_bind" ]] || default_bind=127.0.0.1

  ask "服务端口" "$default_port" port
  if ! [[ "$port" =~ ^[0-9]+$ ]] || ((port < 1 || port > 65535)); then
    fail "invalid HTTP port: $port"
    return 2
  fi

  printf '\n1) 仅本机访问（127.0.0.1，推荐）\n2) 全部网络（0.0.0.0）\n'
  [[ "$default_bind" == "0.0.0.0" ]] && bind_choice=2 || bind_choice=1
  ask_choice "监听范围" "$bind_choice" "1 2" bind_choice
  if [[ "$bind_choice" == "1" ]]; then
    default_bind="127.0.0.1"
  else
    default_bind="0.0.0.0"
    warn "服务将绑定全部网络；请使用防火墙、安全组和 HTTPS 反向代理"
  fi

  printf '\n1) 拉取 GHCR 预构建镜像（推荐）\n2) 使用当前源码本地构建\n'
  ask_choice "部署来源" "1" "1 2" source_choice
  if [[ "$source_choice" == "1" ]]; then
    ask "镜像" "$default_image" image
  else
    build_mode="local"
  fi

  if [[ "$mode" == "update" ]]; then
    if confirm "更新前备份 PostgreSQL？(y/n)" "y"; then
      ask "保留最近备份数量" "$default_keep_backups" keep_backups
      if ! [[ "$keep_backups" =~ ^[0-9]+$ ]] || ((keep_backups < 1)); then
        fail "backup retention must be an integer greater than or equal to 1"
        return 2
      fi
    else
      skip_backup="true"
      warn "本次更新将跳过 PostgreSQL 备份"
    fi
  fi

  printf '\n%s部署摘要%s\n' "$color_bold" "$color_reset"
  printf '  模式：%s\n' "$mode"
  printf '  地址：%s:%s\n' "$default_bind" "$port"
  if [[ "$build_mode" == "local" ]]; then
    printf '  来源：本地源码构建\n'
  else
    printf '  来源：%s\n' "$image"
    if [[ "$image" == *":latest" ]]; then
      warn "latest 是可变标签；生产发布推荐使用版本标签或镜像 digest"
    fi
  fi
  if [[ "$mode" == "update" ]]; then
    [[ "$skip_backup" == "true" ]] && printf '  备份：跳过\n' || printf '  备份：保留最近 %s 份\n' "$keep_backups"
  fi
  printf '  密钥：沿用或自动生成（不会显示）\n\n'

  if ! confirm "确认部署？(y/n)" "n"; then
    log "部署已取消"
    return 0
  fi

  if [[ "$mode" == "install" && ! -f "$env_file" ]]; then
    log "generating $env_file"
    generate_env "$repo_root/.env.production.example" "$env_file"
  fi
  set_env_value "$env_file" HTTP_PORT "$port"
  set_env_value "$env_file" HTTP_BIND "$default_bind"
  deploy "$mode" "$env_file" "$compose_file" "$image" "$skip_backup" "$build_mode" "$keep_backups"
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
  local build_mode=${6:-image}
  local keep_backups=${7:-$default_keep_backups}
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

  if [[ "$build_mode" == "local" ]]; then
    if [[ "$mode" == "install" ]]; then
      log "pulling PostgreSQL and Redis images"
      "${compose[@]}" pull postgres redis
    fi
    log "building application image from the current source tree"
    "${compose[@]}" build --pull api
  elif [[ "$mode" == "install" ]]; then
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
  if (($# == 0)); then
    run_wizard
    return $?
  fi
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
  local build_mode="image"
  local keep_backups=$default_keep_backups
  local image_explicit="false"
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
        image_explicit="true"
        shift 2
        ;;
      --local-build)
        build_mode="local"
        shift
        ;;
      --skip-backup)
        skip_backup="true"
        shift
        ;;
      --keep-backups)
        [[ $# -ge 2 ]] || fail "--keep-backups requires a value" || return 2
        keep_backups=$2
        shift 2
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

  if [[ "$build_mode" == "local" && "$image_explicit" == "true" ]]; then
    fail "--local-build cannot be used with --image"
    return 2
  fi
  if ! [[ "$keep_backups" =~ ^[0-9]+$ ]] || ((keep_backups < 1)); then
    fail "--keep-backups must be an integer greater than or equal to 1"
    return 2
  fi

  if [[ "$env_file" != /* ]]; then
    env_file="$PWD/$env_file"
  fi
  deploy "$mode" "$env_file" "$compose_file" "$image" "$skip_backup" "$build_mode" "$keep_backups"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
