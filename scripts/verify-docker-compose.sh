#!/usr/bin/env bash
#
# verify-docker-compose.sh — validate and smoke-test all docker-compose*.yml files.
#
# Usage:
#   bash scripts/verify-docker-compose.sh
#   bash scripts/verify-docker-compose.sh docker-compose.yml docker-compose.with-infra.yml
#
set -euo pipefail

readonly SCRIPT_NAME="${0##*/}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly REPO_ROOT

GO_RSS_UI_SESSION_SECRET="${GO_RSS_UI_SESSION_SECRET:-}"
GO_RSS_UI_DB_PASSWORD="${GO_RSS_UI_DB_PASSWORD:-postgres}"
GO_RSS_UI_REDIS_PASSWORD="${GO_RSS_UI_REDIS_PASSWORD:-ci-redis}"
GO_RSS_UI_PORT="${GO_RSS_UI_PORT:-18082}"
COMPOSE_TIMEOUT_SECONDS="${COMPOSE_TIMEOUT_SECONDS:-180}"

INFRA_NETWORK="${INFRA_NETWORK:-infra}"
CI_POSTGRES_CONTAINER="${CI_POSTGRES_CONTAINER:-go-rss-ci-postgresql}"
CI_REDIS_CONTAINER="${CI_REDIS_CONTAINER:-go-rss-ci-redis}"

log() {
  printf '[%s] %s\n' "$SCRIPT_NAME" "$*"
}

die() {
  log "ERROR: $*" >&2
  exit 1
}

require_docker() {
  command -v docker >/dev/null 2>&1 || die "docker is required"
  docker compose version >/dev/null 2>&1 || die "docker compose plugin is required"
}

