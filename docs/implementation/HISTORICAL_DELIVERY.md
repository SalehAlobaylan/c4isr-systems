# Historical delivery record: the pre-map-first v2 sequence (F0–S4)

> **Historical record — superseded.** This merges the F0/P1–P3/S0–S4 v2 delivery sequence and its acceptance evidence. It documents code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

## Contents

- [F0.1 — Baseline verification and migration inventory](#f0-1)
- [F0.2–F0.4 — Principals, ownership and application isolation](#f0-2-4)
- [F0.5 — Realtime authorization and metadata protection](#f0-5)
- [F0.6 — COP workspaces and compatibility acceptance](#f0-6)
- [P1.1 — Public source admission and durable evidence](#p1-1)
- [P1.2–P1.4 — Public event semantics, reference snapshots and USGS](#p1-2-4)
- [P2.1–P2.3 — Retained imagery, environmental analysis and reviewed assessments](#p2-1-3)
- [P2.4 / P3.1 / P3.2 — Approved context and bounded moving sources](#p2-4-p3-1-2)
- [P3.3 — Station weather and selected official reports](#p3-3)
- [S0.1–S0.3 — Frozen runs, spatial contracts and context packages](#s0-1-3)
- [S0.4, P1.5 and P1.6 — Headless adapter and public evidence history](#s0-4-p1-5-6)
- [S1.1 — Model and environment asset registry](#s1-1)
- [S1.2 — Rendering engine decision](#s1-2)
- [S1.3 / S2.1 — Georeferenced Studio and native runtime](#s1-3-s2-1)
- [S2.2–S2.3 and S3.1 — Native perception and exercise tasking](#s2-2-s2-3-s3-1)
- [S3.2, S3.3 and S4.1 — Recorded exercises, evaluation and sensor presets](#s3-2-s3-3-s4-1)

---
<a id="f0-1"></a>

## F0.1 — Baseline verification and migration inventory

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

**Date:** 2026-10-20 (Asia/Riyadh)

**Status:** Complete for baseline verification and migration planning. F0 isolation is not implemented or accepted by this report.

**Baseline commit:** `2c0c65548036ca73be8e8c259c60171ddecfc46b` on `main`.

**Plan:** [EMAD Implementation Plan v2](../../EMAD_IMPLEMENTATION_PLAN_v2.md), task F0.1.

### 1. Scope and evidence

This task executed the existing baseline checks, inspected the current application boundaries, catalogued migration targets, and performed a read-only provenance audit of the existing local development database. It introduces no application, API, schema, authorization, or stored-data changes.

At the start, `README.md` and `IMPLEMENTATION_PLAN.md` already had planning changes. The two v2 source proposals and the new v2 implementation plan were untracked. The application source matched the baseline commit. Those planning changes were retained.

The detailed [source inventory](F0_1_INVENTORY.json) records:

- **72 mounted method/path combinations:** 70 under `/api/v1`, plus `/health` and `/metrics`. All 70 authenticated route patterns resolve to an existing authorization policy when evaluated through `httpx.PolicyFor`.
- **29 application tables** defined by 18 migrations. The extension-owned `spatial_ref_sys` and Goose-owned `goose_db_version` are additional operational tables, not domain aggregates.
- **136 named SQL queries** across 17 query files, including lists, counts, projections, histories, mutations, and relationship operations.
- **31 domain event topics**, their browser mapping and explicit audit subscription coverage.
- **12 UI paths**, **94 query-key expression sites** including invalidations, and **five deterministic scenario fixtures**.

The inventory is a source snapshot, with file/line references and intended migration classifications. It is not an implemented permission model. Mounted routes were extracted from transport declarations, checked against the composition root, and evaluated against the actual policy implementation. Generated SQL/types remain derived artifacts.

Raw check logs and temporary inspection/configuration helpers are retained locally in ignored `tmp/f0-1-baseline/`. Durable conclusions and reproducible database inspection SQL are in this directory. Logs can be regenerated; they are not required runtime artifacts.

### 2. Environment

- Windows/amd64; PowerShell; native Go **1.25.0**.
- Node **24.13.0**, pnpm **10.14.0**, Task **3.54.0**; existing dependencies were already installed.
- Docker client/server **29.8.2**, Linux/x86_64 daemon.
- Native `CGO_ENABLED=0`, with no C compiler found on PATH. Race checks used the existing cached `golang:1.25` image: Go **1.25.14**, Linux/amd64, CGO enabled.
- Native Buf was unavailable. Containerized Buf **1.74.0** supplied lint and compatibility checks.
- The local CI definition uses Node 20 and Go from `go.mod`. The successful local browser/build run used Node 24; it does not replace a future CI run on its declared environment.
- `core.autocrlf=true`; 72 Go working-tree files had CRLF endings while their indexed versions use LF.

Container image IDs used for the portable checks:

- Go: `sha256:9bae33b05e4e159803d2ad714fecbda886969dce7ca369216dece578cad664c1`.
- Buf: `sha256:468b04918c77a05e7f575d7f0736c363d0813539996ae1766c5291ea31aad956`.

### 3. Executed verification

- **PASS — Go baseline:** `go test -count=1 ./...` and `go vet ./...`. Integration tests are opt-in, so this result alone does not establish database acceptance.
- **PASS — Real PostGIS:** `C4ISR_TEST_INTEGRATION=1 go test -count=1 -timeout 20m ./internal/integration/...`, using the existing Testcontainers harness and `imresamu/postgis:17-3.5`. Completed in 158.899 seconds. This exercised fresh schema migration and existing integration acceptance, failure, area, picture, and authentication cases.
- **PASS — Race detection:** the CI package selection passed with `go test -race` in Linux/Go 1.25.14. The source tree and native module cache were mounted read-only. The original native invocation could not run because CGO is disabled; it reported no application test failure.
- **PASS — Frontend:** `pnpm --dir apps/operator-ui build`, including TypeScript build checks. Vite retained its existing warning about chunks above 500 kB; build completion was successful.
- **PASS — Browser baseline:** existing Playwright tests completed with **65 passed, 49 intentionally skipped**, two workers, both original token-isolation projects, in 54.8 seconds. The first invocation stopped because port 5173 was occupied. A temporary configuration used ports **25173/25174**, preserved project names/token settings and tests, and left the existing process running. These tests mock API responses; they do not establish a real-backend v2 isolation flow.
- **PASS — OpenAPI generation:** `openapi-typescript` 7.13.0 generated into a temporary file. Its contents match the committed generated UI types after normalizing line endings; the tracked generated file was not rewritten.
- **PASS — OpenAPI lint:** the CI Redocly lint command accepted `api/openapi/openapi.yaml`.
- **PASS — Protobuf:** containerized `buf lint api/proto` and `buf breaking api/proto --against '.git#branch=main,subdir=api/proto'`. The comparison is against the local `main` at the recorded baseline, not an assertion that a remote branch was refreshed.
- **PASS — Canonical Go formatting:** every Go file under `apps/` and `internal/` matches gofmt after CRLF→LF normalization. Native `gofmt -l` reports 72 files solely because of CRLF; no substantive formatting differences were found. No bulk formatting was applied.
- **PASS — Shell syntax:** all `scripts/ops/*.sh` and `deployments/nginx/40-c4isr-config.sh` passed `sh -n` through Git's shell.
- **PASS — Staging configuration:** Compose `config --quiet`, Prometheus configuration and all 10 alert rules, Alloy formatting/parser invocation, and Loki configuration verification passed with the versions specified in the existing validation script. These constituent checks were executed separately with read-only configuration mounts. An initial unquoted PowerShell Loki argument was parsed incorrectly; quoting the full flag fixed the invocation and configuration validation passed.
- **PASS — Read-only legacy audit:** the SQL below completed against `c4isr-postgres`, in a repeatable-read, read-only transaction with a 30-second statement timeout.

Not executed: staging deployment/smoke, backup creation/restore drill, production migration/backfill, live public providers, native 3D execution, or new v2 permission tests. This task verifies the present foundation and identifies future acceptance work; it does not certify those planned capabilities.

### 4. Current ingress and authorization boundaries

#### HTTP surface

The JSON inventory lists every method/path, current permission, allowed operator roles, source declaration, and intended scope family. Preserve both scenario start routes until an explicit compatibility migration: `/scenarios/definitions/{name}/start` and `/scenarios/{name}/start`.

Current authentication maps configured bearer tokens to human operator records. WebSocket upgrades additionally accept an access-token query parameter. Explicitly disabled-auth development/test paths can use the legacy operator header. `/auth/me` reports the role-derived permission matrix.

Role checks exist at the HTTP boundary. Ordinary reads are broadly available to registered operator, supervisor, administrator, and analyst roles, with a separate operator-directory rule. There is no workspace/environment/run grant in the current principal. A known route policy therefore does not establish ownership authorization.

Target readers and writers for the migration:

- **Sources, observations, tracks, classifications, assessments:** authorized public analysts/connectors in `PUBLIC_EVIDENCE`, or authorized exercise principals in `EXERCISE_OBSERVATION`. Writers, derivations, and evidence links inherit the validated domain context.
- **Assets, telemetry, missions, tasks, commands:** run-bound exercise services and operators in `EXERCISE_OBSERVATION`. Public track IDs cannot become authorized recipients.
- **Geofences:** explicitly owned public reference/rule data or exercise inputs. Containment state and alerts belong to the relevant projection. A matching polygon is not cross-run authority.
- **Alerts and incidents:** public or exercise workflows, with every related subject checked. Counts, attachment selectors, and detail expansions require the same boundary as lists.
- **Scenario definitions and event inspection:** restricted `EXERCISE_INPUT`/run-control views for authors/controllers. The definition handler currently returns the authored scenario; names, actor templates, actions, and event errors require visibility review before a perception-limited exercise operator role is introduced.
- **Picture and realtime:** authorized public or exercise projections. Current optional picture prefixes are filters, not grants; alert/geofence reads and historical attributes require their own scope/temporal review.
- **Audit:** partitioned, authorized history, with explicit privileged evaluation access later. Platform identity administration remains distinct from domain and truth access.
- **Health/metrics:** current public probes. Review their deployment exposure and metric labels when partition-aware observability is introduced.

#### Non-HTTP ingress and background work

- Scenario setup calls `Sources.Ensure`, `Assets.Ensure`, and `Geospatial.Ensure` directly. It already records and verifies run/namespace metadata for reused resources. Keep these checks when adding the validated resource context.
- The scenario engine calls normal observation, telemetry, classification, assessment, incident, asset-status, and command services directly. HTTP-only enforcement would leave this ingress unprotected.
- Scenario command listeners select in-memory runs owning an asset and schedule acknowledgments/outcomes; cancellation/finalization and pending event persistence are additional mutation paths.
- Application startup ensures the default operator and marks abandoned active runs. Both need explicit platform/run authority rather than inferred operator identity.
- The periodic freshness monitor scans assets/tracks and publishes status changes without an incoming HTTP request. Its repository reads and event routing must recover the resource's trusted scope.
- Synchronous domain subscribers update tracks/history, containment state, alerts, and audit. Subscriber failures do not roll back the original evidence insert; retry and scope context must survive projection recovery.
- No implemented public-provider connector, artifact service, independent simulation executable, or gRPC server listener was located. Protobuf service declarations are contracts awaiting a transport implementation.

### 5. Persistence and legacy classification

The inventory names all 29 application tables and their ownership evidence. Migration groups are:

- Evidence/projections: `sources`, `observations`, `tracks`, `track_state`, `track_observations`, `track_history`, `classifications`, `assessments`, `assessment_evidence`.
- Exercise asset state/history: `assets`, `asset_state`, `asset_telemetry`.
- Spatial context/state: `geofences`, `geofence_states`.
- Attention and relations: `alerts`, `incidents`, `incident_alerts`, `incident_tracks`, `incident_assets`, `incident_observations`, `incident_assessments`.
- Exercise tasking: `missions`, `mission_assets`, `mission_tasks`, `commands`.
- Platform/history/run administration: `operators`, `audit_events`, `scenario_runs`, `scenario_run_events`.

Parent ownership must be propagated into state/history. Relationship rows must validate both ends. `assessment_evidence` and assessment subjects are polymorphic IDs, so database foreign keys alone do not establish compatible ownership.

Existing global uniqueness needs migration review: source/resource IDs, `tracks.external_ref`, `asset_telemetry.message_id`, observation IDs, and current alert/projection concurrency guards. Preserve IDs while changing scope-aware uniqueness and lookups deliberately.

Track primary IDs are generated `trk_...`; the namespaced logical identity lives in `external_ref` and metadata. Consequently `tracks.id LIKE run-prefix` cannot be used as a general run ownership check or backfill rule. Audit attribution and input payloads supplement these markers; none is an authenticated grant by itself.

#### Local development data audit

Run [F0_1_LEGACY_AUDIT.sql](F0_1_LEGACY_AUDIT.sql) with an explicitly selected database. The recorded local snapshot contained:

- Two completed runs; four sources, all labelled `sensor`.
- Consistent recorded run + namespace candidates for **4 sources, 41 observations, 2 geofences, and 1 track**. No assets or telemetry samples were present.
- **41 observation/source links**, **41 track/observation links**, **41 evidence/source links**, and **1 geofence/track state** with no missing or conflicting run markers in the inspected relationships.
- **140 audit records**, all carrying a marker matching a recorded run.
- One alert, two classifications, 40 track-history rows, one track-state row, 88 scenario-run events, and one operator.
- No incidents, missions, tasks, commands, assessments, or corresponding populated join rows in this database. Their behavior is covered by the separate integration suite, not by this local data sample.

These are **backfill candidates**, not assigned partitions. The audit emits aggregate counts and never updates records. It does not prove that metadata was originally written by a trusted principal, or exhaustively classify every aggregate. F0.3 must combine creator audit, parent relationships, valid run records, and resource metadata; a source type or ID prefix alone is insufficient. Ambiguous or conflicting records remain unresolved for explicit review and must not be exposed by a default public scope.

No database rows were relabelled, quarantined, migrated, or promoted to public evidence in F0.1. Quarantine enforcement belongs to the additive migration and service changes in F0.3/F0.4.

### 6. Events, projections, caches, and artifacts

#### Event and broadcast inventory

The synchronous dispatcher defines 31 topics. Their source/asset/evidence/track/classification/spatial/alert/incident/mission/command/assessment/run families are enumerated in JSON. The composition root logs and measures all topics; the WebSocket hub subscribes to all and broadcasts to all connected clients through explicit payload mapping.

Keep existing safeguards: bounded client buffers, slow-client disconnection, explicit message mapping, scenario namespace propagation, and scenario-aware geofence filtering/reconciliation. These are useful partial controls, not a complete resource authorization layer.

Two subscription/mapping details must be retained in the migration inventory:

- `entity.freshness.changed` has an explicit browser mapping, but is not in the audit service's explicit subscription list.
- `area.geofence.overlapped` is an internal intermediate event without explicit browser mapping or audit subscription; the hub nevertheless subscribes to it through `AllTopics` and can emit its envelope. Decide its permitted audience explicitly rather than expanding its payload incidentally.

Hidden actor identity may travel through observation `trackHint`, logical references, payloads, scenario definitions/event names, audit data, and structured logs. F0.5 must evaluate those paths structurally; removing one visible position field is insufficient. There is no sealed truth store to migrate yet.

#### Browser read models

Current UI paths: `/`, `/board`, `/map`, `/tracks`, `/tracks/$id`, `/systems`, `/assets`, `/alerts`, `/incidents`, `/incidents/$id`, `/timeline`, `/scenarios`.

The query factory covers health, current operator, operator directory, sources, assets/telemetry, tracks/history/classifications, observations, geofences, alerts, incidents, missions, commands, assessments, audit, and scenarios/runs/events. The inventory also records every inline query-key use.

F0.6 must include the inline `['picture', at-or-live]` and `['picture-window']` keys in map/detail/scrubber components, as well as factory keys. Realtime reconnect invalidates broad resource prefixes; it does not currently select a workspace/run subscription. UI selections and replay state have no ownership context. The persisted layout store contains presentation preferences, not a duplicate authoritative evidence database; retain that distinction.

#### Files and artifacts

- Five YAML fixtures are current authored exercise input: `normal-patrol` (seed 42), `restricted-area-intrusion` (1007), `multi-source-track` (2027), `operational-failure-matrix` (9090), and `area-hazard-intrusion` (2211). Preserve their fault cases and logical identities when versions are frozen in S0.
- The loader reads mutable YAML definitions from the configured scenarios directory; completed runs do not yet bind a content-addressed frozen bundle.
- SQL migrations/queries, OpenAPI/types, and protobuf declarations are versioned developer artifacts; regenerate derived outputs from their source definitions.
- Screenshot references, Playwright traces/reports, and frontend build outputs are development evidence. They are not a partition-aware product artifact store.
- Operational dumps/backups and centralized logs can contain data from multiple future partitions. Include retention, privileged access, and recovery paths in the ownership design; existing Compose secret files are deployment credentials, not public evidence.
- Imagery/terrain/model bundles, signed download references, recordings, and a truth artifact store are future capabilities. Add access policy at their first implementation, not after a shared public bucket is established.

### 7. Implementation handoff

F0.1 acceptance is satisfied by the executed checks, reviewed route/table/event/read-model inventory, safe classification policy, local read-only audit, and documentation precedence updates.

The next task is **F0.2**: define human/service principals, resource context, grants, and one canonical permission/envelope vocabulary. Its implementation must account for:

1. Existing human operator authentication and explicitly disabled-auth compatibility paths.
2. Direct scenario/background service calls as well as HTTP.
3. Platform metadata versus domain ownership; no implicit truth grant for administrator/analyst roles.
4. Reusable authoring inputs versus run-bound outputs.
5. Current namespace safeguards and ambiguous legacy provenance.
6. Authorization denials that are auditable without leaking hidden state.

F0.3 then persists ownership and classifies/backfills with evidence. F0.4/F0.5/F0.6 enforce it across services, queries, subscriptions, and browser state. Public connectors stay disabled until the full F0 gate passes.

<a id="f0-2-4"></a>

## F0.2–F0.4 — Principals, ownership and application isolation

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

**Date:** 2026-10-20 (Asia/Riyadh).\
**Status:** Complete for F0.2, F0.3, and F0.4.\
**Plan:** [EMAD Implementation Plan v2](../../EMAD_IMPLEMENTATION_PLAN_v2.md).\
**Scope:** Implement the next three tasks after the [F0.1 baseline](#f0-1). These changes preserve the modular monolith and the existing exercise workflow. F0.5 and F0.6 remain required before the full F0 gate or any live public connector is enabled.

### F0.2 — Principals and the resource contract

`internal/platform/access` defines human and service principals, credential expiry, explicit permission grants, source restrictions, and a validated resource context. A resource consists of `workspaceId`, `environmentId`, one of the four approved partitions, and an optional `runId`. Public evidence has no run; exercise observations and truth require one. Exercise input may be run-bound. There is no quarantine partition.

Requests authenticate before selecting a resource. The optional `X-EMAD-Workspace-ID`, `X-EMAD-Environment-ID`, `X-EMAD-Partition`, and `X-EMAD-Run-ID` headers select an existing resource within the live database grants. Payload metadata, track hints, prefixes, and headers cannot grant access. Scenario run routes select the run in their path and still validate its grant.

OpenAPI documents these headers and the additive `/auth/me` fields `principal` and `resource`; generated UI types are updated. The existing `operator` and `permissions` response fields remain. Protobuf adds an optional versioned `ResourceContext` selector to both ingestion requests without changing existing field numbers. This is a machine contract; the server still has no gRPC listener.

Permissions retain the existing domain vocabulary (`sources.read`, `observations.ingest`, `commands.issue`, and so on). Scenario definition access additionally requires `sim.scenarios.author`. A connector receives only source/read/ingest permissions and canonical source IDs in its granted resource. A publisher receives exact-run ingestion permissions. Exercise operators receive exercise-scoped domain permissions. `assessments.review`, `context.export`, and `imagery.jobs.submit` are reserved capabilities for subsequent work, not implemented workflows. `sim.truth.read`, `sim.truth.grant`, and `sim.evaluation.read` are separate privileged capabilities. Neither ordinary administration nor synchronous projections imply them.

Private projection permissions (`tracks.project`, `observations.process`, `alerts.project`, `geofences.evaluate`, `audit.append`) cannot be provisioned to callers. The synchronous domain dispatcher installs projection authority only after an authorized input service accepts work; it retains the same resource. Scenario runners and freshness workers use explicit internal service identities and exact resources. They do not reuse human bearer tokens. Maintenance authority is separate from projection authority.

`principal_credentials` stores a SHA-256 digest of a random 256-bit bearer token, its principal, default resource, expiry, and optional revocation time. Issuance allows at most 90 days; the CLI defaults to 24 hours. Rotation issues a replacement, durably writes its secret file, and then revokes the old credential. Every request reloads credential validity and grants. Identity/provisioning changes are recorded in `authorization_changes`. Denials record only principal, permission, resource, a fixed reason, and time in `authorization_denials`; bearer secrets and request payloads are excluded. Service-originated domain audit entries retain the initiating principal and identify the service actor.

Existing `C4ISR_AUTH_TOKENS` credentials become `human:<operator-id>` identities. Their first persisted expiry is 30 days; restart does not extend it or reactivate a revoked key. Removing a configured token revokes its legacy credential. These identities retain the current operator-role ceiling and an explicitly stored wildcard-run grant in `default/local/EXERCISE_OBSERVATION`. Only the run dimension permits `*`; workspace, environment, and partition do not.

Omitted selection headers use the credential's stored resource. Starting an authorized scenario selects its new run for that credential, preserving the original single-run browser workflow. Concurrent callers must select runs explicitly. Implicit realtime clients follow the selected default; explicit subscriptions remain pinned. This compatibility mechanism does not grant new workspaces or partitions. `C4ISR_DATABASE_AUTH=true` allows an authenticated server to start entirely from provisioned database credentials without configured bootstrap tokens. It does not permit anonymous production access.

### F0.3 — Persistence and legacy ownership

Migration `00019_principals.sql` adds the identity, workspace/environment, scope, and authorization audit tables. Migration `00020_resource_ownership.sql` adds an indexed `scope_id` foreign key to all 28 domain tables identified by F0.1: aggregates, evidence, current state, history, workflow joins, audit, and scenario records. Operators remain platform identities, outside the domain ownership partitioning.

The scope relation stores the explicit ownership dimensions once. Scope is immutable after assignment. Database triggers verify same-scope parents for domain foreign keys and validate polymorphic assessment subjects/evidence. Legacy scenario assessments may use a track's logical `external_ref`; it must resolve within the selected scope. Assessment creation and its evidence links commit atomically, so a rejected foreign reference leaves no partial assessment.

Track external references and telemetry message IDs are unique within their owning scope. Caller-supplied source, observation, asset, geofence, and telemetry IDs receive deterministic scope namespaces outside the legacy scope. Already run-qualified IDs and existing legacy IDs are preserved. Clients should use returned canonical IDs for references; retrying the same input remains idempotent.

`emad_backfill_ownership()` uses recorded runs, matching run namespaces, creator audits, provenance metadata, and same-scope relationships. It computes candidate ownership before assignment and removes inconsistent candidates to a fixed point. Mixed-evidence tracks, assessments, incidents, and their dependent records remain unresolved. It neither deletes records nor invents public ownership. IDs, row counts, and evidence links are retained.

Unresolved legacy rows retain `scope_id IS NULL`. The restricted application role cannot see them. `emad_ownership_report` exposes maintenance counts per table; privileged inspection of `WHERE scope_id IS NULL` identifies records requiring an explicit provenance decision. Do not assign them all to the compatibility exercise. A missing record under an authorized resource is returned as missing rather than revealing foreign ownership.

The backfill is atomic, takes write-excluding locks on the affected tables, and requires a superuser or BYPASSRLS migration role. On retry, it temporarily disables only ownership triggers under those locks, assigns verified unowned rows, and restores the triggers in the same transaction. The ordinary role cannot execute it. Tests retry twice after migration on an already-populated legacy graph. The ownership migration deliberately refuses an automatic Down operation; recovery requires a reviewed forward migration or restoration of a verified pre-migration backup.

### F0.4 — Services and queries

Application services require their domain permissions even when called directly without HTTP. Mission, command, controlled-asset, and telemetry operations require `EXERCISE_OBSERVATION`; public tracks cannot become command recipients. HTTP, scenario lifecycles, command acknowledgements, event projections, and background freshness work retain the validated resource.

All domain repositories use `db.Scoped`, including sqlc-generated queries, handwritten replay/spatial queries, counts, histories, and relational transactions. Each statement or existing transaction sets a transaction-local resource and source selection and switches to `emad_data`, a non-owner, non-superuser, NOBYPASSRLS role. All 28 tables enable and force RLS. Restrictive source policies cover source registration and observation/telemetry reads and writes. The wrapper commits or rolls back before releasing a connection, so pooled sessions retain neither role nor resource. Identity provisioning and operator authentication use the control-plane pool separately.

This implements query scope centrally without duplicating 136 query variants. The runtime database login must be a member of `emad_data`; migration `00020` grants membership to its current migration user. If deployment separates migration and application logins, grant membership to the application login explicitly. Application login access to the identity tables remains necessary; this change does not introduce a separate identity service.

The existing hub now filters delivery by the validated event resource and refreshes subscriber credentials/grants before enqueueing. Truth and events without a resource are dropped. This minimal routing change closes an immediate bypass of scoped HTTP reads. It does not complete F0.5's serialized metadata review, privileged event design, queued-message revocation, or reconnect/resynchronization policy. The freshness monitor also keeps separate previous-state maps per resource, so ticking another run cannot emit false disappearances in the current run.

### Provisioning and rollout

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

### Verification

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

**Subsequent execution:** [F0.5](#f0-5) completes the realtime/disclosure task and adds migration 21. The rollout above records the F0.2–F0.4 baseline; current deployments must apply the latest migrations and follow the newer contract notes. F0.6 remains open.

<a id="f0-5"></a>

## F0.5 — Realtime authorization and metadata protection

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

**Date:** 2026-10-20 (Asia/Riyadh).\
**Status:** Complete for F0.5.\
**Plan:** [EMAD Implementation Plan v2](../../EMAD_IMPLEMENTATION_PLAN_v2.md).\
**Scope:** Complete F0.5 after the [principal/ownership/query work](#f0-2-4). The full F0 gate remains open until F0.6 introduces explicit workspaces, scoped query caches, and the representative real-backend browser workflow.

### Ordinary COP delivery

`internal/realtime` publishes a version **2** envelope with `id`, `type`, `version`, `occurredAt`, `resource`, and explicitly mapped `data`. The resource is taken from authenticated application context, not event payloads. Only `PUBLIC_EVIDENCE` and `EXERCISE_OBSERVATION` are ordinary COP resources. Input, truth, unscoped events, unknown topics, and the internal area-rule event are not ordinary subscriptions. A privileged truth grant does not enable this socket for truth. No privileged truth/input socket is implemented; those event families remain reserved for separately authorized delivery.

Every connection requires `operational.read` plus at least one permitted topic. The explicit topic catalogue in `realtime/policy.go` applies the corresponding domain read permission. Geofence track events additionally require track visibility; freshness messages check the entity kind. Observation, source, and telemetry events also apply the live source allowlist. Source-bound subscribers cannot subscribe to mixed-source track projections through the COP stream. A new domain event is private until its transport mapping and subscription policy are added deliberately.

Clients may request a comma-separated `topics` list; an unknown or unauthorized requested topic rejects the upgrade and records a sanitized denial. Omitting `topics` selects the current permitted catalogue. New grants do not silently expand an already-selected topic set. Removed permissions and source restrictions take effect during delivery and periodic refresh.

Workspace/environment/partition/run may be selected with existing headers or WebSocket query parameters `workspaceId`, `environmentId`, `partition`, `runId`. Headers take precedence. Selection must match existing live grants. Explicit selections stay pinned. The documented legacy default-resource behavior remains available when no selector is supplied.

The hub checks live authority before enqueueing and again immediately before writing each queued frame. Queue entries retain their immutable resource, topic policy, source, and serialized bytes. Resource changes cannot relabel old frames. Credential expiry/revocation, principal disablement, and grant changes are reloaded through the identity store. Idle connections refresh every second. Invalid sessions receive close code **1008**; changes to permitted topics/source restrictions or the selected default resource require resynchronization. A frame already handed to the network cannot be recalled; checks do not claim atomicity with concurrent credential updates after the last check.

Each connection has at most **256** queued frames, each no more than **64 KiB**. Writes and authorization lookups have a **5-second** bound; keepalive runs every **30 seconds**. Overflow closes the affected connection with **1013** so it can obtain a fresh snapshot. Closure signalling does not wait for the peer inside synchronous domain publication. The close handshake is completed by the connection handler before its read context is cancelled.

### Reconnect and browser behavior

`realtime.ready` is the first frame. `realtime.resync` signals a resource or authorization change. Both carry the authorized resource, current permitted topics, and `resyncRequired: true`. They are control messages, not evidence. There is no durable cursor or claim that disconnected clients receive every event. Consumers must obtain authorized API snapshots after connection, reconnect, a resync signal, or a suspected gap. Events are notifications that invalidate those snapshots; they are never the authoritative state store.

The React provider accepts version 2 ordinary-resource frames only after a ready/resync binding, discards frames for another resource, and refreshes queries from the API. It resets queries on resync/resource changes and policy closure. Network/overflow closures use the existing bounded exponential reconnect backoff with jitter. A **1008** policy closure stops automatic retries and leaves the connection disconnected. Full identity/workspace-aware query-key and selection management remains F0.6.

The browser offers `emad.realtime.v2` and `emad.auth.<URI-encoded-token>` WebSocket subprotocols. The server consumes the authentication offer and selects only `emad.realtime.v2`; it never echoes the credential. The URL no longer contains the browser credential. Legacy `access_token` query authentication remains compatible, but the outer request wrapper consumes it and removes it from URL/RequestURI before HTTP instrumentation and logging. Credential protocol offers are similarly removed before instrumentation. Existing HTTP bearer authentication remains supported.

### Exercise disclosure policy

Existing domain HTTP DTOs and the explicit realtime mapper remain the transport boundary. `ResponseDisclosure` and `internal/platform/presentation` apply the same exercise policy to detail/list/ingress, history, audit, relationships, and replay responses. This policy does not alter stored observations or public evidence:

- Exercise observation payloads are not ordinary COP fields. Numeric quality/uncertainty values and declared Boolean attributes remain visible. Extensible registration metadata and quality maps reject unknown/nested values and unverified reference strings, including strings disguised under an otherwise familiar key such as `sourceId`.
- Correlation `trackHint` and track `externalRef` are empty in exercise responses. Track primary IDs remain the visible hypothesis identity. Legacy assessment subjects/evidence using logical track references are resolved to the canonical authorized track ID before encoding.
- Source and area display names use ordinary exercise labels. Alert prose preserves supported measured classification/hedging and spatial evidence without copying private resource names. Controlled exercise asset identifiers and names remain visible under their existing permissions; those assets are deliberately operated by the exercise user. Approved source IDs are operational provenance, not hidden actor mappings. Publishers must not use those declared visible fields as covert truth channels.
- Internally generated audit/rule reference maps expose only declared operational references and measured values. Observation references use public aliases. Arbitrary input metadata does not become trusted merely because its key resembles a domain ID. Raw truth, actor mappings, and unknown metadata remain outside ordinary responses.
- Raw action/failure diagnostic text is replaced in ordinary exercise transport. HTTP logs use route patterns rather than supplied object paths; HTTP internal-error logs and error spans retain categories rather than exception text. Selected runner/track logging also removes action names, raw observation identifiers, and exception text. Stored diagnostic evidence remains retained for its authorized purpose.
- Scenario definitions and scheduled-event inspection require `sim.scenarios.author` at the application service boundary. Run status remains available to scoped `scenarios.read` users, while definition names, seeds, namespaces, and scheduled action names are author-only fields. Scenario realtime events carry lifecycle status/run identity, never the seed or definition. This does not create a truth reader or grant evaluation access.

Schema-declared operator-authored assessments and controlled-operation inputs remain subject to their existing domain permissions and validation. The presentation policy is not a classifier for malicious text covertly encoded into otherwise permitted measurements or operator-authored prose. Future synthetic sensor adapters must publish observation schemas that exclude sealed actor mappings by construction.

### Durable observation references and rollout

Migration **21**, `00021_observation_public_references.sql`, adds `observations.public_id`. It issues a random UUID-based `view_obs_` reference for every existing and new observation and indexes it within scope. This reference is stable and immutable. It is not a public hash of a predictable scenario name, so an operator cannot test name guesses against a deterministic alias. Original primary IDs, evidence links, payloads, row counts, and ownership are preserved; public-evidence HTTP IDs remain unchanged.

Exercise rendering resolves references in batches through `db.Scoped`. Detail lookup, idempotent retries using a returned reference, assessment subjects/evidence, incident observation links, and observation-specific audit/assessment filters resolve back to the stored identity under the selected scope. A copied alias cannot resolve in another run/workspace/source restriction. A rejected input with no stored observation has no published evidence identifier. Original storage IDs remain accepted for compatible authorized callers; ordinary responses return the public reference. The `view_obs_` format is reserved for server publication references.

Apply the existing migrations through **21 before starting the updated server**. Startup now verifies the publication-reference column. Existing binaries using their generated explicit column lists can coexist with the additive column during a coordinated rollout, but the updated API and browser should be released together for the v2 stream and disclosure contract. Removing already-published references would break evidence consumers, so migration 21 refuses automatic Down; use an explicit recovery plan and verified backup/restore procedure.

The existing development database was not migrated or changed by this task. Tests use disposable PostGIS containers. No real public provider or native 3D engine is enabled.

### Verification

All required checks passed; local raw logs are in ignored `tmp/f0-5/`.

- `go test -count=1 ./...` and `go vet ./...` passed. The new presentation, queued-delivery, source-policy, reserved-partition, and credential-redaction tests are retained in the repository.
- The complete opt-in PostGIS integration suite passed **33 tests in 195.651 seconds**, including migration 21, the populated legacy ownership fixture, and the original scenario/alert/incident/mission/command/audit/replay workflows. Following the final input-map allowlist refinement, the affected integration selection passed again in **32.099 seconds**. The random-reference storage/immutability assertion also passed in a focused check.
- The race suite passed in the cached Linux Go 1.25.14 container with repository and module cache mounted read-only. CI and Taskfile now include the realtime and presentation packages. Native Windows CGO remains disabled.
- Frontend production build, regenerated OpenAPI TypeScript types, and Redocly validation passed. The baseline Vite large-bundle warning remains; OpenAPI validation has no new warnings. Protobuf was not changed by F0.5.
- Existing Playwright projects passed **65 tests with 49 intentional skips** using two workers and the baseline temporary ports 25173/25174. These tests mock API responses; they do not replace the F0.6 real-backend browser gate. The scoped socket, disclosure, and reconnect cases above use the real assembled API and PostGIS.
- `sqlc generate -f db/sqlc.yaml` completed and the generated observation model/query scans include the new column. Canonical Go formatting, UTF-8 source validation, and diff whitespace checks passed.

The first regression run identified documented wire-visibility assumptions in older tests (raw correlation names and storage IDs), and an over-filtered measured overlap. The implementation preserves measured overlap, classified/hedged alert meaning, stable evidence traversal, and compatible idempotent retries while removing the private values. No failing check is accepted as completion.

New tests inspect serialized envelopes and exercise HTTP bytes containing hidden-identity fixtures, rather than relying on the UI to hide fields. They cover simultaneous public/two-run delivery, explicit unauthorized/unknown/input/truth topics, source restrictions, idle revocation and policy close codes, reconnect/snapshot handshakes, authorization of queued frames, resource switches, overflow, missing context, reserved partitions even with truth authority, random-reference stability/immutability, evidence-link resolution, and preservation of raw public/stored evidence. The existing exercise acceptance and replay/failure/spatial cases remain required.

<a id="f0-6"></a>

## F0.6 — COP workspaces and compatibility acceptance

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

Implemented 2026-10-21 against the v2 architecture and data strategy. This closes the implemented F0/M0 foundation gate, together with the retained F0.1–F0.5 checks. P1 source admission and S0 simulation contracts remain future work; this task does not enable a public connector or select a 3D engine.

### Workspace behavior

The existing React app has explicit **Public Context** and **Exercise COP** navigation. The persistent context bar shows workspace, environment, run (or public context), source mode, clock basis, and live versus historical replay. Public evidence uses UTC evidence timestamps. The existing exercise adapter exposes UTC evidence timestamps alongside the run's virtual clock; this is not the richer S0 clock contract.

`GET /api/v1/auth/me` adds `resources` and `accessRevision`. The resource catalog is computed from live grants and existing scopes, including explicitly granted wildcard runs. It requires `operational.read` in each offered resource and excludes exercise input and truth even for a principal with explicit truth authority. It publishes no credential identifiers, token hashes, source allowlists, or grant rows. The opaque access fingerprint changes on grant and source-policy changes.

The UI first authenticates, then validates a previously selected resource against that identity's authorized catalog. Only the resource tuple is optionally remembered in local storage, under the principal ID. API credentials are still supplied through the established runtime/build configuration; changing configured identity requires a page reload. No login or identity administration product was added. A missing or unauthorized identity renders an access/reconnect screen without mounting domain views.

Every mounted workspace captures an API instance, resource, abort controller, query client, and query-key prefix containing principal ID, resource tuple, access revision, and session generation. Ordinary HTTP calls send explicit `X-EMAD-*` resource headers. Realtime connections use the same tuple in their URL and retain the existing credential subprotocol transport. There is no token in the URL or cache key.

Switching clears the previous client and mutation cache, cancels queries and network work, closes the socket, clears entity selection/drawing/replay/toasts, and navigates to the map without carrying selected object IDs. Late socket callbacks and mutation callbacks check the captured lifetime. Queries without the current capability remain disabled. Revalidation runs every five seconds and on focus; server authorization is checked on every request and stream delivery independently of that interval. Credential revocation or realtime authorization resynchronization removes the workspace. Reconnect authenticates again. Permission or source-policy changes detected by session revalidation create a fresh workspace lifetime.

Scenario controls remain in Exercise COP. Definitions and event queries require author capability; ordinary readers retain scoped run status. Starting/restarting a scenario explicitly selects the returned run. Assets/scenarios routes and mission/command tabs are unavailable in Public Context. Public tracks stay observational records.

### Compatibility and rollout

The new identity fields are additive. Existing REST methods and paths, source/observation/track IDs, evidence traversal, and configured legacy operator credentials retain the documented F0.2–F0.5 compatibility behavior. Clients omitting selection headers still use their server-bound credential default; starting a scenario updates that default. The browser now pins every mounted context explicitly, so another client changing the credential default cannot move its active COP. Historical run selection is also explicit.

Apply migrations through **21**, then deploy the matching API and UI. F0.6 introduces no migration. Existing development data was not changed: integration and browser gates use disposable PostGIS containers. Public workspace navigation becomes available only after a trusted administrator grants that principal a public resource with the required read permissions. Labels, local storage, URL parameters, and frontend visibility never grant authority.

The existing realtime v2 and observation presentation changes remain documented in [F0.5](#f0-5). The broader ownership migration and legacy quarantine remain documented in [F0.2–F0.4](#f0-2-4). Future native runtime, imagery, artifact storage, and provider adapters must enforce the same boundaries when introduced.

### Acceptance evidence

`internal/integration/workspace_browser_test.go` serves the built production UI and the assembled Go application from one test origin, using a freshly migrated PostGIS database. Playwright does not mock domain HTTP endpoints or WebSockets in this gate. The fixture-only credential-revocation route exists solely inside that test server and is absent from the application.

The real browser verifies scenario start and explicit run selection; geofence-derived alert acknowledgement; linked incident and mission creation; virtual asset command issue and automatic simulated completion; audit inspection; public-context switching; public tracks; inaccessible exercise detail in public scope; return to the authorized exercise; opaque observation references and hidden-actor fixture absence for an ordinary operator; and revocation removing the UI. It also checks resource headers and run-pinned, credential-free WebSocket URLs.

The negative matrix is retained in `workspace_catalog_test.go`, `scope_test.go`, `principal_realtime_test.go`, and `realtime_boundaries_test.go`:

- Workspace, environment, run, and unknown-action denials; public assets/commands denied.
- Input/truth excluded from catalog and ordinary transports, including explicitly truth-authorized callers.
- Cross-scope details, lists, counts, histories, relations, spatial reads, attachments, writes, and commands denied or scoped empty as appropriate.
- Explicit unauthorized subscriptions denied/audited; simultaneous public and two-run delivery; source policy, reconnect, queued-delivery authorization, revocation, overflow, and disclosure fixtures.
- Ownership backfill preserves known graphs, quarantines unknown legacy ownership, and enforces row policies and relation guards.
- The browser lifecycle regression delays old exercise requests, switches during replay with an entity selected, releases those responses, and verifies public data, cleared selection/replay, and retained public scope after reload. Existing mocked design, inspection, and operator-workflow suites remain in place.

Run the gate with `task test:browser:integration`, or build the UI and run:

```powershell
pnpm --dir apps/operator-ui exec playwright install chromium
pnpm --dir apps/operator-ui build
$env:C4ISR_TEST_INTEGRATION = '1'
$env:C4ISR_TEST_BROWSER = '1'
go test -count=1 -run TestWorkspaceBrowserCompatibilityGate -v ./internal/integration
```

Plain Go tests intentionally skip Docker/browser gates. `task test:integration` exercises the complete database suite. CI installs Chromium and runs the real-backend gate explicitly after building the UI and running the existing browser suites.

Verification: **34 PostGIS integration tests and 20 subtests passed in 204.742 seconds**; **65 existing browser tests passed with 49 intentional project skips**, plus the new workspace lifecycle regression. Both real-backend production browser flows passed (7.5 seconds of browser execution; 12.47 seconds including the disposable fixture). Go tests and vet; complete PostGIS integration suite; browser compatibility gate; existing Playwright suites plus the workspace lifecycle regression; access/HTTP/realtime race tests in Linux Go 1.25; production TypeScript/Vite build; OpenAPI generation/lint; canonical Go formatting and whitespace checks. Local raw logs and the public-workspace inspection image are in ignored `tmp/f0-6-*` files. The existing Vite large-bundle advisory remains.

<a id="p1-1"></a>

## P1.1 — Public source admission and durable evidence

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

Current installations also include [P1.2–P1.4](#p1-2-4) and [S0.1–S0.3](#s0-1-3), and require migrations through **25**. The version-22 rollout below records this increment's original baseline.

Implemented 2026-10-21 against the accepted F0/M0 foundation. This increment supplies public ingestion infrastructure in the existing modular monolith. Provider-specific USGS/FIRMS adapters, semantic observation routing, GIS imports, and the full evidence inspection UI retain their later task IDs. No live provider is enabled by this change.

### Source admission and authority

An existing `external` source in `PUBLIC_EVIDENCE` receives a reviewed, immutable admission version. It records the original producer, upstream provider, dataset/product version, connector location and service principal, revision policy, rights, coverage description, AOI, freshness thresholds, and retrieval budgets. The server records the human reviewer and approval time. A changed producer/dataset/product requires a new source identity. A changed rights document requires a new rights ID, including when returning to an older admission's ID.

Review requires a human principal with `operational.read`, `sources.read`, and `sources.manage` in that public resource. Execution requires a separate service principal with `operational.read`, `sources.read`, and `connectors.execute`, restricted to the source's returned canonical ID. The enabled admission validates that service's live grants. Human callers cannot execute a connector even if provisioned the execution permission. Service callers cannot approve an admission. The existing human role matrix gains no execution authority.

The admitted service ID, resource, source, and server location must agree. Current credentials/grants are checked again after external I/O and before evidence/publication writes. Revocation prevents a pending fetch from committing. A disabled/expired admission prevents new retrieval and pending publication; previously retained evidence keeps its historical rights snapshot. Rights expiry does not implement deletion or a general artifact retention scheduler. Analysis/export flags record reviewed eligibility; they do not grant new analysis or export capabilities.

### Retrieval configuration and adapter contract

`C4ISR_PUBLIC_CONNECTORS_FILE` names a server-owned JSON array. An unset value supplies an empty registry, with no polling or network activity. Invalid configuration fails startup. A deployment operator replaces the placeholder source ID with the canonical ID returned by source registration and reviews the real provider location and terms before enabling its admission. Example shape:

```json
[
  {
    "id": "reviewed-provider-page",
    "sourceId": "RETURNED_CANONICAL_SOURCE_ID",
    "resource": {
      "workspaceId": "public-context",
      "environmentId": "world",
      "partition": "PUBLIC_EVIDENCE"
    },
    "url": "https://provider.example.invalid/emad-page",
    "adapter": "emad-json-page-v1",
    "authTokenEnv": "EMAD_REVIEWED_PROVIDER_TOKEN",
    "referenceOrigins": ["https://provider.example.invalid"],
    "maxRequestsPerDay": 100
  }
]
```

The example is not a usable public feed or a provider approval. Omit `authTokenEnv` for an admitted unauthenticated endpoint. A named credential must exist in the server environment; only the environment variable name appears in configuration. Provider credentials and endpoint selection never come from admission input. Reference links are not fetched automatically.

The initial adapter sends a bounded HTTPS GET to the fixed endpoint with `bbox=west,south,east,north`, `from`, `to`, `limit`, and opaque `checkpoint` parameters. Optional provider authentication uses a server-side bearer header. It expects one JSON page containing `records`, `checkpoint`, `hasMore`, `coverageFrom`, and `coverageTo`. Each record carries `providerRecordId`, `providerRevision` when using provider versions, `referenceUrl`, `originalRecord`, and optional `observedAt`/`providerUpdatedAt`. The page contract lives in `internal/publicingest/domain.go`. The adapter identity is versioned; it is not an assumption that USGS or FIRMS natively serves this envelope. Their native format adapters belong to P1.4/P1.5.

For pagination, `hasMore=true` requires an advancing nonempty cursor. The query time bounds remain frozen, at PostgreSQL microsecond precision, until that query completes, including across failed or interrupted invocations. A final checkpoint is a reusable provider watermark for the next query. A native provider adapter must translate its cursor/watermark semantics accordingly. Coverage must be a nonempty interval within the frozen query and declared lookback; it describes provider query coverage, not verified completeness of the physical world.

Only credential-free HTTPS endpoints on port 443 are allowed. Production fetching rejects redirects, proxy discovery, private/loopback/link-local/metadata and reserved destinations, including unsafe resolved addresses. DNS addresses are checked and the checked address is dialed directly while TLS authenticates the original hostname. Responses have header, byte, timeout, encoding, and strict JSON limits. Reference URLs must use the endpoint origin or an explicitly allowed origin and cannot carry userinfo, query strings, or fragments. No raster download, archive extraction, Python worker, broker, or background daemon is introduced.

### Execution and recovery

Execution is an explicit, bounded worker invocation through the API. A scheduler may repeat it using the admitted service credential and source; the server does not start an autonomous polling loop. Admission policy limits each invocation to 1–10 pages, 1–1,000 records per page, at most 4 MiB per response, 1–10 seconds per fetch, and 30 seconds of total work. Original records are limited to 256 KiB. AOIs are finite WGS84 rectangles of at most 400 square degrees; split dateline crossings. Lookback is at most 31 days. The server manifest also caps requests/day per source binding.

Before any outbound request, a transaction reserves its daily quota and minimum request interval. Failures still consume quota. UTC daily budgets, outage backoff, provider `Retry-After`, checkpoints, and a 45-second execution lease persist across API/process restarts. Re-admission preserves quota and next-attempt constraints. Expired leases and admissions changed during execution produce an interrupted run history; competing invocations cannot share an active lease.

Each page commits immutable evidence, a publication receipt, its checkpoint, and run counts in one transaction. Duplicate source/record/revision keys compare normalized content hashes. An unchanged retry increments duplicate counts; a reused revision with changed content fails the page and preserves the prior checkpoint. New provider revisions retain older evidence.

The catalog is a separate projection. Publication inserts catalog visibility and acknowledges its receipt atomically, using the stable evidence ID. A projection failure leaves durable evidence and its checkpoint intact. The pending count is visible even if the catalog is still empty. `publications/retry` replays pending receipts without provider I/O; repeating it cannot duplicate catalog entries. Retrieval failure backoff is persisted; publication retry is explicitly invoked and does not introduce a second polling service.

Run states are `RUNNING`, `SUCCEEDED`, `PARTIAL`, `FAILED`, and `INTERRUPTED`. A successfully created execution returns HTTP 201 even when its run result is failed/partial; callers must inspect `status` and `failureCode`. HTTP 409 rejects an active lease, deferred invocation, or changed/disabled admission. Quota exhaustion records a partial run without fetching and defers further attempts to the next UTC day. Sanitized fixed failure codes avoid persisting provider error bodies or credentials.

### Evidence identity and provenance

Provider IDs/revisions are trimmed, case-sensitive, UTF-8 strings without control characters, limited to 512 bytes. Evidence IDs hash the JSON array `[sourceId, providerRecordId, providerRevision]`; source IDs already have canonical ownership. An `originRecordKey` hashes `[originalProducer, dataset, productVersion, providerRecordId, providerRevision]`, allowing a later analyst to recognize repeated upstream evidence across different providers/connectors. This key is an attribution aid, not automatic correlation or independent corroboration. Providers that rewrite IDs need an explicit translation in their adapter.

`hashVersion=emad-json-v1` identifies the current normalization: decode one UTF-8 JSON value with Go `UseNumber`, then serialize it with sorted object keys and preserved number lexemes. `contentHash` is SHA-256 of that result. Whitespace and object key order do not change it; `1`, `1.0`, and `1e0` remain different. This is not RFC 8785 canonicalization. For datasets without provider versions, `providerRevision` is `sha256:<contentHash>`.

`rawHash` is SHA-256 of the exact original record JSON bytes. When rights permit retention, those bytes live in `bytea` alongside the evidence document; JSONB cannot preserve their original formatting. The `/original` endpoint returns those exact bytes for hash verification. A JSON detail response embeds the parsed record and may normalize formatting, so it is not the byte-verification download. Reference-only admissions retain hashes, source/product/rights metadata, the permitted reference, retrieval time and HTTP validators, without retaining original bytes. Evidence links to the immutable admission and connector run that first retained it. Lists omit originals and are capped at 100 records; details enforce the same resource/source boundary.

### API and operator behavior

All paths below are relative to `/api/v1/sources/{canonicalSourceId}` and use the existing authenticated resource headers/default binding:

- `POST /admission` saves a reviewed version; `GET /admission` returns the current one.
- `GET /admissions/{admissionId}` resolves the original source/rights snapshot referenced by evidence.
- `GET /health` returns provider coverage, timestamps, next attempt, failure code, and pending publication count.
- `POST /connector-runs` executes the admitted service; `GET /connector-runs` lists its history.
- `GET /evidence` lists catalog revisions; `GET /evidence/{evidenceId}` returns a durable detail/reference, including a pending revision when its ID is already known.
- `GET /evidence/{evidenceId}/original` downloads retained bytes, or returns 404 for reference-only evidence.
- `POST /publications/retry` publishes pending receipts, up to 1,000 per bounded invocation, without refetching.

Provision identities with the existing local `apps/emad-admin` tool. A service spec uses `kind: "service"`, an exact public resource, the three execution permissions above, and `sourceIds: ["RETURNED_CANONICAL_SOURCE_ID"]`. Provision the reviewer separately as a human linked to an existing operator. `go run ./apps/emad-admin -action provision -spec <spec.json> -out <new-secret-file> -lifetime 24h` uses trusted database control-plane access and writes the token only to the named new file. See [F0 provisioning and rollout](#f0-2-4) for credential rotation and resource grants.

`GET /sources` and source detail now include an additive ingestion summary for public external sources. States distinguish `not_admitted`, `unknown`, `current`, `stale`, `unavailable`, and `disabled`. A completed query can become stale/unavailable by age; old resumed coverage cannot become fresh simply because its execution completed now. A later outage preserves previously successful coverage and evidence. The Systems page refreshes source health every 15 seconds and shows provider coverage and pending publication separately from registration status. Missing records never prove absence. The existing exercise inventory remains compatible.

### Persistence and rollout

Apply migration **00022_public_source_ingestion.sql** before starting the updated server. Startup checks for the new evidence storage. It adds six tables: admissions, connector state, runs, evidence, publication receipts, and catalog entries. All use restricted-role FORCE RLS with public-resource and source policies, composite ownership foreign keys, and scoped transactions. Admission/evidence writes are append-only; ordinary database access cannot update their originals. The migration refuses an automatic destructive Down operation. Existing source rows and exercise data are preserved. The developer database was not migrated or populated during implementation; verification uses disposable PostGIS databases.

Deploy with `C4ISR_PUBLIC_CONNECTORS_FILE` unset first. Select an actual provider adapter, source identity, region, current terms, rights, quota, and service credential before adding a trusted location and enabling an admission. Save an admission with `enabled=false` to suspend future retrieval/publication while preserving retained provenance. This task does not activate USGS/FIRMS or promote any record into an observation, track, assessment, exercise input, or ground truth. P1.2 owns semantic event/area/moving routing.

### Verification

Focused unit tests cover public destination rules, trusted location/source bindings, server credentials, bounded HTTP decoding/redirects/timeouts, normalized hashes and revisions, reference-only retention, and shared-origin identity. Five real API/PostGIS ingestion tests cover retries and revisions, atomic checkpoint recovery, publication failure/replay, concurrent leases, credential revocation during I/O, resource/source isolation, initial outages, persistent daily quota, historical rights, exact original hashes, and reference-only downloads. New browser coverage verifies that provider outages and pending evidence remain visible independently of active source registration.

Verification completed on Windows with native Go 1.25, Node 24/pnpm 10, disposable PostgreSQL 17/PostGIS 3.5 containers, and Chromium. All public provider inputs were controlled fixtures; no real credentials or public feed were activated.

- `go test ./...` and `go vet ./...` passed. Docker/browser tests are intentionally opt-in and were also run explicitly.
- The full PostGIS suite passed **39 top-level tests and 24 subtests in 236.678 seconds**, including all five new public-ingestion tests. Its browser gate was skipped in this invocation and executed separately below.
- Linux Go 1.25 race checks passed for `publicingest`, `access`, and `httpx`; `sources` compiled with no standalone test files. CI/Taskfile race targets now include the new ingestion module.
- The complete mocked browser suite passed **68 tests with 50 intentional project skips**, including outage/pending-publication coverage under both token configurations. The occupied default Vite port required the existing ignored alternate-port configuration; tests ran on ports 25173/25174.
- Both production-UI/real-Go/PostGIS workspace compatibility flows passed: **2 browser tests**, 10.3 seconds of browser execution, 19.751 seconds for the Go invocation including fixtures. These preserve scenario-to-command/audit behavior and truth/revocation isolation.
- Production TypeScript/Vite build, regenerated OpenAPI types, Redocly validation, sqlc generation, canonical Go formatting, and whitespace checks passed. The existing Vite large-chunk advisory remains. Mocked socket shutdowns emitted Vite proxy cancellation messages without test failures.

Local raw logs are ignored `tmp/p1-1-*` files, including `full-integration.log`, `browser.log`, `real-browser.log`, `race.log`, `go-test.log`, `vet.log`, `ui-build.log`, and `openapi.log`. The plan marks only P1.1 complete; M1 still requires P1.2–P1.6 and real provider acceptance.

<a id="p1-2-4"></a>

## P1.2–P1.4 — Public event semantics, reference snapshots and USGS

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

**Subsequent increment:** [S0.4 and P1.5–P1.6](#s0-4-p1-5-6) completes the adapter/M2 and public-history/M1 gates. Current installations require migrations through **27**, and the next task is **P2.1**. The original acceptance details below remain historical; statements about these tasks being pending are superseded.

Implemented 2026-10-21 on the accepted P1.1/F0 foundation. These are three completed tasks, not completion of the P1 workstream. P1.5 FIRMS and P1.6 comprehensive evidence/history views remain queued. [S0.1–S0.3 foundations](#s0-1-3) are now implemented; current installations require migration 25. The next task is S0.4/M2 before returning to P1.5–P1.6.

### P1.2: Reported events do not imply motion

Observations now carry `semanticKind` (`moving`, `point_event`, `area`) and `geometryRole` (`reported_location`, `reported_extent`, `measurement_footprint`, `derived_buffer`, `phenomenon_boundary`). Point geometry alone does not select correlation. Existing moving/scenario observations keep their behavior. Legacy area inputs default to area semantics and a reported extent. Named `event.*`/`detection.*` inputs, earthquakes, and thermal detections take the point-event path when no footprint is supplied. Contradictory motion overrides, non-moving track hints, and point/footprint role mismatches are rejected.

An optional normalized provider statement travels with immutable public evidence. It records the provider's report, rather than publishing an assessment or asserting independent corroboration. `normalizerVersion` identifies the trusted adapter. Original record hashes, provider IDs/revisions, observed time, provider revision time, receipt time, source identity, and the reviewed admission remain distinct. Normalized origin time cannot silently differ from the record's origin time.

Publication projects a report into an immutable event revision and, for a located non-withdrawn report, an observation. The event's current pointer selects the newest retained provider revision, with deterministic handling of equal timestamps. Older late arrivals remain in history without rolling the current pointer backward. Newly accepted corrections link to their previously published revision. Pending publication is ordered by provider revision time. History remains bounded to 200 entries per request with an explicit truncation indicator.

The event projection, current pointer, observation, PostGIS spatial alerts, catalog visibility, and publication acknowledgment commit together. A forced catalog failure was verified to leave no partial observation, event, or alert. Its durable evidence and pending receipt remain retryable without a provider fetch.

Spatial rules use the existing active geofences: `ST_Covers` for reported points, `ST_Intersects` for supplied footprints. A match records the triggering observation/evidence, geometry role, geofence ID, stored rule geometry and update time, and `public.event.spatial.v1`. This is a spatial match, not an inferred earthquake impact area or fire perimeter. A correction resolves the prior revision's affected alerts with a correction/withdrawal reason and successor evidence ID, then evaluates the new reported geometry. Prior evidence and alert references remain available. Each event evaluates at most 100 matched zones; excess matches leave publication pending instead of silently omitting alerts.

Direct point-event ingestion also evaluates PostGIS rules and can retry incomplete processing. The existing area and moving-track pipelines remain covered by their prior tests. The API/PostGIS acceptance includes repeated earthquake/thermal reports, a supplied measurement footprint, out-of-order revisions, a withdrawal, publication rollback/recovery, and resource/source isolation, all with zero event-created tracks.

### P1.3: A bounded reference and terrain catalogue

`internal/reference` owns immutable dataset versions, original subset artifacts, and PostGIS reference features. These records do not create observations, tracks, assets, or controllable entities. Imports require a human with `sources.manage` in `PUBLIC_EVIDENCE`; reads require `sources.read`. A registered source and its selected resource/source permissions are enforced through scoped transactions, composite foreign keys, and FORCE RLS. A rights ID continues to name its original reviewed terms across versions.

Every snapshot records producer, source/version, AOI, CRS, vertical datum, resolution/units, validity, quality limitations, transformation, attribution/rights, retrieval reference/date, artifact byte count and SHA-256. A parent artifact checksum is retained when available. The artifact hash is verified before import. An unchanged retry returns the existing version; changing content or metadata requires a new version. Source artifacts and metadata cannot be updated/deleted by ordinary application access.

The initial importer accepts EPSG:4326 GeoJSON and a regular JSON elevation grid. Vectors are validated and clipped with PostGIS; contradictory embedded geometry CRS is rejected rather than relabelled. Features retain provider identifiers and properties. Elevation grids declare point-sample registration, west/south origin, positive steps, south-to-north rows, west-to-east columns, metre heights relative to the declared datum, and null samples for no data. Grid spacing must agree with the stated angular resolution. No height-to-ellipsoid conversion is implied.

Limits are 512 KiB per artifact, 1 MiB per import request, 1,000 features/20,000 vertices per vector snapshot, 256×256 elevation samples, 64 versions/32 MiB per source, and 64 catalog entries per bounded list. Stored artifacts use PostgreSQL `bytea` at this scale; no object-storage service or API raster worker is needed. Larger formats and workloads require a later explicit extension.

The retained sample is the one-degree Jeddah/Makkah region `[39,21,40,22]`:

- **Natural Earth v5.1.2:** Jeddah and Makkah point features at 1:50 million scale. The source is generalized geographic context, with no building or current infrastructure accuracy claim. The subset preserves original coordinates/properties and records the full pinned-source checksum. Natural Earth supplies public-domain data and optional attribution. [Terms](https://www.naturalearthdata.com/about/terms-of-use/), [pinned source](https://raw.githubusercontent.com/nvkelso/natural-earth-vector/v5.1.2/geojson/ne_50m_populated_places_simple.geojson).
- **NOAA ETOPO 2022 v1:** a 60×60 sample of the 60-arc-second bedrock model, using its cell centres as point samples. Heights remain relative to EGM2008, positive up, with bathymetry retained. This is coarse regional relief, about 1.85 km north-south spacing; it is not local engineering terrain. [Product](https://www.ncei.noaa.gov/products/etopo-global-relief-model), [datum guide](https://www.ngdc.noaa.gov/mgg/global/relief/ETOPO2022/docs/1.2%20ETOPO%202022%20User%20Guide.pdf), [distribution terms/metadata](https://coastwatch.pfeg.noaa.gov/erddap/griddap/ETOPO_2022_v1_15s.html).

The prepared vector artifact is 1,436 bytes with SHA-256 `632504397c0c7602b7c925af429f20fa0fb8e19cb8f339e6c77806ba2aab71f7`. The terrain artifact is 62,381 bytes with SHA-256 `d6e4cb7298bf06bab6d4ae8701723616a95c29938a0c61c732e4942449c97fc7`. They are retained in `testdata/reference/jeddah-makkah`. The preparation script retrieved 377,871 elevation-source bytes using checked HTTP ranges, without downloading the approximately 478 MB global TIFF.

`scripts/data/prepare_reference_sample.py` rebuilds these inputs from fixed versioned URLs. It is an offline preparation tool requiring NumPy, tifffile and imagecodecs; the tested versions are pinned in `scripts/data/reference-preparation-requirements.txt`. None runs inside the Go API. Rebuilding produces a new retrieval timestamp; review a new snapshot version rather than changing an existing imported version. The original derived artifacts remain reproducible and downloadable independently of upstream service availability. The initial attempted ERDDAP subset endpoint timed out; the retained data was successfully extracted by byte ranges from NOAA's versioned TIFF instead.

The catalogue's `eligible=true` view exposes exact immutable version IDs/checksums and current rights eligibility for S0. Eligibility does not create an exercise package or grant export authority. S0 will define and authorize the frozen context manifest; this task introduces no competing package format or 3D engine.

### P1.4: A native bounded USGS adapter

The trusted adapter name is `usgs-earthquakes-v1`, restricted to `https://earthquake.usgs.gov/fdsnws/event/1/query`. It uses the existing human-reviewed admission, source-bound service principal, credential refresh, quotas, backoff, byte/time/page limits, exact reference rules, and transactional evidence/publication path. Provider credentials remain server-side; no provider token is needed for this public query.

Queries carry the admitted rectangular AOI, frozen origin-time interval, `eventtype=earthquake`, `format=geojson`, `orderby=time-asc`, one-based offset, bounded limit, `includedeleted=true`, and explicit no-data behavior. A partial cursor preserves the same query interval. A completed query starts the next interval at offset one. The upstream catalogue can change between pages, so its query is not a frozen database snapshot; overlapping bounded queries and revision deduplication allow reconciliation. Offset saturation is rejected instead of skipping remaining results. [Verified native parameters](https://earthquake.usgs.gov/fdsnws/event/1/).

Each feature's ID is retained; `properties.updated` in epoch milliseconds identifies its provider revision. Origin time, reported point, magnitude/type, depth in kilometres, network/code, provider review status, and quality indicators are preserved with the original feature. Depth remains a positive-down provider quantity whose reference may vary by network; it is not terrain or scene altitude. A provider's reviewed status is not an EMAD-reviewed assessment. Native query metadata may omit the summary feed's `count`; the parser handles both forms and validates any count supplied. [GeoJSON contract](https://earthquake.usgs.gov/earthquakes/feed/v1.0/geojson.php).

Explicit `status=deleted` produces a withdrawal revision, preserving older evidence and resolving its prior spatial alerts. It creates no fabricated location/observation for a coordinate-free tombstone. A record missing from a response does not imply deletion. Withdrawals/corrections outside the admitted rolling origin-time window may not be received: this is a bounded catalogue, not a claim to complete global ComCat history. Returned invalid/null geometry for a non-withdrawn record, invalid times, mismatched coverage, malformed JSON, or an outage cannot become a successful empty-world observation.

Recorded fixture data comes from a bounded USGS summary response retrieved on 2026-10-21; its first feature is retained with collection count adjusted to one. Corrected, withdrawn and invalid variants are deliberate test mutations, clearly identified in `internal/publicingest/testdata/usgs/retrieval.json`, not claims about actual historical revisions. An additional recorded native query covers the metadata difference discovered during live acceptance.

The authorized live smoke passed on 2026-10-21 using a temporary human-reviewed admission, exact-source service credential, production restricted HTTPS fetcher, and disposable PostGIS. One Puerto Rico query over the preceding day returned **five reports → five evidence revisions → five event projections → zero tracks**, with no pending publication. This validates the adapter without enabling a permanent source or schedule in the developer installation. ComCat's contributing network remains in each report; review the actual provider/source rights before deployment. [USGS credits policy](https://www.usgs.gov/information-policies-and-instructions/copyrights-and-credits).

### Map and inspection behavior

The existing MapLibre instance owns the new layers. Geographic context uses neutral reference points/lines/outlines; reported events use distinct, screen-stable points and supplied footprint fills. Elevation selection displays dataset coverage, not invented terrain geometry. Dataset selection names an immutable version and preserves its attribution, datum, resolution, quality, eligibility, and checksum. A reference-region action centres the camera. Inputs remain bounded to one selected reference version and at most 200 current retained event revisions; overlaps remain individually inspectable through the report list.

The public panel supplies keyboard/touch controls and a textual alternative to map picking. It shows source coverage/pending publication, supports revision inspection and original provider fields/quality, and downloads exact retained originals or reference artifacts over authenticated APIs. Measurement footprints have dotted outlines, derived buffers dashed outlines, and provider-reported extents/boundaries solid outlines, with explicit role labels. Original hashes are verified by the real-backend browser test. The panel sits outside floating map instruments and has its own bounded scroll area on desktop/mobile. Map or basemap failures leave source facts inspectable. Errors are distinct from a successful empty bounded view. Normalized provider statements are limited to 32 KiB and supplied footprints to 512 vertices, in addition to the ingestion/page and displayed-record limits.

Queries/selection use the established workspace cancellation, cache, permission, and remount lifetimes. The new views poll every 15 seconds for reports/health; they do not add a truth-visible WebSocket topic. Reference/report selection is local to the active workspace lifetime; this increment does not add persistent saved views. The existing track replay control does not reconstruct event history; public overlays explicitly identify that limitation during replay. P1.6 still owns comprehensive temporal filters and effective-time versus known-time views.

### Interfaces, setup, and rollout

New scoped APIs:

- `GET /api/v1/public-events`, `GET /api/v1/public-events/{evidenceId}`, and `GET /api/v1/public-events/{evidenceId}/revisions` require `observations.read` in Public Context. Lists include explicit withdrawals; location overlays omit them.
- `GET/POST /api/v1/reference-datasets`, plus version detail, `/features`, and `/artifact`. Reads use `sources.read`; import uses human `sources.manage`. The `eligible` query restricts versions to current simulation-use eligibility.
- P1.1 source admission, evidence, original-download, run and publication-retry APIs remain the execution and provenance boundary. OpenAPI and generated TypeScript include the additive contracts.

Apply migrations **00023_public_event_semantics.sql** and **00024_reference_catalog.sql** after version 22. Startup now requires schema 24. New event/reference tables enforce public resource/source ownership; retained revisions/artifacts are immutable. Down refuses destructive rollback; use the documented forward recovery/backup procedure. Existing observation IDs, moving scenarios, exercise assets/commands, and earlier evidence remain compatible. No developer database was migrated or populated during implementation.

To load the retained reference sample into an authorized installation:

1. Apply migrations through 24 and provision a human public reviewer with `operational.read`, `sources.read`, and `sources.manage` using the established F0 procedure.
2. Register separate external reference sources for Natural Earth and NOAA using that public resource. Retain their returned canonical IDs; a source is not an authenticated principal.
3. Run the import helper explicitly, using a human credential file and those IDs:

```powershell
./scripts/data/import_reference_sample.ps1 `
  -TokenFile ./private/reviewer-token.txt `
  -WorkspaceId public-context -EnvironmentId world `
  -PlacesSourceId RETURNED_NATURAL_EARTH_ID `
  -ElevationSourceId RETURNED_NOAA_ID
```

The helper sends the checked-in artifacts to the existing API, prints returned IDs/versions/checksums, and never prints the credential. Select Public Context → Map to inspect the imported versions. It does not fetch arbitrary provider URLs or grant an exercise access.

To activate USGS, register a separate external source; provision an exact-source service credential with `operational.read`, `sources.read`, and `connectors.execute`; add a trusted location using the adapter/URL above and an explicit server quota; then approve its producer, product, rights, AOI and retrieval policy through P1.1 admission. Invoke `connector-runs` with the service credential. The manifest remains unset by default, and no polling service or long-running public feed is enabled automatically. See [P1.1 operation](#p1-1) for the manifest, retry and suspension contracts.

For explicit live verification after a build with Docker available:

```powershell
$env:C4ISR_TEST_INTEGRATION = '1'
$env:C4ISR_TEST_USGS_LIVE = '1'
go test -count=1 -run TestUSGSAuthorizedLiveSmoke -v ./internal/integration
```

### Verification

Executed checks cover semantics, raw provider parsing, duplicate/revision/withdrawal handling, exact hashes, PostGIS spatial matching, publication rollback/recovery, reference metadata/geometry/terrain import, immutable snapshots, quotas and resource/source isolation. Existing scenario/area/spatial/workflow regressions remain in the full suite. The production browser gate now includes the public report/reference/original-download workflow in addition to scenario-to-command/audit and truth/revocation checks.

Verification used native Go 1.25, Node 24/pnpm 10, Chromium, PostgreSQL 17/PostGIS 3.5 Testcontainers, and Linux Go 1.25 for the race detector. The developer database was not changed.

- `go test ./...` and `go vet ./...` passed. The ordinary run intentionally skips opt-in database/browser/live cases, which were run separately.
- Full PostGIS suite: **41 top-level tests and 24 subtests passed in 240.902 seconds**. The live and browser gates were skipped in that invocation and passed independently. The final focused public/reference regression run also passed after tightening import validation and normalized-statement budgets.
- Mocked browser suite: **70 passed, 50 intentional project skips**, including the new public inspection, reference selection, original download, layer readiness, and narrow-screen interaction. Existing alternate Vite ports 25173/25174 were used because the default port was occupied.
- Production-UI/real-Go/PostGIS gate: **3 browser flows passed** (12.6 seconds of browser execution; 23.252 seconds for the Go invocation). This includes an exact-original SHA-256 comparison and retained elevation version inspection, plus the prior scenario-to-audit and truth/revocation flows. No API/WS responses are mocked in this gate.
- The authorized native USGS live smoke passed: **1 request, 5 reports/evidence/event projections, 0 tracks**, with no pending publication. It creates and removes a disposable database, rather than a permanent provider schedule.
- Race checks passed for observations, tracks, public ingestion, access and HTTP authorization. Event rules, public event projection and reference packages compiled under the detector; their transaction behavior is exercised in PostGIS tests. CI/Taskfile race targets include the new modules.
- Production TypeScript/Vite build, regenerated OpenAPI types, Redocly lint, sqlc generation, canonical Go formatting, whitespace, Python preparation syntax, and PowerShell import-helper parsing passed. The prior Vite bundle-size advisory and mocked socket shutdown/cancellation messages remain non-failing diagnostics.

Raw local evidence is in ignored `tmp/p1-2-4-*` and `tmp/p1-4-live.log` files. Desktop/mobile inspection images are `tmp/p1-2-4-public-context-desktop.png` and `tmp/p1-2-4-public-context-mobile.png`. F0 compatibility remains accepted; P1/M1 remains incomplete until P1.5–P1.6 pass their own gates.

<a id="p2-1-3"></a>

## P2.1–P2.3 — Retained imagery, environmental analysis and reviewed assessments

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

**Status:** P2.1–P2.3 complete, 2026-10-21. Implemented against the two v2 design proposals. This increment adds bounded Public Context intelligence and preserves the existing exercise, source, reference and simulation infrastructure. P2.4 approved context export remains separate; M3 is not yet complete. No 3D engine or live hardware path is introduced.

### Delivered workflow

A reviewed Copernicus source publishes immutable Sentinel-2 L2A catalogue revisions. An authorized user inspects acquisition, geometry, product baseline, cloud/quality, rights, asset references and exact STAC originals. Catalogue discovery alone never makes a scene analysis-ready.

A source manager prepares a small multispectral subset from licensed local B04/B08/SCL files, retains the subset through the authenticated artifact API, and queues validation. An analyst chooses two successful preparations, submits an NDVI comparison, inspects aligned before/after/difference panels and thresholded finding geometry, and downloads the exact result. The processing executable runs independently of HTTP handlers. The analyst records an explicit interpretation proposal; another human reviews and publishes or rejects it with a rationale. Every assessment revision pins the exact job, scenes, artifact hashes, method, quality limitations and actor identity.

The new `/intelligence` workspace is linked from the Public Context panel. Exercise COP remains independent. No scene or finding is converted into an observation, track identity, asset or simulation truth automatically.

### P2.1: bounded discovery and object storage

`cdse-sentinel2-stac-v1` uses the native CDSE STAC 1.1 endpoint and `sentinel-2-l2a` collection. The server owns the endpoint; browser requests cannot supply arbitrary upstream URLs. The adapter accepts only the expected Sentinel-2 L2A product, identity/time/geometry fields and selected `s3://eodata/Sentinel-2/MSI/L2A/…` asset references. It does not fetch asset URLs. Reported scene-wide cloud cover above 30% is excluded and counted; retained scenes still require local mask validation. A scene-wide percentage is not a local clear-pixel guarantee.

Admission limits remain authoritative: one bounded EPSG:4326 AOI, at most 31 days per retrieval window, no more than 25 records/page for this adapter, minimum five-second request spacing, response byte limits, daily budgets, timeout, durable backoff and bounded pages per invocation. The optional `from`/`to` body on `POST /sources/{id}/connector-runs` selects an ordered historical UTC interval no longer than the admitted lookback. Without it the existing rolling-window behavior remains. A partial retrieval resumes its frozen window and opaque vetted token; it cannot switch windows midway. Complete evidence records can publish while the source run is partial, but none is implicitly prepared for analysis.

Original STAC feature bytes and hashes are retained through existing public evidence. Scene metadata is an immutable projection in the same publication transaction. Known object IDs and scene IDs do not bypass source/resource authorization. Lists are bounded to 100 with explicit truncation; spatial filtering uses PostGIS.

New large artifacts use a private path-style S3-compatible port. PostgreSQL owns metadata, source admission, rights, scope, SHA-256, byte count, state and retention. The browser sees an artifact ID, never a bucket key, storage credential or public presigned URL. Object keys bind hashed workspace/environment, PUBLIC_EVIDENCE, source, artifact identity and content digest. The port requires the final key component to equal the exact content hash; concurrent application writes to one key can therefore only contain identical bytes. It checks existing bytes before writing and refuses a different/corrupt existing object. `If-None-Match: *` supplies additional protection on providers that implement conditional PUT. The implementation does not depend on every S3-compatible provider honoring that header.

Objects are 1 byte–16 MiB; each source/resource has at most 1,000 nondeleted objects and 512 MiB, including pending reservations. Reservation request keys are idempotent and conflict if metadata changes. Upload transitions PENDING to READY only after exact size/hash validation and renewed rights/expiry checks. Interrupted uploads can retry the same reservation. Reads validate current source access, rights, expiry, size and hash. There is no incomplete-object fallback.

Retention is explicit, immutable, no more than 365 days and no later than admitted rights expiry. The UI chooses up to seven days. Metadata and hashes remain after authorized deletion; content reads fail after expiry. `DELETE /artifacts/{id}` is denied before retention ends, and thereafter performs retryable DELETING → DELETED cleanup. No automatic janitor is added; operators invoke authorized cleanup. Source suspension blocks use, while expired-content cleanup remains possible. Configure private bucket access and preserve blobs together with database backups; existing PostgreSQL-only backups do not include new S3 content. Use storage versioning/backups when required by deployment policy. Existing small reference/context artifacts remain compatible in PostgreSQL.

### P2.2: declared environmental baseline

The first use case is **NDVI difference**, version `emad.ndvi-change.v1`, from two retained `emad.multispectral-subset.v1` JSON inputs. This is a reproducible numerical indicator with an explicit human interpretation step. It is not a vegetation-loss classifier, causal assessment or calibrated confidence model.

Each input pins scene revision/hash, UTC acquisition, platform, MGRS tile, orbit, processing baseline, three source-file hashes, preparation method, explicit surface-reflectance unit, red/NIR samples, SCL values and target grid. Source hashes and provenance are preparer assertions; the worker verifies retained subset bytes and catalogue identity, not a fresh upstream raster download. Synthetic inputs remain visibly synthetic in results and assessments.

The initial angular grid has EPSG:4326 longitude/latitude cell edges, north-to-south rows, at most 256×256 cells and 0.000001°–0.001° spacing. The scene footprint must cover it according to PostGIS. Both inputs must have identical grids, source, platform, tile, orbit and processing baseline, ordered acquisition times at most 90 days apart. Mixed sensor/baseline comparisons are rejected. Angular spacing is not a constant metre GSD, equal-area measurement or native 10 m imagery claim.

Only SCL 4/5/6 (vegetation, nonvegetated land, water) are used. No-data, saturated/defective, dark/topographic shadow, cloud shadow, unclassified, cloud, cirrus and snow/ice classes are excluded. Negative/nonfinite/null reflectances and nearly zero NDVI denominators are masked. Common clear coverage must meet the selected fraction (0.1–1); the decline threshold is 0.05–1 NDVI units. The retained result records before/after NDVI, difference, mask counts, coverage, mean difference and thresholded cells. It does not infer meaning from masked pixels. SCL is imperfect; residual cloud, atmosphere, illumination, registration, resampling and seasonal differences remain stated limitations. Same orbit/baseline and a 90-day bound do not remove those effects.

Thresholded cell polygons are normalized with PostGIS `ST_UnaryUnion`. PostGIS build identity is recorded with the output; geometry exceeding 4,096 changed cells fails explicitly instead of silently truncating. Display panels share scales, grid and selected cell, expose numeric text/keyboard inspection and mask labels, and draw the derived boundary over the difference panel. That boundary represents thresholded cells rather than a surveyed environmental boundary.

The queue is PostgreSQL-backed; no broker or mandatory ML service is introduced. Frozen input manifests are at most 64 KiB, with SHA-256 and idempotent request keys. Each source/resource permits 32 active jobs. Statuses are QUEUED, RUNNING, CANCEL_REQUESTED, CANCELED, FAILED and SUCCEEDED. Workers claim with SKIP LOCKED, use a 150-second lease and 120-second processing deadline, and poll cancellation/live credentials every 500 ms. Failed jobs can be explicitly retried up to three total attempts without changing their inputs. Expired leases record an immutable interrupted attempt before authorized recovery. A queued cancellation is immediate; running cancellation cannot publish a result. A revoked worker cannot finalize its lease; another authorized worker can recover it after expiry. Process termination records failure where possible, otherwise lease recovery applies.

The worker records the SHA-256 of its executable, algorithm, parameters, full pinned scene/artifact metadata, attempt and output hash. It rechecks source rights before/after object I/O and before comparison publication. Results retain no longer than either input. Scalar comparison is deterministic for the same bytes/configuration; the acceptance fixture reproduces identical output bytes/SHA with the same build/PostGIS environment. No bitwise promise is made across different preparation/GDAL/PostGIS builds. Retain the worker binary/environment and original subset bytes to repeat a historical computation. Replay of a retained result and rerunning an algorithm are distinct operations.

#### Local raster preparation

The offline helper is optional until real local rasters are available. It runs outside the API process and has no provider credentials or remote raster downloading:

```sh
python -m venv .venv-imagery
# Activate this environment using your shell's normal procedure.
python -m pip install -r scripts/data/imagery-preparation-requirements.txt
python scripts/data/prepare_imagery_subset.py --scene scene.json --red B04_10m.jp2 --nir B08_10m.jp2 --scl SCL_20m.jp2 --metadata MTD_MSIL2A.xml --west 39.16 --north 21.49 --step-lng 0.0002 --step-lat 0.0002 --width 128 --height 128 --output subset.json
```

Download `scene.json` using the catalogue's exact scene manifest button. Obtain permitted product files through your licensed provider account. CDSE S3 raster downloads need account-generated credentials; none were configured during acceptance. They are different from EMAD's own artifact-store credentials. Do not put either credential in frontend config.

The helper accepts bounded local TIFF/JP2 single-band rasters (512 MiB/file; at most 125 million source cells), reads windows through GDAL reprojection rather than loading full scenes, and caps threads/warp/cache memory. It uses average band resampling and nearest SCL, preserves DN=0 as no-data and requires product XML with a matching processing baseline, explicit BOA quantification and explicit B04/B08 offsets. Reflectance is `(DN + BOA_ADD_OFFSET) / BOA_QUANTIFICATION_VALUE`; it never silently assumes a zero offset. Rasterio/GDAL versions, metadata hash, radiometry and source-file hashes are retained in the output. Network projection lookup is disabled. Identical local fixture inputs produce identical subset bytes. Run it with ordinary OS resource/process limits for production workloads. The first helper supports a deliberately narrow explicit-offset product format; unsupported older metadata must be handled by an explicitly versioned preparer rather than guessed.

### P2.3: interpretation and independent review

New imagery findings extend the existing assessments module with source-bound roots and immutable revisions. Legacy assessment rows keep their existing behavior. Review states are DRAFT, PROPOSED, IN_REVIEW, PUBLISHED and REJECTED. Initial proposals are PROPOSED. A revision of a published/rejected/draft/proposed finding starts a new DRAFT; submitting it creates PROPOSED. An independent human moves it through IN_REVIEW to PUBLISHED or REJECTED. The author cannot review their own proposal. Review actions require a rationale; publication additionally requires analyst confidence and its separate rationale, plus still-usable exact evidence. Concurrent stale actions fail with 409.

Source STAC evidence, numeric algorithm outputs and human interpretation remain separate. A proposal explicitly says OPERATOR, ALGORITHM or AI. Optional AI model score is a separate field and cannot substitute for analyst confidence. A reviewer assigns confidence explicitly. Publication never creates an authoritative observation, changes track identity or converts synthetic pixels into public measurements. The shared upstream collection is displayed, so two temporal scenes are not presented as independent corroborating reports.

Each action appends both a revision and a scoped/source-filtered audit entry atomically. Revisions capture author, action actor, times, rationale, evidence, method and quality limitations. The original author remains attached through review; a new authored draft receives its own author. A later draft or rejection preserves the last published revision pointer. Legacy GET/list responses and incident assessment attachments include explicit review state and revision pointers and use the latest reviewed-workflow projection. Incident relation choices also label that state and link to review history. They do not describe a new draft as published. History lists are bounded to the latest 100; `GET /assessments/{id}/revisions/{revisionID}` retrieves a pinned older version directly. Publication provenance remains inspectable if source bytes later expire, but expired content is unavailable and cannot support a new publication until permitted evidence is retained again.

### Rollout and activation

Apply **00028_imagery_artifacts.sql**, **00029_imagery_analysis_jobs.sql** and **00030_reviewed_imagery_assessments.sql** after the user's already-applied version 27, using the established `task db:migrate` procedure. These are additive FORCE-RLS tables and assessment fields/policies; no existing observation, source, reference or scenario evidence is rewritten. Destructive down migration is refused; recover forward. Implementation tests only migrate disposable databases. Startup requires these new tables.

Provision a private S3-compatible bucket yourself; API startup never creates it. Configure the API and worker with the same endpoint/bucket/region and mounted secrets listed in `.env.example`. TLS is required outside development/test. Endpoint/bucket/access/secret must be supplied together; leaving all absent keeps existing functions available while artifact operations report storage unavailable. Storage credentials need the selected private bucket's get/put/delete access and must not be shared with browsers.

Build and provision the independent worker:

```sh
go build -o bin/emad-imagery ./apps/emad-imagery
go run ./apps/emad-admin -action provision -spec private/imagery-worker.json -out private/imagery-worker.token -lifetime 24h
# Use provisioned DB authentication; no bootstrap human token is needed.
# Set C4ISR_DATABASE_AUTH=true and load the database/S3 config in this process.
./bin/emad-imagery -token-file private/imagery-worker.token
# Add -once to process at most one queued job.
```

On Windows, build `bin/emad-imagery.exe` and run `.\bin\emad-imagery.exe -token-file private/imagery-worker.token` in PowerShell. Keep the built binary with the records needed to reproduce a job.

Worker spec (replace the registered source and actual resource binding):

```json
{"id":"imagery-worker-region","kind":"service","name":"Regional imagery processor","resource":{"workspaceId":"public-context","environmentId":"world","partition":"PUBLIC_EVIDENCE"},"permissions":["operational.read","sources.read","imagery.jobs.execute"],"sourceIds":["REGISTERED_SOURCE_ID"]}
```

Use a distinct connector service with `operational.read`, `sources.read`, `connectors.execute`. Merge `scripts/data/cdse-connectors.example.json` into private server-owned connector config and set `C4ISR_PUBLIC_CONNECTORS_FILE`. No credentials are needed for public catalogue discovery and no provider schedule starts automatically. The connector does not receive the worker execution capability.

An authorized human admits the source with dataset `sentinel-2-l2a`, productVersion `stac-1.1-l2a`, revisionPolicy `content_hash`, producer/provider, exact connector/principal, reviewed rights/attribution, selected AOI and policy. Suitable bounded example values: 31-day lookback, 3,600-second fresh period, 7,200-second unavailable period, minimum 5-second spacing, 10-second timeout, one page/invocation, 10 records/page, 1 MiB response and 100 requests/day. Approve rights that permit the intended retention and analysis; an adapter's availability is not a rights decision. For historical discovery, send an explicit interval such as `{"from":"2025-02-01T00:00:00Z","to":"2025-02-28T23:59:59Z"}` with the connector credential. Retry partial retrieval with the same window; inspect source health and publication status.

Human grants: `sources.read` inspects catalogue/jobs/artifacts; `sources.manage` also retains and cleans up subsets; `imagery.jobs.submit` submits/cancels/retries; `assessments.read` inspects findings; `assessments.create` authors/submits; `assessments.review` reviews another person's proposal. Existing analyst/supervisor role permissions are extended compatibly, but trusted provisioning must still grant the public resource explicitly. Reviewers must be separate human identities. The worker execution capability is service-only and is not a human role default.

### Verification evidence

Executed checks pass with Go 1.25, Node 24/pnpm 10, Chromium, Python 3.11/Rasterio 1.4.4/NumPy 2.2.6, actual disposable Garage 2.4.1 and PostgreSQL 17/PostGIS 3.5:

- Go tests and vet; canonical formatting; stable sqlc/TypeScript generation; Redocly validation; production TypeScript/Vite build; tracked whitespace checks.
- Linux race detector for incidents, assessments, imagery analysis, public ingestion, access, HTTP and realtime; artifact/imagery packages compile under the detector and exercise their database/I/O paths in integration acceptance.
- Two offline georeferenced raster preparation cases covering explicit offsets, masks/no-data, repeated bytes and invalid metadata/bounds.
- 74 mocked browser tests pass, with 50 intentional project skips. Mocked socket shutdown messages remain non-failing.
- Five production-browser flows pass through real Go/PostGIS/S3: binary subset upload/validation/comparison/download/independent publication; empty/invalid/unavailable imagery views; existing public history/originals; scenario-to-command/audit across workspaces; and truth/revocation isolation. Automatic response checks confirm zero API 5xx reaches the browser. Backend cancellation/shutdown diagnostics remain non-failing. Desktop comparison geometry and narrow-screen review/history screenshots were visually inspected.
- Final complete PostGIS regression passes **46 top-level tests and 24 subtests**, with zero failures, in **270.522 seconds**. Four intentional opt-in skips cover live CDSE, FIRMS, USGS and the separately executed browser gate. Native CDSE live acceptance and all five browser flows passed separately. Focused imagery and legacy scenario-to-command/incident/audit acceptance also pass after the compatibility projection change.

 Raw local logs/screenshots are ignored under `tmp/p2/`; reproducible source fixtures live under `testdata/imagery/`.

`TestP2CatalogueS3WorkerReproducibilityAndIndependentReview` uses an actual disposable S3-compatible Garage 2.4.1 server and PostgreSQL 17/PostGIS 3.5. It verifies recorded native STAC metadata, partial-window recovery, pending uploads, hash/size checks, exact object bytes, repeat output hashes, PostGIS finding geometry, queued/running cancellation, dead-process lease recovery, storage-failure retry, withdrawn analysis rights, reverse acquisition rejection, independent AI review with separate score/confidence, draft/rejection history with preserved publication, atomic audit, incident review projections, retention/deletion, source/workspace/partition isolation, revoked-worker recovery and queue capacity shared with retries. It confirms zero observations/tracks/assets were created. The fixture's spectral pixels are intentionally synthetic. It makes no claim of observed vegetation change.

The opt-in native live catalogue test passed on 2026-10-21: one bounded public request retained four scene revisions in a disposable database. No raw raster was downloaded, no live raster analysis was executed, no provider schedule or developer service was enabled. The two recorded eligible scene metadata features have retained retrieval URL/hash in `testdata/imagery/retrieval.json`. Offline preparation tests use generated local georeferenced raster fixtures, not observed spectral measurements.

Commands:

```sh
go test ./...
go vet ./...
C4ISR_TEST_INTEGRATION=1 go test -count=1 ./internal/integration/...
C4ISR_TEST_INTEGRATION=1 C4ISR_TEST_CDSE_LIVE=1 go test -count=1 -run TestCDSEAuthorizedLiveCatalogueSmoke -v ./internal/integration
python -m unittest discover -s scripts/data -p test_prepare_imagery_subset.py -v
pnpm --dir apps/operator-ui build
pnpm --dir apps/operator-ui contracts:generate
pnpm --package=@redocly/cli dlx redocly lint api/openapi/openapi.yaml
task test:browser:integration
```

The live test is deliberately opt-in so ordinary CI does not depend on provider availability. Real browser acceptance uses recorded public metadata, synthetic retained pixels, the production Go API, independent test worker, real S3/PostGIS and separate reviewer credentials. Ordinary mocked browser suites continue to cover existing views/workspace behavior.

### Primary references checked during implementation

- [CDSE native STAC](https://documentation.dataspace.copernicus.eu/APIs/STAC.html) and [account-bound S3 raster access](https://documentation.dataspace.copernicus.eu/APIs/S3.html).
- [Copernicus terms](https://dataspace.copernicus.eu/terms-and-conditions) and [Sentinel legal notice](https://sentinels.copernicus.eu/documents/247904/690755/Sentinel_Data_Legal_Notice).
- [Sentinel-2 processing and BOA offset](https://sentiwiki.copernicus.eu/web/s2-processing), [L2A scene classification](https://docs.sentinel-hub.com/api/latest/data/sentinel-2-l2a/) and [Rasterio reprojection](https://rasterio.readthedocs.io/en/stable/topics/reproject.html).
- [S3 conditional PutObject](https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html) and [Garage S3 compatibility](https://garagehq.deuxfleurs.fr/documentation/reference-manual/s3-compatibility/).

Next: **P2.4**, approved public-to-simulation context export through the existing frozen manifest/trust boundary. This increment does not approve or export imagery into exercise input automatically.

Subsequent delivery: **P2.4 is now complete** through [approved frozen context export](#p2-4-p3-1-2), with an M3 headless acceptance run using synthetic spectral fixtures. P3.1-P3.2 are also implemented; P3.3 is next. The original P2.1-P2.3 acceptance scope above remains unchanged.

<a id="p2-4-p3-1-2"></a>

## P2.4 / P3.1 / P3.2 — Approved context and bounded moving sources

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

This increment delivers the three tasks after P2.3. It reuses the existing source admission, immutable evidence/outbox, observation/track projections, private artifact storage, assessment review, and frozen simulation contracts. It adds no 3D engine, hardware integration, engagement controls, weather feed, or general news ingestion.

### P2.4: approved context export

The Intelligence workspace now offers AOI, UTC acquisition interval, reference layer, retained subset and published-assessment selection. Selecting a published assessment pins that exact immutable revision, its comparison result and its two prepared inputs, even if a newer draft exists. Additional successful preparations may be selected as background context. Drafts, unreviewed proposals, restricted sources, failed jobs, uncovered AOIs and out-of-interval acquisitions are rejected.

`POST /api/v1/context-packages/preview` creates a source-restricted, creator-bound candidate for ten minutes. It returns the manifest, byte size, hash, rights, accuracy, transform chain, provenance and omissions. It does not create an approved package or offer candidate bytes as an approved download. A changed selection clears the UI preview, including responses arriving after that change.

An authorized human with `context.export` supplies the preview ID, exact preview hash and a 10-2000 character approval statement to the existing `POST /api/v1/context-packages`. Source admission/export rights and retained dependency hashes/availability are checked again. Source/admission and artifact rows are held through the freeze transaction; permissions are refreshed before persistence. A changed admission requires a fresh preview. Service principals cannot approve packages. Reads, lists, previews and frozen packages retain workspace/source isolation.

The established `emad.context.v1` envelope and offline validator remain in use. Reference-only legacy packages keep `reference-snapshot-v1`; enriched packages declare `context-snapshot-v2`. New asset kinds are `imagery`, `analysis`, `assessment` and `derived_geometry`. Every imagery-derived asset retains the current admission and rights snapshot, original producer/rights, scene/evidence IDs, job and artifact IDs/hashes, and the exact published revision where applicable. Acquisition time, WGS84/angular resolution, processing method and limitations remain explicit. The exported availability deadline is bounded by artifact retention and both applicable rights expiry dates.

Only derived finding geometry is intersected with the AOI, using PostGIS. It carries `syntheticDerived: true`, `scenario_context_only`, its source result hash and the published revision. The package also declares derived geometry as synthetic. Reference files and prepared spectral subsets retain their declared extent; there is no implicit full-raster download, resampling or clipping. Published assessments remain interpretations. Public aircraft/vessel positions are never turned into actors or ground truth.

Frozen bundles download through the existing `GET /api/v1/context-packages/{id}/bundle`, with exact asset bytes encoded in JSON. They cannot mutate the original public records. The existing S0 loader consumes enriched bundles without a competing format. Reference-only bundles explicitly declare external dependencies and fail offline reproduction validation. No resolver dereferences their URLs automatically.

Limits: 16 assets, 8 MiB retained asset bytes, 12 MiB serialized candidate/download, 32 active previews/64 MiB and 64 frozen packages/64 MiB per authorized scope view. Vector/elevation inputs retain their existing 512 KiB per-asset limit. The present headless runner has a separate **4 MiB total frozen scenario JSON** budget; preview omissions identify this, and larger exports are not claimed to run in that adapter. An approved copy does not extend an upstream retention/rights deadline. Availability expiry blocks new loading; it does not erase immutable package metadata or implement a general archive purge.

### P3.1: external moving hypotheses

The new `externaltracks` domain writes source-bound public hypotheses through the existing durable publication transaction. Native moving evidence creates ordinary normalized observations with `semantic_kind=moving` and `geometry_role=reported_location`; the underlying position, reported quality and provider claims remain traceable to the exact evidence record.

`source-bound-motion-v1` is deliberately limited:

- Source + entity kind + claimed ICAO24/MMSI identify a candidate segment. There is no cross-provider merge or automatic verified identity.
- A gap over 30 minutes starts a new segment and closes the prior candidate. A report older than all known segments may create a separate historical segment; continuity is not invented.
- Strictly newer reports update state only within a PostGIS geography gate: aircraft 1500 m/s plus 15 km slack, vessels 100 m/s plus 2 km slack. These are coarse rejection gates, not accuracy estimates or platform physics.
- Late/same-time evidence is recorded as `LATE`, linked without rewinding current state. A failed motion gate records `QUARANTINED`; it cannot update the track position or become an accepted track observation.
- Every `APPLIED`, `LATE` and `QUARANTINED` decision retains origin/known times, method, rationale, reported values and evidence/observation links. Decisions are immutable. No automatic reassociation silently rewrites past decisions; a richer correction workflow remains future work.
- Read-time live/stale/lost states use admitted freshness thresholds. Reception older than 30 minutes is closed; publication maintenance also updates stored lifecycle status. Source-disabled/rights-expired conditions and partial/outdated capture health remain separate from position age.

The API exposes bounded `GET /external-tracks`, `/{id}` and `/{id}/associations` under `/api/v1`. Lists accept AOI/source filters and cap at 200 tracks; histories cap at 100 with explicit truncation. Reported identity, classification and affiliation claims remain separate from analyst confidence. Provider quality is not converted to confidence. A registration-country inference is not affiliation, and repeated network snapshots are not independent corroboration.

The UI shows claims, units, provenance, source health, age, decisions and exact-record downloads, with a bookmarked selection and link into the existing MapLibre map. Public map track/history queries and the hypothesis list poll every 15 seconds. No second map renderer or public asset/command controls are introduced. Restrictive source RLS also protects the existing generic track detail/state/history endpoints, including known identifiers. The native correlation prefix is reserved from legacy scripted/manual ingestion. Exercise workflows retain their existing correlation path.

### P3.2: provider choice, protocols and activation

**OpenSky is the selected ADS-B provider.** Its native bounded `/api/states/all` endpoint is implemented; no generic intermediary response is assumed. ADS-B.lol's current public API documentation marks that API deprecated and directs use toward feeder-restricted access, so it was not selected for this increment.

The initial OpenSky adapter uses the documented anonymous current-state endpoint. Requests include only the admitted latitude/longitude box. ICAO address, callsign, inferred registration country and altitude remain provider claims. Position time comes from `time_position`, never `last_contact` or retrieval time. Missing positions are excluded, invalid coordinates/times are rejected, and original state-vector JSON bytes remain downloadable. A snapshot does not establish historical coverage across its lookback interval. Content revisions/deduplication reuse the existing evidence facilities.

Technical anonymous access is **not a license grant**: OpenSky's current terms require a written agreement for operational API use, including internal automated products, and separately restrict commercial use. Obtain the applicable agreement and record approved retention, analysis, use and attribution terms before enabling an admission. No live OpenSky call was made as part of this increment; software acceptance uses explicitly synthetic native-format records. Account/OAuth support is not implemented or claimed.

AISStream uses its fixed `wss://stream.aisstream.io/v0/stream` endpoint. The key stays on the server in `AISSTREAM_API_KEY`, named by trusted registry `authTokenEnv`. The client negotiates permessage-deflate, subscribes within two seconds using **latitude/longitude** bounding corners, requires the provider's compression confirmation, and accepts compressed binary UTF-8 JSON position frames. PositionReport and both standard/extended class-B position messages are normalized. Full provider UTC timestamps are required; unavailable AIS speed/course/heading values remain absent. Knots convert to m/s. MMSI/name/navigation fields remain claims, and position-accuracy flags remain source quality.

Each AIS connection is a bounded 6-10 second capture, with one in-flight stream per fetcher and the existing per-source database lease across processes. A decoded frame is at most 64 KiB; captures retain at most 200 records/1 MiB. Disconnections/record/byte exhaustion are explicit partial results. Accepted records survive a mid-capture disconnect. Empty completed captures retain capture metadata without manufacturing an observation. Credentials echoed by upstream are rejected before retention. There is no durable upstream replay or continuity guarantee; missing messages between connections cannot be reconstructed. Run metadata records capture bounds, compression and discontinuity even when no record arrives.

Provider-independent limits: one page, AOI at most 25 square degrees and at most five minutes of accepted source-time lookback. OpenSky uses at least 60-second spacing and at most 300 requests/day; AIS uses at least 10-second spacing and at most 1000 connections/day. Prefer the more conservative example intervals. OpenSky's documented anonymous credit ceiling is 400/day, with the smallest AOI tier costing one credit; the adapter's smaller cap leaves headroom. Registry/admission budgets, retries, outage backoff and deduplication persist across restarts. Live moving feeds always use a fresh window after a failure/partial result, not a historical cursor.

Each moving source is bounded to 10,000 retained evidence records/64 MiB of evidence JSON and original bytes, and 2000 hypothesis segments. A page crossing the evidence budget rolls back atomically and records `source_storage_budget`. Limits are archival stop limits, not a rolling retention/purge implementation. Source expiry stops new capture/publication; historical evidence remains immutable. Enable only with rights compatible with this retained archive. Providers requiring automatic timed erasure need a separately approved lifecycle implementation before activation. Do not evade quotas/archive limits by duplicating source identities.

AISStream currently documents three connections per account/IP and no durable replay/SLA. Operate one configured worker/source for the initial account and account for other clients sharing that account/IP; this application cannot reserve third-party connections. Review account-specific access and retention terms before activation. No key was present, no live AIS connection was made, and redistribution permission is not inferred from public API visibility.

### Rollout and operation

Apply new migrations **31 and 32** after the already applied version 30:

```sh
task db:migrate
```

31 adds expiring restricted export previews. 32 adds source-restricted external tracks/segments/immutable associations and the supporting indexes. Existing migrations/data were not rewritten. The API checks for the new tables at startup. These migrations were exercised only against disposable acceptance databases during development; your developer database has not been migrated by this change. External evidence rollback is deliberately refused in favor of forward recovery.

Keep the existing private artifact store and separate imagery worker configured for enriched export. Grant `context.export` explicitly to the intended public human approver, plus source access and `assessments.read` for reviewed assessment export. The supervisor role ceiling now permits this capability; existing stored grants still determine authority. Exercise authors additionally need their existing scenario-author/control permissions. No automatic review or simulation control grant is added.

For moving feeds:

1. Register separate external public sources and copy their canonical IDs into the templates.
2. Review rights/access, retention compatibility, attribution, geography and budgets. The `opensky-admission.example.json` and `aisstream-admission.example.json` templates are deliberately **disabled**, with analysis/raw-retention permissions false and replacement instructions for legal basis.
3. Copy only configured locations from `moving-connectors.example.json` to the private `C4ISR_PUBLIC_CONNECTORS_FILE`. An AIS location requires its server key even before a capture starts. Do not put keys in URLs, source metadata, browser configuration or committed files.
4. Provision exact-source service identities using the existing administrator CLI and corresponding principal example. Record a human-approved admission with the matching source, dataset, identity, rights and location. Enable only after the provider requirements are satisfied.
5. Run the separate connector executable with the same database/registry environment as the API:

```sh
go run ./apps/emad-admin -action provision -spec private/opensky-principal.json -out private/opensky.token
go run ./apps/emad-connectors -token-file private/opensky.token -source REGISTERED_SOURCE_ID -once
# Omit -once for bounded captures with persistent pacing, jitter and backoff.
```

Use a separate scoped credential/process for AISStream. The API starts no feed scheduler automatically. Stop with Ctrl+C/SIGTERM. The worker stops when a source reaches its archive budget. `-once` prints status/evidence count without payloads/keys and returns failure for failed/interrupted captures. Inspect source health/runs for partial results and pending publication; `POST /api/v1/sources/{id}/publications/retry` with the exact-source worker credential retries up to 1000 durable projections without another provider request. Refresh expired/revoked worker credentials through the existing provisioning workflow. Never grant these workers exercise control or truth permissions.

### Acceptance evidence

The real PostGIS/S3 imagery workflow now proves M3: recorded scene metadata -> explicitly synthetic retained pixels -> deterministic comparison -> independent human publication -> bounded preview -> approval -> exact bundle download/offline integrity -> the existing S0 headless loader/run. It verifies that a newer draft is not exported, public inputs are not changed, and invalid intervals/AOIs, unreviewed revisions, changed hashes, expired previews, changed rights, unauthorized sources and reference-only offline claims fail.

Two native moving integration cases verify duplicate/late/jump/gap behavior, a full 200-record page without a permanent publication deficit, immutable associations, no assets/commands/classifications, generic-endpoint source isolation, vessel normalization, partial/disconnected capture, outage recovery, empty capture, freshness, disabled source and the atomic 10,000-record stop limit. A local compressed WebSocket peer verifies the actual AIS transport/subscription/binary decode path, budgets, confirmation/error handling and credential-echo rejection. Neither synthetic fixture grants real provider access.

Six real browser workflows pass against the production UI, assembled Go API, disposable PostGIS and private S3-compatible storage. These include comparison/review/export/download, moving-track claims/history/originals/URL/map selection, desktop/mobile access, public reference evidence and the original scenario/audit/truth-revocation paths. The full PostGIS/S3 suite passed **48 top-level cases plus 24 subcases (72 total)**; four opt-in live/browser cases skipped in that run. The separate real-browser gate passed all six workflows, and the mocked browser suite passed 74 checks with 50 intentional project skips. Go tests, vet, Linux race checks, the UI build, OpenAPI lint and generated-code checks passed. Final focused reruns cover the complete imagery export and the full 200-record moving page after the last boundary/projection changes.

```sh
go test ./...
go vet ./...
C4ISR_TEST_INTEGRATION=1 go test -count=1 ./internal/integration/...
pnpm --dir apps/operator-ui build
pnpm --dir apps/operator-ui contracts:generate
pnpm --package=@redocly/cli dlx redocly lint api/openapi/openapi.yaml
task test:browser:integration
```

P3.3 (weather and selected reports) is the next task. The P3 workstream/live-provider exit gate is not declared complete by local protocol acceptance; live activation still depends on the provider agreement/key and approved admission.

### Primary references checked on 2026-10-21

- [OpenSky native REST/state-vector contract and quotas](https://openskynetwork.github.io/opensky-api/rest.html).
- [OpenSky use terms and operational agreement requirement](https://opensky-network.org/about/terms-of-use).
- [ADS-B.lol public API deprecation](https://www.adsb.lol/docs/open-data/api/) and [feeder-only replacement access](https://www.adsb.lol/docs/feeders-only/re-api/).
- [AISStream subscription, compression, position schemas and limits](https://aisstream.io/documentation).

<a id="p3-3"></a>

## P3.3 — Station weather and selected official reports

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

Delivery date: 2026-10-21. Implementation and acceptance evidence for the third P3 task. P3.1-P3.2 and migrations 31-32 are preserved. The next task is **S1.1 - Model and environment asset registry**. The broader P3 live-provider exit gate remains open for the previously documented OpenSky/AISStream activation prerequisites.

### Pilot question and product scope

The bounded pilot asks: **Which official earthquake bulletins and station weather contextualize existing USGS/FIRMS evidence in the selected region?** Each enabled source admission must record its own explicit analyst question, selected AOI, provider/product identity, reviewed rights, freshness policy and service identity.

The selected providers are NOAA Aviation Weather Center METAR station reports and GDACS structured earthquake bulletins. The official-report option in P3.3 is implemented; there is no generic news collector or GDELT/UCDP deployment. A future humanitarian/media question needs its own bounded admission and adapter evaluation.

Weather samples and report claims remain useful with simulation disabled. Neither becomes a controllable asset, moving track, classification, impact assertion, synthetic observation or simulator truth. Context export is unchanged: these report/weather records do not enter the imagery-only approved export path automatically.

### Implemented behavior

- Native fixed-endpoint adapters reuse human-reviewed admissions, source-bound service identities, safe HTTPS transport, request/byte quotas, timeouts, durable leases/backoff, exact original bytes, content revisions, source provenance and projection retries. No scheduler starts in the API.
- Every retrieval restarts a bounded current query window. Native snapshots are discontinuous; a successful selected query does not imply complete reception or absence of hazards. Empty METAR HTTP 204 is valid. Malformed/null JSON and over-budget results fail explicitly.
- METAR JSON uses the provider's latitude/longitude bbox order, an at-most-six-hour lookback and at most 400 returned records. Only METAR/SPECI samples are selected. The raw METAR and station point survive normalization. Celsius, hPa, statute miles and explicitly converted knots-to-m/s retain their meanings. Zero remains a measurement; missing values remain null; variable wind has no fixed bearing; visibility with `+` is a lower bound. No route/area interpolation or forecast is inferred.
- The GDACS pilot requests EQ bulletins only, at most seven days and one page of at most 100 records. The returned cited event centroid/interval selects the approved AOI/time window locally. This does not claim coverage of a disaster's footprint. An exactly full page is **PARTIAL / provider_result_limit**, with no claim of catalogue completeness. The next attempt queries a fresh bounded window rather than resuming unstable page ordering.
- GDACS structured bulletin features are the retained source documents. Their original feature bytes include the provider's fields and links. The linked HTML bulletin/attachments are not fetched or archived; the external link can change. Source claims, provider alert level, cited producer/identifier, original time strings, location basis, extraction version and uncertainty are exposed separately. Timezone-free timestamps are interpreted as UTC for selection, with that assumption explicitly shown.
- Immutable context projections use same-source/resource foreign keys and forced scope/source RLS. PostGIS owns read-time spatial selection. Current station samples are chosen by observation time; current report revisions by provider modification time, then receipt time and deterministic evidence ID. Newest revision selection precedes AOI filtering, so an older location cannot reappear after a correction moves outside the view. Historical evidence remains addressable by exact ID.
- GDACS event IDs identify copies/revisions of the same bulletin lineage. Known NEIC/USGS citations share an upstream-source key. Other cited source names are retained; absent event identifiers leave event-level independence unresolved. These are lineage indicators, not independent-source counts or inferred cross-provider event matches. Repeated copies never increase an automatic confidence/corroboration score.
- Report interpretations reuse the existing assessment revision, audit, optimistic-concurrency and independent-human review workflow. Each proposal freezes an exact retained evidence revision, its raw/content hashes, extracted claims and lineage. OPERATOR/ALGORITHM/AI methods and optional AI scores are explicit. Publication requires a different human, a separate analyst confidence/rationale and valid original/current retained-analysis rights. Corrections and later drafts preserve the prior published revision. Weather observations cannot be proposed as source reports.

### Operator view and contracts

Open **Public Context -> Weather and source reports** from the public map. `/weather-reports` provides URL-backed AOI/kind filters, exact evidence selection, retained sample/revision history, original downloads, claim/lineage inspection, proposal creation, draft revision and independent review. Age of the cited observation/event and source-capture health are shown separately. Evidence selection remains pinned when a provider revision changes.

React owns one bounded DOM list (100 current records), one inspector and review controls; no new chart/map renderer or heavy visualization dependency is introduced. Mobile uses the same reading order with wrapping cards and ordinary scrolling. Values and caveats are visible without hover; controls are labeled and keyboard accessible. Loading, empty, partial/failed capture, stale sample, missing permission and failed mutation states remain explicit. Raw provenance/review documents are collapsed on demand. Workspace-scoped queries and canceled callbacks preserve the existing isolation behavior.

New endpoints:

- `GET /api/v1/public-context/records?kind=WEATHER|REPORT&bbox=west,south,east,north`
- `GET /api/v1/public-context/records/{evidenceId}`
- `GET /api/v1/public-context/records/{evidenceId}/history`
- `GET /api/v1/assessments/reports`
- `POST /api/v1/assessments/report-proposals`

Existing assessment revision/review/submit endpoints accept report proposals through the same workflow. Imagery proposal/list behavior remains compatible and reports are excluded from the imagery list. OpenAPI and generated TypeScript include typed station/report records, admission questions and report evidence snapshots.

Reads require `sources.read`; assessment reads/authorship/review use `assessments.read`, `assessments.create`, `assessments.review`. Execution remains `connectors.execute` for a source-bound service principal. Human source admission remains `sources.manage`. All new operations require PUBLIC_EVIDENCE and enforce resource/source visibility.

### Rollout

1. Apply **migration 33** after 32: `task db:migrate`. Startup explicitly checks the new projection table. Migrations 1-32 are unchanged; no developer database migration was run by this task. Recovery is forward-only because evidence/review history is immutable.
2. Register a separate external source for METAR and GDACS in the intended Public Context resource. Replace the source IDs in `scripts/data/metar-principal.example.json`, `gdacs-principal.example.json` and `weather-report-connectors.example.json` with the actual registered IDs. Provision the service principals using the existing `apps/emad-admin` workflow.
3. Merge the locations into the server-owned `C4ISR_PUBLIC_CONNECTORS_FILE` manifest. Both endpoints are fixed HTTPS services without credentials. The generic HTTP transport sends an EMAD User-Agent, permits no redirects/proxy/private-network targets, bounds response size and sanitizes retrieval failures.
4. Review the disabled `metar-admission.example.json` and `gdacs-admission.example.json` templates. Record the relevant original producers, upstream rights/attribution and retained-analysis permissions. Change placeholder rights IDs/terms and enable only after admission review. NWS public-domain content does not automatically license every third-party station contribution. GDACS automatic alerts require their source/terms review and are not expert-validated facts.
5. Both pilot admissions require retained originals and analysis rights, one page, at most 1 MiB/response, at most 96 requests/day and a meaningful analyst question. Minimum pacing is 300 seconds for METAR and 900 seconds for GDACS; examples use 900 seconds for both. General bounded AOI/timeout constraints still apply. Quotas are source/location-bound. Admission changes interrupt active leases as before.
6. Run the existing `apps/emad-connectors` executable with its source, scope, token-file and optional `-once` flags. Disabled admissions perform no collection. There is no automatic live activation or permanent credential written by this delivery.
7. Each native source stops at the existing archive budget of 10,000 evidence records or 64 MiB. Publication replay handles a full native batch independently of retrieval. Retention is append-only; admit only archive-compatible rights. A purge/retention migration or changed terms requires a separately documented design. These limits intentionally support bounded pilots, not continuous global archives.

### Validation

Acceptance uses explicitly synthetic native-format station and bulletin records, disposable PostGIS, the assembled Go API and the production UI. No permanent live feed is enabled and no live weather/hazard condition is asserted by fixture results.

- Native adapter tests: station units, nulls versus zero, variable wind, censored visibility, timestamp/position selection, malformed/unsupported samples, report cited times and shared lineage, full-page partial results, exact fixed registry endpoints, provider-specific request parameters and METAR 204.
- Database/API acceptance: deduplication, corrected samples, immutable histories, report evidence pinning, independent review, concurrency conflicts, missing publication confidence, revoked rights, published pointers surviving revisions, source/workspace/partition isolation and zero generated tracks/assets/observations/classifications.
- Real production-UI browser gate: **7 workflows passed**, including weather/report downloads, pinned URL/reload, explicit uncertainty, a second human publishing a report assessment, desktop/mobile captures and all existing imagery/moving/workspace workflows. Screenshots: `tmp/p3-3/weather-desktop.png`, `report-review-desktop.png`, `report-mobile.png`.
- Full PostGIS regression: **74 cases passed** (50 top-level tests and 24 subcases); 4 opt-in skips: the 3 live provider smokes and browser gate, which was run separately. Log: `tmp/p3-3/integration-final.jsonl`.
- Existing mocked browser suite: **74 passed / 50 intentional project skips**. Log: `tmp/p3-3/browser-mocked.log`.
- `go test ./...`, `go vet ./...`, Linux `go test -race` for affected ingestion/review/access/export packages, UI typecheck/production build, OpenAPI lint, SQLC/TypeScript regeneration stability, UTF-8/JSON/Go formatting and `git diff --check` passed. Build retains its existing large-bundle advisory.
- After final blank-question validation and review-refresh/UTC display adjustments, the two P3.3 API acceptance tests and all 7 real browser workflows were rerun. Log: `tmp/p3-3/final-targeted-browser.log`. A final API-only pass also verifies that report payloads are rejected by the imagery-only proposal route (`tmp/p3-3/final-api.log`).

### Provider references

Provider documentation was checked during implementation; these references describe protocol/terms, not proof of deployment authorization:

- [NOAA AWC Data API](https://aviationweather.gov/data/api/) and [official OpenAPI](https://aviationweather.gov/data/schema/openapi.yaml).
- [NWS use and disclaimer](https://www.weather.gov/disclaimer).
- [GDACS official API schema](https://www.gdacs.org/gdacsapi/swagger/v1/swagger.json) and [terms of use](https://www.gdacs.org/About/termofuse.aspx).

A small read-only GDACS metadata probe confirmed the native FeatureCollection shape, cited NEIC origin, timezone-free timestamps and parameter schema; it was not an admitted collector run. Reproducible tests use synthetic native records rather than live conditions.

<a id="s0-1-3"></a>

## S0.1–S0.3 — Frozen runs, spatial contracts and context packages

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

**Subsequent increment:** [S0.4 and P1.5–P1.6](#s0-4-p1-5-6) completes the adapter/M2 and public-history/M1 gates. Current installations require migrations through **27**, and the next task is **P2.1**. The original acceptance details below remain historical; statements about these tasks being pending are superseded.

Implemented 2026-10-21 against the two v2 proposals. Apply migrations through **25** before starting this server. Verification uses disposable PostGIS databases; the developer database has not been migrated or seeded.

These tasks establish simulation foundations in the existing modular monolith. S0.4 and the M2 adapter/isolation demonstration remain pending. No rendering engine, physics service, GPU service, permanent public-feed polling, or hardware bridge is introduced.

### S0.1: authoring, immutable versions, and runs

YAML files remain the mutable authoring interface. Loading validates known fields, identifiers, timing, optional scene coordinates, and component fidelity. Input is bounded to 1 MiB. Starting a run serializes the validated definition, component templates, retained context bundles, adapter version, and actual executable SHA-256 into an immutable `emad.scenario.v1` version. Its `sv_` ID commits to the exact serialized bytes. PostgreSQL stores those bytes without JSON number reformatting; loading uses `json.Number`, preserving large integers and exponent spellings in authored payloads.

`simulation_versions` belongs to `EXERCISE_INPUT`, scoped to the workspace/environment, with forced RLS and an update/delete rejection trigger. An internal input projection is used only behind authorized scenario operations. It is not an ordinary COP resource. Existing `scenario_runs` rows carry an immutable `emad.run.v1` configuration: version ID, runtime/build, seed, virtual epoch, initial scale, context package IDs, and optional `restartOf` lineage. Database triggers protect the frozen configuration, scenario name, and seed while permitting lifecycle/progress updates.

Existing start/control routes remain compatible. New routes are:

- `GET /api/v1/scenarios/runs/{id}/manifest`: selected-run configuration, requiring `scenarios.read`. Ordinary response disclosure removes the seed.
- `GET /api/v1/scenarios/runs/{id}/version`: full frozen authoring input, requiring `sim.scenarios.author`. This privileged response preserves the exact typed definition and retained package data. Ordinary run lists/status responses do not contain actor templates or hidden instructions.

Lifecycle behavior:

- Start creates a new run and resource namespace. A missing YAML clock receives a wall-time-aligned epoch once, truncated to milliseconds; an explicit clock can define a historical epoch.
- Pause waits for the current application action to settle, freezes virtual elapsed time, and persists the pause cursor. Resume continues from that cursor.
- Speed changes preserve elapsed time and event ordering. Information time stops during application/DB processing and cannot advance beyond the next unexecuted event.
- Stop/failure/completion preserve prior evidence and seal virtual progress. Terminal clocks do not continue aging with wall time.
- Restart loads the stored version rather than the YAML file. It preserves epoch, seed, retained context, and prior evidence, creates a new namespace and `restartOf`, and records the new run's initial scale using the previous run's current speed. Editing or deleting the original YAML cannot change the restart.
- Re-simulation requires the original executable build and supported adapter version. Historical frozen definitions remain inspectable after a build change; the server does not claim new code reproduces old behavior.
- Pre-migration runs have an empty frozen manifest. Their evidence remains readable. Restart returns a conflict explaining that an explicit new start is needed, because reconstructing their original inputs from today's YAML would be misleading. Orphaned newer runs can be closed and restarted from their persisted version.

#### Time and reproducibility

Elapsed time is integer milliseconds, event identity is `(run_id, sequence)`, and order is `(virtual_time_ms, sequence)`. The initial runner has a 31-day virtual horizon and the existing 20,000-action bound. Scheduled observation and telemetry times come from the frozen epoch and event time. Delayed observations preserve their source generation time; delivery delay and stale offsets are distinct. Receipt, processing, audit, lifecycle timestamps, and WebSocket delivery continue using wall time.

The request/background context carries an application-owned information-clock resolver. Asset/track reads, telemetry response freshness, COP reads, ingestion future-time checks, and the freshness monitor use the selected exercise clock. Live COP time is not clamped to the last recording sample: silent contacts age while the exercise runs, and remain frozen while paused. Historical scrubber requests retain their bounded recording window. Public data and legacy non-versioned fixtures retain wall/public time semantics. No process-global clock or mutable cross-workspace clock is used.

`simulation.TimeMap` defines explicit identity/anchored-offset placement for historical source time, rejecting missing anchors and duration overflow. The initial runner declares `preserve_source_time`: importing context does not silently rewrite public observation timestamps. The mapping library does not implement historical public-feed playback.

**Declared guarantee:** the same frozen version, seed, epoch, runtime build, and scripted inputs produce the same scheduled event ordering, observation source instants, seeded positions/footprints, and modeled telemetry values despite pause/speed changes. Tests compare persisted observations between executions. Database-generated IDs, run-qualified IDs, receipt/processing/audit/lifecycle times, transport timing/IDs, and uncontrolled external commands are excluded. Dynamic operator actions require the same ordered virtual-time input to claim equivalent results. GPU/physics/cross-platform bit identity is not claimed.

**Recorded replay** reads stored evidence through the existing `/picture` endpoints without executing the engine. **Re-simulation** creates a new execution from frozen inputs. The event inspection endpoint is an execution ledger, not a complete recording of every sensor payload. Full recordings, historical control-state reconstruction, checkpoints, and after-action evaluation remain S3 work; the existing scrubber is not promoted to that guarantee.

### S0.2: renderer-independent coordinates and fidelity

`internal/simulation` specifies horizontal WGS84 (`EPSG:4326`) with explicit longitude then latitude, separate height/datum, and WGS84 ECEF to a right-handed local ENU frame: X east, Y north, Z up, metres. The origin, `wgs84-ecef-enu-v1` transform, axes, order, units, and orientation convention are recorded in `SimulationFrame`.

The orientation convention is heading clockwise from north, pitch nose up, and roll right, all in degrees. It is a contract for future adapters, not a physical attitude dynamics model. Local coordinates are bounded to a 500 km radius. A different engine axis system must use an explicit adapter transform.

Coordinate conversion requires a finite height with `WGS84_ELLIPSOID`. Missing height remains null. MSL, EGM2008 orthometric elevation, AGL, unknown datums, feet, unsupported axes/order, and unsupported transform versions are rejected rather than relabeled. In particular, the NOAA sample retains its EGM2008 metadata and cannot silently become an ellipsoidal scene surface. Geoid/terrain-assisted vertical conversion remains a separately versioned future transform. Existing two-dimensional YAML fixtures need no invented altitude or scene origin.

Fixtures verify known equatorial/polar ECEF axes to 1 micrometre, positive east/north/up directions, longitude/latitude round trips within `1e-9` degrees, and height/local round trips within `1e-5` metres, including poles and antimeridian cases. These numerical tolerances describe this transform implementation, not source-data accuracy. Equations follow [ESA Navipedia's ECEF/ENU transformation](https://gssc.esa.int/navipedia/index.php/Transformations_between_ECEF_and_ENU_coordinates).

Every source/sensor, controlled asset, and observed actor template carries a model version, profile, and stated simplifications. Current profiles are `visual_only`, `abstract_behavior`, and `kinematic`. Legacy YAML receives explicit adapter defaults documenting scripted 2D paths and absent physical dynamics/validated sensors. Specialized validated-model claims are rejected in this increment. PostGIS continues owning authoritative spatial database predicates.

### S0.3: one package format and an offline loader

`emad.context.v1` is the shared manifest/bundle format. It contains selected AOI/time window; original producer/source and dataset/version references; dataset coverage and parent artifact hash when supplied; retrieval dates; exact asset hashes/lengths; CRS/datum/resolution; processing/transform versions; quality/validity; immutable rights and attribution; server-established creator/approval; and reproduction status. Package IDs commit to the typed manifest; every retained asset is keyed and checked by SHA-256. Asset keys are content identities, never file paths.

Public package metadata contains geographic inputs only. Actor templates and hidden instructions remain in protected scenario versions. The builder does not generate scene meshes or synthetic geography; `syntheticDerived=false` records that limitation. A future derivation must introduce its actual transform/provenance and derived marker, not reuse this claim.

Routes, all in `PUBLIC_EVIDENCE`:

- `POST /api/v1/context-packages`: explicit human approval with `context.export` and `sources.read`; accepts dataset IDs, AOI, selected time window, quality acceptance, approval statement, and optional `referenceOnly`.
- `GET /api/v1/context-packages`: approved manifests only, requiring `context.export`.
- `GET /api/v1/context-packages/{id}`: one approved manifest.
- `GET /api/v1/context-packages/{id}/bundle`: JSON download containing the same manifest and retained original bytes as base64.

The server obtains metadata/rights from reviewed reference versions. Source grants must permit every dependency. Dataset updates create new versions and cannot mutate old package bytes. The selected window records the scenario's context selection; it does not assert historical temporal coverage or improve the source's stated validity/quality. AOI must lie within each selected dataset; artifacts remain the original bounded snapshots, without implicit clipping or reprojection.

The builder accepts up to 16 reference artifacts of 512 KiB each, at most 8 MiB original data, a 12 MiB bundle, and a visible catalogue budget of 64 packages/64 MiB. Frozen scenario versions have a stricter total 4 MiB limit including embedded bundles. All storage stays in PostgreSQL for this bounded increment. Nothing is fetched from artifact URLs.

`retained_offline` requires all authorized original bytes. `external_dependencies` explicitly weakens reproduction when reference-only retention is selected; the offline loader rejects such a package. Missing/corrupt/unlisted assets, changed manifests, missing approvals, incompatible transforms, expired or insufficient rights, and incomplete metadata fail validation. Checksums establish integrity; approval metadata in an arbitrary downloaded file is not a cryptographic signature or a new authority grant.

Validate the checked-in hand-authored fixture without a database or feed:

```powershell
go run ./apps/emad-context testdata/context-packages/hand-authored.json
```

The same command accepts an authorized downloaded bundle. Only load bundles from a trusted export. An author can put approved IDs in YAML `context_packages`; start requires an explicit public-resource grant and `context.export` to copy those packages into the immutable input version. The copy must pass offline validation. Restart uses the retained copy, not the public catalogue or current feeds. Consuming those inputs to drive the dedicated scoped headless adapter is S0.4.

The real PostGIS integration fixture imports both P1.3 Jeddah–Makkah artifacts, builds/approves/downloads a retained package, validates it locally with connector calls remaining zero, freezes it into a separate exercise input version, restarts after the authoring file is deleted, publishes another source version, and confirms the existing package is unchanged. It also checks unauthorized/source/workspace reads, immutable database rows, reference-only status, and corrupt bytes.

### Verification

The gate covers Go tests/vet; spatial/time/package corruption fixtures; restart after author edits/deletion; speed/pause/seed equivalence; exact frozen numeric precision; real PostGIS scope/immutability and reference-package checks; the full existing integration regression suite; Linux race checks for affected services; OpenAPI lint/type generation; production UI build; mocked browser regressions; and the existing three real-backend browser workflows.

The next task is **S0.4**, followed by the M2 gate and return to P1.5–P1.6. Engine selection remains deferred to S1 evaluation.

<a id="s0-4-p1-5-6"></a>

## S0.4, P1.5 and P1.6 — Headless adapter and public evidence history

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

Implemented 2026-10-21 on the accepted S0.1–S0.3/P1.1–P1.4 foundation. This increment closes S0/M2 and P1/M1. The next task is **P2.1**. No 3D engine is selected; native rendering, physics, recording/playback and richer exercise control remain in their planned workstreams.

### S0.4: Scoped headless synthetic ingress

The existing YAML runner now emits through `internal/exerciseingest`, a typed application port wired by the assembled Go app. Its service identity must be exactly the scoped internal scenario runner with `exercise.ingest`. Human credentials and public connector workers cannot submit through this port. There is no new network ingestion endpoint or general truth-capable background identity. `exercise.ingest` and `sim.truth.record` are nondelegable provisioning permissions.

Each message carries `emad.synthetic.v1`, run/version/runtime/build identity, adapter ID, virtual action elapsed milliseconds, stable action sequence, observed/source time and an opaque message ID. The selected frozen run must match. An ID retry must have identical sanitized input; changing it conflicts before domain ingestion. Deliberately stale source times remain separate from virtual action elapsed time. Receipt/processing timestamps remain wall time. Observations and telemetry use the existing domain services, including validation, deduplication, correlation, spatial rules and operational compatibility behavior.

Messages retain bounded sanitized inputs and PENDING/DELIVERED/REJECTED processing receipts in EXERCISE_OBSERVATION. Arbitrary author payload strings/instructions are removed; permitted numeric measurement quality survives. The safe `GET /scenarios/runs/{id}/synthetic-messages` receipt view requires `observations.read` and the exact selected run. It is capped at 1,000 with a conservative truncation indicator. Receipts contain no actor mappings or raw input. This is a small adapter receipt boundary, not S3's complete event recording/replay subsystem or an exactly-once distributed transaction. A process failure can leave a pending receipt; a same-input retry uses the domain's idempotence. Status describes the most recent completed delivery attempt.

Hidden actor names no longer supply contact or message identities for new runs. Random opaque contact IDs do not consume the seeded motion/failure RNG. Observation presentation continues to use its independent public aliases. A narrow writer records immutable message-to-actor/contact mappings in EXERCISE_TRUTH only after checking the frozen run; a conflicting mapping fails. The evaluator application port requires explicit `sim.truth.read` in that run's truth resource. It is not mounted on ordinary HTTP or WebSocket transport. Ordinary projections, public ingestion and operators cannot read truth through it.

The adapter runtime is `emad-yaml-runner-v3`. Existing persisted evidence is preserved. New assessment subjects/evidence reference canonical generated track IDs rather than private external actor hints. Deterministic re-simulation means equivalent ordered geometry, measurements, failure actions and operational outcomes from the same frozen input/seed/epoch/runtime build, excluding opaque run/entity/contact/message IDs and wall timestamps. It does not promise byte-identical random identities or re-simulation across builds. Frozen definitions and retained context remain readable after a build changes; re-simulation with another build is rejected explicitly. The seeded failure matrix, mission/command workflow and frozen restart remain compatibility gates.

A scenario may declare `context_anchors` (`track`, `packageId`, `datasetId`, `featureId`). The initial adapter resolves a retained, hash-validated EPSG:4326 vector Point from the run's frozen context package, checks its AOI and uses that horizontal location to initialize private motion. It does not contact a live provider or mutate the frozen definition. An anchor cannot conflict with an authored start. This first headless consumer does not infer terrain height, convert EGM2008 to ellipsoid height, create a renderer, or treat public observations as truth. Missing heights remain missing.

The M2 fixture exports an approved real Natural Earth subset, freezes a scenario and context, runs headlessly with provider retrieval disabled, checks synthetic observation/telemetry receipts and duplicate/rejected inputs, verifies the anchored coordinate, separates virtual/source and wall times, and checks protected mappings, ordinary HTTP/RLS truth denial and unchanged public evidence. Existing frozen-restart/seeded-failure tests cover the declared re-simulation guarantee.

### P1.5: NOAA-20 VIIRS NRT

The selected dataset is **VIIRS_NOAA20_NRT**, product version **2.0NRT**, adapter `firms-viirs-noaa20-v1`. NOAA-20 is chosen for this initial global NRT stream after reviewing the NASA area API, product semantics, access and reuse terms. NASA lists Suomi NPP product delivery ending on **2026-11-01**; the new connector therefore does not depend on SNPP. Coverage is satellite/pass/cloud/processing dependent, with reported thermal anomalies and known omissions; NRT is not a guaranteed complete or instantaneous fire catalogue.

Primary provider references reviewed:

- [NASA FIRMS area API](https://firms.modaps.eosdis.nasa.gov/api/area/) and [MAP_KEY registration/quota](https://firms.modaps.eosdis.nasa.gov/api/map_key/): bounded date/AOI requests, a free registered map key, and the published transaction budget. The server applies a smaller reviewed daily budget in addition to its persistent rate limit/backoff.
- [NASA API tutorial](https://firms.modaps.eosdis.nasa.gov/content/academy/data_api/firms_api_use.html): NOAA-20 CSV columns and published sample rows.
- [VIIRS 375 m active-fire guide](https://www.earthdata.nasa.gov/s3fs-public/2024-07/VIIRS_C2_AF-375m_User_Guide_1.0.pdf): product, confidence, pixel dimensions and interpretation.
- [NASA data-use policy](https://www.earthdata.nasa.gov/engage/open-data-services-software/data-use-policy), [FIRMS reuse guidance](https://forum.earthdata.nasa.gov/viewtopic.php?t=5184), and [NASA SNPP delivery notice](https://www.earthdata.nasa.gov/data/alerts-outages/suomi-npp-data-product-delivery-cease-november-1-2026). Each actual source admission still records reviewed rights and attribution; no deployment-wide rights grant is inferred.

The trusted URL is the fixed credential-free `https://firms.modaps.eosdis.nasa.gov/api/area/csv/`. The secret map key comes from a server environment variable and is added only to the outbound path, never public configuration, evidence references, retrieval metadata or error output. No Bearer header is sent for this API. Existing restricted HTTPS/DNS, redirect, timeout, byte, row, quota and revocation controls remain in force. Do not enable HTTP request URL logging that would capture the provider's credential-bearing path.

A frozen rolling interval is queried one UTC calendar day at a time, with an explicit forward date and day range 1. A persistent cursor resumes the same bounds; completed cursors permit a new rolling query. Actual acquisition timestamps are filtered to the exact interval, and excluded rows are counted separately. Header-only CSV is a successful empty day; malformed/unknown responses are failures. Admissions permit at most ten rolling days, a minimum 60-second interval and at most 1,000 daily requests, further limited by the server registry. An interval can span one more calendar date than its duration in days. The 30-second worker budget therefore normally handles one day per invocation and resumes subsequent days on later invocations; partial windows remain visible.

The decoder preserves UTC acquisition time (including zero-padded HHmm), satellite/instrument, product/version, categorical provider confidence and original code, NRT processing, brightness temperatures, fire radiative power, scan/track kilometre dimensions, nominal 375 m resolution, day/night and source provenance. It derives a documented logical acquisition/center key because this stream supplies no stable pixel UUID. This key is not a fire identity; nearby detections remain separate, and overlap/retries deduplicate by logical key plus content revision. For an unchanged acquisition/center key, provider confidence or measurement corrections create retained content revisions. A changed acquisition time or center produces a new derived key; the feed supplies no reliable identity with which to infer that it corrects another pixel. Without a provider correction timestamp their revision ordering uses receipt time, explicitly distinct from acquisition/effective time.

The report is a `point_event` / `thermal.detection` with `reported_location`. Scan/track dimensions do not define a correctly oriented polygon. The UI therefore shows a fixed screen-space location marker, dimensions and an explicit missing-orientation statement, without inventing a footprint, buffer or fire perimeter. Confidence classes are not probability percentages or EMAD assessments. No confirmed fire, identity, intent or independent corroboration is asserted.

Each exact CSV header plus selected original row is retained as bytea when rights permit and downloaded as `text/csv` with a `.csv` filename. `rawHash` verifies those bytes; `contentHash` identifies canonical parsed CSV column values (`hashVersion: emad-json-v1` describes that canonical column JSON, not original JSON bytes); `retrieval.responseHash` identifies the full bounded HTTP response. The column JSON used for parsing is not presented as an original producer JSON record. Older JSON originals retain their exact download behavior. Reference-only rights do not retain CSV bytes.

The checked-in five-row CSV is transcribed from NASA's published NOAA-20 example; its serialization is reconstructed, not represented as a live captured response. Tests clearly distinguish this source fixture from deliberate clock, AOI, confidence and malformed mutations. The M1 test uses those synthetic mutations and a native USGS-protocol fixture in the same Jeddah/Makkah AOI as the real reference snapshots. It does not claim observed Saudi fires or earthquakes.

### P1.6: Bounded evidence queries and historical meaning

`GET /public-events` adds source, phenomenon, provider quality, WGS84 AOI, observed-time interval and revision selection. Limits are 100 by default, at most 200 per page, with accurate total, offset at most 10,000 and explicit truncation. The map displays the current page only. Source visibility and selected resource remain enforced by RLS. AOI predicates use PostGIS `ST_Intersects` on reported point/area geometry. Bounds are limited to 400 square degrees and cannot cross the dateline. An observation interval with both ends is limited to 366 days. Geometry-less withdrawals are excluded by an AOI filter; this is stated in the UI.

The query selects a provider record's eligible revision **before** spatial, phenomenon, quality and observation-time filters. A corrected location or confidence must not resurrect an old matching revision. `revisions=all` deliberately exposes retained versions with a warning that these are not independent reports. Pending/unpublished evidence never appears as a published event.

Three labeled views are available:

- `latest`: the latest retained published provider revision per record, including explicit withdrawals.
- `known_at` plus `as_of`: select only evidence published by EMAD by the cutoff, then choose the newest eligible provider revision. `knownAt` is catalogue publication time recorded in its transaction, **not** a physical database commit timestamp. Later arrivals/publications are excluded even if their origin time is earlier.
- `effective_at` plus `as_of`: a retrospective view of evidence retained now, using provider revision time where supplied, otherwise observation/acquisition time, then receipt fallback. Each row names its `effectiveBasis`. It may include late arrivals or later corrections learned after the selected date and is explicitly not a reconstruction of what EMAD knew then.

Detail and revision-history requests respect the same time cutoff, including invisible anchors. The UI's source/quality/AOI filters constrain the report list; linked revisions use its historical interpretation/cutoff and preserve correction lineage. Current source health is labeled current. Selected immutable reference versions keep their own validity. This is a report-evidence history view, not a historical reconstruction of all tracks, geofences, alerts or operational workflows. Current tracks/control zones are hidden on the historical report map, and the existing track replay clock is identified separately. A persistent history caption remains visible beside the map; the current entity board, track replay controls, telemetry and operational legends are hidden until returning to latest reports.

The panel exposes original producer/provider, acquisition/origin, provider revision, receipt, publication and effective times, raw and parsed hashes, attribution, rights, normalized fields/quality and bounded revision lineage. Exact originals load/download over authenticated APIs, with cancellation and selection guards preventing a late original response from replacing a newly selected revision. Retrieval failures remain distinguishable from successful filtered-empty lists and from source outages. Dates are entered in local time, queried in UTC and displayed with explicit full UTC evidence timestamps.

Filter state is a bounded, validated map URL bookmark. It carries no credential or authorization. Invalid bookmarks show a visible fallback warning. Selecting another map entity preserves public filters; switching workspace clears selection, URL filters, pending requests and scoped caches through the existing provider. Known-at views do not poll; latest and retrospective effective views can refresh as new evidence arrives. Existing MapLibre ownership, keyboard/text alternatives and narrow-screen layout are preserved.

### Rollout and activation

Apply **00026_synthetic_ingress.sql** and **00027_public_history_indexes.sql** after the already-applied version 25. Version 26 adds FORCE-RLS synthetic receipts and immutable protected mappings; version 27 adds history/publication/spatial indexes. Existing evidence and scenario rows are not rewritten. Down migration refuses destructive receipt loss; recovery is forward. Implementation tests apply these only to disposable PostGIS databases; the developer database is unchanged.

Use the established `task db:migrate` procedure before starting the updated server. Startup verifies the new adapter tables. Existing authoring, controlled assets, missions/commands, observations and public source workflows remain supported.

For optional FIRMS activation:

1. Register an external source in the intended public workspace; retain its canonical source ID. Provision a distinct exact-source service principal with `operational.read`, `sources.read`, and `connectors.execute`.
2. Obtain a [NASA MAP_KEY](https://firms.modaps.eosdis.nasa.gov/api/map_key/) and set `FIRMS_MAP_KEY` in the server environment. Keep it out of frontend configuration and repository files. No key was available in this implementation environment, so no FIRMS live query was executed.
3. Copy `scripts/data/firms-connectors.example.json` to private server configuration, replace its source/resource binding, merge other existing entries and set `C4ISR_PUBLIC_CONNECTORS_FILE`. The example's key is an environment-variable name, not a credential.
4. A human reviewer approves a source admission with the exact dataset/version, `revisionPolicy: content_hash`, producer/provider, reviewed rights/attribution, selected AOI, lookback and bounded retrieval/freshness policy. Example bounds `[39,21,40,22]`, one-day lookback, minimum 60-second spacing, max 1,000 rows/page, 1 MiB response, max one page/invocation and 100 requests/day fit the adapter limits. Bind the exact connector and service IDs.
5. Invoke the existing source `connector-runs` with that service credential. Partial day windows resume through the persistent checkpoint; retry publication independently if needed. No background provider schedule is added or enabled automatically.

An optional disposable live smoke is available after supplying the map key: set `C4ISR_TEST_INTEGRATION=1` and `C4ISR_TEST_FIRMS_LIVE=1`, then run `go test -count=1 -run TestFIRMSAuthorizedLiveSmoke -v ./internal/integration`. A successful empty query is valid bounded coverage, not proof of absence. Without opt-in, the network smoke is intentionally skipped. Fixture acceptance does not depend on an external key/provider outage.

### Verification

Executed with Go 1.25, Node 24/pnpm 10, Chromium, disposable PostgreSQL 17/PostGIS 3.5 and Linux Go 1.25 for race detection. The developer database was not changed.

- `go test ./...` and `go vet ./...` pass; opt-in integration/live/browser cases run separately.
- M2 frozen-context/ingress/truth isolation and M1 native FIRMS/USGS/reference/history gates pass. The final M1 case also verifies historical withdrawals and source/quality/AOI filtering without resurrecting an old revision. The existing direct-service/run-isolation fixture passes after excluding the two newly nondelegable internal capabilities from its test credential.
- The complete mocked browser suite passes **74 tests**, with **50 intentional project skips**. After the final history layout change, the focused public-browser suite passes **6/6**, including invalid bookmarks, validation, historical meaning, exact CSV inspection/download, reload and narrow-screen controls. A resource-pressure `ERR_NO_BUFFER_SPACE` failure while loading the Vite entry module disappeared in the isolated final run.
- The production UI with real Go/PostGIS passes **3/3 browser flows**: public reference/original/history filters/bookmarks; scenario-to-command/audit across workspace switches; and truth/revocation isolation. The automatic response check confirms no API 5xx reaches the browser. Backend cancellation diagnostics during workspace navigation and mocked socket shutdown messages remain non-failing. Final desktop/mobile images were inspected; report history hides current track chrome and retains a persistent interpretation caption.
- Linux race checks pass for scenarios, public ingestion/history, observations, telemetry, tracks, access, presentation, realtime, simulation and context packages. Exercise ingress, reference and event-rule packages compile under the detector; their database behavior is covered by PostGIS integration tests.
- The production TypeScript/Vite build, Redocly OpenAPI validation, stable generated TypeScript, stable sqlc generation, canonical Go formatting and tracked whitespace checks pass. The existing Vite chunk-size advisory remains informational.
- The opt-in FIRMS live smoke is intentionally **not executed** because no map key was configured. Earlier authorized USGS live acceptance remains recorded in the prior report; this increment does not repeat external retrieval or enable any provider schedule.

The final complete PostGIS rerun passes **45 top-level tests and 24 subtests**, with zero failures, in **250.083 seconds**. Its three intentional opt-in skips are FIRMS live, USGS live, and the separately executed browser gate. This includes all prior workflow, spatial, authorization, pool-reuse, clock/frozen-restart and seeded-failure regressions. M0 remains accepted; M1 and M2 now pass. Local raw logs and screenshots are ignored under `tmp/s04-*`, `tmp/p15-*` and `tmp/p16-*`.

<a id="s1-1"></a>

## S1.1 — Model and environment asset registry

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

Delivery date: 2026-10-22. This increment implements the first S1 task from the active v2 plan. It preserves P3.3 and migrations 1-33. The next task is **S1.2 - Engine evaluation**; this delivery selects no engine and does not claim the S1/M4 scene proof has passed.

### Delivered domain boundaries

- `ModelAsset` is an immutable validated visual version with the original source artifact, checksum, source/import revision, license/admission author, importer version, validator/dependency fingerprint, worker build, material/texture summary, scene bounds, units/axes and fidelity.
- `ActorTemplate` is a separately versioned authoring definition referencing one validated model. Static and abstract waypoint-kinematic behavior declarations are supported. Speed bounds and limitations are recorded; templates do not instantiate an actor or execute behavior.
- `EnvironmentAsset` pins a validated model to the existing S0 frame contract: explicit WGS84 longitude/latitude, resolved ellipsoidal height, versioned ECEF/ENU transform, metre units and orientation. The declared model-axis mapping is glTF X east, Y up, Z south. No terrain is silently reprojected, loaded or certified by registry admission.
- `SimulationActor` is a separate runtime instance reference in the simulation contracts. Runtime instantiation, private world state and actor lifecycle remain S2 work. No new actor, controlled asset, track, observation, command or truth record is created by these registry operations.

The asset pipeline uses the existing private S3-compatible `artifacts.Store` and content-addressed writes. Public imagery admission/metadata remain unchanged: simulation authoring has separate metadata tables and object paths under `EXERCISE_INPUT`. Original authoring sources and externally produced engine variants have separate artifact records, hashes and purposes. Native variants pin a parent model version, inherit its complete license snapshot and identify their engine/build; they remain opaque, unexecuted stored objects. Storing one does not select an EMAD engine or certify compatibility.

### Import and storage lifecycle

An authorized human admits metadata, reviewed terms and a declared SHA-256/byte count. Reservations are idempotent by request key within the exact input resource; reuse with changed content/rights conflicts. Upload verifies the exact bytes, writes privately and rechecks live author access after external I/O. Models then enter the persistent validation queue. Authoring sources/engine variants are stored separately and never executed.

The standalone `apps/emad-assets` Go worker requires an input-bound service principal with `sim.assets.import`. It claims a bounded lease, runs a pinned Node/Khronos validation subprocess, validates embedded image payloads in the worker, stores the validation manifest separately, and atomically publishes the immutable model version, attempt and audit outcome. Worker execution is outside API startup and synchronous HTTP handlers. Revocation after validation prevents publication. Expired leases record an interrupted attempt before recovery; at most three attempts are permitted. Storage/process failures retry within that budget; invalid inputs are rejected with retained diagnostics. Source and manifest downloads recheck access and verify hashes/byte counts.

A validated model ID includes its exact source admission, source checksum, importer, worker build, validator/dependency fingerprint and validation-manifest hash. The manifest's original byte length and hash are retained, so JSONB normalization cannot change its downloaded bytes. New source/import revisions create new immutable records; no existing version is silently replaced. Rejected/failed originals and attempts remain inspectable.

### Supported portable profile and resource budgets

The current profile is deliberately small and explicit: core static glTF 2.0 and GLB, triangle primitives, a selected scene, valid node transforms, embedded buffers and embedded PNG/JPEG images. All external file/network references are rejected; no asset URL is fetched. glTF source and terms URLs are retained as references only. Archives, sparse accessors, compressed extensions, other extensions, skins, animations, cameras and morph targets are rejected by this profile.

Budgets are enforced before/alongside Khronos structural/data validation:

- At most 16 MiB uploaded file; at most 4 MiB JSON; at most four buffers and 16 MiB decoded buffer bytes.
- At most 128 meshes, 256 primitives, 256 nodes/visited instances, 16 scenes, 1,024 accessors/views, and depth 32.
- At most 250,000 rendered vertices and 250,000 rendered triangles. Repeated mesh instances consume the rendered budget rather than counting the shared mesh once.
- At most 64 materials, 32 textures and 16 images. Each image is at most 4,096 by 4,096 pixels; aggregate image pixels are at most 16 Mi. Compressed image bytes are bounded, dimensions are checked before full decoding, and PNG/JPEG payload/CRC/decode failures reject the model. No archive decompression is performed.
- Node validation has a 15-second subprocess deadline, a 256 MiB V8 heap limit and bounded stdout/stderr. Truncated Khronos issue reports reject the import rather than certifying an incompletely reported result. The whole worker invocation has a 45-second context and a 60-second database lease. Image decode runs only in the worker with byte/pixel budgets and deadline checks. These limits are not an assertion that all engine loads have a 256 MiB total-RSS budget.
- Scene-coordinate bounds are finite and limited to 500,000 metres per axis. Bounds come from validated accessor data and the selected scene hierarchy, not an unverified caller box. Frame placement/scene loading and their resource measurements belong to S1.2-S1.3.
- Each input environment admits at most 1,000 source/variant/model upload records and 512 MiB reserved original bytes; at most 1,000 actor templates and 1,000 environment definitions. Validation manifests and at-most-three attempt records are separately bounded. Lists return at most 100 items with a truncation flag; pinned model versions and authoring definitions remain addressable by ID.

This is a bounded profile validator, not a claim to accept every valid glTF extension or provide specialist dynamics. The retained importer/validator versions and warnings expose that scope. Opaque source/native artifacts are attachments only and are never fed into a renderer, shell or interpreter.

### Licensing and reproducibility

Admission records the license ID/version, legal basis, source reference/version, terms URL, attribution, storage/simulation permission and separate redistribution permission. This first append-only registry requires an explicit perpetual source-archive permission; unsupported retention terms require a future retention design. A portable model linked to a retained authoring source preserves that source's complete reviewed license snapshot. Engine variants preserve the parent model's snapshot. Human metadata admission is an explicit review record, not automatic interpretation of upstream legal terms.

Three tiny fixtures are authored by `scripts/data/prepare_model_fixtures.py`: an abstract wing triangle and marker in GLB, and a synthetic ground tile in embedded glTF. `testdata/models/README.md` records their internal fixture-use terms. They contain no third-party binary model inputs, surveyed terrain or physical/sensor performance claims. Their source generator and exact files are retained in the repository. They are suitable for repeatable pipeline testing; licensed production models still require their own admission review.

### UI and API

In an authorized Exercise COP, open **Scenarios -> Simulation asset registry**. The page requests the same workspace/environment's `EXERCISE_INPUT` resource explicitly and verifies its author permission separately. COP access alone grants no access to hidden authoring assets. Current browser entry uses the existing COP shell; input-only principals can use the dedicated API, while a broader Studio shell remains S1.3 work.

The page supports source/license review, checksummed uploads, import-state/attempt inspection, exact source/manifest downloads, validated geometry/material metadata, actor-template authoring and explicit environment-frame registration. Model/upload selection is URL-backed. Workspace query caches and cancellation remain isolated; a public workspace cannot open the authoring view. The display is a bounded responsive DOM list/inspector with labeled native controls, ordinary scrolling and on-demand provenance. It adds no renderer or GPU dependency.

All routes are under `/api/v1/simulation-assets/` and require an explicit input-resource `sim.scenarios.author` grant:

- `GET/POST uploads`, `GET uploads/{id}`, `PUT/GET uploads/{id}/content`, `GET uploads/{id}/attempts`.
- `GET models`, `GET models/{id}`, `GET models/{id}/manifest`.
- `GET/POST templates`, `GET templates/{id}`.
- `GET/POST environments`, `GET environments/{id}`.

Writes additionally require a human principal. There is no HTTP endpoint that lets a caller submit a successful validation result or execute a worker. `EXERCISE_INPUT` is admitted only on these dedicated authoring routes and the existing auth introspection endpoint; ordinary COP/realtime routes still reject input/truth partitions. Forced RLS and same-resource foreign keys protect the tables independently of handler checks. Source/license snapshots and model versions/definitions/attempts are immutable, with scoped audit records for admission, definitions and import completion.

### Rollout

1. Apply **migration 34** after the already applied migration 33: `task db:migrate`. Startup checks the new registry tables. This delivery did not migrate the user's database; automated migration acceptance uses disposable PostGIS. Existing migrations and the migration shell script are preserved. Recovery is forward-only.
2. Reuse the private object-store configuration documented for P2 imagery. Configure the same `C4ISR_ARTIFACT_S3_*` credentials on the independent asset worker; no public bucket or public signed URL is introduced.
3. Run `pnpm install --frozen-lockfile` to install the exact pinned Khronos validator package. The worker needs Node 20+ and the trusted `apps/emad-asset-validator/validate.mjs` wrapper with its installed dependencies. Its fingerprint includes the wrapper, package metadata and validator JavaScript bundles.
4. Adapt `scripts/data/model-asset-author-grant.example.json` to the intended existing human principal and input environment. Add that explicit grant with the existing control-plane command: `go run ./apps/emad-admin -action grant -spec <author-grant.json>`. It grants no runtime truth. Browser entry also needs the existing authorized Exercise COP session in that workspace/environment.
5. Adapt `scripts/data/model-asset-worker.example.json` to the same input environment and provision a separate service credential: `go run ./apps/emad-admin -action provision -spec <worker.json> -out <private-token-file>`. Store/rotate that private credential using the established administration workflow.
6. Start validation explicitly: `go run ./apps/emad-assets -token-file <private-token-file> -validator-script apps/emad-asset-validator/validate.mjs`. Add `-once` for at most one claimed job. API startup never launches this process. Missing dependencies, storage failure and process failure remain observable in bounded attempt history; repair the worker configuration before admitting a new import revision after a terminal attempt budget.
7. The accepted scene/context, engine comparison and first renderer remain S1.2-S1.3. Asset admission does not modify frozen S0 scenarios or add an unreviewed public-to-simulation import path.

### Validation evidence

Logs and screenshots are retained in `tmp/s1-1/`:

- `validator-tests.log`: 6 native profile checks covering GLB/embedded glTF, actual scene transforms, false accessor bounds, external resources/path traversal, invalid indices, unsupported extensions/archives, mesh instance budgets and texture metadata bounds.
- Go tests exercise admission rights/profile limits, bounded subprocess output, actual image payload validation and cancellation. The final `api-final.log` reruns both asset acceptance workflows after the strict issue-report limit was added.
- Database/API acceptance covers real S3 uploads/downloads, deduplication, exact validation manifests, source/license immutability, independent original/native variants, actor/environment definitions, invalid geometry, explicit frames, no operational instantiation, cross-environment/partition denial, worker revocation and expired-lease recovery without duplicate model versions.
- `acceptance-browser.log` and the final `browser-final.log`: **8 production-UI/backend browser workflows passed**, including the new import-to-template/environment path, checksum-preserving downloads, pinned URL/reload, public-workspace denial and all previous imagery/public-context/moving/workspace flows. Desktop/mobile screenshots: `asset-registry-desktop.png`, `asset-registry-mobile.png`.
- Full PostGIS regression: **76 passed cases** (52 top-level tests and 24 subcases); 4 opt-in skips are the 3 live provider smokes and the separately executed browser gate. No failures. Log: `integration-final.jsonl`.
- Existing mocked browser suite: **74 passed / 50 intentional project skips**. Log: `browser-mocked.log`.
- `go test ./...`, `go vet ./...`, affected-package Linux race tests, UI typecheck/production build and OpenAPI lint passed. SQLC/TypeScript regeneration stability, deterministic fixture regeneration, UTF-8/JSON/Go formatting and `git diff --check` also passed. Migrations 1-33, the user’s `.gitattributes` and `scripts/ops/migrate.sh` match their pre-task hashes. The UI build retains the existing large-chunk advisory.
- Environment: Go 1.25.0 Windows/amd64, Node 24.13.0, pnpm 10.14.0; disposable PostGIS 17/3.5 and the existing pinned Garage S3 fixture. No GPU or engine performance claim is made.

### Primary format references

- [Khronos glTF 2.0 specification](https://registry.khronos.org/glTF/specs/2.0/glTF-2.0.html).
- [Khronos glTF Validator](https://github.com/KhronosGroup/glTF-Validator); pinned package `gltf-validator@2.0.0-dev.3.10` and its dependency integrity are recorded in `pnpm-lock.yaml`.

<a id="s1-2"></a>

## S1.2 — Rendering engine decision

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

Accepted 2026-10-22 for the bounded S1 scene and S2 presentation boundary. This supersedes the earlier statement that no initial renderer has been selected.

EMAD uses **Three.js 0.180.0** for its first integrated Studio viewport. A separate, compiled **Go `emad-runtime` executable** owns the initial headless abstract kinematics. Browser scene objects are presentation state; they never become authoritative actor truth. The existing YAML adapter remains supported. The rendering choice does not select a specialist physics engine or commit later native GPU workloads to a browser.

### Requirements and selection scope

The initial workload is a local georeferenced scene with at most 16 abstract actors, eight validated models, 500,000 model vertices/triangles, 32 MiB of source geometry, 32 million decoded texture pixels, 4,096 terrain samples and a 10 km radius. glTF Y-up maps to ENU east/up/south in the renderer. S0 WGS84/ECEF/ENU transformations and runtime positions use float64; shader buffers use local float32 coordinates. Acceptance requires under 2 mm of sampled local float32 quantization error, not a claim of 2 mm terrain accuracy or a complete bound on rasterization error.

The renderer must integrate with React without owning routing, identity or application state; load the admitted self-contained glTF profile; support picking, camera controls and a 2D/DOM alternative; work on Windows/Linux browsers; retain offline model bytes and license notices; and dispose GPU resources when a scene or authorization scope changes. Headless execution cannot require a browser, graphics driver or display server. Behavior code is compiled project code rather than arbitrary uploaded scripts. Native rendering/physics may later be a separate adapter to these same contracts; native WebView embedding is not implemented by this increment.

Prototype budgets on the declared machine are under 1 s to the shader-ready first frame from local fixture data, under 200 ms for the fixed 120-frame batch, and under 256 MiB end-of-test RSS / 3 s CPU for each no-GPU scene test. These are small-scene admission checks, not production or maximum-library performance guarantees. The runtime publishes at 2 Hz while its local clock loop runs at 20 Hz; it has no GPU dependency.

### Candidates

Three.js and Babylon.js 8.32.0 were the runnable shortlist. Both use JavaScript/TypeScript, can parse the existing portable assets, integrate directly into the present browser stack, and offer permissive distribution licenses. Babylon provides a built-in no-GPU NullEngine and more engine systems. Three exposes a smaller renderer/scene graph; EMAD must own authoring, lifecycle and later physics/sensor adapters. Babylon's additional features are valuable when a concrete workload needs them, but are not used to imply simulation fidelity here.

Godot, Unity and Unreal were reviewed as alternatives, **not benchmarked**. Godot supports headless servers and native desktop delivery, with distinct large-world and web-export constraints. Unity and Unreal introduce separate editor/export toolchains and project-specific licensing/distribution considerations. They remain plausible candidates for future native GPU or physics requirements. No installed/editor builds, performance numbers, or rejection on technical capability are claimed for those three. This decision compares the two adapters that meet the current embedded-web workload; it does not declare a universal winner among game engines.

### Measured evidence

The reproducible harness is `apps/operator-ui/scripts/evaluate-engines.mjs`, with shared data in `engine-evaluation/fixture.mjs`. It builds a standalone comparison, creates five alternating fresh browser contexts for each candidate in one Chromium process, loads the exact ground tile, abstract wing and marker files, adds the same synthetic terrain, checks the same coordinate fixtures, and submits 120 identical position updates. It waits for shader readiness before the measured batch. Both candidates must render **13 triangles in four draws**. Node tests additionally load the same assets/terrain and execute 20,000 identical 50 ms waypoint steps without a GPU. Asset hashes and every individual sample are retained in the JSON evidence.

Machine: Windows 11 build 26200, AMD Ryzen 9 7900X / 24 logical CPUs, 33,479,524,352 bytes RAM; Node 24.13.0 and Chromium 153.0.8010.12. Hardware samples identify ANGLE / NVIDIA GeForce RTX 4070 Ti SUPER / Direct3D11, driver 32.0.16.1656. Integrated AMD graphics are present but were not the measured renderer.

Medians from [the hardware run](evidence/S1_2_ENGINE_BENCHMARK_HARDWARE.json):

- Three.js: **49.5 ms** shader-ready load; **5.5 ms** wall time and **1.350 ms** GPU elapsed time for the 120-frame batch; **54.68 ms** Chromium renderer task time for the whole measured page. No-GPU test: **45.36 ms** load, **93 ms** process CPU, **74.41 MiB** end RSS.
- Babylon.js: **357.2 ms** shader-ready load; **15.0 ms** wall time and **6.409 ms** GPU elapsed time for the same batch; **118.59 ms** Chromium renderer task time. No-GPU test: **1,979.25 ms** load, **2,046 ms** process CPU, **163.36 MiB** end RSS.
- Both: maximum sampled local float32 quantization error **0.000580 m** and headless waypoint end-position error **0 m** for the declared fixture. The native runtime's separate spatial/kinematic tests verify S0 alignment and state at intermediate virtual instants.

The [software-renderer run](evidence/S1_2_ENGINE_BENCHMARK.json) identifies SwiftShader explicitly. Its GPU timer is unavailable and recorded as null. Browser heap measurements are privacy-rounded, omit driver allocations and are not peak memory; RSS is an end sample. GPU figures are elapsed query time for this batch, not device utilization percentages. Contexts are fresh, while process/driver caches may remain warm. Other host activity, tiny geometry, material defaults and import/toolchain scope limit extrapolation. This is sufficient to choose the current bounded adapter, not to forecast advanced terrain, imagery, animation, physical simulation or large model libraries.

### Decision and consequences

Both pass the prototype budgets. Three's measured load, process footprint and current integration cost favor the small renderer plus explicit EMAD contracts. Studio ships a lazy viewport chunk of about 601 kB minified / 154 kB gzip in this build; Babylon remains an evaluation-only development dependency and is excluded from normal Vite entry discovery. There is no continuous render loop for a static authored scene; camera/selection/resize changes redraw it. DPR is capped at 1.5, model bytes are hash-checked before parsing, and exact properties plus a 2D overview remain available.

EMAD owns a narrow render adapter, not a replacement general-purpose game engine. A new physics, native embedding, high-rate sensing, animation, global terrain/3D Tiles or large-world workload must first define fidelity and performance requirements, compare appropriate engines on that workload, and record a compatible adapter/migration decision. Preserve frozen versions, portable model sources and truth/perception separation during that work.

### Reproduction and primary references

After `pnpm install --frozen-lockfile` and installing Playwright Chromium, run `node apps/operator-ui/scripts/evaluate-engines.mjs`. On Windows, set `EMAD_BENCHMARK_GPU=1` for the Direct3D11 hardware run. The default portable run identifies the actual software/device renderer and does not silently label it hardware. CI runs `evaluate-headless.mjs three` and `evaluate-headless.mjs babylon`; benchmark timings are evidence, not flaky CI thresholds across machines.

- [Three.js GLTFLoader](https://threejs.org/docs/pages/GLTFLoader.html), [WebGLRenderer](https://threejs.org/docs/pages/WebGLRenderer.html), and [MIT license](https://threejs.org/license/).
- [Babylon server-side / NullEngine documentation](https://github.com/BabylonJS/Documentation/blob/master/content/setup/support/serverSide.md) and [Babylon.js source/license](https://github.com/BabylonJS/Babylon.js).
- [Godot large-world coordinates](https://docs.godotengine.org/en/stable/tutorials/physics/large_world_coordinates.html), [headless command-line support](https://docs.godotengine.org/en/stable/tutorials/editor/command_line_tutorial.html), [web export](https://docs.godotengine.org/en/stable/tutorials/export/exporting_for_web.html), and [MIT license](https://godotengine.org/license/).
- [Unity Editor Software Terms](https://unity.com/legal/editor-terms-of-service/software) and [Unreal licensing](https://www.unrealengine.com/license). These are project/licensing research inputs, not legal or financial advice or assertions of EMAD's eligibility for a particular tier.

<a id="s1-3-s2-1"></a>

## S1.3 / S2.1 — Georeferenced Studio and native runtime

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

Delivered 2026-10-22 after S1.1 / migration 34. **Migration 35** is additive and required before API startup. The existing migrations, legacy scenarios and public pipelines are preserved.

### Scene authoring and M4 scope

`internal/simscene` freezes `emad.scene.v1` into content-addressed **bytea** snapshots in EXERCISE_INPUT. The frozen version contains an approved retained context bundle, environment frame, immutable model descriptors and source/validation/build hashes, separate actor templates, placements and waypoint schedules, and bounded preview geometry. It never copies live external tracks into controllable actors. Context import requires both input authoring and the caller's real `context.export` grant in PUBLIC_EVIDENCE of the same workspace/environment. Stored bundles are checked offline; loading a frozen scene does not fetch providers or need current public access.

The initial profile is at most 128 scene versions per environment; a 4 MiB snapshot; 16 actors; eight models; 32 MiB total original model bytes; 500,000 model vertices and triangles; 32 million decoded texture pixels; 4,096 aggregate terrain samples; 1,000 reference points and 10 km radius. Missing terrain samples remain holes. Non-point vectors and imagery/analysis retain lineage with explicit rendering omissions. Model source bytes are fetched through the existing private authoring transport and checked by checksum before parsing. Simulation and perpetual archive rights are pinned through the existing admission profile.

Scene-space transformations use S0 float64 WGS84/ECEF/ENU. The renderer maps east/north/up to X/−Z/Y. A heading of zero points glTF +Z forward north, with clockwise positive headings. Terrain must already use WGS84_ELLIPSOID heights. The earlier NOAA ETOPO sample uses EGM2008 orthometric heights and is rejected until an explicit versioned conversion exists. Reference points without resolved altitude are separately labeled as projected onto the origin-height surface for orientation only.

The M4 acceptance scene combines the accepted Natural Earth Jeddah reference point, retained as an explicitly hashed one-point subset, with **project-authored synthetic ellipsoidal relief** and the three project-authored portable models. It does not claim converted/surveyed ETOPO terrain, real aircraft performance or validated physics. Re-loading the same version preserves coordinates, hashes, rights and fidelity; edits create a different immutable version. The renderer choice and measured budget are in [the S1.2 decision](#s1-2).

`/simulation-studio` integrates the viewport, reset/focus controls, actor selection, exact placement/properties, environment/template selection, context-package import, rights/fidelity, a geographic overview and a 2D alternative. Camera/selection/resize changes redraw a static scene; no render loop drives truth. The shipped renderer license notice is linked from the rights panel. A downloaded scene preserves the exact frozen numeric representation for offline use; do not parse/stringify protected JSON when computing its content ID. URL scene/run selection survives reload. Desktop/mobile checks include renderer readiness and viewport disposal. Input-only authors enter a separate Studio shell with no ordinary COP queries or WebSocket. Public and Exercise COP resources remain separate; input and truth are not offered as COPs.

### Independent runtime and S2.1 scope

`apps/emad-runtime` is a compiled Go executable. It owns the private in-memory world, local 20 Hz wall-to-virtual clock loop, actor spawn/despawn and constant-speed straight segments between resolved ENU waypoints. A static actor stays fixed. An actor reaching the last waypoint remains there until despawn. State is an analytic function of frozen input and integer virtual milliseconds, so tick ordering does not change the state at an instant. Seed, epoch, scale, duration, scene ID, runtime contract and exact executable SHA-256 are frozen per run; the current behavior has no stochastic component and does not use the reserved seed.

The API broker administers runs and validates bounded contracts. It does not execute world ticks, physics or rendering. Human `sim.runtime.control` and service `sim.runtime.execute` are separate EXERCISE_INPUT capabilities; neither is a truth-reader grant. The worker attaches to one explicitly selected queued run, negotiates `emad-kinematic-v1` and the admitted executable hash, and receives a random exclusive lease. The database retains only the lease hash. A different service principal, wrong build, expired/revoked credential, different scope or already attached run is rejected.

Runs transition QUEUED → RUNNING/PAUSED → STOPPING/STOPPED, COMPLETED or FAILED. Pause/resume/stop update desired state and the worker acknowledges it. An older acknowledgment already in flight is accepted while returning the latest desired state; this avoids a pause/resume race without silently discarding newer intent. A queued run can stop before attachment. A lost heartbeat expires after six seconds and materializes FAILED / HEARTBEAT_TIMEOUT on the next authorized control/status request. A failed run cannot be taken over or resumed; create a new run from its frozen scene. In-flight credentials are checked by the real API on every request.

The worker sends an initial snapshot and at most the normal 2 Hz latest-state cadence. Network uncertainty suspends virtual advancement and retries the exact same sequence/payload for at most five seconds; failure discards the local world. Successful retransmission is idempotent. Changed, skipped or reordered sequences conflict. Stop or signal cancellation terminates the worker; cooperative cancellation reports FAILED / INTERRUPTED. Failures use fixed codes, not arbitrary actor/error text. HTTPS is required off loopback and redirects are rejected. API logs and run status contain lifecycle/clock metadata, not private actor positions.

Latest actor snapshots are sealed transactionally in **EXERCISE_TRUTH**, using a narrowly bound writer only after validating the run's service owner, lease, build, sequence and position budgets. The truth resource receives no implicit reader. Successful truth reads require an explicit run truth grant and are audited. The input status endpoint contains no actor array. Ordinary tracks, assets, observations, audit and realtime endpoints reject this truth transport. This delivery retains one bounded latest snapshot, not a recording or continuous truth-history archive.

Limits: four queued/active runs and 1,000 retained run configurations per input environment; 128 control/lifecycle events before new pause/resume requests are refused (stop and its terminal acknowledgment remain available); 8,000 accepted heartbeat sequences per run; 64 KiB latest truth document; virtual duration at most one hour, scale 0.1–20, and one-hour worker wall lifetime. The one-hour wall cap may interrupt slower or heavily paused runs before their requested virtual duration. Renderer frames and local 20 Hz evaluations do not enter the observations table. Event history records lifecycle changes rather than every tick. Old snapshots remain snapshots after a worker stops; they are not a live perception feed.

**S2.2 and S2.3 remain pending.** No native synthetic sensors, ingestion gateway, Exercise COP projection, operational tasking, recording/replay or evaluation metrics are claimed here. Existing YAML scenarios continue through their established adapter. M5 therefore remains pending.

### Contracts and permissions

OpenAPI and generated browser contracts document these implemented REST transports:

- `/api/v1/simulation-studio/context-packages`: permitted package selection; `/scenes`: list/freeze; `/scenes/{id}`: protected frozen input and exact download.
- `/api/v1/simulation-runtime/runs`: human list/start; `/runs/{id}`: human lifecycle status; `/pause`, `/resume`, `/stop`: human controls.
- `/runs/{id}/attach` and `/heartbeat`: authenticated service only, exact EXERCISE_INPUT environment, lease-bound run publication.
- `/runs/{id}/truth`: separate privileged GET, explicitly selected EXERCISE_TRUTH run with `sim.truth.read`. The worker's internal writer is not a delegable browser capability.

All input/truth route exceptions are narrow, known-method/path cases in the existing authorization middleware. Other input/truth transports remain closed. Forced RLS, same-resource scene/run foreign keys, immutable scene bytes, append-only lifecycle events, quotas and scoped audit support the service checks.

### Rollout

1. Apply migration **35** after the user's already applied migration 34 with the existing migration workflow (`task db:migrate`). This delivery tests migrations against disposable PostGIS; it does not migrate the user's database. Recovery remains forward-only.
2. Run `pnpm install --frozen-lockfile` and build/restart the API and UI. Keep the existing private S3 configuration and independent asset validator; no provider, runtime, renderer or worker starts automatically with the API.
3. Adapt `scripts/data/simulation-studio-author-grant.example.json` for an existing human principal and add it using `go run ./apps/emad-admin -action grant -spec <file>`. For new input-only human identities, provision through the existing human/operator administration workflow with this same input grant; Studio requires no ordinary COP permission. Add a separate same-environment PUBLIC_EVIDENCE `context.export` grant only when the author needs to import packages. Existing frozen scenes can be read without public-feed access.
4. Adapt `simulation-runtime-worker.example.json`, then provision a service credential with `go run ./apps/emad-admin -action provision -spec <file> -out <private-token-file>`. The runtime requires no PostgreSQL or S3 credentials. Keep this service token private and rotate it through the established control-plane workflow.
5. Build `go build -o emad-runtime.exe ./apps/emad-runtime` on Windows, or `go build -o emad-runtime ./apps/emad-runtime` on Linux. Run `emad-runtime -describe` and admit that exact SHA-256 in Studio. Binary hashes differ between builds and platforms by design. Freeze a scene from admitted models/templates/environment and a retained context package, then queue a native run.
6. Attach the independently launched worker: `emad-runtime -url https://your-emad-origin -token-file <private-token-file> -run <queued-run-id>`. Local development may use `http://127.0.0.1:8080`. Pause/resume/stop in Studio; no direct real-world hardware dispatch is introduced.
7. For deterministic offline inspection, download the exact protected frozen scene and run `emad-runtime -offline-scene <scene.json> -elapsed-ms 5000 -out <private-truth.json>`. This writes private world state intentionally; it does not send observations or create a run. Keep these authoring/truth files in the appropriate protected storage.

### Executed acceptance

Logs and screenshots are under `tmp/s1-s2/` in the working checkout; comparative engine evidence is retained under `docs/implementation/evidence/`.

- Shared engine comparison: five hardware-rendered and five software-rendered trials per adapter, plus fresh-process no-GPU scene/behavior tests, exact asset hashes and declared coordinate/budget checks.
- Real PostGIS / private S3 / normal admission: immutable accepted scene, terrain/reference alignment and height assumptions, deterministic same-input ID, forbidden COP reads, frozen SQL immutability, exact model loading, offline execution and native compiled-process lifecycle. Typed protocol tests cover duplicates, altered/reordered sequences, worker ownership and scope, wrong builds, pause/resume races, stop, lost leases, credential revocation, cancellation and zero observation ingestion.
- Production UI + real API/PostGIS/S3: ten browser workflows, including the scene/version/runtime path and input-only Studio entry, preserve the previous eight workflows. Desktop, mobile and actor-focus screenshots exercise the actual renderer rather than API mocks.
- Full integration regression: **77 passed cases (53 top-level and 24 subcases)** with four opt-in skips; the final native acceptance test is rerun after the clock/quota/cancellation refinements. Existing mocked browser regression: **74 passed / 50 intentional project skips**. Go tests/vet, affected-package Linux race tests, Windows/Linux native offline execution (identical scene, virtual instant and positions), UI build and OpenAPI/generator stability pass. The build retains the existing large-chunk advisory and a separately loaded Three viewport chunk.

M4 is accepted within the explicitly synthetic-terrain / visual-only / abstract-behavior scope above. S1.2, S1.3 and S2.1 are complete; the next task is **S2.2 — Synthetic sensor adapters**.

<a id="s2-2-s2-3-s3-1"></a>

## S2.2–S2.3 and S3.1 — Native perception and exercise tasking

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

Date: 2026-10-22. Implements the next three tasks in the v2 plan. This delivery extends the accepted Three.js authoring viewport and independent Go runtime; it does not select a specialist physics engine or add a hardware bridge.

### Delivered behavior

Simulation Studio can freeze up to two abstract observers and an explicit controlled-actor list into an immutable scene. A scene without these additions retains the existing `emad-kinematic-v1` contract and content identity. A perception/controlled scene uses `emad-kinematic-v2`. The native executable advertises both versions and its exact binary SHA-256. Queue the version declared by the frozen scene, not a manually substituted runtime version.

Observers have a 1,000–60,000 ms sampling interval, a scheduled missed-sample pattern (`dropEvery`; zero disables misses), 0–10,000 ms delivery delay, a 0–200 m uniform error bound independently on local east/north, and configured observation confidence. The horizontal error bound is `sqrt(2) * noiseM`. Noise is deterministic from the run seed, observer, actor, and sample time. These are abstract synthetic error profiles, not validated radar/camera physics, source-reliability measurements, ML scores, or evidence of real entities. No report means no observation; it does not establish absence.

Sampling, actor behavior, virtual time and command execution run in `apps/emad-runtime`. The worker evaluates actors and command progress at 20 Hz and publishes normally at 2 Hz. Command execution does not wait for the next publication boundary, including at the maximum 20× clock scale. The synchronous Go API validates bounded contracts and converts permitted samples into geospatial evidence; PostGIS continues to own containment, spatial relations and derived alerts.

Each observer/actor pair gets an opaque contact identity derived with a lease-keyed HMAC. Observation message identities are separately keyed by contact and sample time. Neither contains the hidden placement or observer name. A new run gets new contact and controlled-asset identities. An independently random public asset ID is persisted in a sealed actor mapping only for each explicitly controlled declaration. Its declared public callsign and capabilities are visible to authorized run operators. Self-reported telemetry belongs only to that mapped asset. Hidden actors do not become assets merely because they produced contacts.

### Gateway, durability and isolation

The implemented gateway extends the existing REST runtime attachment/heartbeat transport; it is not an unimplemented protobuf listener. HTTPS is required off loopback, redirects are rejected, and the independent process needs only its service credential, API origin and admitted run ID. It needs no database or object-store credential. The runtime version, immutable scene, binary hash, producer principal, run, lease, sequence, virtual sample/delivery times and server-bound resource establish the accepted envelope. Existing S0 coordinates, clocks, partition semantics and domain pipelines are reused. The legacy YAML adapter and its restricted writer are preserved.

Attachment creates one six-second lease bound to the admitted service principal. Changed, skipped or reordered heartbeats conflict; an identical accepted heartbeat is idempotent. A private perception batch commits in `EXERCISE_TRUTH` with the broker's snapshot/sequence and scoped public clock update before projection. Projection then reuses Observations, Tracks, Telemetry, Assets, PostGIS alerts, the event dispatcher, Audit and authorized realtime delivery. No raw actor snapshot is sent to an ordinary COP channel. A retry after a partial failure resumes the private outbox and reuses domain message IDs. Pending batches are bounded to the admitted snapshot watermark; an advisory lock serializes projection/reconciliation for a run.

The worker suspends virtual advancement while a heartbeat is uncertain and retries the same payload for at most five seconds. A short connection failure retains the in-process world and command memoization. Lost attachment replies, expired leases, credential revocation and process loss fail closed; there is no silent takeover or checkpoint recovery. Start a fresh run with a new request key from the same frozen scene. Recording/checkpoint recovery remains S3.2.

Publication is capped at 512 readings, 64 command outcomes and 256 KiB per heartbeat; at most 20,000 submitted readings and 8,000 heartbeats belong to one run. The frozen maximum sample estimate is checked before admission and a transactional cumulative reading counter enforces the runtime cap. Existing actor/model/run/duration/control-event budgets remain. Native command types are capped at 80 characters and payloads at 8 KiB before they can enter dispatch. Replies include the complete bounded roster of at most 128 unfinished commands so cancellation synchronization cannot mistake pagination for cancellation. The worker admits at most 32 into active execution per publication window, keeping their ACK/completion output within the publication budget. The private outbox is recovery metadata, not a replay archive.

Stop/failure suppresses unpublished perception. Database admission checks serialize evidence inserts against closure: evidence committed before closure remains evidence; a later insert is rejected. COMPLETED runs flush already admitted final samples. Pending command cleanup runs on heartbeat projection, controller reads/stops and native COP status reads. Lease expiry is materialized on those reads; it does not require a background worker to keep accepting commands. The command admission trigger independently checks current database lease time, so an idle or disconnected client cannot task an expired run.

Forced RLS separates input, perception and truth. Actor mappings, raw batches and latest truth remain in run-specific `EXERCISE_TRUTH`. Ordinary APIs, streams, audit and process logs carry only permitted evidence, public controlled identifiers and fixed outcome codes. Explicit evaluator truth reads use the separate truth endpoint/grant and are audited. Input-author permission does not automatically grant a COP or truth permission. Public evidence and source-provider activation remain independent.

### Exercise COP and supporting operations

`/scenarios/runs` and its scoped run detail can expose a native run's safe lifecycle, virtual elapsed time, scale and epoch to `scenarios.read`. The Exercise COP uses virtual time for freshness and preserves sample time separately from wall receipt time. Its map distinguishes observed tracks from controlled assets. The native run band links to `/exercise`; older YAML pause/restart/speed actions are not presented for native runs. Human native lifecycle control remains in the protected Studio.

Map display smoothing uses only two already permitted point snapshots, lasts at most 250 ms, is capped at 64 points per layer, and respects reduced motion. It never extrapolates, writes cached evidence, advances virtual time or computes authoritative actor state. Inspector values remain the received evidence. Historical/scrubbed views and Public Context do not use this native display transition.

The new exercise operations view shows status, virtual clock, controlled recipients, mission tasks and command outcomes. It retains last received values with an unavailable status and disables tasking when current status is unavailable or closed. Desktop places outcomes beside tasking; mobile presents outcomes before the form. Polling is 2 seconds for status/commands/missions and 5 seconds for assets. The view is bounded, uses existing scoped query/cache cancellation, and does not request authoring or truth data.

Operators issue `HOLD` or `RESUME_ROUTE` to explicitly controlled virtual recipients through the existing command service. Existing incident-linked missions, assets, evidence relations, human operators, audit and realtime remain in use. The incident command console also supports native exercise actions and retained retry keys. Native execution outcomes are adapter-owned; a granted operator may cancel, but cannot manually fabricate an ACK or completion through the ordinary transition endpoint.

A request key is scoped to the run and authenticated principal, separately from the human operator used in audit. Retrying the same request returns the same command; changing its fingerprint conflicts. Two principals sharing an operator record do not share request keys. Admission locks native run metadata, verifies current lifecycle/lease, controlled recipient, same-run mission/incident relations, mission assignment and optional task membership. Stopped, terminal or expired runs cannot receive new tasking.

The abstract adapter acknowledges after at least 500 virtual ms and executes after at least 1,000 virtual ms from task receipt, at the next runtime tick. HOLD pauses path-distance advancement; RESUME_ROUTE resumes it. Unsupported actions are REJECTED, unavailable actors produce FAILED, and the API closes overdue commands as TIMED_OUT after the bounded issue-to-outcome deadline: `max(10,000, ceil(scale * 2,000))` virtual ms. This leaves room for two-Hz gateway delivery at accelerated clock scales without letting wall time advance the exercise clock. Queue delay counts toward that deadline. Pause freezes virtual time and retains queued work; stop, terminal completion or lease loss cancels unfinished commands. ACK can time out while waiting for completion. Outcomes retain command/mission/task identity, are run-bound and idempotent. Commands cannot be dispatched into another run or re-executed after a lost reply. Successful active replies synchronize cancellations into the worker dispatch window; cancellation cannot undo an action that already executed before the worker learned of it.

An `assets.read`/`telemetry.read` grant over the selected run authorizes its declared controlled assets. This delivery preserves the existing run-level authorization model; it does not claim a new per-asset ACL system. Do not grant input authoring or truth to ordinary operators merely to make the COP selectable.

### Rollout

1. Apply new migration **36** after the user's already applied **35** using the existing migration workflow (`task db:migrate`). The agent tested disposable PostGIS only; the user's database was not migrated. Migrations 1–35 and the migration script are preserved. Migration 36 adds scoped native metadata, immutable controlled bindings, private projection batches, bounded dispatch and principal-owned retry keys. Recovery is forward-only.
2. Build/restart the API and UI. No new queue, service framework, GPU dependency or public provider activation is required. Preserve the existing private object store and independent model validator.
3. Build the native executable (`go build -o emad-runtime.exe ./apps/emad-runtime` on Windows; omit `.exe` on Linux). Use `-describe` and admit that exact binary hash. Older build hashes cannot attach to a newly admitted build. Existing v1 frozen scenes remain usable with a correctly pinned compatible build.
4. Retain the existing `simulation-runtime-worker.example.json` service and input-author grants. Freeze observer/controlled settings in Studio, queue the matching scene version, then launch `emad-runtime -url https://your-emad-origin -token-file <private-token-file> -run <run-id>`.
5. Adapt `scripts/data/native-exercise-operator-grant.example.json` to an existing human principal and the **exact admitted run ID**, then apply it through the existing administration grant command. An explicitly reviewed observation-run wildcard is optional for controllers who must select future runs; Studio does not create that grant automatically. “Open Exercise COP tasking” checks the separate COP grant and selects that resource.
6. Select the run in Exercise COP, inspect synthetic evidence/own telemetry, and create a virtual mission/task. Keep evaluator credentials and offline truth files separate. A stopped/failed run is replaced with a fresh run, not retasked or restarted into its old evidence.

### Executed verification

Acceptance results and final counts are recorded in `evidence/S2_2_S2_3_S3_1_ACCEPTANCE.json`. Logs and desktop/mobile screenshots are under `tmp/s2-s3/` in the working checkout. The focused test runs the compiled native executable against a real disposable PostGIS and private S3 store with public feeds disabled. It contrasts evaluator truth with delayed perception, inspects serialized ordinary WebSocket frames and process logs, reuses PostGIS alerts/incidents/missions, and verifies runtime task outcomes. Protocol tests force projection failure after private commit, reconnect/retry, reordered messages, principal ownership, run isolation, virtual deadlines and stop/lease handling. A separate maximum-scale native run checks tick-based task execution.

Executed checks pass: **78 integration cases (54 top-level, 24 subcases)** with four opt-in skips; a final native recheck plus **12 real-backend browser checks**; **74 existing mocked-browser checks** with 50 intentional project skips; Go tests/vet, affected-package Linux race checks, production UI build, OpenAPI lint and generated-contract/sqlc checks. The production build retains the existing large-chunk advisory. Migrations 1–35, the migration script and user attributes retain their original hashes.

The production UI gate uses the assembled API, real PostGIS/S3 and independent compiled runtime without mocked domain APIs. It freezes sensing settings, selects the resulting COP, creates a mission, observes task completion, inspects the map/run clock, verifies mobile layout and stops tasking. Display interpolation is also checked for antimeridian handling, clamping, removed contacts and source immutability. Existing regression suites remain required.

M5 acceptance is limited to the declared abstract kinematic/synthetic-sensor profile and permitted controlled assets. M6 remains pending S3.2 recording/replay and S3.3 privileged evaluation. This delivery does not certify physical sensor fidelity, create a complete native recording/replay system or expose hidden runtime actors in an operator 3D viewport.

<a id="s3-2-s3-3-s4-1"></a>

## S3.2, S3.3 and S4.1 — Recorded exercises, evaluation and sensor presets

> **Historical record — superseded.** This documents the pre-map-first v2 delivery sequence (F0–S4) and is retained as traceability history for code still installed behind the explicit `historical-full` profile. It is **not** the current product status and does not describe the map-first product. Current records: [index](README.md) · [scope](../../EMAD_CURRENT_SCOPE.md) · [current operations](MAP_FIRST_OPERATIONS.md#map-profile).

Delivery: 2026-10-22. This increment completes the original 32-task backlog through S3.3 and adds the user-selected first S4 increment, reusable sensor-quality presets. S4 remains an ongoing backlog, not a completed workstream. The native profile remains abstract kinematics and scheduled synthetic sensing, without physical sensor certification or hardware control.

The subsequent [review-readiness closeout](HISTORICAL_RECOVERY.md#review-readiness) completes worker recovery, durable retry status and evaluation v2 operand corrections without another migration. Earlier acceptance evidence below retains the original v1 measurements.

### S3.2 — Durable recording and read-only replay

Migration **37** adds append-only, forced-RLS journals under separate `EXERCISE_OBSERVATION` and `EXERCISE_TRUTH` resources. New native perception runs record their initial clock and subsequent safe domain changes: sources, observations, controlled assets and telemetry, tracks and state/history/associations, classifications, geofences, alerts, incidents and evidence relations, missions/tasks/assignments, and command lifecycle. Geographic values are converted by PostGIS. An explicit column allowlist excludes arbitrary payloads, metadata, correlation hints, source-reference strings and private runtime configuration. Observation documents retain their existing random publication references; opaque lease-keyed sample references support comparison in the restricted evaluator.

The validated heartbeat transaction separately seals selected world snapshots, accepted synthetic batches and outcomes, and the frozen configuration. The private configuration binds the exact runtime build/version, scenario/scene content identity, asset/context versions and hashes, seed, coordinate frame, epoch, scale and duration. Lease tokens and credentials are excluded. Duplicate heartbeat retries do not append duplicate recording events.

Shared-kernel identity audit records bounded grant and credential-lifecycle changes without tokens or credential hashes. Only relevant environment/run changes are copied into the truth archive. Actual wall times are preserved; their final virtual-time position is explicitly an archive anchor, not an invented virtual effective time. Existing grants are labeled migration baselines with unknown earlier effective times. This history never authorizes a present-day read.

An operator with the complete run read grant requests an archive after the native run closes. The separate `apps/emad-recordings` executable claims a bounded lease, reads immutable journal watermarks, stores content-addressed JSON chunks and selected-state checkpoints in the existing private S3 store, computes evaluation, then atomically publishes both scoped indexes and the privileged result. Large archive construction and metric computation stay outside synchronous HTTP handlers. No new broker, renderer or GPU service is introduced.

An interrupted archive remains unreadable and can retry, reusing identical content-addressed blobs. A lease generation and a final credential/grant refresh prevent publication by an abandoned or revoked worker. Partial blobs have no readable published index. At most three archive attempts are allowed. Already-ready archives are never reprocessed.

Each archive explicitly records whether gateway projection had settled. If publication was interrupted, the recording preserves the output operators actually received; accepted private samples are not counted as delivered observations. The incomplete-publication notice is visible in replay and evaluation. Later COP edits do not rewrite that archive cut.

`/recordings` provides virtual-time and journal-sequence seeking, step controls, a recorded position overview with accessible coordinate values, historical layer counts and workflow state. Its resource, time and sequence are URL-backed. Seeks read immutable chunks, never execute simulator logic, dispatch commands, ingest observations or modify the original run. There are no simulation ticks in the API. Replay folds bounded recorded events; it does not fall back to current COP attributes. Source-restricted callers are denied the complete recording rather than receiving an incorrectly filtered archive. Current full COP read grants are required for operator playback; truth playback additionally requires explicit truth access. Every recording/event/frame/checkpoint/evaluation read is audited.

Checkpoint compatibility is **recorded-output restoration only**: matching `emad.recording.v1`, verified content hash, event watermark and virtual position. Checkpoints retain selected mutable state; sample histories remain in ordered chunks. This delivery does not resume a native process or restore a lost lease from a checkpoint. Stored replay and a fresh re-simulation remain different operations. Existing v1/v2 scenes without presets preserve their frozen identities; a new admitted native run is required to capture complete history after migration 37. Old evidence is not relabeled as a complete recording.

### S3.3 — Restricted after-action evaluation

`emad.abstract-evaluation.v2` consumes the two recorded streams, never live projections. Its stable output includes definitions, numerator, denominator, unit, time basis, source recording hashes and limitations. Mean operands contain measured sum and sample count; fraction operands contain counts. Stored v1 reports retain their original count-based mean operands and correct displayed values. Metrics cover missed and scheduled observations, late publication and mean delay, horizontal error where a truth sample exists at the exact sample time, track merges and actor fragmentation, duplicate accepted sample IDs, terminal/completed commands, acknowledgment/completion timing, and runtime scenario completion.

Expected observations are active actor/observer sample slots whose configured delivery is due by the recorded end. Scheduled losses are included and reported separately. A publication is late when it exceeds configured delivery by more than 1,000 virtual milliseconds. Missing denominators and unmatched exact-time truth are **unavailable**, not zero scores. Command timing uses virtual time; wall receipt timestamps remain separate. Completion of the runtime and completion of operator workflows have separate metrics. No confidence interval, semantic classification accuracy, physical sensor validity or weapon effectiveness is inferred from this synthetic fixture.

The operator explicitly opens the evaluator panel. The request uses the exact `EXERCISE_TRUTH` resource and requires `sim.truth.read` and `sim.evaluation.read`; normal operator or platform-administrator status does not imply them. Truth query/cache keys are separate, and changing the run closes the privileged view before a new truth request can start.

A human with `sim.evaluation.release` may explicitly release selected numeric aggregates and a reason into one immutable operator summary. Actor identities, mappings, positions, frozen input and private hashes are not automatically copied. An identical release retry returns the original result; a different replacement conflicts. Ordinary operators can read that intentionally released summary without receiving evaluator authority.

### S4.1 — Reusable quality presets

Migration **38** adds immutable, environment-scoped sensor presets in `EXERCISE_INPUT`. Authors save named profiles containing sample period, scheduled misses, delivery delay, XY noise, confidence, limitations, schema and declared abstract fidelity. The content hash identifies the version. Repeating an identical save is idempotent; changing a value creates a new version.

Studio applies a selected profile to an observer and freezes both the preset document and exact numeric settings into the scene. The API and standalone frozen-scene validator reject mismatched or missing preset versions. Editing a value explicitly clears the preset reference and returns to custom settings. Existing scenes and custom observers remain valid. A baseline comparison verifies that adding preset attribution changes neither seeded sampling nor the existing runtime limits. This increment adds no external asset rights dependency and no new physical sensing model.

### Budgets and retention

- Existing run limits remain: 16 actors, two observers, 20 Hz native ticks, approximately 2 Hz gateway publication, at most 20,000 readings and 8,000 heartbeats, virtual duration up to one hour, scale 0.1–20, and 1,000 retained run configurations per environment.
- Each partition journal admits at most **100,000 events / 128 MiB** of document data. Oversized or exhausted capture is marked truncated; an incomplete journal cannot be published as a complete archive. Ordinary event documents are at most 256 KiB; the frozen private configuration has a separate 4 MiB bound.
- Archive chunks target 2 MiB; each artifact, including the selected-state checkpoint, must fit the existing 16 MiB object limit. Playback validates at most 128 chunks / 160 MiB of serialized events per stream. An archive with more than 4,096 relevant identity changes or incomplete bounded authorization capture fails visibly.
- Events page at 100 rows. Frames show at most 200 rows per layer with explicit total counts and a partial-frame indicator; an encoded frame is limited to 8 MiB. All events remain available through pagination. Frame folding is request-bounded, not a high-rate or unbounded historical query. Slider changes commit on release/key completion rather than issuing a request per pointer move.
- One final archive per run; three attempts; a ten-minute archive lease. Each index has a **90-day read window**, and expired artifact/replay/evaluation reads are denied. Immutable journals, archive dependencies and intentionally released decision summaries are not automatically erased; existing backup/administrative retention applies. This release does not claim a physical deletion daemon or unlimited archival capacity.
- At most 128 sensor preset versions per input environment, two references per frozen scene, and the existing observer numeric limits. Presets introduce no rendering cost and no new runtime algorithm.

Acceptance uses Windows/amd64, a Ryzen 9 7900X host (24 logical processors), disposable PostGIS and private S3, and the compiled headless native runtime. The M6 fixture uses three actors, one observer, one controlled asset, eight virtual seconds at 2x, and a mission-linked HOLD task. Its expected 24 due observations include 12 scheduled losses; the remaining 12 are matched to published COP evidence and three distinct tracks. Archive/frame timings and exact archive sizes are retained in the machine-readable evidence. These are fixture measurements, not a maximum-scale benchmark. No additional GPU budget is claimed.

### Rollout

1. Apply **37 then 38** after the already applied 36 using `task db:migrate`. Only disposable databases were migrated during verification. Migrations 1–36, the migration script and existing evidence are preserved. Recovery is forward-only.
2. Build/restart API and UI, and rebuild/admit the exact `emad-runtime` binary hash as usual. Preserve the existing private S3 configuration and runtime/asset workers. Start a fresh native perception run for complete recording capture; migration cannot reconstruct earlier history.
3. Adapt `scripts/data/simulation-recording-worker.example.json` and provision its environment-bound service credential using the existing `emad-admin` flow. Build with `task simulation:recordings:build`, or build all Go executables with `task build:all`. Run `bin/emad-recordings -token-file <private-file>` (use `bin/emad-recordings.exe` on Windows); `-once` processes at most one eligible archive attempt and exits nonzero on failure. The long-running worker survives archive failures and transient queue outages. It uses the existing database and artifact environment/secrets configuration.
4. Add `scripts/data/simulation-recording-operator-grant.example.json` to an existing human for the exact admitted observation run. Omit `sim.recordings.manage` for replay-only readers. Full run/source read access is required for this first complete-stream archive format. Open **Exercise operations → Recording and replay**, close the run, request its archive, and wait for READY.
5. For a deliberately privileged evaluator, adapt `scripts/data/simulation-evaluator-grant.example.json` for the same human/run and apply it with `emad-admin -action grant -allow-truth-grants -spec <file>`. The local CLI flag explicitly enables restricted grant provisioning; its default remains denied. The evaluator also needs ordinary recording access to use the combined browser view. Do not grant truth to ordinary operators or the runtime just to enable replay.
6. Review metric definitions/limitations before releasing a numeric summary. No public-provider activation, additional engine selection, external messaging, live hardware or engagement authority is part of rollout.

### Verification and acceptance

See [machine-readable acceptance](evidence/S3_2_S3_3_S4_1_ACCEPTANCE.json). Logs, exact measurement output and desktop/mobile screenshots are under `tmp/s3-s4/` in this checkout.

Coverage includes a real permitted reference/context package, frozen georeferenced scene, compiled independent actor runtime, synthetic perception, a mission/task and command completion, historical classification revisions, separate operator/truth archive artifacts, virtual-time replay, audited privileged evaluation, and explicit numeric release. It checks original evidence/command counts, current-state independence, wrong-run/source-restricted/revoked/expired reads, interrupted archive retry, corrupted bytes, compatible/incompatible checkpoints, heartbeat deduplication and immutable release conflicts. Preset checks cover content identity, revisions, frozen-value mismatch, custom edits, scope enforcement and unchanged sampling behavior. The real UI workflow uses the assembled backend and actual PostGIS/S3 without mocked domain APIs.

The required checks are Go tests/vet, disposable integration cases, real-backend browser checks, existing mocked browser regressions, affected-package Linux race checks, production UI build, OpenAPI lint and generated contracts/sqlc. The UI build retains the existing large-chunk advisory. A legacy 50x acceptance poll now waits for an actual returned alert row as well as its count, avoiding a race between separate count/list reads.

M6 is accepted only for this declared abstract/reference/synthetic-terrain profile after these checks pass. The original F0–S3 backlog is complete within its documented acceptance scopes. Further S4 work must choose a concrete scenario, versioned contracts/assets, fidelity limits and measured resource budget before implementation.
