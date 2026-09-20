# Backup scheduling

The repository’s `scripts/ops/backup.sh` creates a PostgreSQL custom-format
dump and a SHA-256 checksum. The systemd units provide a reference daily
schedule for a dedicated staging or single-host deployment:

```bash
sudo install -d -m 0750 /etc/c4isr/secrets /var/lib/c4isr/backups
sudo install -m 0600 database_url /etc/c4isr/secrets/database_url
sudo install -m 0644 deployments/backup/systemd/c4isr-backup.service /etc/systemd/system/
sudo install -m 0644 deployments/backup/systemd/c4isr-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now c4isr-backup.timer
sudo systemctl start c4isr-backup.service
```

Production should run the equivalent job in the platform scheduler, upload each
dump and checksum to encrypted off-site object storage with retention and
immutability, and monitor the job result. Do not treat the local backup
directory as durable storage.
