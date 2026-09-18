# aftercredits-backend

This is the backend part of [this project](https://github.com/lealre/aftercredits)

It's written in Go, and the docker-compose file includes the respective Postgres database image.

Title metadata (ratings, seasons/episodes, posters, cast) comes from a pluggable
provider selected by the `TITLE_PROVIDER` env var. The default deployment uses the
**hybrid** provider — TMDB for rich metadata plus OMDb for the real IMDb rating.
See **[Title metadata providers](internal/titleprovider/README.md)** for a
comparison of each provider's strengths and weaknesses and the required API keys.

Backend code conventions (handlers vs. services, error mapping, comment
invariants) are documented in **[docs/CONVENTIONS.md](docs/CONVENTIONS.md)**.

## Table of Contents

- [Title metadata providers](internal/titleprovider/README.md)
- [Code conventions](docs/CONVENTIONS.md)
- [How to Run](#how-to-run)
  - [Prerequisites](#prerequisites)
  - [Setup](#setup)
- [Metrics and dashboards](#metrics-and-dashboards)
  - [What is collected](#what-is-collected)
  - [Running it on the Pi](#running-it-on-the-pi)
- [Running Tests](#running-tests)
- [Database Backup & Restore](#database-backup--restore)
  - [1. Backup Postgres Data (Local)](#1-backup-postgres-data-local)
  - [2. Scheduled Backups and Updates (Raspberry Pi / Cron)](#2-scheduled-backups-and-updates-raspberry-pi--cron)
  - [3. Restore Postgres Data](#3-restore-postgres-data)

## How to Run

### Prerequisites

- Go 1.24 or later
- Docker and Docker Compose

### Setup

1. **Copy the environment file:**

   ```bash
   cp env.example .env
   ```

2. **Edit `.env` file** with your configuration (Postgres credentials, superuser details, etc.)

3. **Start Postgres using Docker Compose:**

   ```bash
   docker compose up -d postgres
   ```

4. **Install Go dependencies:**

   ```bash
   go mod download
   ```

5. **Apply schema migrations:**

   ```bash
   go run ./cmd/database -migrate
   ```

   This applies the embedded goose schema migrations, creating all necessary tables.

6. **Create a superuser (optional):**

   ```bash
   go run ./cmd/database -superuser
   ```

   This will create an admin user using the credentials from your `.env` file:

   - `SUPERUSER_USERNAME` (defaults to "admin" if not set)
   - `SUPERUSER_EMAIL` (optional)
   - `SUPERUSER_PASSWORD` (defaults to "admin" if not set)

7. **Run the application:**
   ```bash
   go run .
   ```

The server will start and connect to the Postgres database. Make sure the postgres container is running (and migrated) before starting the application.

## Metrics and dashboards

The server exposes Prometheus metrics on a **separate listener** (`:9090` by
default, `METRICS_ADDR`) — not a route on the API, and on by default
(`METRICS_ENABLED=false` turns it off; the API is unaffected either way). It is
never published on the Pi, and the local dev compose publishes it on `127.0.0.1`
only. Prometheus reaches it container-to-container, and nginx never sees it.
There is no authentication on it: it is safe because it is not published, which
is exactly why publishing it would expose it. That safety belongs to the
container run, not to the address — `METRICS_ADDR` defaults to `:9090`, which
binds every interface, so a bare `go run .` on a machine on a shared network
serves the whole exposition to anyone who asks. For non-container runs there,
set `METRICS_ADDR=127.0.0.1:9090`.

Prometheus and Grafana run as their own stack, deliberately separate from the
application one:

The backend exposes Prometheus metrics on a separate listener (`METRICS_ADDR`,
default `:9090`). Scraping them and rendering a dashboard is handled by the
host's own infrastructure setup, not by this repository.

> The first command runs the backend **as a container**, which replaces the
> `go run .` from step 7 of Setup — it does not sit alongside it. Running both
> gives whichever starts second `bind: address already in use` on 8080, and
> leaves `curl` reading whichever listener won. Stop `go run .` first.

**Raw metrics** (loopback only) — the port comes from `METRICS_PORT_HOST` in
`.env`, which is 9090 unless you remapped it:

```bash
# From the repo root, where .env lives:
port=$(grep -oE '^METRICS_PORT_HOST=[0-9]+' .env | cut -d= -f2)
curl -s "localhost:${port:-9090}/metrics" | head -3
```

The response must be `text/plain` and begin with `# HELP`. A `text/html` body
means an unrelated service already holds that port and you are reading *its*
metrics — a 200 from someone else's server is not a pass. (The default 9090 does
exactly that on the machine this was built on.)

**Grafana** is at http://localhost:3000 (port from `GRAFANA_PORT_HOST`, password
from `GRAFANA_ADMIN_PASSWORD`). The instance you want carries a dashboard named
**Aftercredits Backend**; another project's Grafana on the same port looks
entirely plausible and has no such dashboard.

Those are the default host ports. Set `API_PORT_HOST`, `METRICS_PORT_HOST` or
`GRAFANA_PORT_HOST` in `.env` to change one — the same escape hatch
`POSTGRES_PORT_HOST` already provides, and not a hypothetical one: on the
machine this stack was built on, all three defaults were held by unrelated
projects. Only the host side moves; nothing in the containers or in Prometheus's
scrape config changes.

The backend only ever *answers* a scrape. It never contacts Prometheus or
Grafana, so stopping the observability stack cannot affect the API.

Grafana's log is a clean error signal here: the two `level=error` lines it used
to print on every start (`provisioning.plugins` and `provisioning.alerting`)
were "directory does not exist", and both directories are now tracked with a
placeholder — `provisioning/plugins/.gitkeep` and
`provisioning/alerting/empty.yaml`. Anything at `level=error` in that log is
therefore worth reading.

### What is collected

| Area | Examples |
|---|---|
| Runtime | heap, RSS, goroutine count, GC pauses, CPU |
| HTTP | request rate by route, status codes, p50/p95/p99 latency, in-flight |
| Database | pool connections by state, acquires that waited, cancelled acquires |

Route labels use the registered route pattern (`POST /login`), never the
concrete path, so cardinality stays bounded. A request that never reached the
router — rejected by auth, or a path matching no route — is labelled
`unmatched`, which keeps those visible without letting callers mint series.

A dashboard covering the above is provisioned automatically from
`observability/grafana/` once the observability stack is up.

### Running it on the Pi

Set `STACK_NETWORK=aftercredits_default` in the deploy `.env` so the stack joins
the application network, and set `GRAFANA_ADMIN_PASSWORD`. No change to the
frontend repo's compose file is needed: Prometheus reaches the backend over the
shared network, so the metrics port never has to be published.

Prometheus keeps its samples for 15 days or 512MB, whichever comes first, in its
own volume — bounded on both axes so a growing number of series cannot fill the
disk.

## Running Tests

The test folder contain the tests to the api. The test setup is using testcontainers to start a Postgres container and run the tests.

To run the tests:

```bash
go test ./tests -v
```

This will run all the tests in the tests directory.

## Database Backup & Restore

### 1. Backup Postgres Data (Local)

Create a local backup of your Postgres data:

```bash
./scripts/backup.sh
```

This script:
- Creates a logical backup using `pg_dump` (custom format) from the Docker container
- Wraps the dump into a compressed tar.gz at `./backups/pg_dump_<timestamp>.tar.gz`
- Requires Postgres to be running in Docker container named `aftercredits-postgres`

**Setup rclone:**
```bash
rclone config
# Create a remote named "drive-pi" pointing to your Google Drive
```

### 2. Scheduled Backups and Updates (Raspberry Pi / Cron)

For automated scheduled tasks (backups to Google Drive and movie information updates), see the `pi/` directory.

**Features:**
- **Automated Postgres backups** to Google Drive using rclone
- **Automated movie information updates** from IMDb API
- Uses OS-level cron jobs for scheduling
- Runs in Docker containers with automatic cleanup

**Requirements:**
- rclone configured with Google Drive remote (see `pi/README.md` for details)
- Docker and Docker Compose
- Postgres running in docker-compose network

**Setup:**
1. Configure rclone with your Google Drive:
   ```bash
   rclone config
   # Create a remote named "drive-pi"
   ```

2. Set up scheduled tasks:
   ```bash
   cd pi
   cp .env.example .env
   # Edit .env with your configuration
   ./setup-cron.sh
   ```

**Schedules (configurable in `pi/.env`):**
- **Backup**: Saturday at midnight (midnight after Friday) - `0 0 * * 6`
- **Movies Update**: Monday at midnight (midnight after Sunday) - `0 0 * * 1`

For detailed documentation, see [pi/README.md](pi/README.md).

### 3. Restore Postgres Data

To restore data from a backup:

```bash
./scripts/restore.sh backups/pg_dump_YYYYMMDD_HHMMSS.tar.gz
```

This script:

- Extracts the backup file
- Restores data using `pg_restore --clean --if-exists`
- Automatically cleans up temporary files
