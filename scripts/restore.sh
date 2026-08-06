#!/bin/bash
set -e

# Load .env
if [ ! -f .env ]; then
  echo "❌ .env file not found"
  exit 1
fi

export $(grep -v '^#' .env | xargs)

BACKUP_DIR="./backups"

if [ -z "$1" ]; then
  echo "❌ Usage: $0 <backup_file.tar.gz>"
  echo "   Example: $0 backups/pg_dump_20260806_120000.tar.gz"
  exit 1
fi

BACKUP_FILE="$1"

if [ ! -f "${BACKUP_FILE}" ]; then
  echo "❌ Backup file not found: ${BACKUP_FILE}"
  exit 1
fi

BACKUP_NAME=$(basename "${BACKUP_FILE}" .tar.gz)

echo "📦 Extracting backup file..."
tar -xzf "${BACKUP_FILE}" -C "${BACKUP_DIR}"

echo "📦 Copying backup to container..."
docker cp "${BACKUP_DIR}/${BACKUP_NAME}.dump" "aftercredits-postgres:/tmp/${BACKUP_NAME}.dump"

echo "📦 Starting Postgres restore..."
docker exec aftercredits-postgres pg_restore \
  -U "${POSTGRES_USER:-aftercredits}" \
  -d "${POSTGRES_DB:-aftercredits}" \
  --clean --if-exists \
  "/tmp/${BACKUP_NAME}.dump"

echo "🧹 Cleaning up..."
docker exec aftercredits-postgres rm -f "/tmp/${BACKUP_NAME}.dump"
rm -f "${BACKUP_DIR}/${BACKUP_NAME}.dump"

echo "✅ Restore completed!"

