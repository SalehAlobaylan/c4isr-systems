# F08 — Operations, metrics, alerts and commissioning

[Plan index](../README.md)

**Status** Shared staging passed scoped Prometheus commissioning after the isolated Riyadh fault checks. Separate-host and external alert delivery commissioning remain open. [Operations evidence](../../docs/implementation/GEOGRAPHIC_OPERATIONS_AND_UPGRADE.md#f08-operations-and-alerting)

**Implementation owner** Platform engineering and environment operations.

**Dependencies** [F04 — Durable preparation workers and execution transport](04-durable-workers-and-transport.md), [F05 — Artifact delivery, integrity and storage lifecycle](05-artifact-delivery-and-lifecycle.md)

## Outcome and baseline

Operators can distinguish a stalled worker, unavailable object store, exhausted capacity and expiring source admission. Scoped Prometheus metrics include worker scratch usage and retained attempt history. Viewer performance remains part of the viewer qualification. Separate-host commissioning depends on a target host.

## Features and contracts

Add environment build duration, queue age, state counts, expired leases, retries, publication failures, reservations, scratch usage and cleanup backlog. Keep metric label sets bounded by operation, profile, state and failure class. Link detailed diagnostics through authorized job records.

Provide alerts with an owner role, trigger window, recovery action and verification step. Make worker capability and active toolchain visible. Commission Windows and Linux through recorded configuration and image digests. Treat separate-host installation as a rollout target with its own network, storage and credentials.

## Implementation sequence

1. Derive durable counters and gauges from current events and reservation state, documenting restart behavior.
2. Add the exporter and dashboard to the existing Prometheus stack.
3. Configure initial alert thresholds from existing queue and request budgets. Record tunable values before qualification.
4. Exercise each alert with a bounded staging fault and retain detection and clearance times.
5. Produce a commissioning command/runbook that checks schema, object access, worker grants, capacity, time synchronization and toolchain identity.
6. Commission a supplied separate host when available and retain its independent evidence.

## Acceptance

- Worker restart does not erase queue age or produce incorrect success counts.
- A paused worker, failed object read and quota exhaustion generate distinguishable alerts.
- Source expiry is visible before it blocks a dependent package.
- Metrics exclude credentials, precise AOIs, filenames and unbounded identities.
- Readiness identifies the failing dependency and recovers after repair.
- A fresh installation verifies the pinned qualification releases, including Riyadh, with its scoped credentials.
- Separate-host readiness is claimed only after that host is exercised.

## Rollout and recovery

Deploy metric producers before alerts. Introduce alerts with a commissioning verification window, then enable operator delivery through the organization’s chosen channel. Roll back alert configuration independently of data services. Existing scoped logs remain available if the metrics collector fails.

## Deliverable evidence

Deliver dashboard and alert definitions, fault results, owner/runbook assignments, installation inventory and measured capacity. Host selection and organization-specific alert recipients are deployment inputs.

## Existing implementation references

- [internal/platform/observability/metrics.go](../../internal/platform/observability/metrics.go)
- [internal/platform/readiness/http.go](../../internal/platform/readiness/http.go)
- [internal/publicingest/scheduler_metrics.go](../../internal/publicingest/scheduler_metrics.go)
- [deployments/observability/prometheus.yml](../../deployments/observability/prometheus.yml)
- [deployments/staging/delivery.yml](../../deployments/staging/delivery.yml)

[Foundation evidence](../../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md) · [Original implementation plan](../../docs/implementation/ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md)
