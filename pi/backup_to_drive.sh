#!/bin/bash
set -e

# Function to log with timestamp (prints to console)
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

log ""
log "=========================================="
log "📦 Starting Postgres backup..."
log "=========================================="

# Set defaults if not provided (environment variables should be passed from .env)
POSTGRES_HOST=${POSTGRES_HOST:-localhost}
POSTGRES_PORT=${POSTGRES_PORT:-5432}

# The destination is configurable so this image is not welded to one remote
# name. The default is what the Pi has always used, so an existing deployment
# behaves identically without setting anything.
BACKUP_REMOTE=${BACKUP_REMOTE:-drive-pi:aftercredits_backups}

# Fail loudly and *accurately* before doing any work.
#
# These two failures look alike unless rclone's output is read closely, and
# confusing them is expensive: when the host's rclone.conf was left root-owned,
# a sibling project on the same machine reported "auth expired", and the remedy
# that implies (`rclone config reconnect`) failed the same way — because the
# real problem was that the file could not be opened at all. Say which it is.
preflight() {
  local cfg
  cfg="$(rclone config file 2>/dev/null | tail -1)"

  # The directory is checked before the file: if it is not traversable, the
  # file cannot even be stat'ed, `[ -e ]` reports false, and this would fall
  # through to the "remote not configured" branch below — reporting the wrong
  # cause, which is the exact failure mode this function exists to prevent.
  local cfg_dir
  cfg_dir="$(dirname "${cfg:-/nonexistent}")"
  if [ -n "${cfg}" ] && [ -d "${cfg_dir}" ] && [ ! -x "${cfg_dir}" ]; then
    log "Cannot ENTER the rclone config directory ${cfg_dir}"
    log "  This is a permissions problem, not an authentication one."
    log "  Check its owner: this container runs unprivileged."
    exit 1
  fi

  if [ -n "${cfg}" ] && [ -e "${cfg}" ] && [ ! -r "${cfg}" ]; then
    log "Cannot READ the rclone config at ${cfg}"
    log "  This is a permissions problem, not an authentication one."
    log "  This container runs unprivileged, so a config left owned by root"
    log "  is unreadable here. Check the file's owner before touching auth."
    exit 1
  fi

  if ! rclone listremotes 2>/dev/null | grep -q "^${BACKUP_REMOTE%%:*}:"; then
    log "Remote '${BACKUP_REMOTE%%:*}:' is not configured in ${cfg:-the rclone config}"
    log "  Configured remotes: $(rclone listremotes 2>/dev/null | tr '\n' ' ')"
    exit 1
  fi
}

preflight

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_NAME="pg_dump_${TIMESTAMP}"
TEMP_DIR="/tmp"
COMPRESSED_PATH="${TEMP_DIR}/${BACKUP_NAME}.tar.gz"

# Use pg_dump
log "📥 Using pg_dump..."
export PGPASSWORD="${POSTGRES_PASSWORD}"
if ! pg_dump -h "${POSTGRES_HOST}" -p "${POSTGRES_PORT}" \
    -U "${POSTGRES_USER:-aftercredits}" -d "${POSTGRES_DB:-aftercredits}" \
    --format=custom --file "${TEMP_DIR}/${BACKUP_NAME}.dump" 2>&1; then
  log "❌ pg_dump failed"
  exit 1
fi

# Compress the dump file into a .tar.gz (same format as scripts/backup.sh)
log "🗜️  Compressing backup file..."
if ! tar -czf "${COMPRESSED_PATH}" -C "${TEMP_DIR}" "${BACKUP_NAME}.dump" 2>&1; then
  log "❌ Compression failed"
  exit 1
fi

# Upload to Google Drive using rclone
log "☁️  Uploading to Google Drive..."
if ! rclone copy "${COMPRESSED_PATH}" "${BACKUP_REMOTE}" -v 2>&1; then
  log "❌ Upload to ${BACKUP_REMOTE} failed — rclone's own output is above"
  exit 1
fi

# Cleanup
log "🧹 Cleaning up temporary files..."
rm -f "${TEMP_DIR}/${BACKUP_NAME}.dump"
rm -f "${COMPRESSED_PATH}"

log "✅ Backup completed and uploaded to Google Drive:"
log "   ${BACKUP_REMOTE}/${BACKUP_NAME}.tar.gz"
log "=========================================="

