#!/bin/bash
set -e

# Get script directory and project root (always use absolute paths)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"
PI_DIR="${SCRIPT_DIR}"
LOG_DIR="${PI_DIR}"

# Optional: absolute path to the deploy .env (the compose stack's env file). When
# set, the scheduled tasks read the database credentials and app config from it,
# so those values live in exactly one place and pi/.env only carries the schedules
# and the docker network. Set it in pi/.env or export it before running this
# script. When unset, the tasks use pi/.env alone, which is how this worked
# before.
DEPLOY_ENV_FILE="${DEPLOY_ENV_FILE:-}"

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
# A config directory of this project's own, NOT the shared ~/.config/rclone.
#
# The shared one is used by other projects on this machine, and sharing a single
# credential file across projects is what caused a real outage: this backup runs
# in a container, rclone rewrites the config on every OAuth refresh, and the
# rewrite handed ownership to whoever was running it. A sibling project that
# reads the same file host-side then lost access to its own credentials and its
# weekly backup failed silently for two cycles.
#
# Isolating the config is the fix that actually resolves this — not the fact
# that the job runs in a container. A containerized job still needs the
# credential and still gets it from a bind mount, so it would have caused the
# same damage pointed at the same shared file. Running unprivileged (below)
# stops this job writing root-owned files anywhere; a config of its own stops
# it touching anyone else's credentials at all. Both matter, for different
# reasons. Override with RCLONE_CONFIG_DIR if a deployment needs to.
RCLONE_CONFIG_DIR="${RCLONE_CONFIG_DIR:-${HOME}/.config/rclone-aftercredits}"
RCLONE_CONFIG="${RCLONE_CONFIG_DIR}/rclone.conf"

# Must match the UID the backup image is built for; the bind-mounted config has
# to be readable by the container's unprivileged user, and anything it writes
# back has to stay readable by this account.
BACKUP_UID="${BACKUP_UID:-$(id -u)}"
BACKUP_GID="${BACKUP_GID:-$(id -g)}"
ENV_FILE_ABS="${ENV_FILE}"
LOG_DIR_ABS="${LOG_DIR}"

# Build the --env-file list: deploy env first (credentials/app config), pi/.env
# second (schedules, network).
if [ -n "${DEPLOY_ENV_FILE}" ] && [ -f "${DEPLOY_ENV_FILE}" ]; then
    DEPLOY_ENV_ABS="$(cd "$(dirname "${DEPLOY_ENV_FILE}")" && pwd)/$(basename "${DEPLOY_ENV_FILE}")"
    ENV_FILE_ARGS="--env-file ${DEPLOY_ENV_ABS} --env-file ${ENV_FILE_ABS}"
    log "Using deploy env ${DEPLOY_ENV_ABS} for database credentials"
else
    ENV_FILE_ARGS="--env-file ${ENV_FILE_ABS}"
    log "DEPLOY_ENV_FILE not set (or not found); using only ${ENV_FILE_ABS}"
fi

# This project now keeps its own rclone config, so an existing machine that has
# only the shared one needs a one-time setup step. Authorizing a remote is
# interactive (it opens a browser flow), so it cannot be scripted here — say
# exactly what to run rather than failing with a bare "not found".
if [ ! -f "${RCLONE_CONFIG}" ]; then
    error "No rclone config for this project at ${RCLONE_CONFIG}"
    error ""
    error "This is expected the first time after the config was isolated."
    error ""
    error "Copy the existing one across — the refresh token travels with the"
    error "file, so no re-authorization is needed:"
    error ""
    error "  mkdir -p \"${RCLONE_CONFIG_DIR}\""
    error "  cp \"${HOME}/.config/rclone/rclone.conf\" \"${RCLONE_CONFIG}\""
    error ""
    error "Or authorize a fresh remote instead, if you would rather the two"
    error "configs not share a token at all:"
    error ""
    error "  RCLONE_CONFIG=\"${RCLONE_CONFIG}\" rclone config"
    error ""
    error "Either way the remote must be named as before ('drive-pi' by"
    error "default); set BACKUP_REMOTE in the env file to use another name."
    exit 1
fi

log "Rclone config directory: ${RCLONE_CONFIG_DIR}"

# Build Docker images
log "Building Docker images..."

# Build backup image
log "Building backup image..."
docker build -f "${PI_DIR}/Dockerfile.backup" \
    --build-arg "APP_UID=${BACKUP_UID}" --build-arg "APP_GID=${BACKUP_GID}" \
    -t aftercredits-backup:latest "${PROJECT_ROOT}" || {
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
# --user pins the container to this account, and the config is mounted at the
# image's HOME rather than root's. Still :rw, because rclone must persist a
# refreshed token — but a refresh now writes as this user, so the file stays
# readable by the host afterwards instead of flipping to root.
BACKUP_CRON="${BACKUP_SCHEDULE} docker run --rm --user ${BACKUP_UID}:${BACKUP_GID} ${ENV_FILE_ARGS} -v ${RCLONE_CONFIG_DIR}:/home/backup/.config/rclone:rw --network ${DOCKER_NETWORK} aftercredits-backup:latest >> ${LOG_DIR_ABS}/backup.log 2>&1"

MOVIES_CRON="${MOVIES_UPDATE_SCHEDULE} docker run --rm ${ENV_FILE_ARGS} -v ${ENV_FILE_ABS}:/app/.env:ro --network ${DOCKER_NETWORK} aftercredits-routines:latest >> ${LOG_DIR_ABS}/movies-update.log 2>&1"

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

