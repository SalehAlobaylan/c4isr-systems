# C4ISR operations runbook

This runbook is the minimum operating procedure for staging and production. The
Compose stack is a staging reference deployment; production should use the same
separation of concerns with a managed PostgreSQL/PostGIS service, a secret
manager, durable object storage, and an authenticated central observability
platform.

## Operating targets

The initial targets are an RPO of 24 hours and an RTO of 60 minutes. Production
owners should tighten these values before launch and record the approved values
with the deployment configuration.

Never expose PostgreSQL, Prometheus, Loki, Alertmanager, or Grafana directly to
the public internet. Put the operator UI, API, and observability interfaces
behind the organization’s TLS ingress, identity provider, and network policy.

## Staging

Create file-backed secrets once on a trusted workstation or staging host:

```bash
task staging:init
task ops:validate
task staging:up
task staging:status
task staging:smoke
```

The generated files live under `deployments/staging/.secrets/` and are ignored
by Git. The server reads `C4ISR_DATABASE_URL_FILE` and
`C4ISR_AUTH_TOKENS_FILE`; the UI reads its token only at container startup and
serves it from the same-origin runtime configuration. This is still a bearer
token visible to the browser, so staging should use a dedicated, rotatable
operator token rather than a production credential.

`task staging:init` is idempotent when all five secret files are present; use
`FORCE=1 task staging:init` only when an intentional local rotation is needed.

`task staging:smoke` is an executable readiness check. It validates public
health and metrics, authenticated operator access, the UI shell, Prometheus,
Alertmanager, Loki, Alloy, and Grafana. It allows a bounded startup window for
observability services to converge, then fails when the stack is down or a
required secret or endpoint is missing; it does not mutate application data.

Open:

- Operator UI: <http://localhost:8081>
- Grafana: <http://localhost:3001>
- Prometheus: <http://localhost:9090>
- Alertmanager: <http://localhost:9093>
- Alloy health/debug UI: <http://localhost:12345>

Set `ALERTMANAGER_WEBHOOK_URL` in `deployments/staging/.env.staging` to the
staging incident or chat webhook. The default local placeholder keeps alerts
visible in Alertmanager but does not page anyone.

Stop staging without deleting evidence or persistent volumes:

```bash
task staging:down
```

## Secrets and rotation

Production must inject secrets through a manager such as the platform’s native
secret store, Vault, or an equivalent workload identity. Mount these values as
files and set the corresponding `_FILE` variables:

- `C4ISR_DATABASE_URL_FILE`
- `C4ISR_AUTH_TOKENS_FILE`
- `C4ISR_UI_TOKEN_FILE` for the UI runtime container
- `GF_SECURITY_ADMIN_PASSWORD__FILE` for Grafana

Do not put tokens, database URLs, backup contents, or webhook credentials in
GitHub Actions logs, Docker image layers, Compose YAML, or application logs.

To rotate a bearer token, provision the replacement token under a second
operator entry, deploy it, verify `/api/v1/auth/me`, then remove the old entry
and redeploy. Rotate the database password by creating the replacement
credential in the managed database first, updating the mounted DSN, and
restarting the API only after a connectivity check succeeds.

## Database backup and restore

Backups use PostgreSQL’s custom format, omit ownership/privilege statements,
and write a SHA-256 checksum beside the dump. Run them from a host with the
PostgreSQL client tools:

```bash
task db:backup
task db:backup BACKUP=backups/c4isr-before-migration.dump
task staging:backup BACKUP=backups/c4isr-staging-before-migration.dump
```

Production scheduling must run this at least daily, copy the dump and checksum
to encrypted off-site object storage, retain daily/weekly/monthly recovery
points, and enable object lock or an equivalent immutability policy. A local
Docker volume is not a backup.

Every backup must pass a restore drill before it is considered usable:

```bash
task db:restore:test BACKUP=backups/c4isr-before-migration.dump
```

The restore test creates a uniquely named temporary database, restores the
dump, verifies PostGIS, Goose metadata, and the application schema, then drops
only that temporary database. To use a separate administrative connection, set
`C4ISR_DATABASE_ADMIN_URL` or `C4ISR_DATABASE_ADMIN_URL_FILE`.

The restore drill should run weekly in staging and quarterly against a fresh
production-like environment. Record the dump identifier, restore duration,
row/schema verification result, and operator who performed the drill.

## Migration procedure

Migrations are forward-only deployment artifacts. The API does not mutate the
schema on startup.

1. Review the SQL and its rollback implications.
2. Run a fresh backup and verify its checksum.
3. Apply the migration in staging:

   ```bash
   C4ISR_ENV=staging task db:status
   C4ISR_ENV=staging task db:migrate
   ```

4. Run `task test:integration` and the operator smoke flow against staging.
5. Apply the same migration in production during the approved change window.
6. Verify `/health`, `/metrics`, error rate, and the main operator workflow.

Rollback is not an automatic response. It is allowed in staging, but staging or
production rollback requires an explicit approval gate:

```bash
C4ISR_ENV=staging C4ISR_ALLOW_MIGRATION_DOWN=1 task db:rollback
```