require_session_secret() {
  [[ -n "$GO_RSS_UI_SESSION_SECRET" ]] || die "GO_RSS_UI_SESSION_SECRET is required"
  [[ ${#GO_RSS_UI_SESSION_SECRET} -ge 32 ]] || die "GO_RSS_UI_SESSION_SECRET is too short"
}

wait_for_http() {
  local url="$1"
  local deadline=$((SECONDS + COMPOSE_TIMEOUT_SECONDS))

  while (( SECONDS < deadline )); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done

  return 1
}

write_env_file() {
  local env_file="$1"
  local include_db_secrets="${2:-false}"

  cat > "$env_file" <<EOF
GO_RSS_UI_SESSION_SECRET=${GO_RSS_UI_SESSION_SECRET}
GO_RSS_UI_PORT=${GO_RSS_UI_PORT}
EOF

  if [[ "$include_db_secrets" == true ]]; then
    cat >> "$env_file" <<EOF
GO_RSS_UI_DB_PASSWORD=${GO_RSS_UI_DB_PASSWORD}
GO_RSS_UI_REDIS_PASSWORD=${GO_RSS_UI_REDIS_PASSWORD}
EOF
  fi
}

validate_compose_file() {
  local compose_file="$1"
  local env_file="$2"

  log "validating compose config: ${compose_file}"
  docker compose -f "$compose_file" --env-file "$env_file" config >/dev/null
}

discover_compose_files() {
  local file
  shopt -s nullglob
  for file in "${REPO_ROOT}"/docker-compose*.yml; do
    printf '%s\n' "$(basename "$file")"
  done
}

start_shared_infra_stubs() {
  log "creating shared infra network: ${INFRA_NETWORK}"
  docker network inspect "$INFRA_NETWORK" >/dev/null 2>&1 || docker network create "$INFRA_NETWORK"

  log "starting stub PostgreSQL on network ${INFRA_NETWORK}"
  docker rm -f "$CI_POSTGRES_CONTAINER" >/dev/null 2>&1 || true
  docker run -d --name "$CI_POSTGRES_CONTAINER" --network "$INFRA_NETWORK" --network-alias postgresql \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD="$GO_RSS_UI_DB_PASSWORD" \
    -e POSTGRES_DB=go_rss_ui \
    postgres:15-alpine >/dev/null

  log "starting stub Redis on network ${INFRA_NETWORK}"
  docker rm -f "$CI_REDIS_CONTAINER" >/dev/null 2>&1 || true
  docker run -d --name "$CI_REDIS_CONTAINER" --network "$INFRA_NETWORK" --network-alias redis \
    redis:7-alpine redis-server --requirepass "$GO_RSS_UI_REDIS_PASSWORD" >/dev/null

  local deadline=$((SECONDS + 60))
  while (( SECONDS < deadline )); do
    if docker exec "$CI_POSTGRES_CONTAINER" pg_isready -U postgres >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done

  die "timed out waiting for stub PostgreSQL"
}

stop_shared_infra_stubs() {
  docker rm -f "$CI_POSTGRES_CONTAINER" "$CI_REDIS_CONTAINER" >/dev/null 2>&1 || true
  docker network rm -f "$INFRA_NETWORK" >/dev/null 2>&1 || true
}

remove_known_compose_containers() {
  docker rm -f go-rss-ui-app go-rss-ui-postgres go-rss-ui-redis >/dev/null 2>&1 || true
}

smoke_test_compose_file() {
  local compose_file="$1"
  local project_name="$2"
  local include_db_secrets="$3"
  local needs_shared_infra="$4"
  local env_file
  env_file="$(mktemp)"

  finish_smoke_test() {
    docker compose -f "$compose_file" --env-file "$env_file" down -v --remove-orphans >/dev/null 2>&1 || true
    rm -f "$env_file"
    if [[ "$needs_shared_infra" == true ]]; then
      stop_shared_infra_stubs
    fi
  }

  log "smoke test: ${compose_file}"

  remove_known_compose_containers
  write_env_file "$env_file" "$include_db_secrets"

  if [[ "$needs_shared_infra" == true ]]; then
    start_shared_infra_stubs
  fi

  validate_compose_file "$compose_file" "$env_file"

  export COMPOSE_PROJECT_NAME="$project_name"
  docker compose -f "$compose_file" --env-file "$env_file" build
  docker compose -f "$compose_file" --env-file "$env_file" up -d --remove-orphans

  if ! wait_for_http "http://127.0.0.1:${GO_RSS_UI_PORT}/"; then
    finish_smoke_test
    die "timed out waiting for http://127.0.0.1:${GO_RSS_UI_PORT}/"
  fi

  docker compose -f "$compose_file" --env-file "$env_file" ps
  log "smoke test passed: ${compose_file}"
  finish_smoke_test
}

validate_only_compose_file() {
  local compose_file="$1"
  local env_file
  env_file="$(mktemp)"

  write_env_file "$env_file" true
  validate_compose_file "$compose_file" "$env_file"
  rm -f "$env_file"
  log "validated config only (no smoke test): ${compose_file}"
}

run_compose_checks() {
  local compose_file="$1"

  case "$compose_file" in
    docker-compose.with-infra.yml)
      smoke_test_compose_file "$compose_file" "go-rss-ui-with-infra-ci" false false
      ;;
    docker-compose.yml)
      smoke_test_compose_file "$compose_file" "go-rss-ui-prod-ci" true true
      ;;
    *)
      validate_only_compose_file "$compose_file"
      ;;
  esac
}

main() {
  require_docker
  require_session_secret

  local compose_files=()
  local compose_file

  if (($# > 0)); then
    compose_files=("$@")
  else
    while IFS= read -r compose_file; do
      compose_files+=("$compose_file")
    done < <(discover_compose_files)
  fi

  ((${#compose_files[@]} > 0)) || die "no docker compose files to verify"

  cd "$REPO_ROOT"

  for compose_file in "${compose_files[@]}"; do
    [[ -f "$compose_file" ]] || die "compose file not found: ${compose_file}"
    run_compose_checks "$compose_file"
  done

  log "all docker compose files verified"
}

main "$@"
