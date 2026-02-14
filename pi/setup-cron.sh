#!/bin/bash
set -e

# Get script directory and project root (always use absolute paths)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"
PI_DIR="${SCRIPT_DIR}"
LOG_DIR="${PI_DIR}"

# Ensure all paths are absolute
ENV_FILE="$(cd "$(dirname "${ENV_FILE}")" && pwd)/$(basename "${ENV_FILE}")"
LOG_DIR="$(cd "${LOG_DIR}" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $*"
}

error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

# Check if .env file exists
if [ ! -f "${ENV_FILE}" ]; then
    error ".env file not found at ${ENV_FILE}"
    exit 1
fi

log "Loading environment variables from ${ENV_FILE}"

# Load environment variables from pi/.env file (only valid KEY=VALUE pairs)
# This safely exports only lines that match the pattern KEY=VALUE
set -a
while IFS= read -r line || [ -n "$line" ]; do
    # Skip comments and empty lines
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ -z "${line// }" ]] && continue
    # Only process lines that look like KEY=VALUE (starts with alphanumeric or underscore)
    if [[ "$line" =~ ^[[:alnum:]_]+= ]]; then
        export "$line" 2>/dev/null || true
    fi
done < "${ENV_FILE}"
set +a

# Set default schedules if not provided
BACKUP_SCHEDULE=${BACKUP_SCHEDULE:-"0 0 * * 6"}
MOVIES_UPDATE_SCHEDULE=${MOVIES_UPDATE_SCHEDULE:-"0 0 * * 1"}

# Set default Docker network (docker-compose network)
DOCKER_NETWORK=${DOCKER_NETWORK:-"aftercredits_default"}

log "Backup schedule: ${BACKUP_SCHEDULE}"
log "Movies update schedule: ${MOVIES_UPDATE_SCHEDULE}"
log "Docker network: ${DOCKER_NETWORK}"

# Ensure log directory exists
mkdir -p "${LOG_DIR}"
log "Log directory: ${LOG_DIR}"

# Create log files if they don't exist
touch "${LOG_DIR}/backup.log"
touch "${LOG_DIR}/movies-update.log"
log "Log files created/verified"

# Get absolute paths (already absolute, but ensure it)
RCLONE_CONFIG_DIR="${HOME}/.config/rclone"
RCLONE_CONFIG="${RCLONE_CONFIG_DIR}/rclone.conf"
ENV_FILE_ABS="${ENV_FILE}"
LOG_DIR_ABS="${LOG_DIR}"

# Check if rclone config directory exists
if [ ! -d "${RCLONE_CONFIG_DIR}" ]; then
    error "Rclone config directory not found at ${RCLONE_CONFIG_DIR}"
    exit 1
fi

# Check if rclone config file exists
if [ ! -f "${RCLONE_CONFIG}" ]; then
    error "Rclone config file not found at ${RCLONE_CONFIG}"
    exit 1
fi

log "Rclone config directory: ${RCLONE_CONFIG_DIR}"

# Build Docker images
log "Building Docker images..."

# Build backup image
log "Building backup image..."
docker build -f "${PI_DIR}/Dockerfile.backup" -t aftercredits-backup:latest "${PROJECT_ROOT}" || {
    error "Failed to build backup image"
    exit 1
}

# Build routines image
log "Building routines image..."
docker build -f "${PI_DIR}/Dockerfile.routines" -t aftercredits-routines:latest "${PROJECT_ROOT}" || {
    error "Failed to build routines image"
    exit 1
}

log "Docker images built successfully"

# Generate cron entries
# Use docker-compose network from aftercredits repository compose
# The backup script in pi/backup_to_drive.sh uses env vars directly
# The routines binary uses godotenv.Load() which reads from /app/.env (mounted)
BACKUP_CRON="${BACKUP_SCHEDULE} docker run --rm --env-file ${ENV_FILE_ABS} -v ${RCLONE_CONFIG_DIR}:/root/.config/rclone:rw --network ${DOCKER_NETWORK} aftercredits-backup:latest >> ${LOG_DIR_ABS}/backup.log 2>&1"

MOVIES_CRON="${MOVIES_UPDATE_SCHEDULE} docker run --rm --env-file ${ENV_FILE_ABS} -v ${ENV_FILE_ABS}:/app/.env:ro --network ${DOCKER_NETWORK} aftercredits-routines:latest >> ${LOG_DIR_ABS}/movies-update.log 2>&1"

# Create temporary crontab file
TEMP_CRONTAB=$(mktemp)

# Get existing crontab (ignore errors if no crontab exists)
crontab -l 2>/dev/null > "${TEMP_CRONTAB}" || true

# Remove existing cron jobs for these tasks (if they exist)
log "Removing existing cron jobs for backup and movies-update..."
sed -i '/aftercredits-backup:latest/d' "${TEMP_CRONTAB}"
sed -i '/aftercredits-routines:latest/d' "${TEMP_CRONTAB}"

# Add new cron jobs
log "Adding new cron jobs..."
echo "${BACKUP_CRON}" >> "${TEMP_CRONTAB}"
echo "${MOVIES_CRON}" >> "${TEMP_CRONTAB}"

# Install the new crontab
crontab "${TEMP_CRONTAB}"
rm "${TEMP_CRONTAB}"

log "Cron jobs installed successfully!"
log ""
log "Current crontab:"
crontab -l | grep -E "(aftercredits-backup|aftercredits-routines)" || true
log ""
log "Log files:"
log "  - Backup: ${LOG_DIR_ABS}/backup.log"
log "  - Movies Update: ${LOG_DIR_ABS}/movies-update.log"
log ""
log "To view logs:"
log "  tail -f ${LOG_DIR_ABS}/backup.log"
log "  tail -f ${LOG_DIR_ABS}/movies-update.log"
log ""
log "To remove cron jobs, run:"
log "  crontab -e"
log "  (then remove the lines with aftercredits-backup or aftercredits-routines)"

