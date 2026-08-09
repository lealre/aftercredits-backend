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
