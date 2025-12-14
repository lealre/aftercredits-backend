#!/bin/bash
set -e

# Load .env
if [ ! -f .env ]; then
  echo "❌ .env file not found"
  exit 1
fi

export $(grep -v '^#' .env | xargs)

BACKUP_DIR="./backups"

# Check if backup file is provided
if [ -z "$1" ]; then
  echo "❌ Usage: $0 <backup_file.tar.gz>"
  echo "   Example: $0 backups/mongo_dump_20231214_160208.tar.gz"
  exit 1
fi

BACKUP_FILE="$1"

# Check if backup file exists
if [ ! -f "${BACKUP_FILE}" ]; then
  echo "❌ Backup file not found: ${BACKUP_FILE}"
  exit 1
fi

# Extract backup name from file path
BACKUP_NAME=$(basename "${BACKUP_FILE}" .tar.gz)
EXTRACT_DIR="${BACKUP_DIR}/${BACKUP_NAME}"

echo "📦 Extracting backup file..."
tar -xzf "${BACKUP_FILE}" -C "${BACKUP_DIR}"

echo "📦 Copying backup to container..."
docker cp "${EXTRACT_DIR}" "movies-mongo:/tmp/${BACKUP_NAME}"

echo "📦 Starting MongoDB restore..."
docker exec movies-mongo mongorestore \
  -u "${MONGO_USER}" \
  -p "${MONGO_PASSWORD}" \
  --authenticationDatabase admin \
  --drop \
  "/tmp/${BACKUP_NAME}"

echo "🧹 Cleaning up..."
docker exec movies-mongo rm -rf "/tmp/${BACKUP_NAME}"
rm -rf "${EXTRACT_DIR}"

echo "✅ Restore completed!"

