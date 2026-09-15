# C4ISR Systems

A C4ISR-oriented operational software platform. The first implementation milestone is a working **C2 + Operational Awareness** core: sources and observations become a traceable operational picture of assets and tracks, PostGIS evaluates geographic rules, and operators can acknowledge alerts, open incidents, assign assets, and issue auditable commands.

The system is designed C4ISR-aware but C2-first: the information model (sources, observations, provenance, uncertainty, assessments) is in place from the beginning so future ISR, intelligence, AI, and hardware integrations do not require a redesign.

## Status

| Phase | Scope | State |
| --- | --- | --- |
| 0-6 | Tooling, modular skeleton, sources, observations, assets, telemetry, WebSocket | Done |
| 7-12 | Scenario runner v1, tracks, classifications, PostGIS geofences, alerts, audit | Done |
| 13-16 | Incidents, missions, commands, assessments | Done |
| 21 | Unit, integration (Testcontainers + PostGIS), and acceptance tests | Done |
| 17-20 | Authentication/RBAC, scenario runner v2, OpenAPI/gRPC contracts, OpenTelemetry | Next |
| 22-26 | ISR expansion, intelligence, AI assistance, external integrations, edge/hardware | Later |

The first product milestone is covered end to end by `internal/integration/acceptance_test.go`: scenario source -> observation -> track -> geofence breach -> alert -> operator acknowledgement -> incident -> mission -> command -> audit, including a deterministic replay check.

## Architecture at a glance

```text
Operator UI (React + Vite + MapLibre)
        │  REST + WebSocket
        ▼
Go C4ISR Core (modular monolith)
  sources · observations · assets · telemetry · tracks · classifications
  geospatial · alerts · incidents · missions · commands · assessments
  audit · realtime · scenarios
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

See `ARCHITECTURE.md`, `PRODUCT.md`, and `IMPLEMENTATION_PLAN.md` for the full design. Material implementation decisions are recorded in `docs/decisions.md`.

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
```

## Prerequisites

- Go 1.24+
- Node 20+ and pnpm 10+
- Docker (Compose v2)
- [Task](https://taskfile.dev): `go install github.com/go-task/task/v3/cmd/task@latest`

Make sure Go's bin directory is on your PATH (the same directory `go install` writes to):

```bash
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

Alternative on macOS: `brew install go-task`.

## Quick start

```bash
# 1. One-time setup: installs sqlc + goose and frontend dependencies
task setup

# 2. Start PostgreSQL/PostGIS in Docker
task dev:db

# 3. Apply migrations
task db:migrate

# 4. Run the Go server (terminal A)
task dev:api

# 5. Run the operator UI (terminal B)
task dev:web

# 6. Start the deterministic acceptance scenario (terminal C)
task scenario:run NAME=restricted-area-intrusion SPEED=1
```

Open http://localhost:5173. The scenario registers `simulator-01`, moves `unknown-01` toward Restricted Zone A, breaches the geofence (~23s), and raises an alert. Patrol-01 is also on the map, moving independently. Use `SPEED=10` to run the scenario ten times faster.

### The first acceptance flow

1. Watch the alert appear in the alert center (delivered over WebSocket).
2. Open the track and inspect its supporting observations (source + observed times).
3. Acknowledge the alert.
4. Create an incident, attach the patrol asset.
5. Create a mission, assign `patrol-01`.
6. Issue a command from the mission/incident view; the scenario acknowledges and completes it.
7. Inspect the timeline; every step is reconstructed from the audit log.

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
| `C4ISR_LOG_FORMAT` | `text` | `text` or `json` |
| `C4ISR_SCENARIOS_DIR` | `./scenarios` | Scenario YAML directory |
| `C4ISR_ALLOWED_ORIGINS` | empty | Comma-separated CORS origins |
| `VITE_MAP_STYLE_URL` | OSM raster | Optional MapLibre style URL for the UI |

## API overview

All routes are under `/api/v1`. Collections return `{"items": [...], "total": n}`; errors return `{"error": {"code", "message"}}`.

```text
GET/POST        /sources, /assets, /tracks, /geofences, /alerts, /incidents,
                /missions, /commands, /assessments, /operators
GET/POST        /observations                    (POST ingests evidence)
POST            /telemetry                       (asset telemetry ingestion)
GET             /tracks/{id}/history, /tracks/{id}/classifications
GET             /assets/{id}/telemetry
POST            /alerts/{id}/acknowledge|resolve
POST            /incidents/{id}/status|relations
POST            /missions/{id}/status|assets
POST            /commands/{id}/transition
POST            /tracks/{id}/classifications
GET             /geospatial/geofences-containing|assets-within|nearest-assets
GET             /audit                            (filters: subject_type, subject_id, action, since)
GET/POST        /scenarios, /scenarios/{name}/start, /scenarios/runs/{id}/...
GET             /realtime                         (WebSocket)
GET             /health
```

Operator actions are attributed with the `X-Operator-ID` header (default `operator-01`) until Phase 17 adds authentication and RBAC.

### Realtime

`GET /api/v1/realtime` streams versioned envelopes:

```json
{ "id": "evt_...", "type": "track.updated", "version": 1, "occurredAt": "...", "data": { } }
```

The frontend treats events as cache invalidations for TanStack Query, not as a second state model. Payload mapping is explicit in `internal/realtime/mapper.go`; internal event structs are never the wire contract.

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

Supported actions: `observe` (with `interval`, `until`, `jitter_m`, `delay`, `duplicate`, `stale_for`), `move` (assets and tracks), `classify`, `connection`, `issue_command`. `command_simulation` controls how the runner acknowledges commands issued against scenario assets.

Control endpoints:

```text
GET  /api/v1/scenarios                     list scenario files
GET  /api/v1/scenarios/{name}              parsed scenario
POST /api/v1/scenarios/{name}/start        {"speed": 1, "seed": 1007}
GET  /api/v1/scenarios/runs                recent runs
GET  /api/v1/scenarios/runs/{id}           run state + virtual time
POST /api/v1/scenarios/runs/{id}/pause|resume|stop
POST /api/v1/scenarios/runs/{id}/speed     {"speed": 5}
```

Replay with the same seed produces the same observation sequence (positions included), which the acceptance test asserts exactly.

## Generated code and migrations

- Migrations are goose SQL files in `db/migrations/`.
- Queries are sqlc definitions in `db/queries/`; generated code lives in `internal/dbgen/`.

```bash
task db:migrate          # apply migrations
task db:rollback         # roll back the latest migration
task db:status           # migration status
task sqlc                # regenerate internal/dbgen after changing SQL
```

Never edit `internal/dbgen/` by hand.

## Local development model

```text
Host
├── Go server      (task dev:api)
└── Vite frontend  (task dev:web)

Docker Compose
└── PostgreSQL/PostGIS
```

The Docker Compose file uses `imresamu/postgis:17-3.5`, a community multi-arch build of the official PostGIS image, because the official image does not publish `linux/arm64` manifests yet.

## Deferred by design

Kafka, NATS, Redis, Kubernetes, TimescaleDB, Elasticsearch, MQTT, MAVLink, Rust edge agents, dedicated telemetry microservices, API gateways, and full sensor fusion are intentionally not part of this implementation. Each requires a concrete requirement first.