For destructive or irreversible changes, restore into a new database and cut
over rather than running `down` against the live database.

## Centralized logs, metrics, and alerts

The server emits JSON logs in staging/production with `service`, `environment`,
`version`, request id, trace id, operator id/role, route, status, bytes, and
duration. Alloy discovers Docker containers and forwards their logs to Loki;
Grafana provisions both Loki and Prometheus automatically.

The API exposes low-cardinality Prometheus metrics at `/metrics`, including:

- request count, response bytes, and duration totals;
- database health;
- authentication failures;
- domain event aggregates;
- observation/telemetry throughput, rejection rate, stale entities, track and
  geofence latency, realtime publish latency, command acknowledgment latency,
  and database query latency.

Prometheus evaluates the rules in `deployments/observability/alerts.yml` and
routes grouped notifications through Alertmanager. The initial alerts are:

| Alert | Severity | First response |
| --- | --- | --- |
| `C4ISRAPIDown` | critical | Check API container, `/health`, deployment rollout, and recent logs. |
| `C4ISRDatabaseUnavailable` | critical | Check database availability, credentials, pool errors, and provider events. |
| `C4ISRHighServerErrorRate` | warning | Filter Loki by `level="ERROR"` and inspect the affected route/status. |
| `C4ISRHighRequestLatency` | warning | Check database latency, pool saturation, and recent migration or load changes. |
| `C4ISRAuthenticationFailures` | warning | Check token rotation, ingress clients, and possible credential abuse. |
| `C4ISRObservationRejectionRate` | warning | Inspect validation errors, source registration, input coordinates, and source health. |
| `C4ISRStaleEntities` | warning | Check telemetry producers, connectivity, asset clocks, and the last-seen view. |
| `C4ISRCommandAcknowledgmentLatency` | warning | Inspect transport connectivity, command state transitions, and asset availability. |
| `C4ISRDatabaseQueryLatency` | warning | Check pool saturation, query plans, locks, and database provider health. |
| `C4ISRWebSocketPublishLatency` | warning | Check slow clients, bounded buffers, proxy timeouts, and realtime connection counts. |

Each alert carries a `runbook` label. On an alert, capture the alert payload,
request/trace ids from logs, the current migration status, and the relevant
operator-visible failure before changing state. Do not resolve an alert until
the underlying signal has recovered and the operator workflow has been
retested.

### API unavailable

Check the API container and ingress first, then `/health`, recent deployment
events, and centralized logs. If the database is healthy, restart only the API
workload; do not remove scenario or evidence volumes while investigating.

### Database unavailable

Check the managed database/provider status, mounted DSN secret, migration
status, connection limits, and recent credential rotation. Keep the API in its
degraded/readiness state until a successful health probe and a read-only
operator request both succeed.

### High server error rate

Group Loki logs by route, status, request ID, and trace ID. Reproduce the
affected operator action in staging, capture the response envelope, and stop
the rollout if the error is tied to a migration or contract change.

### High request latency

Compare HTTP latency with database-query latency and pool saturation. Inspect
recent migrations and traffic changes before increasing timeouts; a timeout
increase must not hide a failing command or scenario lifecycle.

### Authentication failures

Verify the expected token was rotated into the secret manager and that the
operator identity still exists. Review ingress clients and request IDs for
credential abuse before issuing a replacement token.

### Observation rejection rate

Inspect rejected-observation domain logs and the source/track provenance. Check
validation errors, unknown source IDs, coordinate bounds, and upstream schema
changes. Rejected evidence is retained in audit/log context but must not be
silently promoted into a track.

### Stale entities

Open the operator map and asset detail view to identify last-seen timestamps.
Check source connectivity and telemetry timestamps for clock skew or delayed
delivery. The API marks an asset stale after `C4ISR_TELEMETRY_STALE_AFTER`
(default `5m`) without accepted telemetry. Keep the stale state visible until
a newer accepted sample clears it.

### Command acknowledgment latency

Trace the command ID from `SENT` through `ACKNOWLEDGED`. Check the selected
asset's connection and availability, transport/simulator health, and whether
the command entered a terminal failure state. Do not manually mark a command
complete to clear the alert.

### Database query latency

Correlate query-latency metrics with API routes and database provider metrics.
Inspect locks, indexes, pool usage, and the latest migration. Use a read-only
staging reproduction and restore drill before applying a production schema
change.

### WebSocket publish latency

Check realtime connection counts and slow-client disconnect logs. Verify the
reverse proxy's WebSocket upgrade and timeout settings, then inspect whether a
single client is repeatedly filling the bounded send buffer.

## Readiness checks

Before declaring a deployment healthy:

```bash
curl --fail --silent https://staging.example/health | jq
curl --fail --silent https://staging.example/metrics | grep c4isr_database_up
task test:integration
pnpm --dir apps/operator-ui exec playwright test
```

Then execute the operator path: observation → track → alert → incident →
mission → command → outcome, and confirm provenance, audit history, errors,
and scenario evidence are visible without reading logs.
