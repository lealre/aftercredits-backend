# aftercredits-backend

This is the backend part of [this project](https://github.com/lealre/aftercredits)

It's written in Go, and the docker-compose file includes the respective MongoDB database image.

## Table of Contents

- [How to Run](#how-to-run)
  - [Prerequisites](#prerequisites)
  - [Setup](#setup)
- [Running Tests](#running-tests)
- [MongoDB Data Backup & Restore](#mongodb-data-backup--restore)
  - [1. Backup MongoDB Data](#1-backup-mongodb-data)
  - [2. Restore MongoDB Data](#2-restore-mongodb-data)

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

### 1. Backup MongoDB Data

Create a backup of your MongoDB data:

```bash
./scripts/backup.sh
```

This script:

- Creates a logical backup using `mongodump`
- Stores backup in `./backups/` directory as a compressed tar.gz file

### 2. Restore MongoDB Data

To restore data from a backup:

```bash
./scripts/restore.sh backups/mongo_dump_YYYYMMDD_HHMMSS.tar.gz
```

This script:

- Extracts the backup file
- Restores data using `mongorestore`
- Automatically cleans up temporary files
