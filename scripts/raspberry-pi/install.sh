#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# go-trmnl (trmnld) installer (systemd, headless)
#
# Usage:
#   sudo bash install.sh [vX.Y.Z]
#
# Without a version, the latest GitHub release is installed. Re-running the
# script updates an existing installation (see README.md).
#
# Examples (override defaults via env vars):
#
#   sudo \
#     VERSION=v0.4.0 \
#     TRMNL_BASE_URL=http://trmnl.local:8080 \
#     TRMNL_LISTEN=:8080 \
#     TRMNL_DATA_DIR=/var/lib/trmnld \
#     bash install.sh
#
# Notes:
# - Downloads the release binary for your CPU (auto-detects armv7/arm64/amd64)
#   and verifies its SHA-256 checksum.
# - Installs a systemd unit for trmnld.
# - Safe to re-run: updates the binary and unit and restarts the service.
#   Existing values in the env file are kept.
# ==============================================================================

# Bump when the installer changes in a way that older copies must not undo.
# The self-update only switches to a downloaded installer with a revision at
# least this high, so a fixed installer never re-executes an older, buggy one.
INSTALLER_REVISION=2

REPO="gesellix/go-trmnl"
VERSION="${1:-${VERSION:-latest}}"
SERVICE_NAME="${SERVICE_NAME:-trmnld}"
BIN_PATH="${BIN_PATH:-/usr/local/bin/trmnld}"

CONFIG_DIR="${CONFIG_DIR:-/etc/trmnld}"
ENV_FILE="${ENV_FILE:-$CONFIG_DIR/trmnld.env}"
TRMNL_DATA_DIR="${TRMNL_DATA_DIR:-/var/lib/trmnld}"

SERVICE_USER="${SERVICE_USER:-trmnl}"
SERVICE_GROUP="${SERVICE_GROUP:-trmnl}"

# Default trmnld settings (only written for keys missing from the env file)
TRMNL_LISTEN="${TRMNL_LISTEN:-:8080}"
TRMNL_ADMIN_USER="${TRMNL_ADMIN_USER:-admin}"
TRMNL_ADMIN_PASSWORD="${TRMNL_ADMIN_PASSWORD:-}"
TRMNL_BASE_URL="${TRMNL_BASE_URL:-http://$(hostname).local:${TRMNL_LISTEN##*:}}"

# Internal variables
SCRIPT_PATH="$(realpath "$0" 2>/dev/null || echo "$0")"
IS_SELF_UPDATE="${IS_SELF_UPDATE:-false}"
TMP_DIR=""

log() { printf "\n==> %s\n" "$*"; }
die() { echo "ERROR: $*" >&2; exit 1; }

cleanup() {
  if [[ -n "${TMP_DIR}" ]]; then
    rm -rf "${TMP_DIR}"
  fi
}
trap cleanup EXIT

need_root() {
  [[ "${EUID}" -eq 0 ]] || die "Please run as root (e.g. sudo bash $0)."
}

ensure_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

apt_install_if_missing() {
  log "Installing dependencies: $*"
  apt-get update -y
  apt-get install -y --no-install-recommends "$@"
}

resolve_version() {
  if [[ "${VERSION}" == "latest" ]]; then
    local url
    url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest")" ||
      die "Could not determine the latest release (set VERSION=vX.Y.Z)."
    VERSION="${url##*/}"
    [[ "${VERSION}" =~ ^v[0-9] ]] || die "Unexpected latest release URL: ${url} (set VERSION=vX.Y.Z)."
    log "Latest release: ${VERSION}"
  elif [[ ! "${VERSION}" =~ ^v ]]; then
    VERSION="v${VERSION}"
  fi
}

detect_arch_asset() {
  # Upstream release naming: linux-armv7, linux-arm64, linux-amd64
  local m
  m="$(uname -m)"

  case "$m" in
    armv7l)
      echo "linux-armv7"
      ;;
    aarch64|arm64)
      echo "linux-arm64"
      ;;
    x86_64|amd64)
      echo "linux-amd64"
      ;;
    armv6l)
      die "ARMv6 (e.g. Raspberry Pi Zero/1) is not supported: release binaries need ARMv7 or newer."
      ;;
    *)
      die "Unsupported architecture from uname -m: $m (set ARCH_ASSET manually)"
      ;;
  esac
}

