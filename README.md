# C4ISR Systems

## Open the app

**Use the installed app on this workstation.** Open the [EMAD Map](http://127.0.0.1:5286/map) or go directly to the [Riyadh reference release](http://127.0.0.1:5286/map?tool=environments&workbench=%7B%22id%22%3A%22georelease_01m786xtwrr6d0sm1by5n5r4dc%22%7D). The configured staging account opens automatically. On the Riyadh release, expand **3D terrain preview**, click **Open terrain**, then turn on imagery, roads, buildings or water as needed.

After a Windows restart, open Docker Desktop, wait for it to finish starting, and reopen the same link. The installed app starts automatically unless its containers were manually stopped. If the browser still cannot connect, check the containers with `docker ps -a --filter name=emad-environment-foundation-staging` and open the [API health endpoint](http://127.0.0.1:8286/health).

**Run from source for development.** Follow the [development setup steps](#start-the-map-installation) below. The development server uses its own local data.

## Project status

EMAD supports map-centered Public Context and Intelligence for the Middle East and configured worldwide workflows. Geographic preparation prioritizes detailed environments in Riyadh, national discovery in Saudi Arabia, and regional coverage across the Middle East. Simulation design is a later phase. [Current product scope](EMAD_CURRENT_SCOPE.md)

- **Environment and 3D infrastructure:** Riyadh has a staged reference release. The plans track implementation and the remaining acceptance steps for the installed workflow. [Plan index and acceptance sequence](plans/README.md)
- **Map-first release:** T01–T21 are implemented. T22–T24 qualification and independent user acceptance remain open. [Implementation records](docs/implementation/README.md#current-status) · [Map-first backlog](docs/implementation/MAP_FIRST_BACKLOG_2026_10_25.json)
- **Diagrams:** Architecture and workflow illustrations and reusable prompts for generating diagrams. [Illustrations](docs/diagram/README.md) · [Diagram prompts](Diagram-prompts.md)
- **Historical delivery:** Original acceptance runs, provider activation records, and exercise, replay and evaluation workflows. [Historical delivery](docs/implementation/HISTORICAL_DELIVERY.md) · [Historical recovery](docs/implementation/HISTORICAL_RECOVERY.md)

## Primary data-source discovery reference

[World Monitor — Data Sources](https://www.worldmonitor.app/docs/data-sources) is a **primary discovery reference for expanding EMAD's environmental and public-context data** in preparation for future simulations. Consult it when investigating weather, aviation, maritime activity, hazards, population context and infrastructure. Its [source-attribution inventory](https://www.worldmonitor.app/docs/source-attribution) helps trace providers and upstream hosts.

Follow each candidate to the original provider and record the exact product, Saudi/Middle East coverage, spatial and temporal resolution, historical availability, access requirements and permitted retention/export. World Monitor supplies discovery leads; its integrations and derived scores do not establish EMAD's coverage, source permissions or physical simulation accuracy.

For the first simulation data package, prioritize Riyadh municipal incident reports, connected roads, inspection evidence and civilian service demand. These inputs support a concrete exercise in which crews verify a reported obstruction and decide when access can reopen. The 18 November revision also identifies measured civilian drone flight logs, labeled aerial imagery, historical air traffic and coastal activity for follow-on exercises in eastern and southern Saudi Arabia. [Source assessment and acquisition priorities](docs/data/WORLDMONITOR_SOURCE_REVIEW_2026_11_17.md)

The [archived simulation source and feature plans](plans/archive/simulation-sources/README.md) record the proposed source uses, implemented workflows and acceptance tests. Start with the Riyadh road-reopening exercise, then use the aerial inspection, Asir service-access and Jubail port-activity plans for their respective environments.

[Desert Maps regional references](docs/data/DESERT_MAPS.md) lists the publisher’s maps and the admission workflow for licensed Arabic place names.

## Start the map installation

Start Docker first. This repository uses Go Task. Complete the [prerequisites](#prerequisites) for a fresh checkout, then run these commands from the repository root.

1. Run `task local:setup` for the initial installation and after pulling changes that require new migrations or Go binaries. Setup waits for PostgreSQL and preserves existing data.
2. Once setup succeeds, run `task dev` and keep that terminal open.
3. Open the [development map](http://127.0.0.1:5173/map). It signs in automatically with the local map analyst account.

For subsequent sessions, use `task dev`. Use `task local:doctor` to diagnose startup and `task local:stop` to stop the development supervisor before rerunning setup. The supervisor also serves the existing bookmark on [port 5198](http://127.0.0.1:5198/map) when that port is free. Historical facilities use the explicit `task local:setup:historical` and `task dev:historical` commands. [Map operations and recovery](docs/implementation/MAP_FIRST_OPERATIONS.md#local-operation)

## Documented baseline status

The historical implementation plan reports the following status. [F0.1 baseline verification](docs/implementation/HISTORICAL_DELIVERY.md#f0-1) records subsequently executed checks and the migration inventory; it does not certify the planned v2 scope boundaries.

| Phase | Scope | Historical implementation/profile evidence; see current delivery audit |
| --- | --- | --- |
| 0-6 | Tooling, modular skeleton, sources, observations, assets, telemetry, WebSocket | Done |
| 7-12 | Scenario runner v1, tracks, classifications, PostGIS geofences, alerts, audit | Done |
| 13-16 | Incidents, missions, commands, assessments | Done |
| 21 | Unit, integration (Testcontainers + PostGIS), and acceptance tests | Done |
| 17-20 | Authentication/RBAC, scenario runner v2, OpenAPI/gRPC contracts, OpenTelemetry | Done |
| F0 | Baseline verification, identity, scope isolation, and workspace separation | Complete; F0/M0 gate passed |
| P1 | Public source admission, provenance, reference data, and events | Complete; P1.1–P1.6 and M1 accepted |
| S0 | Simulation contracts and frozen inputs | Complete; S0.1–S0.4 and M2 accepted |
| P2 | Imagery intelligence and approved context export | Complete; P2.1-P2.4 and M3 accepted with synthetic spectral fixtures |
| P3 | Bounded moving sources, weather and reports | P3.1-P3.3 implemented/accepted; live feeds require approved access |
| S1 | Model pipeline and georeferenced 3D proof | S1.1–S1.3 complete; M4 accepted for reference/synthetic terrain |
| S2 | Native runtime, synthetic sensing and Exercise COP | Complete within the declared abstract profile; M5 accepted |
| S3 | Exercise operations, recording/replay and evaluation | S3.1–S3.3 complete; M6 accepted for the abstract native profile |
| S4 | Sustained simulation expansion | S4.1 reusable sensor-quality presets delivered; further increments remain backlog |

[F0.2–F0.4 implementation and rollout](docs/implementation/HISTORICAL_DELIVERY.md#f0-2-4) adds persisted human/service principals, scoped ownership and legacy quarantine, and application/query isolation. [F0.5 realtime and disclosure protection](docs/implementation/HISTORICAL_DELIVERY.md#f0-5) adds scoped version 2 delivery and random observation publication references. [F0.6 workspace separation and compatibility acceptance](docs/implementation/HISTORICAL_DELIVERY.md#f0-6) adds explicit Public Context/Exercise COP navigation, scoped caches and connections, and a real-backend browser gate. The implemented F0/M0 gate has passed.

[P1.1 source admission and durable evidence](docs/implementation/HISTORICAL_DELIVERY.md#p1-1) adds reviewed source/rights versions, service-bound bounded retrieval, persistent quotas/backoff/checkpoints, immutable evidence revisions, recoverable catalog publication, and provider coverage in Systems.

[P1.2–P1.4 public events, reference snapshots, and USGS](docs/implementation/HISTORICAL_DELIVERY.md#p1-2-4) adds point/area/moving semantics, revision/withdrawal-aware event projections and PostGIS alerts, a versioned Natural Earth/NOAA reference catalogue, and a native bounded USGS adapter. MapLibre shows reported events and selected reference versions with attribution and original/revision inspection. Apply migrations through **version 34** before starting the updated server. A disposable authorized live USGS smoke passed; permanent retrieval remains off by default. [S0.1–S0.3 simulation foundations](docs/implementation/HISTORICAL_DELIVERY.md#s0-1-3) now freeze scenario versions, seed/runtime/clock configuration and retained context packages, with renderer-independent WGS84/ECEF/ENU transforms and explicit fidelity. Restart uses the frozen definition; offline packages validate with `go run ./apps/emad-context <bundle.json>`. [S0.4 and P1.5–P1.6](docs/implementation/HISTORICAL_DELIVERY.md#s0-4-p1-5-6) now add authorized synthetic ingress with protected actor mappings, a retained-context headless fixture, NOAA-20 VIIRS thermal detections, exact CSV originals, and bounded public filters with separately labeled provider-effective and EMAD-publication history. M1/M2 acceptance gates pass. FIRMS retrieval remains off until a server map key, source binding and human admission are configured. [P2.1–P2.3 imagery intelligence](docs/implementation/HISTORICAL_DELIVERY.md#p2-1-3) adds private S3-compatible immutable artifacts, bounded native Copernicus catalogue discovery, an independent queued NDVI worker, local raster subset preparation, aligned comparison panels and versioned independent assessment review. Live catalogue acceptance passed; numerical/browser acceptance uses clearly labeled synthetic pixels, not an observed environmental-change claim. Configure the optional private object store and source-bound worker before processing subsets. P2.4 export and M3 are now accepted within the documented fixture scope; see the latest delivery above.

The first product milestone is covered end to end by `internal/integration/acceptance_test.go`: scenario source -> observation -> track -> geofence breach -> alert -> operator acknowledgement -> incident -> mission -> command -> audit, including a deterministic replay check. Phase 17–20 additions are covered by focused unit tests and contract generation checks.

## COP workspace selection

The persistent context bar selects Public Context or Exercise COP, then an authorized workspace/environment/run. `GET /api/v1/auth/me` offers existing ordinary COP resources from live grants; no input/truth scope is offered. Configured legacy operators initially have exercise access only. Trusted provisioning must grant public access explicitly. Starting a scenario selects its run; switching clears selection, replay, requests, cache, and realtime state. The context bar keeps source mode and time basis visible.

Run `task test:browser:integration` to build and verify the production UI against the real Go API and disposable PostGIS/S3 (Docker and installed Playwright Chromium required). CI runs this separately from the mocked presentation suites.

## Architecture at a glance

```text
Operator UI (React + Vite + MapLibre)
        │  REST + WebSocket
        ▼
Go C4ISR Core (modular monolith)
  sources · observations · assets · telemetry · tracks · classifications
  geospatial · alerts · incidents · missions · commands · assessments
  audit · realtime · scenarios · publicingest · publicevents · reference
        │  pgx + sqlc
        ▼
PostgreSQL + PostGIS
        ▲
        │  HTTP (application boundaries only)
Scenario Runner (external source)
```

Key rules preserved by the implementation:

- **Asset != Track** — controlled entities and observed entities are separate models.
- **Source != Observation != Track** — evidence is append-oriented and never overwritten by interpretation.
- **Provenance is mandatory** — tracks link to supporting observations; alerts carry rule provenance.
- **Time is first class** — `observed_at`, `received_at`, and `processed_at` are recorded separately.
- **PostGIS is authoritative** — containment, proximity, and distance decisions run in SQL (`ST_Contains`, `ST_DWithin`, `ST_Distance`).
- **Scenarios are deterministic** — fixed seed, fixed ordering, replayable via the same application boundaries real integrations will use.

The [v2 implementation plan](EMAD_IMPLEMENTATION_PLAN_v2.md) is retained as historical design and implementation history; **new work follows [EMAD_CURRENT_SCOPE.md](EMAD_CURRENT_SCOPE.md)** and the current preparation plan at [docs/implementation/ENVIRONMENT_FOUNDATION_PLAN_2026_11_11.md](docs/implementation/ENVIRONMENT_FOUNDATION_PLAN_2026_11_11.md). `ARCHITECTURE.md`, `PRODUCT.md`, and `IMPLEMENTATION_PLAN.md` retain earlier design and implementation history where compatible with the v2 direction. Material implementation decisions are recorded in `docs/decisions.md`.

## Repository layout

```text
apps/
  c4isr-server/        Go entrypoint
  operator-ui/         React + TypeScript operator interface
internal/
  app/                 composition root (wiring, router)
  platform/            config, logging, httpx, db, geo, ids, pgconv
  events/              in-process domain events + dispatcher
  <domain modules>/    sources, observations, assets, telemetry, tracks,
                       classifications, geospatial, alerts, incidents,
                       missions, commands, assessments, audit, realtime,
                       scenarios, operators
db/
  migrations/          goose SQL migrations
  queries/             sqlc query definitions
  sqlc.yaml
api/openapi/           browser contract (Phase 19)
api/proto/             machine contract (Phase 19)
scenarios/             deterministic YAML scenarios
deployments/           docker-compose for PostgreSQL/PostGIS
  staging/              production-shaped staging Compose stack
  observability/        Prometheus, Alertmanager, Loki, Alloy, Grafana config
scripts/ops/            secret bootstrap, migration, backup, restore tooling
deployments/backup/     scheduled backup reference units and procedure
```

## Prerequisites

- Go 1.25+, Python 3.12, Node 20+ and pnpm 10.
- Docker with Compose support for `up --wait`.
- Go Task and the Go tool bin directory on your PATH. Install Task with `go install github.com/go-task/task/v3/cmd/task@latest`.
- Git for Windows when running the migration and maintenance tasks from PowerShell. The Taskfile uses the Git Bash executable and supports `C4ISR_SHELL_BIN` for a custom install path.

After installing Task, run `task setup` once to install the Go tools and frontend dependencies. For Windows, add `$env:USERPROFILE\go\bin` to your PATH if PowerShell cannot find `task` or `goose`.

For app startup, follow [Start the map installation](#start-the-map-installation). The separate API and web tasks are available for component debugging. The deterministic scenario walkthrough belongs to the [historical recovery profile](docs/implementation/MAP_FIRST_OPERATIONS.md#historical-recovery-operations).

## Testing

```bash
task test:unit           # fast, no Docker
task test:integration    # Testcontainers + real PostGIS (Docker required)
```

Integration coverage includes observation provenance, duplicate/stale/out-of-order telemetry, PostGIS containment and proximity, alert dedupe and lifecycle, and the full first-milestone acceptance scenario with deterministic replay.

Frontend:

```bash
cd apps/operator-ui
pnpm build               # type-check + production build
pnpm dev                 # dev server on :5173 (proxies /api and /health)
```

## Configuration

Copy `.env.example` to `.env` for reference; the Taskfile exports `C4ISR_DATABASE_URL` automatically for `dev:api`.

| Variable | Default | Purpose |
| --- | --- | --- |
| `C4ISR_HTTP_ADDR` | `:8080` | HTTP listen address |
| `C4ISR_DATABASE_URL` | required | PostgreSQL/PostGIS DSN |
| `C4ISR_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `C4ISR_LOG_FORMAT` | `json` outside development/test | `text` or `json`; staging/production require `json` |
| `C4ISR_SCENARIOS_DIR` | `./scenarios` | Scenario YAML directory |
| `C4ISR_TELEMETRY_STALE_AFTER` | `5m` | Middle freshness band: an entity stops being current after this long without a report |
| `C4ISR_FRESHNESS_DELAYED_AFTER` | `1m` | An entity starts being flagged as aging after this long |
| `C4ISR_FRESHNESS_UNAVAILABLE_AFTER` | `30m` | Contact is treated as lost after this long even with no link-down signal |
| `C4ISR_ALLOWED_ORIGINS` | empty | Comma-separated CORS origins |
| `C4ISR_ENV` | `production` | `development`, `test`, `staging`, or `production` |
| `C4ISR_AUTH_REQUIRED` | `true` | Keep bearer authentication enabled outside local test harnesses |
| `C4ISR_AUTH_TOKENS` | required in production | Comma-separated `operator-id=token` pairs; development has an explicit local fallback |
| `C4ISR_DATABASE_URL_FILE` | empty | File-backed database DSN; mutually exclusive with `C4ISR_DATABASE_URL` |
| `C4ISR_AUTH_TOKENS_FILE` | empty | File-backed bearer-token mapping; mutually exclusive with `C4ISR_AUTH_TOKENS` |
| `C4ISR_VERSION` | `dev` | Build/release identifier included in structured logs |
| `VITE_API_TOKEN` | empty | Local-development fallback; staging/production injects the token at UI container startup |
| `VITE_MAP_STYLE_URL` | OSM raster | Optional MapLibre style URL for the UI |

## API overview

All routes are under `/api/v1`. Collections return `{"items": [...], "total": n}`; errors return `{"error": {"code", "message"}}`.

```text
GET/POST        /sources, /assets, /geofences, /incidents, /missions,
                /commands, /assessments, /operators
GET             /tracks, /alerts
GET/POST        /observations                    (POST ingests evidence)
POST            /telemetry                       (asset telemetry ingestion)
GET             /tracks/{id}/history, /tracks/{id}/classifications
GET             /assets/{id}/telemetry
POST            /alerts/{id}/acknowledge|resolve
POST            /incidents/{id}/status|relations
POST            /missions/{id}/status|assets
POST            /commands/{id}/transition
GET             /geospatial/geofences-containing|assets-within|nearest-assets
GET             /audit                            (filters: subject_type, subject_id, action, since)
GET             /scenarios, /scenarios/{name}, /scenarios/runs/{id}/events
POST            /scenarios/definitions/{name}/start, /scenarios/runs/{id}/...
GET             /picture                         (replay: ?at=<RFC3339> or ?live=true)
GET             /picture/window                  (evidence span the scrubber can traverse)
GET             /auth/me
GET             /realtime                         (WebSocket)
GET             /health
GET             /metrics                          (Prometheus text)
```

The server requires a bearer token by default. Local development uses `dev-operator-token`, which resolves to `operator-01`; the UI sends it through `VITE_API_TOKEN`. Replace both values in deployment configuration with secrets and registered operator ids. The legacy `X-Operator-ID` header is accepted only when authentication is explicitly disabled in development or test mode; the opt-in integration suite uses bearer authentication as well.

`GET /api/v1/auth/me` returns the authenticated operator and effective permissions. Roles are stored in the `operators` table: `operator`, `supervisor`, `administrator`, and `analyst`. Restricted writes return `403` and never trust a spoofed operator header.

### Realtime

`GET /api/v1/realtime` streams versioned envelopes:

```json
{ "id": "evt_...", "type": "track.updated", "version": 1, "occurredAt": "...", "data": { } }
```

The frontend treats events as cache invalidations for TanStack Query, not as a second state model. Payload mapping is explicit in `internal/realtime/mapper.go`; internal event structs are never the wire contract.

While the operator is scrubbed back in time, the frontend suspends event application entirely: overlaying present-day state onto a reconstructed picture would splice two moments into a picture that never existed.

### Freshness

Every asset and track carries a server-computed `freshness` verdict — `live`, `delayed`, `stale`, `unavailable`, or `unknown` — alongside a display label.

The server is the only place the thresholds are evaluated; the browser renders the verdict it is given. Re-deriving freshness client-side would let the interface disagree with the API about what is trustworthy, which is the failure this exists to prevent. Classification confidence is a separate question and stays with the classification and assessment modules.

A `unknown` verdict means the entity has never reported, which is distinct from `stale`: nothing is known, rather than something being known to be old. Freshness transitions are published on the realtime stream so a client already holding a rendered picture learns that an asset went silent without waiting for an unrelated refetch — including when contact is regained.

### Replay

`GET /api/v1/picture` reconstructs the operational picture at a point in time, reading `track_history` and `asset_telemetry` rather than the current-state projections, which have already moved on. An instant outside the recorded evidence window is clamped to the nearest real one and the resolved instant is returned in `at`, so a caller can always tell which moment it received.

Freshness in a reconstruction is derived against the *requested* instant, not the wall clock. Replaying "T+23s" must not paint a breach-age track in today's colours, or the operator is looking at a picture that mixes two moments.

Replay is read-only and carries the same `operational.read` permission as the live picture: scrubbing cannot reveal anything an operator could not already see.

### Events and areas

Not every observation describes a moving object. A wildfire, an earthquake, or a weather cell is an *area* at an instant: it has extent, it has no track, and forcing it through the track pipeline produces either a nonsense track or an alert that never fires because the correlation rules were written for points.

An observation may therefore carry an `area` — a `kind` (`hazard`, `weather`, `environmental`, `infrastructure`, `crowd`) and a polygon — instead of a `position`. It remains an observation: same table, same sources, same timestamps, same provenance. What changes is that area observations raise alerts directly instead of being correlated.

```jsonc
POST /api/v1/observations
{
  "sourceId": "fire-monitor",
  "type": "hazard.wildfire",
  "observedAt": "2026-10-13T12:00:00Z",
  "area": {
    "kind": "hazard",
    "polygon": [
      { "lat": 24.705, "lng": 46.685 },
      { "lat": 24.715, "lng": 46.685 },
      { "lat": 24.715, "lng": 46.695 },
      { "lat": 24.705, "lng": 46.695 }
    ]
  }
}
```

Four decisions are worth stating outright, because each of them would be easy to get wrong in a way that only shows up operationally:

- **An area is never correlated into a track.** A wildfire reported twice has a centroid displacement, and a correlation pipeline would turn that into a velocity with no physical meaning which then drives geofence containment and alerting. The track pipeline returns immediately for an area observation.
- **`ST_Intersects`, not `ST_Contains`.** A hazard burning *across* a zone boundary is the most operationally interesting case there is, and containment testing would silently drop it.
- **The alert links to the observation, not a track.** An area has no track, so pointing at one would be a fabricated relationship. The evidence trail from alert back to reporting source is preserved through `sourceReference.observationId` instead, and the measured overlap area is recorded so an operator can tell a graze from a headline event.
- **Repeats are suppressed while an alert stands unresolved.** Public hazard feeds re-report the same fire constantly; without this the alert centre becomes noise an operator learns to ignore, which is worse than having no alert.

`area` is mutually exclusive with both `position` and `trackHint`, and the database enforces it: an area always has a kind, and the two are present-or-absent together. Contradictory input is rejected at the edge rather than stored as evidence that means nothing.

`GET /api/v1/observations?area=true&area_kind=hazard` returns only footprints. The filter is applied in the database because point observations outnumber areas by orders of magnitude on a real feed — paging the newest N observations and filtering in the browser would return mostly radar pings and no hazards. An unrecognised `area_kind` is a `400`, not an empty list, since an empty list reads as "no hazards here".

A failed containment query leaves the operator with no alert and no visible reason, which is indistinguishable from calm. That silence is counted as `area_evaluation_failures_total` so it can be alerted on rather than discovered later.

## Scenarios

Scenarios live in `scenarios/*.yaml` and are executed by the scenario runner, which acts as an external information source: it registers sources/assets/geofences and emits observations and telemetry through the same application services used by real integrations. It never writes to the database directly.

```yaml
scenario: restricted-area-intrusion
seed: 1007
playback_speed: 1

sources:
  - id: simulator-01
    type: synthetic

assets:
  - id: patrol-01
    type: vehicle
    start: { lat: 24.7130, lng: 46.6820 }

tracks:
  - id: unknown-01
    start: { lat: 24.7175, lng: 46.6700 }

events:
  - after: 5s
    action: move
    track: unknown-01
    speed_mps: 25
    waypoints: [{ lat: 24.7175, lng: 46.6815 }]

  - after: 5s
    action: observe
    source: simulator-01
    track: unknown-01
    interval: 3s
    until: 150s
    jitter_m: 15
```

Supported actions: `observe` (with `interval`, `until`, `jitter_m`, `delay`, `duplicate`, `stale_for`, `quality`, `fault`, and explicit `position`), `move` (assets and tracks), `classify`, `assessment`, `connection`, `asset_status`, `incident`, `issue_command`, and `area`. `command_simulation` controls normal, rejected, failed, or timed-out commands issued against scenario assets. Run responses expose the last virtual-time action and the `/events` endpoint exposes the scheduled event cursor. Restarting a run preserves its scenario and seed while creating a new run id.

`area` emits an event with extent — a wildfire, a weather cell, a disruption — and takes `area_kind`, `polygon`, and the same `interval`/`until`/`jitter_m` controls as `observe`. Repeating one on an interval models a persistent hazard being re-reported, which is how a real public feed behaves and which the alert de-duplication has to survive. `scenarios/area-hazard-intrusion.yaml` exercises the path end to end.

Control endpoints:

```text
GET  /api/v1/scenarios                     list scenario files
GET  /api/v1/scenarios/{name}              parsed scenario
POST /api/v1/scenarios/definitions/{name}/start  {"speed": 1, "seed": 1007}
GET  /api/v1/scenarios/runs                recent runs
GET  /api/v1/scenarios/runs/{id}           run state, namespace, cursor + virtual time
POST /api/v1/scenarios/runs/{id}/pause|resume|stop
POST /api/v1/scenarios/runs/{id}/speed     {"speed": 5}
POST /api/v1/scenarios/runs/{id}/restart
```

Scenario resources use a run-qualified namespace (`{runId}__...`) and are retained after completion, failure, stop, or restart for audit and investigation. The `/events` endpoint remains inspectable after the in-memory engine is released.

Replay with the same seed produces the same observation sequence (positions included), which the acceptance test asserts exactly.

## Generated code and migrations

- Migrations are goose SQL files in `db/migrations/`.
- Queries are sqlc definitions in `db/queries/`; generated code lives in `internal/dbgen/`.

```bash
task db:migrate          # apply migrations after the migration preflight
task db:rollback         # guarded rollback; staging/production require approval
task db:status           # migration status
task db:backup           # custom-format dump plus SHA-256 checksum
task db:restore:test BACKUP=backups/example.dump
task sqlc                # regenerate internal/dbgen after changing SQL
```

Operational deployment, secret rotation, backup/restore, migration, log,
metric, and alert procedures are documented in [docs/operations.md](docs/operations.md).
The production-shaped local staging stack is started with `task staging:init`
and `task staging:up`; validate it first with `task ops:validate`.

Never edit `internal/dbgen/` by hand.

Browser and machine contracts:

- `api/openapi/openapi.yaml` is the REST/WebSocket contract.
- `apps/operator-ui/src/lib/openapi.generated.ts` is generated with `pnpm --dir apps/operator-ui contracts:generate`.
- `api/proto/c4isr/v1/ingestion.proto` defines versioned observation and telemetry ingestion services; `buf lint api/proto` is enforced in CI.

The server wraps HTTP with OpenTelemetry HTTP instrumentation and emits JSON
structured request logs in staging/production containing service, environment,
version, request, trace, operator, and role identifiers. `/metrics` exposes
request duration/counts, response bytes, database health, domain event counts,
ingestion aggregates, and authentication failures in Prometheus text format.

## Local development model

The host runs the Go API on port 8081, Vite on port 5173 and the configured workers under `task dev`. Docker Compose runs PostgreSQL/PostGIS on port 5433 and Garage object storage on port 3900. Setup and development startup wait for PostgreSQL before the API starts. See the [local operations guide](docs/implementation/MAP_FIRST_OPERATIONS.md#local-operation).

## Deferred by design

Kafka, NATS, Redis, Kubernetes, TimescaleDB, Elasticsearch, MQTT, MAVLink, Rust edge agents, dedicated telemetry microservices, API gateways, and full sensor fusion are intentionally not part of this implementation. Each requires a concrete requirement first.


## S1.2-S1.3 / S2.1 delivery update

The first Simulation Studio viewport uses measured Three.js rendering over immutable georeferenced authoring scenes. A separate native Go executable owns abstract actor kinematics, virtual time and authenticated runtime lifecycle. Authoring, latest sealed runtime truth and ordinary operator perception remain separate. Migration 35 is required. M4 is accepted within the documented reference/synthetic-terrain visual-only scope; native sensing and Exercise COP integration remain S2.2-S2.3. See [engine comparison and decision](docs/implementation/HISTORICAL_DELIVERY.md#s1-2) and [implementation, acceptance and rollout](docs/implementation/HISTORICAL_DELIVERY.md#s1-3-s2-1).


## S2.2-S2.3 / S3.1 delivery update

Native scenes now freeze abstract synthetic observer profiles and explicit controlled actors. The independent runtime publishes lease-bound private batches through a durable scoped gateway, and the existing observation/track/PostGIS/telemetry pipeline supplies the Exercise COP without hidden actor identities. Exercise operations reuse incident-linked missions, commands, operators and audit for virtual HOLD/RESUME_ROUTE actions with bounded deadlines, ACK/completion/rejection/failure/timeout, cancellation and idempotent retry. Migration **36** is required after 35. M5 was accepted for this abstract profile. The subsequent delivery below also completes S3.2 recording/replay and M6 within its declared scope. See [implementation, acceptance and rollout](docs/implementation/HISTORICAL_DELIVERY.md#s2-2-s2-3-s3-1).

## S3.2-S3.3 / S4.1 delivery update

The original F0–S3 task backlog is complete within its recorded acceptance scopes. Native exercises now retain separate operator/truth journals and private content-addressed archives with selected-state checkpoints. Recorded output can be sought by virtual time and sequence without reissuing commands or consulting current COP state. Audited evaluators receive defined perception/workflow metrics; numeric operator summaries require an explicit human release. Studio also saves and pins reusable abstract sensor-quality presets. Apply migrations **37 then 38**, rebuild/admit the runtime binary, and run `apps/emad-recordings` with a separate environment-bound service credential. M6 is accepted for the documented abstract/reference/synthetic-terrain profile; further S4 expansion remains backlog. [Implementation, limits, verification and rollout](docs/implementation/HISTORICAL_DELIVERY.md#s3-2-s3-3-s4-1).


## Implementation review closeout

The [review-readiness closeout](docs/implementation/HISTORICAL_RECOVERY.md#review-readiness) completes recording-worker recovery, durable retry visibility and evaluation v2 mean operands. Migrations 1–38 are preserved; no additional migration is needed. `task build:all` builds the Go API, tools, workers and runtime. Rebuild/restart the API, UI and recording worker before reviewing the updated behavior.

The [full implementation-plan review](docs/implementation/HISTORICAL_RECOVERY.md#plan-review) traces all 32 tasks and S4.1 to their implementation, records the authorization/replay/reproducibility and UI fixes, and links the final verification evidence.

[Commit readiness](docs/implementation/HISTORICAL_RECOVERY.md#precommit-readiness) records clean-export verification, private build-context exclusions and detached-checkout CI compatibility.


### Map-first T13–T15

Saved catalog reads now page geography, scenes, captures, jobs, findings and investigations with exact counts and opaque continuation. Aggregate maps expose their original individual revisions. Role-led navigation, Arabic input/local RTL assets, narrow-screen recovery and exact imagery footprint/pair checks share the map workflow. Original imagery intent keys and saved case revision links survive retries.

This increment requires `00051_catalog_and_imagery_workflows.sql`. Read the [implementation and verification record](docs/implementation/MAP_FIRST_DELIVERY_LOG.md#t13-t15) and [upgrade guidance](docs/implementation/MAP_FIRST_OPERATIONS.md#map-profile). Fresh UAE/France pixels and offline comparisons are isolated evidence; independent users, production admission and full release qualification remain separate gates.
