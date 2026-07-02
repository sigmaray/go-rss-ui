#!/usr/bin/env bash
#
# setup-ubuntu-server.sh — prepare a fresh Ubuntu server for deploying go-rss-ui.
#
# Run on the server (requires root):
#   sudo bash scripts/setup-ubuntu-server.sh
#
# Full install (setup + git clone + docker compose up):
#   sudo GIT_REPO=https://github.com/you/go-rss-ui.git \
#        GO_RSS_UI_SESSION_SECRET="$(openssl rand -hex 64)" \
#        bash scripts/setup-ubuntu-server.sh --deploy
#
set -euo pipefail

readonly SCRIPT_NAME="${0##*/}"
readonly DEFAULT_APP_DIR="/opt/go-rss-ui"
readonly DEFAULT_APP_PORT="8082"

APP_DIR="${APP_DIR:-$DEFAULT_APP_DIR}"
APP_PORT="${APP_PORT:-$DEFAULT_APP_PORT}"
GIT_VERSION="${GIT_VERSION:-main}"

WITH_UFW=true
DO_DEPLOY=false
SKIP_CLONE=false
DOCKER_USER=""
GIT_REPO="${GIT_REPO:-}"
GO_RSS_UI_SESSION_SECRET="${GO_RSS_UI_SESSION_SECRET:-}"

log() {
  printf '[%s] %s\n' "$SCRIPT_NAME" "$*"
}

die() {
  log "ERROR: $*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Prepare an Ubuntu server for deploying go-rss-ui (Docker Compose).

Usage:
  sudo bash scripts/setup-ubuntu-server.sh [options]

Options:
  --no-ufw                  Do not configure UFW
  --app-dir PATH            Application directory (default: /opt/go-rss-ui)
  --docker-user USER        Add user to the docker group
  --deploy                  Clone repository and start docker compose
  --skip-clone              Do not clone repository (code already in --app-dir; for CI)
  -h, --help                Show this help

Environment variables (for --deploy):
  GIT_REPO                  Git repository URL (required unless --skip-clone)
  GO_RSS_UI_SESSION_SECRET        Session secret, 64+ bytes hex/base64 (required)
  GIT_VERSION               Branch or tag (default: main)
  APP_PORT                  Application port (default: 8082)

Examples:
  # Server setup only (Docker, UFW)
  sudo bash scripts/setup-ubuntu-server.sh

  # Setup + first deploy
  sudo GIT_REPO=https://github.com/you/go-rss-ui.git \
       GO_RSS_UI_SESSION_SECRET="$(openssl rand -hex 64)" \
       bash scripts/setup-ubuntu-server.sh --deploy
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --no-ufw)
        WITH_UFW=false
        shift
        ;;
      --app-dir)
        [[ $# -ge 2 ]] || die "option --app-dir requires an argument"
        APP_DIR="$2"
        shift 2
        ;;
      --docker-user)
        [[ $# -ge 2 ]] || die "option --docker-user requires an argument"
        DOCKER_USER="$2"
        shift 2
        ;;
      --deploy)
        DO_DEPLOY=true
        shift
        ;;
      --skip-clone)
        SKIP_CLONE=true
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1 (use --help)"
        ;;
    esac
  done
}

require_root() {
  [[ "${EUID:-$(id -u)}" -eq 0 ]] || die "run this script as root: sudo bash $SCRIPT_NAME"
}

check_ubuntu() {
  [[ -f /etc/os-release ]] || die "this script is intended for Ubuntu"

  # shellcheck disable=SC1091
  source /etc/os-release
  [[ "${ID:-}" == "ubuntu" ]] || die "only Ubuntu is supported (detected: ${ID:-unknown})"

  case "${VERSION_ID:-}" in
    22.04|24.04)
      log "Ubuntu ${VERSION_ID} (${VERSION_CODENAME:-})"
      ;;
    *)
      log "warning: tested on Ubuntu 22.04/24.04, you have ${VERSION_ID:-unknown}"
      ;;
  esac
}

apt_install() {
  log "updating apt cache"
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y --no-install-recommends "$@"
}

install_base_packages() {
  log "installing base packages"
  apt_install ca-certificates curl git gnupg
}

install_docker() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    log "Docker already installed: $(docker --version), $(docker compose version)"
    return 0
  fi

  log "installing Docker Engine and Compose plugin"

  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc

  local arch codename
  arch="$(dpkg --print-architecture)"
  # shellcheck disable=SC1091
  source /etc/os-release
  codename="${VERSION_CODENAME:?failed to determine Ubuntu codename}"

  cat > /etc/apt/sources.list.d/docker.list <<EOF
deb [arch=${arch} signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${codename} stable
EOF

  apt-get update -qq
  apt-get install -y --no-install-recommends \
    docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

  systemctl enable --now docker
  log "Docker installed: $(docker --version)"
}