self_update() {
  # If we are already a self-update re-exec, don't do it again
  if [[ "$IS_SELF_UPDATE" == "true" ]]; then
    return
  fi

  local url="https://raw.githubusercontent.com/${REPO}/${VERSION}/scripts/raspberry-pi/install.sh"
  local tmp_script="${TMP_DIR}/install.sh"

  log "Checking for installer updates for ${VERSION}..."
  if ! curl -fsSL -o "${tmp_script}" "${url}"; then
    log "⚠️ Could not fetch installer for ${VERSION}, continuing with current script."
    return
  fi

  if cmp -s "${SCRIPT_PATH}" "${tmp_script}"; then
    log "Installer is already up to date."
    return
  fi

  local remote_revision
  remote_revision="$(sed -n 's/^INSTALLER_REVISION=\([0-9][0-9]*\)$/\1/p;T;q' "${tmp_script}")"
  if [[ -z "${remote_revision}" || "${remote_revision}" -lt "${INSTALLER_REVISION}" ]]; then
    log "Installer for ${VERSION} is older than this one, continuing with current script."
    return
  fi

  log "Newer installer found for ${VERSION}. Updating ${SCRIPT_PATH} and re-executing..."
  install -m 0755 "${tmp_script}" "${SCRIPT_PATH}"

  # Pass the resolved settings on to the new script
  export IS_SELF_UPDATE="true"
  export VERSION TRMNL_LISTEN TRMNL_BASE_URL TRMNL_DATA_DIR TRMNL_ADMIN_USER TRMNL_ADMIN_PASSWORD
  export BIN_PATH CONFIG_DIR ENV_FILE SERVICE_NAME SERVICE_USER SERVICE_GROUP

  cleanup
  exec bash "${SCRIPT_PATH}" "${VERSION}"
}

ensure_user_group() {
  log "Ensuring service user/group exist: ${SERVICE_USER}:${SERVICE_GROUP}"
  if ! getent group "${SERVICE_GROUP}" >/dev/null; then
    groupadd --system "${SERVICE_GROUP}"
  fi
  if ! id -u "${SERVICE_USER}" >/dev/null 2>&1; then
    useradd --system \
      --home-dir "${TRMNL_DATA_DIR}" \
      --no-create-home \
      --shell /usr/sbin/nologin \
      --gid "${SERVICE_GROUP}" \
      "${SERVICE_USER}"
  fi
}

ensure_dirs() {
  log "Creating directories"
  mkdir -p "${CONFIG_DIR}" "${TRMNL_DATA_DIR}"

  # Only chown recursively if the directory is not already owned by the service user
  if [[ "$(stat -c '%U:%G' "${TRMNL_DATA_DIR}")" != "${SERVICE_USER}:${SERVICE_GROUP}" ]]; then
    log "Adjusting ownership of ${TRMNL_DATA_DIR} to ${SERVICE_USER}:${SERVICE_GROUP}"
    chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "${TRMNL_DATA_DIR}"
  fi

  chmod 0755 "${CONFIG_DIR}"
  # The data dir holds the database and the credential encryption key.
  chmod 0750 "${TRMNL_DATA_DIR}"
}

download_binary() {
  local asset url name expected actual
  asset="${ARCH_ASSET:-$(detect_arch_asset)}"
  name="trmnld-${VERSION}-${asset}"
  url="https://github.com/${REPO}/releases/download/${VERSION}/${name}"

  log "Downloading binary for ${asset}: ${url}"
  curl -fsSL -o "${TMP_DIR}/trmnld" "${url}" || die "Download failed: ${url}"
  curl -fsSL -o "${TMP_DIR}/trmnld.sha256" "${url}.sha256" || die "Checksum download failed: ${url}.sha256"

  expected="$(awk '{print $1; exit}' "${TMP_DIR}/trmnld.sha256")"
  actual="$(sha256sum "${TMP_DIR}/trmnld" | awk '{print $1}')"
  [[ -n "${expected}" && "${expected}" == "${actual}" ]] ||
    die "Checksum mismatch for ${name} (expected ${expected:-<empty>}, got ${actual})."
  log "Checksum verified (${actual})"

  # Backup existing binary if it exists
  if [[ -f "${BIN_PATH}" ]]; then
    log "Backing up existing binary to ${BIN_PATH}.old"
    cp -p "${BIN_PATH}" "${BIN_PATH}.old"
  fi

  install -m 0755 "${TMP_DIR}/trmnld" "${BIN_PATH}"
  log "Installed binary to ${BIN_PATH}"
}

