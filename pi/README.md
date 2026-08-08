# Scheduled Tasks Setup

This directory contains Dockerfiles and setup scripts for running scheduled tasks on a Raspberry Pi (or any Linux system).

## Overview

Two scheduled tasks are configured:
1. **Backup Task** - Backs up the Postgres database to Google Drive using `pg_dump` + rclone
2. **Movies Update Task** - Refreshes stored title metadata from the configured
   title provider (`TITLE_PROVIDER`, e.g. the TMDB + OMDb hybrid)

## Files

- `Dockerfile.backup` - Docker image for backup task
- `Dockerfile.routines` - Docker image for movies update task
- `backup_to_drive.sh` - Backup script run inside the backup image
- `setup-cron.sh` - Script to set up OS-level cron jobs
- `.env.example` - Example configuration for `pi/.env`
- `backup.log` - Log file for backup task (created automatically)
- `movies-update.log` - Log file for movies update task (created automatically)

## Deployment compose

This directory holds only the **scheduled-task** assets. The compose file that
runs the actual stack (`postgres`, `db-setup`, `backend`, `frontend`) lives in
the frontend repository as `docker-compose.yaml`, because the `frontend`
service is built from that repository's source. Deploy from a checkout of the
frontend repo on the Pi; the commands in the runbook below assume you are in
that directory.

## Setup

1. **Create and configure `.env` file** in the `pi/` directory:
   ```bash
   cd pi
   cp .env.example .env
   # Edit .env with your configuration
   ```

   The `.env` file should contain:
   - **Cron schedules**: `BACKUP_SCHEDULE` and `MOVIES_UPDATE_SCHEDULE`
   - **Docker network**: `DOCKER_NETWORK` (default: `aftercredits_default`)
   - **Postgres settings**: set `DEPLOY_ENV_FILE` to the absolute path of the
     compose stack's `.env` and `setup-cron.sh` passes it as an additional
     `--env-file`, so the database credentials and app config live in exactly one
     place. Leave it unset to define `POSTGRES_*` here instead.

   See `.env.example` for a complete example with comments.

2. **Ensure rclone config exists**:
   ```bash
   # Rclone config should be at:
   ~/.config/rclone/rclone.conf
   ```

3. **Run the setup script**:
   ```bash
   cd pi
   ./setup-cron.sh
   ```

The setup script will:
- Read configuration from `pi/.env` file
- Build Docker images for both tasks
- Create OS-level cron jobs
- Set up log files

**Note**: The setup script reads from `pi/.env`, not the root `.env` file. Make sure to configure your settings in `pi/.env`.

## Cron Schedule Format

Cron uses the following format:
```
* * * * *
│ │ │ │ │
│ │ │ │ └─── Day of week (0-7, Sunday = 0 or 7)
│ │ │ └───── Month (1-12)
│ │ └─────── Day of month (1-31)
│ └───────── Hour (0-23)
└─────────── Minute (0-59)
```

Examples:
- `0 0 * * 6` - Every Saturday at midnight (midnight after Friday)
- `0 0 * * 1` - Every Monday at midnight (midnight after Sunday)
- `0 0 * * 0` - Every Sunday at midnight
- `0 0 * * *` - Every day at midnight
- `*/5 * * * *` - Every 5 minutes
- `*/2 * * * *` - Every 2 minutes

## Viewing Logs

View backup logs:
```bash
tail -f pi/backup.log
```

View movies update logs:
```bash
tail -f pi/movies-update.log
```

## Managing Cron Jobs

View current cron jobs:
```bash
crontab -l
```

Edit cron jobs manually:
```bash
crontab -e
```

Remove all scheduled tasks:
```bash
crontab -e
# Then delete the lines with aftercredits-backup or aftercredits-routines
```

Re-run setup to update schedules:
```bash
cd pi
./setup-cron.sh
```

## How It Works

1. **OS-level cron** runs the scheduled tasks
2. Each task runs a **Docker container with `--rm` flag**:
   - Container starts, executes the task, then is automatically removed
   - No long-running containers
3. **All output** (stdout + stderr) is appended to log files
4. **Environment variables** are passed into containers using `--env-file` flag, making all `.env` variables available as environment variables inside the containers
5. **`.env` file** is also mounted as a file (read-only) for scripts that read it directly
6. **Rclone config** is mounted from `~/.config/rclone/rclone.conf`
7. **Docker network**: Containers join the docker-compose network (default: `aftercredits_default`) to communicate with the postgres container
8. **Postgres connection**: Uses `POSTGRES_HOST` from the deploy `.env` (set to
   `aftercredits-postgres`, the container name on the compose network)

## Troubleshooting

### Check if cron jobs are installed:
```bash
crontab -l | grep aftercredits
```

### Check Docker images:
```bash
docker images | grep aftercredits
```

### Test backup manually:
```bash
docker run --rm \
  --env-file .env \
  -v ~/.config/rclone/rclone.conf:/root/.config/rclone/rclone.conf:ro \
  -v $(pwd)/.env:/app/.env:ro \
  --network aftercredits_default \
  aftercredits-backup:latest
```

### Test movies update manually:
```bash
docker run --rm \
  --env-file .env \
  -v $(pwd)/.env:/app/.env:ro \
  --network aftercredits_default \
  aftercredits-routines:latest
```

### Check cron service:
```bash
# On systemd systems
sudo systemctl status cron

# On older systems
sudo service cron status
```

