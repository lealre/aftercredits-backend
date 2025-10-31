#!/bin/bash

# MongoDB Data Directory Backup Script
# This script backs up the MongoDB data directory to a compressed archive
# Only creates backup if data has changed since last backup

set -e

# Load environment variables from .env file
if [ ! -f .env ]; then
    echo "Error: .env file not found!"
    echo "Please create a .env file with MONGO_DATA_DIR variable"
    exit 1
fi

# Source .env file
export $(grep -v '^#' .env | xargs)

# Check if MONGO_DATA_DIR is set
if [ -z "${MONGO_DATA_DIR}" ]; then
    echo "Error: MONGO_DATA_DIR is not set in .env file!"
    exit 1
fi

# Configuration
MONGO_DATA_PATH="${MONGO_DATA_DIR}/mongodb"
BACKUP_DIR="./backups"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_NAME="mongo_volume_backup_${TIMESTAMP}.tar.gz"
HASH_FILE="${BACKUP_DIR}/.last_backup_hash"

# Create backup directory if it doesn't exist
mkdir -p "${BACKUP_DIR}"

echo "Starting MongoDB data backup..."
echo "Data directory: ${MONGO_DATA_PATH}"

# Check if data directory exists
if [ ! -d "${MONGO_DATA_PATH}" ]; then
    echo "Error: MongoDB data directory '${MONGO_DATA_PATH}' not found!"
    exit 1
fi

# Calculate current data hash using Docker to handle permissions
echo "Calculating data hash..."
CURRENT_HASH=$(docker run --rm \
  -v "${MONGO_DATA_PATH}:/source:ro" \
  alpine:latest \
  find /source -type f -exec md5sum {} \; | sort | md5sum | cut -d' ' -f1)

echo "Current data hash: ${CURRENT_HASH}"

# Check if we have a previous hash
if [ -f "${HASH_FILE}" ]; then
    LAST_HASH=$(cat "${HASH_FILE}")
    echo "Last backup hash: ${LAST_HASH}"
    
    if [ "${CURRENT_HASH}" = "${LAST_HASH}" ]; then
        echo "❌ Data has not changed since last backup. Failing as requested."
        echo "Last backup: $(ls -t ${BACKUP_DIR}/mongo_volume_backup_*.tar.gz 2>/dev/null | head -1)"
        exit 1
    else
        echo "🔄 Data has changed. Creating new backup..."
    fi
else
    echo "No previous backup found. Creating first backup..."
fi

# Create backup using Docker to handle file permissions
echo "Creating backup: ${BACKUP_DIR}/${BACKUP_NAME}"
docker run --rm \
  -v "${MONGO_DATA_PATH}:/source:ro" \
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