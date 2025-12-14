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
BACKUP_NAME="mongo_dump_${TIMESTAMP}"

mkdir -p "${BACKUP_DIR}"

echo "📦 Starting MongoDB logical backup..."

docker exec movies-mongo mongodump \
  -u "${MONGO_USER}" \
  -p "${MONGO_PASSWORD}" \
  --authenticationDatabase admin \
  --out "/tmp/${BACKUP_NAME}"

docker cp "movies-mongo:/tmp/${BACKUP_NAME}" "${BACKUP_DIR}/"

docker exec movies-mongo rm -rf "/tmp/${BACKUP_NAME}"

tar -czf "${BACKUP_DIR}/${BACKUP_NAME}.tar.gz" -C "${BACKUP_DIR}" "${BACKUP_NAME}"
rm -rf "${BACKUP_DIR}/${BACKUP_NAME}"

echo "✅ Backup completed:"
echo "   ${BACKUP_DIR}/${BACKUP_NAME}.tar.gz"
