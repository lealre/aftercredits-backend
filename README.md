# movies-backend

## MongoDB Data Backup & Restore

### 1. Backup MongoDB Data

Create a backup of your MongoDB data:

```bash
./scripts/backup-volume.sh
```

This script:
- Stores backup in `./backups/` directory
- Automatically cleans up old backups (older than 30 days)

### 2. Restore MongoDB Data

To restore data from a backup:

1. **Set the environment variable:**
   ```bash
   export MONGO_DATA_DIR="/path/to/your/data/directory"
   ```

2. **Extract the backup:**
   ```bash
   # Create data directory
   mkdir -p ${MONGO_DATA_DIR}/mongodb
   
   # Extract backup
   tar -xzf backups/mongo_volume_backup_YYYYMMDD_HHMMSS.tar.gz -C ${MONGO_DATA_DIR}/mongodb
   ```

3. **Start MongoDB:**
   ```bash
   docker compose up -d mongo
   ```