add_docker_user() {
  local user="${DOCKER_USER:-${SUDO_USER:-}}"
  [[ -n "$user" && "$user" != "root" ]] || return 0

  if id -nG "$user" | grep -qw docker; then
    log "user ${user} is already in the docker group"
  else
    log "adding ${user} to the docker group"
    usermod -aG docker "$user"
    log "log in again as ${user} to use docker without sudo"
  fi
}

configure_ufw() {
  [[ "$WITH_UFW" == true ]] || return 0

  if ! command -v ufw >/dev/null 2>&1; then
    log "installing ufw"
    apt_install ufw
  fi

  log "configuring UFW (22, ${APP_PORT})"
  ufw --force reset >/dev/null
  ufw default deny incoming
  ufw default allow outgoing
  ufw allow 22/tcp comment 'SSH'
  ufw allow "${APP_PORT}/tcp" comment 'go-rss-ui app'
  ufw --force enable
  ufw status verbose
}

prepare_app_directory() {
  log "creating application directory: ${APP_DIR}"
  mkdir -p "${APP_DIR}"
  chown -R root:root "${APP_DIR}"
  chmod 0755 "${APP_DIR}"
}

write_env_file() {
  local env_file="${APP_DIR}/.env"
  log "writing ${env_file}"
  cat > "$env_file" <<EOF
GO_RSS_UI_SESSION_SECRET=${GO_RSS_UI_SESSION_SECRET}
GO_RSS_UI_ENV=production
GO_RSS_UI_PORT=${APP_PORT}
EOF
  chmod 0600 "$env_file"
}

validate_deploy_vars() {
  if [[ "$SKIP_CLONE" != true ]]; then
    [[ -n "$GIT_REPO" ]] || die "for --deploy set GIT_REPO (or use --skip-clone)"
  fi
  [[ -n "$GO_RSS_UI_SESSION_SECRET" ]] || die "for --deploy set GO_RSS_UI_SESSION_SECRET (openssl rand -hex 64)"
  [[ ${#GO_RSS_UI_SESSION_SECRET} -ge 32 ]] || die "GO_RSS_UI_SESSION_SECRET is too short (need 64+ bytes hex)"
}

clone_or_update_repo() {
  if [[ -d "${APP_DIR}/.git" ]]; then
    log "updating repository in ${APP_DIR}"
    git -C "$APP_DIR" fetch --all --prune
    git -C "$APP_DIR" checkout "$GIT_VERSION"
    git -C "$APP_DIR" pull --ff-only origin "$GIT_VERSION" 2>/dev/null || \
      git -C "$APP_DIR" reset --hard "origin/${GIT_VERSION}"
  else
    log "cloning ${GIT_REPO} -> ${APP_DIR}"
    rm -rf "${APP_DIR:?}"/*
    git clone --branch "$GIT_VERSION" --depth 1 "$GIT_REPO" "$APP_DIR"
  fi
}

deploy_application() {
  validate_deploy_vars
  prepare_app_directory
  if [[ "$SKIP_CLONE" == true ]]; then
    [[ -f "${APP_DIR}/docker-compose.yml" ]] || die "docker-compose.yml not found in ${APP_DIR} (--skip-clone)"
    log "using code in ${APP_DIR} (--skip-clone)"
  else
    clone_or_update_repo
  fi
  write_env_file

  log "building and starting docker compose"
  cd "$APP_DIR"
  export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-go-rss-ui}"
  docker compose build --pull
  docker compose up -d --remove-orphans

  log "waiting for containers to be ready"
  for _ in $(seq 1 30); do
    if docker compose ps --status running 2>/dev/null | grep -q "go-rss-ui-app"; then
      if curl -fsS "http://127.0.0.1:${APP_PORT}/" >/dev/null 2>&1; then
        log "application is responding on port ${APP_PORT}"
        return 0
      fi
    fi
    sleep 2
  done

  log "warning: application is not responding yet; check: docker compose -f ${APP_DIR}/docker-compose.yml ps"
}

print_next_steps() {
  cat <<EOF

=== Done ===

Application directory: ${APP_DIR}

Verification:
  docker compose -f ${APP_DIR}/docker-compose.yml ps
  curl -I http://127.0.0.1:${APP_PORT}/

EOF

  if [[ "$DO_DEPLOY" != true ]]; then
    cat <<EOF
Next step — deploy the application:
  1. Clone the repository into ${APP_DIR}
  2. Create ${APP_DIR}/.env (see .env.example)
  3. docker compose -f ${APP_DIR}/docker-compose.yml up -d --build

Or rerun with --deploy and GIT_REPO, GO_RSS_UI_SESSION_SECRET.

EOF
  fi
}

main() {
  parse_args "$@"
  require_root
  check_ubuntu

  install_base_packages
  install_docker
  add_docker_user
  configure_ufw

  if [[ "$DO_DEPLOY" == true ]]; then
    deploy_application
  else
    prepare_app_directory
  fi

  print_next_steps
}

main "$@"
