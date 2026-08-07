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
   - **Postgres settings**: `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_DB`
   - **Cutover only**: `MONGO_*` settings, retained temporarily for the one-time Mongo -> Postgres
     migration (see [Cutover runbook](#cutover-runbook-one-time) below); removed after the cleanup PR

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
8. **Postgres connection**: Uses `POSTGRES_HOST` value from `.env` file (should be set to `aftercredits-postgres` when using docker-compose network)

## Cutover runbook (one-time)

This is the runbook for the one-time Mongo -> Postgres cutover of the production
Pi deployment. It runs once, on the Pi, against the live stack. Once the cleanup
PR lands (dropping the `mongo` service and the `mongo-to-postgres` binary from the
images), this section can be deleted.

Run these steps from the deploy directory on the Pi (where `docker-compose.yaml`
and `.env` live — a checkout of the frontend repository), in order:

0. **Update the deploy files on the Pi.** Pull the frontend repository's
   post-cutover `docker-compose.yaml` (it replaces the `mongo` service with
   `postgres` and changes `db-setup` to `-migrate && -superuser`), and update
   `.env` so it contains the `POSTGRES_*` block — see `pi/.env.example`. Every
   step below runs from that directory.

1. **Take a final Mongo backup.** Run the existing backup cron job once more (it
   is still pointing at the pre-cutover image, which does a `mongodump`) so you
   have a tarball that reflects the database at the moment of cutover. This is
   the rollback point if anything below goes wrong.

2. **Stop the old backend.** Take the running `backend` service down so nothing
   writes to Mongo while the migration runs, without touching the rest of the
   old stack:
   ```bash
   docker compose stop backend
   ```

3. **Bring up Postgres and apply the schema.** Start the new `postgres` service
   and wait for it to report healthy — `--wait` blocks until the `pg_isready`
   healthcheck passes, which matters here because the first boot runs `initdb`
   on the USB storage and is slow — then apply the schema migrations only. Do
   **not** run `-superuser` yet: the migration tool refuses to load into a
   non-empty database, and creating a superuser first would trip that check:
   ```bash
   docker compose up -d --wait postgres
   docker run --rm --network <network> --env-file .env \
     lealre/aftercredits-backend:latest /app/database -migrate
   ```

4. **Run the migration.** Extract the `mongodump` tarball from step 1 to a local
   directory. It unpacks to `mongo_dump_<ts>/<database>/…`, and `mongodump` also
   writes a sibling `admin/` directory containing its own `.bson` files.

   **Point `-dump` at the database directory itself**, not at its parent. Given
   the parent, the tool resolves the database directory by looking for a
   subdirectory named after `MONGO_DB` (defaulting to `aftercreditsdb`). If that
   name doesn't match the dump's actual database directory — because `.env`
   omits `MONGO_DB`, or because it carries the `aftercreditsdb` placeholder from
   `.env.example` while the real database is named something else — the tool
   falls back to scanning for subdirectories containing `.bson` files, finds
   both `admin/` and the real database, and stops with `multiple database
   subdirectories with .bson files`.

   Mount the extracted `mongo_dump_<ts>` directory and point `-dump` one level
   in:
   ```bash
   docker run --rm --network <network> --env-file .env \
     -v /path/to/mongo_dump_<ts>:/dump \
     lealre/aftercredits-backend:latest \
     /app/mongo-to-postgres -dump /dump/<database>
   ```
   It must print `✅ Migration complete` and exit `0`. If it doesn't, stop and
   re-check the dump before proceeding — nothing further should run against a
   partially loaded database.

5. **Create the superuser.** Now that the data is loaded:
   ```bash
   docker run --rm --network <network> --env-file .env \
     lealre/aftercredits-backend:latest /app/database -superuser
   ```

6. **Start the full new stack.**
   ```bash
   docker compose up -d
   ```
   (Compose will also run its own `db-setup` service as part of this; that's
   fine — `-migrate` is a no-op once the schema is at the latest version, and
   `-superuser` skips if the user already exists. Note `db-setup` runs the
   *schema* migrations only; it never re-runs the one-time data migration.)

7. **Smoke-test.** Log in from the frontend and browse a group to confirm the
   migrated data is reachable end-to-end.

8. **Leave Mongo intact.** Leave the old `mongo` container stopped-but-intact as
   a rollback option — don't remove its volume or image — until the cleanup PR
   lands.

9. **Rebuild the scheduled-task images.** Run `pi/setup-cron.sh` so the backup
   and movies-update images are rebuilt against the new (Postgres) code.

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

