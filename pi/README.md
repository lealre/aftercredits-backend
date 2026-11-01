# Scheduled tasks on the Pi

This directory builds the one scheduled task that belongs to this repository:
the **movies update**, which refreshes stored title metadata from the configured
title provider (`TITLE_PROVIDER`, e.g. the TMDB + OMDb hybrid).

Two things that used to live here no longer do:

- **Backups** are done by **pi-watcher**, the Pi's shared jobs repository. It
  dumps this database with `pg_dump` as a read-only role (`pi_watcher_backup`,
  `pg_read_all_data`), checks the dump with `pg_restore --list`, uploads it to
  Drive (`pi-backups/aftercredits-backups`, 90 days) and reports each run to its
  notifier. Nothing in this repository runs a backup any more.
- **The crontab** is not managed by any script. Every job on the Pi is wrapped by
  pi-watcher's `notifier-run`, which reports start/finish, keeps a log file per
  run and alerts on a missed, hung or failed run. An earlier `setup-cron.sh`
  deleted and rewrote its own lines, which would drop that wrapper.

## Files

- `Dockerfile.routines` - image for the movies update (runs unprivileged)
- `setup-cron.sh` - builds that image and **prints** the crontab line; it never edits the crontab
- `.env.example` - example `pi/.env`
- `movies-update.log` - output of each run (created automatically)

The compose file for the running stack (`db-setup`, `backend`, `frontend`) lives
in the frontend repository. Postgres and the Cloudflare tunnel live in
**pi-infra**: one shared cluster, a database per service, each owned by its own
plain role.

## Setup

1. Configure `pi/.env` (schedule, network, `DEPLOY_ENV_FILE`):
   ```bash
   cd pi
   cp .env.example .env
   ```
2. Build the image and get the line:
   ```bash
   ./setup-cron.sh
   ```
3. Install or update the printed line with `crontab -e`. With pi-watcher checked
   out at `~/Github/pi-watcher` the line is wrapped by `notifier-run`; the token
   (`NOTIFIER_TOKEN_AFTERCREDITS`) comes from pi-watcher's `.env`.

After a backend release, re-run `./setup-cron.sh` on the Pi (from a `git pull`ed
checkout) so the next run uses the new code. The installed line doesn't change.

## How it works

1. OS-level cron starts `notifier-run aftercredits movies-update -- docker run --rm ...`.
2. The container connects to the shared Postgres over pi-infra's `infra` network
   (`POSTGRES_HOST` from the deploy `.env`), updates the titles and exits.
3. Output goes to `pi/movies-update.log` and to the run's log file under
   `~/pi-watcher-data/runlogs/aftercredits/movies-update/`; the notifier records
   the run with the path to that file.

## Restoring a backup

pi-watcher's backups are plain `pg_dump --format=custom` files:

```bash
# in pi-watcher: download one (newest first with `backup list aftercredits`)
docker compose run --rm backup fetch aftercredits aftercredits-YYYYMMDD_HHMMSS.dump
# then restore it into a database, as the admin role
docker cp ~/pi-watcher-data/staging/fetch/aftercredits-YYYYMMDD_HHMMSS.dump aftercredits-postgres:/tmp/r.dump
docker exec aftercredits-postgres pg_restore -U pgadmin -d <database> --no-owner /tmp/r.dump
```

Restore into a scratch database first and compare before replacing live data.

## Troubleshooting

```bash
crontab -l | grep aftercredits-routines        # the installed line
docker images | grep aftercredits-routines     # the image
tail -f pi/movies-update.log                   # last runs
```

Run the job once by hand (it is the real update, safe to repeat):

```bash
docker run --rm --env-file .env.routines --env-file .env -v $(pwd)/.env:/app/.env:ro \
  --network infra aftercredits-routines:latest
```
