# Cleanup Jobs

Background jobs for database maintenance in the CBWeb3 API Gateway.

## delete_expired_quotes

Deletes expired swap quotes from the `swap_quotes` table to prevent unbounded growth.

### Usage

```bash
# Delete quotes older than 1 hour (default)
go run cmd/cleanup_jobs/delete_expired_quotes.go

# Delete quotes older than 24 hours
go run cmd/cleanup_jobs/delete_expired_quotes.go -retention-hours 24

# Dry run (check what would be deleted without actually deleting)
go run cmd/cleanup_jobs/delete_expired_quotes.go -dry-run
```

### Environment Variables

- `DATABASE_URL` (required): PostgreSQL connection string (e.g., `postgres://user:pass@localhost:5432/dbname?sslmode=disable`)

### Cron Setup

Run hourly via crontab:

```cron
# Delete quotes older than 1 hour every hour at :05
5 * * * * cd /path/to/api-gateway && /usr/local/go/bin/go run cmd/cleanup_jobs/delete_expired_quotes.go >> /var/log/api-gateway-cleanup.log 2>&1
```

Or use systemd timer:

```ini
# /etc/systemd/system/api-gateway-cleanup-quotes.service
[Unit]
Description=API Gateway - Delete Expired Quotes
After=network.target postgresql.service

[Service]
Type=oneshot
User=api-gateway
WorkingDirectory=/opt/api-gateway
Environment="DATABASE_URL=postgres://user:pass@localhost:5432/cbweb3_gateway?sslmode=disable"
ExecStart=/usr/local/go/bin/go run cmd/cleanup_jobs/delete_expired_quotes.go
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

```ini
# /etc/systemd/system/api-gateway-cleanup-quotes.timer
[Unit]
Description=API Gateway - Delete Expired Quotes (Hourly)

[Timer]
OnCalendar=hourly
Persistent=true

[Install]
WantedBy=timers.target
```

Enable timer:

```bash
sudo systemctl enable api-gateway-cleanup-quotes.timer
sudo systemctl start api-gateway-cleanup-quotes.timer
```

### Monitoring

Check logs:

```bash
# Systemd journal
journalctl -u api-gateway-cleanup-quotes.service -f

# Cron log
tail -f /var/log/api-gateway-cleanup.log
```

### Performance

- Default retention: 1 hour (quotes are valid for 15 seconds, retention provides buffer)
- Query uses indexed `created_at` column for fast deletion
- Typical runtime: <100ms for thousands of quotes
- No impact on API availability (runs as separate process)
