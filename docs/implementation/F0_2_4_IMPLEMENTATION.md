# F0.2–F0.4 — Principals, ownership, and application isolation

**Date:** 2026-10-20 (Asia/Riyadh).  
**Status:** Complete for F0.2, F0.3, and F0.4.  
**Plan:** [EMAD Implementation Plan v2](../../EMAD_IMPLEMENTATION_PLAN_v2.md).  
**Scope:** Implement the next three tasks after the [F0.1 baseline](F0_1_BASELINE.md). These changes preserve the modular monolith and the existing exercise workflow. F0.5 and F0.6 remain required before the full F0 gate or any live public connector is enabled.

## F0.2 — Principals and the resource contract

`internal/platform/access` defines human and service principals, credential expiry, explicit permission grants, source restrictions, and a validated resource context. A resource consists of `workspaceId`, `environmentId`, one of the four approved partitions, and an optional `runId`. Public evidence has no run; exercise observations and truth require one. Exercise input may be run-bound. There is no quarantine partition.

Requests authenticate before selecting a resource. The optional `X-EMAD-Workspace-ID`, `X-EMAD-Environment-ID`, `X-EMAD-Partition`, and `X-EMAD-Run-ID` headers select an existing resource within the live database grants. Payload metadata, track hints, prefixes, and headers cannot grant access. Scenario run routes select the run in their path and still validate its grant.

OpenAPI documents these headers and the additive `/auth/me` fields `principal` and `resource`; generated UI types are updated. The existing `operator` and `permissions` response fields remain. Protobuf adds an optional versioned `ResourceContext` selector to both ingestion requests without changing existing field numbers. This is a machine contract; the server still has no gRPC listener.

Permissions retain the existing domain vocabulary (`sources.read`, `observations.ingest`, `commands.issue`, and so on). Scenario definition access additionally requires `sim.scenarios.author`. A connector receives only source/read/ingest permissions and canonical source IDs in its granted resource. A publisher receives exact-run ingestion permissions. Exercise operators receive exercise-scoped domain permissions. `assessments.review`, `context.export`, and `imagery.jobs.submit` are reserved capabilities for subsequent work, not implemented workflows. `sim.truth.read`, `sim.truth.grant`, and `sim.evaluation.read` are separate privileged capabilities. Neither ordinary administration nor synchronous projections imply them.

Private projection permissions (`tracks.project`, `observations.process`, `alerts.project`, `geofences.evaluate`, `audit.append`) cannot be provisioned to callers. The synchronous domain dispatcher installs projection authority only after an authorized input service accepts work; it retains the same resource. Scenario runners and freshness workers use explicit internal service identities and exact resources. They do not reuse human bearer tokens. Maintenance authority is separate from projection authority.

`principal_credentials` stores a SHA-256 digest of a random 256-bit bearer token, its principal, default resource, expiry, and optional revocation time. Issuance allows at most 90 days; the CLI defaults to 24 hours. Rotation issues a replacement, durably writes its secret file, and then revokes the old credential. Every request reloads credential validity and grants. Identity/provisioning changes are recorded in `authorization_changes`. Denials record only principal, permission, resource, a fixed reason, and time in `authorization_denials`; bearer secrets and request payloads are excluded. Service-originated domain audit entries retain the initiating principal and identify the service actor.

Existing `C4ISR_AUTH_TOKENS` credentials become `human:<operator-id>` identities. Their first persisted expiry is 30 days; restart does not extend it or reactivate a revoked key. Removing a configured token revokes its legacy credential. These identities retain the current operator-role ceiling and an explicitly stored wildcard-run grant in `default/local/EXERCISE_OBSERVATION`. Only the run dimension permits `*`; workspace, environment, and partition do not.

Omitted selection headers use the credential's stored resource. Starting an authorized scenario selects its new run for that credential, preserving the original single-run browser workflow. Concurrent callers must select runs explicitly. Implicit realtime clients follow the selected default; explicit subscriptions remain pinned. This compatibility mechanism does not grant new workspaces or partitions. `C4ISR_DATABASE_AUTH=true` allows an authenticated server to start entirely from provisioned database credentials without configured bootstrap tokens. It does not permit anonymous production access.

## F0.3 — Persistence and legacy ownership

Migration `00019_principals.sql` adds the identity, workspace/environment, scope, and authorization audit tables. Migration `00020_resource_ownership.sql` adds an indexed `scope_id` foreign key to all 28 domain tables identified by F0.1: aggregates, evidence, current state, history, workflow joins, audit, and scenario records. Operators remain platform identities, outside the domain ownership partitioning.

