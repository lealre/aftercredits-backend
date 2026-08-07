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
if ! rclone copy "${COMPRESSED_PATH}" "drive-pi:aftercredits_backups" -v 2>&1; then
  log "❌ Upload to Google Drive failed"
  exit 1
fi

# Cleanup
log "🧹 Cleaning up temporary files..."
rm -f "${TEMP_DIR}/${BACKUP_NAME}.dump"
rm -f "${COMPRESSED_PATH}"

log "✅ Backup completed and uploaded to Google Drive:"
log "   drive-pi:aftercredits_backups/${BACKUP_NAME}.tar.gz"
log "=========================================="