write_env_file() {
  log "Updating env file: ${ENV_FILE}"

  local vars=(
    "TRMNL_LISTEN=${TRMNL_LISTEN}"
    "TRMNL_BASE_URL=${TRMNL_BASE_URL}"
    "TRMNL_DATA_DIR=${TRMNL_DATA_DIR}"
    "TRMNL_ADMIN_USER=${TRMNL_ADMIN_USER}"
    "TRMNL_ADMIN_PASSWORD=${TRMNL_ADMIN_PASSWORD}"
  )

  # Create the file private from the start: it may contain the admin password.
  if [[ ! -f "${ENV_FILE}" ]]; then
    (umask 077 && : > "${ENV_FILE}")
  fi
  # group-readable so you can add yourself to the group if desired
  chown root:"${SERVICE_GROUP}" "${ENV_FILE}"
  chmod 0640 "${ENV_FILE}"

  # Make sure appended entries start on their own line
  if [[ -s "${ENV_FILE}" && -n "$(tail -c1 "${ENV_FILE}")" ]]; then
    echo >> "${ENV_FILE}"
  fi

  # Only add keys that are missing; existing values are never overwritten.
  local entry key
  for entry in "${vars[@]}"; do
    key="${entry%%=*}"
    if ! grep -q "^${key}=" "${ENV_FILE}"; then
      echo "${entry}" >> "${ENV_FILE}"
    fi
  done

  # Use the effective values for the health check and summary below.
  TRMNL_LISTEN="$(sed -n 's/^TRMNL_LISTEN=//p' "${ENV_FILE}" | tail -n1)"
  TRMNL_BASE_URL="$(sed -n 's/^TRMNL_BASE_URL=//p' "${ENV_FILE}" | tail -n1)"
}

write_systemd_unit() {
  log "Writing systemd unit: /etc/systemd/system/${SERVICE_NAME}.service"
  cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=go-trmnl (trmnld) Service
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
EnvironmentFile=${ENV_FILE}
WorkingDirectory=${TRMNL_DATA_DIR}
ExecStart=${BIN_PATH}

# Allow binding to privileged ports (80/443) without running as root
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

Restart=on-failure
RestartSec=2

# Sensible hardening
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=${TRMNL_DATA_DIR}

[Install]
WantedBy=multi-user.target
EOF
}

reload_enable_start() {
  log "Reloading systemd, enabling and starting service"
  systemctl daemon-reload
  systemctl enable "${SERVICE_NAME}.service"
  systemctl restart "${SERVICE_NAME}.service"

  log "Verifying service health..."
  # Extract the port from TRMNL_LISTEN (e.g. :8080 -> 8080)
  local port="${TRMNL_LISTEN##*:}"
  local health_url="http://localhost:${port}/healthz"
  local max_retries=5
  local count=0
  local success=false

  while [[ $count -lt $max_retries ]]; do
    if curl -fs "$health_url" >/dev/null 2>&1; then
      success=true
      break
    fi
    echo "Waiting for service to respond at $health_url... ($((count+1))/$max_retries)"
    sleep 2
    count=$((count+1))
  done

  if [[ "$success" = true ]]; then
    log "✅ Service is healthy and responding!"
  else
    log "⚠️ Service started but did not respond to /healthz at $health_url within timeout."
    log "Check logs with: journalctl -u ${SERVICE_NAME}.service -n 50"
  fi
}

show_status() {
  log "Service status"
  systemctl --no-pager --full status "${SERVICE_NAME}.service" || true

  local port="${TRMNL_LISTEN##*:}"
  log "Listening sockets (${port})"
  ss -tulpn | grep -E ":(${port})\b" || true

  cat <<EOF

Installed trmnld ${VERSION}.

Try from another machine:
  ${TRMNL_BASE_URL}

Logs:
  journalctl -u ${SERVICE_NAME}.service -e --no-pager

Unless you set TRMNL_SECRET_KEY, back up ${TRMNL_DATA_DIR}/secret.key:
without it, stored calendar credentials cannot be decrypted.
EOF
}

main() {
  need_root
  ensure_cmd systemctl
  ensure_cmd ss

  if ! command -v curl >/dev/null 2>&1; then
    apt_install_if_missing curl ca-certificates
  fi
  ensure_cmd sha256sum

  TMP_DIR="$(mktemp -d)"

  resolve_version
  self_update

  ensure_user_group
  ensure_dirs
  download_binary
  write_env_file
  write_systemd_unit
  reload_enable_start
  show_status
}

main "$@"