The scope relation stores the explicit ownership dimensions once. Scope is immutable after assignment. Database triggers verify same-scope parents for domain foreign keys and validate polymorphic assessment subjects/evidence. Legacy scenario assessments may use a track's logical `external_ref`; it must resolve within the selected scope. Assessment creation and its evidence links commit atomically, so a rejected foreign reference leaves no partial assessment.

Track external references and telemetry message IDs are unique within their owning scope. Caller-supplied source, observation, asset, geofence, and telemetry IDs receive deterministic scope namespaces outside the legacy scope. Already run-qualified IDs and existing legacy IDs are preserved. Clients should use returned canonical IDs for references; retrying the same input remains idempotent.

`emad_backfill_ownership()` uses recorded runs, matching run namespaces, creator audits, provenance metadata, and same-scope relationships. It computes candidate ownership before assignment and removes inconsistent candidates to a fixed point. Mixed-evidence tracks, assessments, incidents, and their dependent records remain unresolved. It neither deletes records nor invents public ownership. IDs, row counts, and evidence links are retained.

Unresolved legacy rows retain `scope_id IS NULL`. The restricted application role cannot see them. `emad_ownership_report` exposes maintenance counts per table; privileged inspection of `WHERE scope_id IS NULL` identifies records requiring an explicit provenance decision. Do not assign them all to the compatibility exercise. A missing record under an authorized resource is returned as missing rather than revealing foreign ownership.

The backfill is atomic, takes write-excluding locks on the affected tables, and requires a superuser or BYPASSRLS migration role. On retry, it temporarily disables only ownership triggers under those locks, assigns verified unowned rows, and restores the triggers in the same transaction. The ordinary role cannot execute it. Tests retry twice after migration on an already-populated legacy graph. The ownership migration deliberately refuses an automatic Down operation; recovery requires a reviewed forward migration or restoration of a verified pre-migration backup.

## F0.4 — Services and queries

Application services require their domain permissions even when called directly without HTTP. Mission, command, controlled-asset, and telemetry operations require `EXERCISE_OBSERVATION`; public tracks cannot become command recipients. HTTP, scenario lifecycles, command acknowledgements, event projections, and background freshness work retain the validated resource.

All domain repositories use `db.Scoped`, including sqlc-generated queries, handwritten replay/spatial queries, counts, histories, and relational transactions. Each statement or existing transaction sets a transaction-local resource and source selection and switches to `emad_data`, a non-owner, non-superuser, NOBYPASSRLS role. All 28 tables enable and force RLS. Restrictive source policies cover source registration and observation/telemetry reads and writes. The wrapper commits or rolls back before releasing a connection, so pooled sessions retain neither role nor resource. Identity provisioning and operator authentication use the control-plane pool separately.

This implements query scope centrally without duplicating 136 query variants. The runtime database login must be a member of `emad_data`; migration `00020` grants membership to its current migration user. If deployment separates migration and application logins, grant membership to the application login explicitly. Application login access to the identity tables remains necessary; this change does not introduce a separate identity service.

The existing hub now filters delivery by the validated event resource and refreshes subscriber credentials/grants before enqueueing. Truth and events without a resource are dropped. This minimal routing change closes an immediate bypass of scoped HTTP reads. It does not complete F0.5's serialized metadata review, privileged event design, queued-message revocation, or reconnect/resynchronization policy. The freshness monitor also keeps separate previous-state maps per resource, so ticking another run cannot emit false disappearances in the current run.

## Provisioning and rollout

The existing local development database was not modified by this implementation. Integration tests use disposable PostGIS containers.

1. Stop the previous writer version, preserve a verified database backup, and apply the existing migration procedure through version 20. Use a migration role with schema ownership, role-creation/membership authority, and superuser/BYPASSRLS authority for the ownership backfill. Run during a maintenance window because the backfill locks domain tables.
2. Run `go run ./apps/emad-admin -action report`. Resolve/quarantine counts are maintenance output; investigate unknown ownership before exposing those records. `-action backfill` retries only the recorded-provenance procedure and prints the same report.
3. Provision the initial authorized identities and sources, then source-bound connector keys. Give the future runtime login membership in `emad_data` when it differs from the migration login. Run the updated server only after migrations succeed.
4. Keep the compatibility tokens initially, or switch explicitly to `C4ISR_DATABASE_AUTH=true` after provisioning database credentials. Verify `/api/v1/auth/me` returns the intended resource. Select a run explicitly for concurrent sessions.

