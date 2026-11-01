#!/bin/bash
set -e

# Builds the movies-update image and prints the crontab line for it.
#
# It never edits the crontab. The Pi's crontab is shared with other projects and
# its lines are wrapped by pi-watcher's notifier-run (start/finish reports, a log
# file per run, alerts on a missed or failed run); an earlier version of this
# script deleted and rewrote its own lines, which silently dropped that wrapper.
# Install or change the line by hand with `crontab -e`.
#
# Backups are no longer done here: pi-watcher dumps this database with a
# read-only role and uploads it to Drive (see pi/README.md).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"
LOG_DIR="${SCRIPT_DIR}"

# Optional: absolute path to the deploy .env (the compose stack's env file). When
# set, the job reads the database credentials and app config from it, so those
# values live in exactly one place and pi/.env only carries the schedule and the
# docker network.
DEPLOY_ENV_FILE="${DEPLOY_ENV_FILE:-}"

# pi-watcher checkout, for the notifier-run wrapper. Without it the printed line
# runs the job unwrapped (still works, but nobody hears about a failed run).
PI_WATCHER_DIR="${PI_WATCHER_DIR:-${HOME}/Github/pi-watcher}"
NOTIFIER_URL="${NOTIFIER_URL:-http://192.168.1.29:8070}"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'
log() { echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

if [ ! -f "${ENV_FILE}" ]; then
    error ".env file not found at ${ENV_FILE} (copy pi/.env.example)"
    exit 1
fi
set -a
while IFS= read -r line || [ -n "$line" ]; do
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ -z "${line// }" ]] && continue
    if [[ "$line" =~ ^[[:alnum:]_]+= ]]; then
        export "$line" 2>/dev/null || true
    fi
done < "${ENV_FILE}"
set +a

MOVIES_UPDATE_SCHEDULE=${MOVIES_UPDATE_SCHEDULE:-"0 0 * * 1"}
# pi-infra's shared network: the shared Postgres lives there.
DOCKER_NETWORK=${DOCKER_NETWORK:-"infra"}

ROUTINES_ENV_ARGS="--env-file ${ENV_FILE}"
if [ -n "${DEPLOY_ENV_FILE}" ] && [ -f "${DEPLOY_ENV_FILE}" ]; then
    # Only the keys the job needs, so the container never sees JWT_SECRET.
    derived="${SCRIPT_DIR}/.env.routines"
    : > "${derived}"; chmod 600 "${derived}"
    for key in POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB POSTGRES_HOST POSTGRES_PORT \
               TITLE_PROVIDER TMDB_API_KEY OMDB_API_KEY ACTIVITY_RETENTION_DAYS; do
        grep -E "^[[:space:]]*${key}=" "${DEPLOY_ENV_FILE}" >> "${derived}" 2>/dev/null || true
    done
    ROUTINES_ENV_ARGS="--env-file ${derived} --env-file ${ENV_FILE}"
fi

log "Building routines image..."
docker build -f "${SCRIPT_DIR}/Dockerfile.routines" -t aftercredits-routines:latest "${PROJECT_ROOT}"
touch "${LOG_DIR}/movies-update.log"

JOB="docker run --rm ${ROUTINES_ENV_ARGS} -v ${ENV_FILE}:/app/.env:ro --network ${DOCKER_NETWORK} aftercredits-routines:latest >> ${LOG_DIR}/movies-update.log 2>&1"
if [ -x "${PI_WATCHER_DIR}/scripts/notifier-run" ]; then
    LINE="${MOVIES_UPDATE_SCHEDULE} cd \$HOME && set -a && . ${PI_WATCHER_DIR}/.env && set +a && NOTIFIER_URL=${NOTIFIER_URL} NOTIFIER_TOKEN=\$NOTIFIER_TOKEN_AFTERCREDITS NOTIFIER_LOG_DIR=\$HOME/pi-watcher-data/runlogs ${PI_WATCHER_DIR}/scripts/notifier-run aftercredits movies-update -- ${JOB}"
else
    LINE="${MOVIES_UPDATE_SCHEDULE} ${JOB}"
    log "pi-watcher not found at ${PI_WATCHER_DIR}: the line below is not reported to the notifier"
fi

log "Image ready. This script does not edit the crontab; add or update this line with 'crontab -e':"
echo
echo "${LINE}"
echo
if crontab -l 2>/dev/null | grep -qF "aftercredits-routines:latest"; then
    log "A movies-update line is already installed:"
    crontab -l | grep -F "aftercredits-routines:latest"
fi
