# aftercredits-backend

This is the backend part of [this project](https://github.com/lealre/aftercredits)

It's written in Go, and the docker-compose file includes the respective MongoDB database image.

## Table of Contents

- [How to Run](#how-to-run)
  - [Prerequisites](#prerequisites)
  - [Setup](#setup)
- [Running Tests](#running-tests)
- [MongoDB Data Backup & Restore](#mongodb-data-backup--restore)
  - [1. Backup MongoDB Data (Local)](#1-backup-mongodb-data-local)
  - [2. Backup MongoDB Data to Google Drive](#2-backup-mongodb-data-to-google-drive)
  - [3. Scheduled Backups and Updates (Raspberry Pi / Cron)](#3-scheduled-backups-and-updates-raspberry-pi--cron)
  - [4. Restore MongoDB Data](#4-restore-mongodb-data)

## How to Run

### Prerequisites

- Go 1.24 or later
- Docker and Docker Compose

### Setup

1. **Copy the environment file:**

   ```bash
   cp env.example .env
   ```

2. **Edit `.env` file** with your configuration (MongoDB credentials, superuser details, etc.)

3. **Start MongoDB using Docker Compose:**

   ```bash
   docker-compose up -d
   ```

4. **Install Go dependencies:**

   ```bash
   go mod download
   ```

5. **Create database indexes:**

   ```bash
   go run cmd/database/main.go -indexes
   ```

   This will create all necessary indexes for users, ratings, and comments collections.

6. **Create a superuser (optional):**

   ```bash
   go run cmd/database/main.go -superuser
   ```

   This will create an admin user using the credentials from your `.env` file:

   - `SUPERUSER_USERNAME` (defaults to "admin" if not set)
   - `SUPERUSER_EMAIL` (optional)
   - `SUPERUSER_PASSWORD` (defaults to "admin" if not set)

7. **Run the application:**
   ```bash
   go run main.go
   ```

The server will start and connect to the MongoDB database. Make sure the MongoDB container is running before starting the application.

## Running Tests

The test folder contain the tests to the api. The test setup is using testcontainers to start a MongoDB container and run the tests.

To run the tests:

```bash
go test ./tests -v
```

This will run all the tests in the tests directory.

## MongoDB Data Backup & Restore

### 1. Backup MongoDB Data (Local)

Create a local backup of your MongoDB data:

```bash
./scripts/backup.sh
```

This script:
- Creates a logical backup using `mongodump` from the Docker container
- Stores backup in `./backups/` directory as a compressed tar.gz file
- Requires MongoDB to be running in Docker container named `aftercredits-mongo`

### 2. Backup MongoDB Data to Google Drive

Backup MongoDB data directly to Google Drive using rclone:

```bash
./scripts/backup_to_drive.sh
```

**Requirements:**
- rclone installed and configured
- Google Drive remote configured (default remote name: `drive-pi`)
- MongoDB accessible (either locally or via Docker container)

This script:
- Creates a logical backup using `mongodump`
- Compresses the backup
- Uploads to Google Drive at `drive-pi:aftercredits_backups/`
- Automatically cleans up temporary files

**Setup rclone:**
```bash
rclone config
# Create a remote named "drive-pi" pointing to your Google Drive
```

### 3. Scheduled Backups and Updates (Raspberry Pi / Cron)

For automated scheduled tasks (backups to Google Drive and movie information updates), see the `pi/` directory.

**Features:**
- **Automated MongoDB backups** to Google Drive using rclone
- **Automated movie information updates** from IMDb API
- Uses OS-level cron jobs for scheduling
- Runs in Docker containers with automatic cleanup

**Requirements:**
- rclone configured with Google Drive remote (see `pi/README.md` for details)
- Docker and Docker Compose
- MongoDB running in docker-compose network

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

### 4. Restore MongoDB Data

To restore data from a backup:

```bash
./scripts/restore.sh backups/mongo_dump_YYYYMMDD_HHMMSS.tar.gz
```

This script:

- Extracts the backup file
- Restores data using `mongorestore`
- Automatically cleans up temporary files