The admin CLI reads `C4ISR_DATABASE_URL` or `C4ISR_DATABASE_URL_FILE` independently of HTTP configuration. It requires trusted database access. It supports `report`, `backfill`, `provision`, `grant`, `issue`, `rotate`, and `revoke`. Its bootstrap authority cannot delegate truth. Privileged truth administration requires separately provisioned, explicitly reviewed authority; it is not implicitly enabled by this tool.

For example, a service spec for a previously registered public source is:

```json
{
  "id": "usgs-connector",
  "kind": "service",
  "name": "USGS connector",
  "resource": {
    "workspaceId": "public",
    "environmentId": "world",
    "partition": "PUBLIC_EVIDENCE"
  },
  "permissions": ["sources.read", "observations.ingest"],
  "sourceIds": ["<canonical-source-id-returned-by-EMAD>"]
}
```

```text
go run ./apps/emad-admin -action provision -spec connector.json -out connector.token -lifetime 24h
go run ./apps/emad-admin -action rotate -spec connector.json -credential <old-key-id> -out connector-next.token
go run ./apps/emad-admin -action revoke -credential <key-id>
```

Secret output files must not already exist. The CLI creates them with mode `0600`, never prints the bearer token, and prints the credential ID needed for revocation. On Windows, use a directory whose ACL restricts access to the operator/service account. Supply the secret through the existing bearer transport. Do not commit secret files. `grant` uses the same resource/permissions/sourceIds fields; `issue` uses the ID and a concrete granted resource. An initial human spec uses `kind: human` and an existing `operatorId`. An environment-wide scenario controller needs an explicit additional grant with `runId: "*"`; an ordinary exact-run publisher does not.

## Verification

All checks below passed. Raw local logs are in ignored `tmp/f0-2-4/`.

- `go test -count=1 ./...` and `go vet ./...` passed. Existing fixture tests now provide explicit authority; negative tests construct restricted principals rather than receiving an implicit bypass.
- `C4ISR_TEST_INTEGRATION=1 go test -count=1 -v -timeout 15m ./internal/integration/...` passed all **31 tests** in **191.223 seconds** against disposable PostGIS databases. This includes the original end-to-end exercise acceptance, concurrent scenario controls, operational failure matrix, replay/spatial cases, populated ownership migration, and real provisioning CLI. The additional explicit two-runs-plus-public-workspace coexistence assertions also passed in a focused rerun (5.949 seconds).
- The CI race package selection plus `internal/platform/access` passed in the cached Linux Go 1.25.14 container with repository and native module cache mounted read-only. Native Windows CGO remains disabled. Taskfile and CI now retain access-package race coverage.
- `pnpm --dir apps/operator-ui build`, OpenAPI TypeScript generation, Redocly lint, containerized Buf lint, and protobuf compatibility against local `main` passed. Vite retains the baseline warning about large bundles. The protobuf comparison does not assert a refreshed remote branch.
- Existing Playwright tests passed with **65 passed and 49 intentionally skipped**, two workers, and the F0.1 temporary ports 25173/25174. These preserve the original token-isolation projects and use mocked API responses; the real-backend browser isolation gate remains F0.6.
- `sqlc generate -f db/sqlc.yaml` completed; the generated database code is included. Canonical Go formatting passed after CRLF normalization, as in F0.1; unrelated CRLF-only files were not reformatted. Diff whitespace and UTF-8 checks passed.

During development, the broad suite found and resolved legacy migration-fixture scope setup, explicit selection for concurrent-run commands, and legacy logical track references in assessment evidence. The source-read regression fixture was reordered after its additional observation correctly created another track. No failing check is being accepted as completion.

The new negative cases exercise source-constrained ingestion and evidence reads, wrong workspace/environment/run selectors, unknown direct-service context, service/human identity distinction, public command denial, explicit truth separation, live grant changes, credential rotation/revocation/expiry, simultaneous scoped queries, and pooled role reset. Integration cases also cover foreign parent/evidence rejection, atomic assessment failure, telemetry idempotency in two runs, history/spatial/replay isolation, incident/mission attachments and mutation denial, minimal realtime routing, real CLI provisioning, and populated migration quarantine/retry.

Remaining product work is F0.5 realtime/payload hardening, then F0.6 explicit UI workspaces, scoped caches, and the final F0 compatibility gate. No public provider, heavy 3D engine, GPU worker, or live hardware integration is enabled here.

**Subsequent execution:** [F0.5](F0_5_REALTIME.md) completes the realtime/disclosure task and adds migration 21. The rollout above records the F0.2–F0.4 baseline; current deployments must apply the latest migrations and follow the newer contract notes. F0.6 remains open.
