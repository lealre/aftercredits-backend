#!/bin/bash
set -e

# Get script directory and set paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"

# Function to log with timestamp (prints to console)
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

log "=========================================="
log "📦 Starting MongoDB backup..."

# Load .env from project root
if [ ! -f "${ENV_FILE}" ]; then
  log "❌ .env file not found at ${ENV_FILE}"
  exit 1
fi

export $(grep -v '^#' "${ENV_FILE}" | xargs)

# Set defaults if not provided
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

# Try to use mongodump directly first (create a DIRECTORY, not an --archive file)
if command -v mongodump &> /dev/null; then
  log "📥 Using mongodump directly (filesystem dump)..."
  if ! mongodump --uri "${MONGO_URI}" --out "${BACKUP_DIR_PATH}" 2>&1; then
    log "❌ mongodump failed"
    exit 1
  fi
# Otherwise, try using docker exec if container is running
elif docker ps --format '{{.Names}}' | grep -q "^aftercredits-mongo$"; then
  log "📥 Using mongodump via Docker container (filesystem dump)..."
  if ! docker exec aftercredits-mongo mongodump \
    -u "${MONGO_USER}" \
    -p "${MONGO_PASSWORD}" \
    --authenticationDatabase admin \
    --out "/tmp/${BACKUP_NAME}" 2>&1; then
    log "❌ mongodump via Docker failed"
    exit 1
  fi
  # Copy the dump DIRECTORY from the container to host /tmp, matching scripts/backup.sh layout
  if ! docker cp "aftercredits-mongo:/tmp/${BACKUP_NAME}" "${TEMP_DIR}/" 2>&1; then
    log "❌ Failed to copy dump directory from Docker container"
    exit 1
  fi
  docker exec aftercredits-mongo rm -rf "/tmp/${BACKUP_NAME}" 2>&1
else
  log "❌ mongodump not found and Docker container 'aftercredits-mongo' is not running"
  log "   Please install mongodb-tools or start the Docker container"
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