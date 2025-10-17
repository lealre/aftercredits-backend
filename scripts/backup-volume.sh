#!/bin/bash

# MongoDB Volume Backup Script
# This script backs up the MongoDB Docker volume data to your local machine
# Only creates backup if data has changed since last backup

set -e

# Configuration
VOLUME_NAME="movies-backend_mongo_data"
BACKUP_DIR="./backups"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_NAME="mongo_volume_backup_${TIMESTAMP}.tar.gz"
HASH_FILE="${BACKUP_DIR}/.last_backup_hash"

# Create backup directory if it doesn't exist
mkdir -p "${BACKUP_DIR}"

echo "Starting MongoDB volume backup..."
echo "Volume: ${VOLUME_NAME}"

# Check if volume exists
if ! docker volume inspect "${VOLUME_NAME}" > /dev/null 2>&1; then
    echo "Error: Volume '${VOLUME_NAME}' not found!"
    echo "Available volumes:"
    docker volume ls | grep movies
    exit 1
fi

# Calculate current data hash
echo "Calculating data hash..."
CURRENT_HASH=$(docker run --rm \
  -v "${VOLUME_NAME}:/source:ro" \
  alpine:latest \
  find /source -type f -exec md5sum {} \; | sort | md5sum | cut -d' ' -f1)

echo "Current data hash: ${CURRENT_HASH}"

# Check if we have a previous hash
if [ -f "${HASH_FILE}" ]; then
    LAST_HASH=$(cat "${HASH_FILE}")
    echo "Last backup hash: ${LAST_HASH}"
    
    if [ "${CURRENT_HASH}" = "${LAST_HASH}" ]; then
        echo "✅ Data has not changed since last backup. Skipping backup."
        echo "Last backup: $(ls -t ${BACKUP_DIR}/mongo_volume_backup_*.tar.gz 2>/dev/null | head -1)"
        exit 0
    else
        echo "🔄 Data has changed. Creating new backup..."
    fi
else
    echo "No previous backup found. Creating first backup..."
fi

# Create backup using tar through a temporary container
echo "Creating backup: ${BACKUP_DIR}/${BACKUP_NAME}"
docker run --rm \
  -v "${VOLUME_NAME}:/source:ro" \
  -v "$(pwd)/${BACKUP_DIR}:/backup" \
  alpine:latest \
  tar -czf "/backup/${BACKUP_NAME}" -C /source .

# Save current hash for next comparison
echo "${CURRENT_HASH}" > "${HASH_FILE}"

echo "✅ Backup completed successfully!"
echo "Backup file: ${BACKUP_DIR}/${BACKUP_NAME}"

# Show backup size
if [ -f "${BACKUP_DIR}/${BACKUP_NAME}" ]; then
    BACKUP_SIZE=$(du -h "${BACKUP_DIR}/${BACKUP_NAME}" | cut -f1)
    echo "Backup size: ${BACKUP_SIZE}"
fi

# Optional: Keep only last 7 days of backups
find "${BACKUP_DIR}" -name "mongo_volume_backup_*.tar.gz" -type f -mtime +7 -delete

echo "Old backups (older than 7 days) have been cleaned up."