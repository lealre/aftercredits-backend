#!/bin/bash
set -e

# Function to log with timestamp (prints to console)
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

log ""
log "=========================================="
log "📦 Starting MongoDB backup..."
log "=========================================="

# Set defaults if not provided (environment variables should be passed from .env)
MONGO_HOST=${MONGO_HOST:-localhost}
MONGO_PORT=${MONGO_PORT:-27017}
MONGODB_DB=${MONGODB_DB:-brunan}

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_NAME="mongo_dump_${TIMESTAMP}"
TEMP_DIR="/tmp"
# This will be a DIRECTORY (same structure as scripts/backup.sh)
BACKUP_DIR_PATH="${TEMP_DIR}/${BACKUP_NAME}"
COMPRESSED_PATH="${TEMP_DIR}/${BACKUP_NAME}.tar.gz"

# Build MongoDB URI
if [ -n "${MONGO_USER}" ] && [ -n "${MONGO_PASSWORD}" ]; then
  MONGO_URI="mongodb://${MONGO_USER}:${MONGO_PASSWORD}@${MONGO_HOST}:${MONGO_PORT}/?authSource=admin"
else
  MONGO_URI="mongodb://${MONGO_HOST}:${MONGO_PORT}"
fi

# Use mongodump directly (available in container)
log "📥 Using mongodump directly (filesystem dump)..."
if ! mongodump --uri "${MONGO_URI}" --out "${BACKUP_DIR_PATH}" 2>&1; then
  log "❌ mongodump failed"
  exit 1
fi

# Compress the dump directory into a .tar.gz (same format as scripts/backup.sh)
log "🗜️  Compressing backup directory..."
if ! tar -czf "${COMPRESSED_PATH}" -C "${TEMP_DIR}" "${BACKUP_NAME}" 2>&1; then
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
rm -rf "${BACKUP_DIR_PATH}"
rm -f "${COMPRESSED_PATH}"

log "✅ Backup completed and uploaded to Google Drive:"
log "   drive-pi:aftercredits_backups/${BACKUP_NAME}.tar.gz"
log "=========================================="

