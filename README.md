# aftercredits-backend

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