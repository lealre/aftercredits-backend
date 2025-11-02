#!/bin/bash

# MongoDB Data Directory Backup Script
# This script backs up the MongoDB data directory to a compressed archive

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

# Create backup directory if it doesn't exist
mkdir -p "${BACKUP_DIR}"

echo "Starting MongoDB data backup..."
echo "Data directory: ${MONGO_DATA_PATH}"

# Check if data directory exists
if [ ! -d "${MONGO_DATA_PATH}" ]; then
    echo "Error: MongoDB data directory '${MONGO_DATA_PATH}' not found!"
    exit 1
fi

# Create backup using Docker to handle file permissions
echo "Creating backup: ${BACKUP_DIR}/${BACKUP_NAME}"
docker run --rm \
  -v "${MONGO_DATA_PATH}:/source:ro" \
  -v "$(pwd)/${BACKUP_DIR}:/backup" \
  alpine:latest \
  tar -czf "/backup/${BACKUP_NAME}" -C /source .

echo "✅ Backup completed successfully!"
echo "Backup file: ${BACKUP_DIR}/${BACKUP_NAME}"

# Show backup size
if [ -f "${BACKUP_DIR}/${BACKUP_NAME}" ]; then
    BACKUP_SIZE=$(du -h "${BACKUP_DIR}/${BACKUP_NAME}" | cut -f1)
    echo "Backup size: ${BACKUP_SIZE}"
fi