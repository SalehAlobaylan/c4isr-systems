# F04 — Durable preparation workers and execution transport

[Plan index](../README.md)

**Status** Riyadh process-loss and exact-package recovery passed. Shared staging runs two independent HTTP workers and one direct worker for terrain displays. [Transport evidence](../../docs/implementation/GEOGRAPHIC_OPERATIONS_AND_UPGRADE.md#f04-database-free-preparation-workers)

**Implementation owner** Backend and platform engineering.

**Dependencies** [F01 — Source admission, lineage and retention](01-source-admission-and-lineage.md)

## Outcome and baseline

Preparations survive worker loss and retry with one consistent publication. The direct transport recovered an interrupted Al Ain attempt. The HTTP transport recovered an interrupted Riyadh attempt and preserved the Al Ain archive in a restored staging copy. [Implementation record](../IMPLEMENTATION_PROGRESS.md)

## Features and contracts

Retain job IDs, immutable recipes, request keys, attempt numbers and lease fencing. Expose claim, heartbeat, fail and completion operations through an authenticated internal worker API for deployments that isolate worker database access. Define proposed routes and schemas in the environment contract before implementing them. Reuse the same transaction functions for database and HTTP callers during migration.

Bind lease operations to service identity, scope, exact source grants, toolchain and job. A lease token is an opaque capability supplied in an authenticated request body or header. Completion references uploaded objects and their verified hashes. Only the API commits the release record.

## Implementation sequence

1. Extract the current claim/renew/complete operations into shared domain methods preserving row locking and fencing.
2. Add HTTP contracts, bounded requests and current-grant checks at claim, heartbeat and publication.
3. Adapt the worker to fetch authorized inputs, upload staged output, and report progress through the selected transport.
4. Handle lost completion responses by querying job state and retrying with the same identity.
5. Commission two HTTP workers in staging, then qualify a worker process without database credentials. Preserve the direct mode until equivalence is demonstrated.

## Acceptance

- Concurrent claims respect capacity and yield one live lease per job.
- A stale or foreign lease cannot renew, fail or publish another attempt.
- Worker death during processing and upload produces a bounded retry and one staged release.
- Cancellation and source revocation during work prevent publication.
- Duplicate completion returns the prior publication or a clear conflict.
- Exhausted scratch, object failures and toolchain mismatch produce persistent diagnostics and bounded cleanup.
- HTTP mode passes the existing database-mode fault cases.

## Rollout and recovery

Deploy API support before switching workers. Drain or finish live leases during a transport change. Keep existing recipes and event histories readable. Suspend new claims when the API cannot validate grants. F08 monitors transport and lease health.

## Deliverable evidence

Produce contract fixtures, two-worker fault results, a database-free worker configuration and an explicit transport cutover record. Record recovery latency and actual attempts without claiming zero duplicate execution.

## Existing implementation references

- [internal/geoenv/worker.go](../../internal/geoenv/worker.go)
- [internal/geoenv/build.go](../../internal/geoenv/build.go)
- [internal/geoenv/releases.go](../../internal/geoenv/releases.go)
- [apps/emad-environment/main.go](../../apps/emad-environment/main.go)

[Foundation evidence](../../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md) · [Original implementation plan](../../docs/implementation/ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md)
