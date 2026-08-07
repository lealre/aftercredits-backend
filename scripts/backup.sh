#!/bin/bash
set -e

# Load .env
if [ ! -f .env ]; then
  echo "❌ .env file not found"
  exit 1
fi

export $(grep -v '^#' .env | xargs)

BACKUP_DIR="./backups"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_NAME="pg_dump_${TIMESTAMP}"

mkdir -p "${BACKUP_DIR}"

echo "📦 Starting Postgres logical backup..."

docker exec aftercredits-postgres pg_dump \
  -U "${POSTGRES_USER:-aftercredits}" \
  -d "${POSTGRES_DB:-aftercredits}" \
  --format=custom \
  --file "/tmp/${BACKUP_NAME}.dump"

docker cp "aftercredits-postgres:/tmp/${BACKUP_NAME}.dump" "${BACKUP_DIR}/"

docker exec aftercredits-postgres rm -f "/tmp/${BACKUP_NAME}.dump"

tar -czf "${BACKUP_DIR}/${BACKUP_NAME}.tar.gz" -C "${BACKUP_DIR}" "${BACKUP_NAME}.dump"
rm -f "${BACKUP_DIR}/${BACKUP_NAME}.dump"

echo "✅ Backup completed:"
echo "   ${BACKUP_DIR}/${BACKUP_NAME}.tar.gz"
