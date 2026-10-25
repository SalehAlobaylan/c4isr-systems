# F0.1 — Baseline verification and migration inventory

**Date:** 2026-10-20 (Asia/Riyadh)

**Status:** Complete for baseline verification and migration planning. F0 isolation is not implemented or accepted by this report.

**Baseline commit:** `2c0c65548036ca73be8e8c259c60171ddecfc46b` on `main`.

**Plan:** [EMAD Implementation Plan v2](../../EMAD_IMPLEMENTATION_PLAN_v2.md), task F0.1.

## 1. Scope and evidence

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

## 2. Environment

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

## 3. Executed verification

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

## 4. Current ingress and authorization boundaries

### HTTP surface

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

### Non-HTTP ingress and background work

- Scenario setup calls `Sources.Ensure`, `Assets.Ensure`, and `Geospatial.Ensure` directly. It already records and verifies run/namespace metadata for reused resources. Keep these checks when adding the validated resource context.
- The scenario engine calls normal observation, telemetry, classification, assessment, incident, asset-status, and command services directly. HTTP-only enforcement would leave this ingress unprotected.
- Scenario command listeners select in-memory runs owning an asset and schedule acknowledgments/outcomes; cancellation/finalization and pending event persistence are additional mutation paths.
- Application startup ensures the default operator and marks abandoned active runs. Both need explicit platform/run authority rather than inferred operator identity.
- The periodic freshness monitor scans assets/tracks and publishes status changes without an incoming HTTP request. Its repository reads and event routing must recover the resource's trusted scope.
- Synchronous domain subscribers update tracks/history, containment state, alerts, and audit. Subscriber failures do not roll back the original evidence insert; retry and scope context must survive projection recovery.
- No implemented public-provider connector, artifact service, independent simulation executable, or gRPC server listener was located. Protobuf service declarations are contracts awaiting a transport implementation.

## 5. Persistence and legacy classification

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

### Local development data audit

Run [F0_1_LEGACY_AUDIT.sql](F0_1_LEGACY_AUDIT.sql) with an explicitly selected database. The recorded local snapshot contained:

- Two completed runs; four sources, all labelled `sensor`.
- Consistent recorded run + namespace candidates for **4 sources, 41 observations, 2 geofences, and 1 track**. No assets or telemetry samples were present.
- **41 observation/source links**, **41 track/observation links**, **41 evidence/source links**, and **1 geofence/track state** with no missing or conflicting run markers in the inspected relationships.
- **140 audit records**, all carrying a marker matching a recorded run.
- One alert, two classifications, 40 track-history rows, one track-state row, 88 scenario-run events, and one operator.
- No incidents, missions, tasks, commands, assessments, or corresponding populated join rows in this database. Their behavior is covered by the separate integration suite, not by this local data sample.

These are **backfill candidates**, not assigned partitions. The audit emits aggregate counts and never updates records. It does not prove that metadata was originally written by a trusted principal, or exhaustively classify every aggregate. F0.3 must combine creator audit, parent relationships, valid run records, and resource metadata; a source type or ID prefix alone is insufficient. Ambiguous or conflicting records remain unresolved for explicit review and must not be exposed by a default public scope.

No database rows were relabelled, quarantined, migrated, or promoted to public evidence in F0.1. Quarantine enforcement belongs to the additive migration and service changes in F0.3/F0.4.

## 6. Events, projections, caches, and artifacts

### Event and broadcast inventory

The synchronous dispatcher defines 31 topics. Their source/asset/evidence/track/classification/spatial/alert/incident/mission/command/assessment/run families are enumerated in JSON. The composition root logs and measures all topics; the WebSocket hub subscribes to all and broadcasts to all connected clients through explicit payload mapping.

Keep existing safeguards: bounded client buffers, slow-client disconnection, explicit message mapping, scenario namespace propagation, and scenario-aware geofence filtering/reconciliation. These are useful partial controls, not a complete resource authorization layer.

Two subscription/mapping details must be retained in the migration inventory:

- `entity.freshness.changed` has an explicit browser mapping, but is not in the audit service's explicit subscription list.
- `area.geofence.overlapped` is an internal intermediate event without explicit browser mapping or audit subscription; the hub nevertheless subscribes to it through `AllTopics` and can emit its envelope. Decide its permitted audience explicitly rather than expanding its payload incidentally.

Hidden actor identity may travel through observation `trackHint`, logical references, payloads, scenario definitions/event names, audit data, and structured logs. F0.5 must evaluate those paths structurally; removing one visible position field is insufficient. There is no sealed truth store to migrate yet.

### Browser read models

Current UI paths: `/`, `/board`, `/map`, `/tracks`, `/tracks/$id`, `/systems`, `/assets`, `/alerts`, `/incidents`, `/incidents/$id`, `/timeline`, `/scenarios`.

The query factory covers health, current operator, operator directory, sources, assets/telemetry, tracks/history/classifications, observations, geofences, alerts, incidents, missions, commands, assessments, audit, and scenarios/runs/events. The inventory also records every inline query-key use.

F0.6 must include the inline `['picture', at-or-live]` and `['picture-window']` keys in map/detail/scrubber components, as well as factory keys. Realtime reconnect invalidates broad resource prefixes; it does not currently select a workspace/run subscription. UI selections and replay state have no ownership context. The persisted layout store contains presentation preferences, not a duplicate authoritative evidence database; retain that distinction.

### Files and artifacts

- Five YAML fixtures are current authored exercise input: `normal-patrol` (seed 42), `restricted-area-intrusion` (1007), `multi-source-track` (2027), `operational-failure-matrix` (9090), and `area-hazard-intrusion` (2211). Preserve their fault cases and logical identities when versions are frozen in S0.
- The loader reads mutable YAML definitions from the configured scenarios directory; completed runs do not yet bind a content-addressed frozen bundle.
- SQL migrations/queries, OpenAPI/types, and protobuf declarations are versioned developer artifacts; regenerate derived outputs from their source definitions.
- Screenshot references, Playwright traces/reports, and frontend build outputs are development evidence. They are not a partition-aware product artifact store.
- Operational dumps/backups and centralized logs can contain data from multiple future partitions. Include retention, privileged access, and recovery paths in the ownership design; existing Compose secret files are deployment credentials, not public evidence.
- Imagery/terrain/model bundles, signed download references, recordings, and a truth artifact store are future capabilities. Add access policy at their first implementation, not after a shared public bucket is established.

## 7. Implementation handoff

F0.1 acceptance is satisfied by the executed checks, reviewed route/table/event/read-model inventory, safe classification policy, local read-only audit, and documentation precedence updates.

The next task is **F0.2**: define human/service principals, resource context, grants, and one canonical permission/envelope vocabulary. Its implementation must account for:

1. Existing human operator authentication and explicitly disabled-auth compatibility paths.
2. Direct scenario/background service calls as well as HTTP.
3. Platform metadata versus domain ownership; no implicit truth grant for administrator/analyst roles.
4. Reusable authoring inputs versus run-bound outputs.
5. Current namespace safeguards and ambiguous legacy provenance.
6. Authorization denials that are auditable without leaking hidden state.

F0.3 then persists ownership and classifies/backfills with evidence. F0.4/F0.5/F0.6 enforce it across services, queries, subscriptions, and browser state. Public connectors stay disabled until the full F0 gate passes.
