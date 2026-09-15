# ARCHITECTURE.md

# C4ISR-Oriented Platform Architecture

## 1. Purpose

This document defines the software architecture for a C4ISR-oriented operational platform whose first implementation milestone is a C2 + Operational Awareness system.

The platform is designed from the beginning around the larger information flow required by C4ISR systems:

```text
Sources
   ↓
Observations
   ↓
Detections
   ↓
Correlation / Fusion
   ↓
Tracks
   ↓
Assessments / Intelligence
   ↓
Operational Picture
   ↓
C2 Decision
   ↓
Mission / Task / Command
```

The first version will implement only a subset of these capabilities, but the architecture must avoid decisions that would prevent later ISR and Intelligence capabilities from being added cleanly.

The platform should optimize for:

- explicit domain boundaries
- traceability
- provenance
- real-time operational state
- geospatial correctness
- deterministic behavior
- uncertainty-aware information modeling
- clean integration boundaries
- future service extraction where justified

It should not prematurely optimize for:

- microservices
- Kubernetes
- high-scale distributed ingestion
- specialized hardware protocols
- full sensor fusion
- advanced AI pipelines
- military communication infrastructure

---

# 2. Architectural Strategy

The architectural strategy is:

> Design for C4ISR, implement C2 first.

This means:

```text
Architecture vision: C4ISR
Implementation scope: C2 + Operational Awareness
```

The product should not be modeled as a drone application, telemetry dashboard, or map application.

It should be modeled as an operational information platform.

---

# 3. Core Architectural Principles

## 3.1 Evidence Before Interpretation

The system must distinguish between:

- what a source observed
- what the system detected
- what the platform believes
- what an analyst assessed
- what the operator decided

Example:

```text
Camera-01
    ↓
Observation
    ↓
Vehicle Detection
    ↓
Track-42
    ↓
Assessment: likely same vehicle as Track-17
    ↓
Incident
```

Do not collapse these layers into a single entity.

---

## 3.2 Provenance Is a First-Class Concern

Every important derived fact should be traceable back to its origin.

The architecture should support questions such as:

- Which source produced this observation?
- Which observations support this track?
- Which rule generated this alert?
- Which model generated this assessment?
- Which operator changed this classification?
- When was the information observed?
- When was it received?
- When was it processed?

---

## 3.3 Uncertainty Must Be Representable

Operational data is not always absolute truth.

The platform should be able to represent:

- confidence
- competing classifications
- uncertain identity
- conflicting observations
- source quality
- assessment confidence

The first implementation can keep these features simple, but the model must not assume every field is authoritative.

---

## 3.4 Operational State Is a Projection

The Common Operational Picture is a current best interpretation of the available information.

It is not the raw evidence.

The platform should preserve:

```text
Raw Observations
        │
        ▼
Derived Tracks / Assessments
        │
        ▼
Operational Picture
```

---

## 3.5 Transport Is Not the Domain

REST, WebSocket, gRPC, and future MQTT are adapters.

The domain must not depend on transport-specific types.

```text
HTTP / WebSocket / gRPC
          │
          ▼
   Application Layer
          │
          ▼
      Domain Layer
          │
          ▼
 Infrastructure Layer
```

---

## 3.6 Modular Monolith First

The initial application is one Go deployment with explicit internal modules.

Reasons:

- simpler transactions
- easier debugging
- easier iteration
- less operational overhead
- domain boundaries can be discovered before service extraction

Microservices should only appear when justified by:

- independent scaling
- failure isolation
- language/runtime requirements
- GPU workloads
- device connectivity requirements
- security isolation
- different deployment cadence
- separate team ownership

---

# 4. Technology Stack

## Backend

- Go
- `net/http`
- Chi
- pgx
- sqlc
- PostgreSQL
- PostGIS
- coder/websocket
- gRPC
- Protocol Buffers
- Buf
- OpenAPI
- `log/slog`
- OpenTelemetry

## Frontend

- React
- TypeScript
- Vite
- TanStack Router
- TanStack Query
- TanStack Form
- TanStack Table
- Zustand
- shadcn/ui
- Base UI
- Tailwind CSS
- MapLibre GL JS
- Turf.js for non-authoritative client-side geometry

## Testing

- Go standard testing
- Testcontainers
- Vitest
- Playwright

## Local Development

- Docker
- Docker Compose
- PostgreSQL/PostGIS
- Go backend on host
- Vite frontend on host

## Repository Tooling

- pnpm
- Taskfile
- Buf
- sqlc
- Goose or equivalent migration tooling

---

# 5. High-Level Architecture

```text
┌──────────────────────────────────────────────────────┐
│                  Operator Interface                  │
│ React + TypeScript + Vite                            │
│ TanStack Router / Query / Form / Table               │
│ Zustand                                              │
│ MapLibre                                             │
└────────────────────────┬─────────────────────────────┘
                         │
                 REST + WebSocket
                         │
┌────────────────────────▼─────────────────────────────┐
│                  Go C4ISR Core                      │
│                                                      │
│ C2                                                   │
│ ├── Assets                                           │
│ ├── Missions                                         │
│ ├── Commands                                         │
│ ├── Incidents                                        │
│ └── Operators                                        │
│                                                      │
│ Operational Awareness                                │
│ ├── Tracks                                           │
│ ├── Alerts                                           │
│ ├── Events                                           │
│ ├── Geospatial                                       │
│ └── Operational Picture                              │
│                                                      │
│ ISR Foundation                                       │
│ ├── Sources                                          │
│ ├── Observations                                     │
│ └── Detections                                       │
│                                                      │
│ Intelligence Foundation                              │
│ ├── Classifications                                  │
│ ├── Confidence                                       │
│ ├── Provenance                                       │
│ └── Assessments                                      │
└────────────────────────┬─────────────────────────────┘
                         │
                    pgx + sqlc
                         │
┌────────────────────────▼─────────────────────────────┐
│              PostgreSQL + PostGIS                   │
└──────────────────────────────────────────────────────┘
```

Future external components:

```text
Scenario Runner
Simulator
Telemetry Gateway
AI Service
Media Service
Rust Edge Agent
Sensors
External C2 / C4ISR Systems
```

These should integrate through stable contracts.

---

# 6. Repository Structure

```text
c4isr-platform/
├── apps/
│   ├── c4isr-server/
│   │   └── main.go
│   └── operator-ui/
│       ├── src/
│       ├── public/
│       └── package.json
│
├── internal/
│   ├── sources/
│   ├── observations/
│   ├── detections/
│   ├── assets/
│   ├── tracks/
│   ├── classifications/
│   ├── assessments/
│   ├── telemetry/
│   ├── geospatial/
│   ├── alerts/
│   ├── incidents/
│   ├── missions/
│   ├── commands/
│   ├── operators/
│   ├── realtime/
│   ├── audit/
│   └── scenarios/
│
├── api/
│   ├── openapi/
│   └── proto/
│
├── db/
│   ├── migrations/
│   ├── queries/
│   └── sqlc.yaml
│
├── scenarios/
│   ├── normal-patrol.yaml
│   ├── geofence-breach.yaml
│   ├── conflicting-observations.yaml
│   └── multi-source-track.yaml
│
├── deployments/
│   └── docker-compose.yml
│
├── Taskfile.yml
├── buf.yaml
├── buf.gen.yaml
├── go.mod
├── pnpm-workspace.yaml
└── README.md
```

---

# 7. Layering Model

Each significant module should generally follow:

```text
Domain
  ↓
Application
  ↓
Infrastructure
  ↓
Transport
```

But avoid unnecessary ceremony.

For a simple module:

```text
internal/assets/
├── domain.go
├── service.go
├── repository.go
├── events.go
├── transport_http.go
└── postgres_repository.go
```

For a larger module:

```text
internal/assessments/
├── domain/
├── application/
├── infrastructure/
└── transport/
```

Rules:

- HTTP handlers contain no business logic
- Postgres repositories contain no operator workflow logic
- domain objects do not import Chi or pgx
- WebSocket transport consumes application events
- gRPC transports adapt external requests into domain/application commands

---

# 8. C2 Modules

## 8.1 Assets

Owns controlled or coordinated entities.

Examples:

- UAV
- vehicle
- team
- vessel
- robot
- camera platform
- radar platform

Responsibilities:

- registration
- capabilities
- availability
- current state
- health
- connection state
- mission assignment

Assets and tracks remain distinct.

---

## 8.2 Missions

Represents operational objectives.

Responsibilities:

- mission lifecycle
- objective
- geographic scope
- priority
- assigned assets
- tasks
- related incident
- status

---

## 8.3 Commands

Represents explicit instructions.

Lifecycle:

```text
CREATED
QUEUED
SENT
ACKNOWLEDGED
COMPLETED
REJECTED
FAILED
TIMED_OUT
CANCELLED
```

Commands must record:

- actor
- target
- payload
- timestamps
- acknowledgment
- failure reason
- correlation ID

---

## 8.4 Incidents

Represents a coordinated operational situation.

Relationships may include:

- alerts
- assets
- tracks
- observations
- assessments
- missions
- commands
- operators

Suggested states:

```text
OPEN
ACKNOWLEDGED
INVESTIGATING
RESPONDING
RESOLVED
CLOSED
```

---

## 8.5 Operators

Owns:

- identity
- roles
- permissions
- assignments
- authorization context

---

# 9. Operational Awareness Modules

## 9.1 Tracks

A track represents a real-world entity inferred or observed over time.

It may be supported by many observations.

```text
Observation A ─┐
Observation B ─┼──► Track-42
Observation C ─┘
```

Track responsibilities:

- current position
- movement
- lifecycle
- first seen
- last seen
- supporting observations
- current classification
- confidence
- source history

A track should not directly own raw observation records.

---

## 9.2 Alerts

Alerts represent conditions requiring attention.

Examples:

- geofence breach
- stale track
- lost asset connection
- conflicting observations
- sensor offline
- unusual movement

Alert responsibilities:

- state
- severity
- source
- evidence
- acknowledgment
- resolution
- incident association

---

## 9.3 Events

Operational events represent timestamped facts.

Examples:

```text
ObservationReceived
TrackCreated
TrackUpdated
AssessmentCreated
GeofenceBreached
AlertCreated
IncidentOpened
CommandIssued
```

Events support:

- real-time notifications
- auditing
- replay
- downstream processing

---

## 9.4 Operational Picture

The operational picture is a read model combining:

- assets
- tracks
- alerts
- incidents
- missions
- sensors
- operational areas
- active assessments

It should be optimized for operator consumption.

It is not the persistence model for raw observations.

---

# 10. ISR Foundation Modules

## 10.1 Sources

A source is the origin of information.

Examples:

- simulator
- sensor
- camera
- radar
- external API
- human report
- AI subsystem

Fields may include:

```text
id
name
type
status
capabilities
trust/quality metadata
created_at
```

The first implementation only needs minimal metadata.

---

## 10.2 Observations

An observation represents what a source reported.

Core fields:

```text
id
source_id
observation_type
observed_at
received_at
processed_at?
position?
payload
quality?
```

Important requirements:

- preserve original source identity
- preserve observation time
- preserve received time
- allow late arrival
- allow duplicates to be recognized
- preserve raw metadata where useful

Observations should be append-oriented.

---

## 10.3 Detections

A detection is an interpreted observation identifying something of interest.

Example:

```text
Observation:
camera frame at 10:00:01

Detection:
vehicle at coordinates X/Y, confidence 0.88
```

Detections may originate from:

- sensor logic
- deterministic algorithm
- AI
- operator

The first milestone can keep detections minimal.

---

# 11. Intelligence Foundation Modules

## 11.1 Classifications

A classification is a hypothesis about an entity.

Example:

```text
Track-42
classification: vehicle
confidence: 0.91
```

Future support:

- multiple competing classifications
- classification history
- source attribution
- AI-generated classification
- operator override

---

## 11.2 Confidence

Confidence must be representable where interpretation is uncertain.

Do not force confidence onto every deterministic fact.

Use it where meaningful:

- classification
- correlation
- identity
- assessment
- anomaly

---

## 11.3 Provenance

Derived information should reference supporting evidence.

Example:

```text
Assessment-8
  evidence:
    observation-101
    observation-108
    track-42
  method:
    analyst
```

or:

```text
Classification
  source:
    ai-model-x
  based_on:
    detection-55
```

---

## 11.4 Assessments

An assessment is an analytical conclusion.

Possible fields:

```text
id
type
subject_type
subject_id
conclusion
confidence
method
created_by
created_at
```

Method:

```text
OPERATOR
RULE
ALGORITHM
AI
```

Assessments should not silently mutate core state.

They should be explicit analytical objects.

---

# 12. Time Model

Operational systems must distinguish several timestamps.

Important fields:

```text
observed_at
received_at
processed_at
effective_at
created_at
updated_at
```

Example:

```text
Object observed:   10:00:01
Message received:  10:00:04
Processed:         10:00:05
```

This distinction is required for:

- delayed telemetry
- disconnected sources
- out-of-order observations
- replay
- analysis
- stale-state detection

---

# 13. Data Architecture

## 13.1 PostgreSQL + PostGIS

Primary source of truth for:

- sources
- observations
- detections
- assets
- tracks
- alerts
- incidents
- missions
- commands
- classifications
- assessments
- operators
- audit
- scenario metadata

PostGIS owns authoritative spatial behavior.

---

## 13.2 Current State vs Evidence vs History

The architecture should distinguish:

```text
Evidence
├── observations
└── detections

Current Operational State
├── asset_state
└── track_state

Interpretation
├── classifications
└── assessments

History
├── telemetry
├── track history
├── audit
└── events
```

Do not reconstruct every current-state query from raw history.

---

## 13.3 Example Tables

Potential initial tables:

```text
sources
observations
detections

assets
asset_state
asset_telemetry

tracks
track_state
track_observations

classifications
assessments
assessment_evidence

geofences

alerts
incidents
incident_alerts
incident_tracks
incident_assets

missions
mission_assets
commands

operators
roles
permissions

audit_events
```

Exact schema should evolve with implementation.

---

# 14. Geospatial Architecture

Authoritative GIS behavior belongs in PostGIS.

Initial spatial data:

```text
asset_state.position        geography(Point, 4326)
track_state.position        geography(Point, 4326)
observations.position       geography(Point, 4326)
geofences.geometry          geography(Polygon, 4326)
routes.geometry             geography(LineString, 4326)
```

Operations:

```text
ST_Contains
ST_Intersects
ST_DWithin
ST_Distance
```

Map UI may use Turf.js for preview only.

---

# 15. Browser API Architecture

Use REST + OpenAPI.

Examples:

```text
GET    /api/v1/sources
POST   /api/v1/sources

GET    /api/v1/observations
GET    /api/v1/observations/{id}

GET    /api/v1/assets
POST   /api/v1/assets

GET    /api/v1/tracks
GET    /api/v1/tracks/{id}

GET    /api/v1/alerts
POST   /api/v1/alerts/{id}/acknowledge

GET    /api/v1/incidents
POST   /api/v1/incidents

GET    /api/v1/assessments
POST   /api/v1/assessments

POST   /api/v1/commands
```

Generate TypeScript client/types from OpenAPI.

Do not manually duplicate API models.

---

# 16. Realtime Architecture

Use WebSocket for operator updates.

Endpoint:

```text
GET /api/v1/realtime
```

Event envelope:

```json
{
  "id": "evt-123",
  "type": "track.updated",
  "version": 1,
  "occurredAt": "2026-09-13T19:00:00Z",
  "data": {}
}
```

Initial event types:

```text
source.updated
observation.received

asset.created
asset.updated
asset.position.updated
asset.connection.changed

track.created
track.updated
track.closed

classification.updated
assessment.created

alert.created
alert.acknowledged
alert.resolved

incident.created
incident.updated

mission.updated
command.status.changed
```

---

# 17. gRPC Architecture

gRPC is for machine-to-machine integration.

Potential future interfaces:

```text
ObservationIngestionService
TelemetryIngestionService
AssessmentService
ExternalIntegrationService
```

Potential consumers/providers:

- telemetry gateway
- simulator
- AI service
- external C2 systems
- media service

Use:

- Protocol Buffers
- Buf lint
- Buf breaking-change checks
- generated Go types

---

# 18. Frontend Architecture

## 18.1 Core

```text
React
TypeScript
Vite
TanStack Router
TanStack Query
Zustand
MapLibre
```

No Next.js.

No TanStack Start initially.

---

## 18.2 State Ownership

### TanStack Query

Server state:

- assets
- tracks
- observations
- alerts
- incidents
- assessments
- missions

### Zustand

Local UI state:

- selected map entity
- active panel
- temporary geometry
- map mode
- unsaved UI preferences

### TanStack Router

URL/shareable state:

- selected incident
- current workspace
- active filters
- map/search parameters

### WebSocket

Realtime change notifications.

WebSocket events should update or invalidate TanStack Query state.

Do not mirror the backend model into Zustand.

---

# 19. Map Architecture

Initial:

```text
PostGIS
   ↓
Go REST
   ↓
GeoJSON
   ↓
MapLibre
```

Later:

```text
PostGIS
   ↓
Vector Tiles
   ↓
MapLibre
```

Potential later addition:

- deck.gl for very dense visualization

---

# 20. Scenario Engine Architecture

The scenario engine acts as a source of operational information.

Example:

```yaml
scenario: multi-source-intrusion
seed: 1007

sources:
  - id: radar-sim
    type: synthetic-radar

  - id: camera-sim
    type: synthetic-camera

tracks:
  - id: unknown-01

events:
  - after: 5s
    action: observe
    source: radar-sim
    target: unknown-01

  - after: 8s
    action: observe
    source: camera-sim
    target: unknown-01

  - after: 20s
    action: move
    target: unknown-01
    toward: restricted-zone-a
```

Scenario engine must support:

- deterministic replay
- fixed seed
- exact ordering
- playback speed
- pause/resume
- failure injection
- conflicting observations
- stale data
- command acknowledgment

---

# 21. AI Architecture

AI remains outside critical deterministic state transitions.

Preferred flow:

```text
Observations / Tracks
        ↓
AI Service
        ↓
Assessment / Classification Suggestion
        ↓
C4ISR Core
        ↓
Operator
```

AI output should include:

- model/source
- generated_at
- confidence where meaningful
- evidence references
- correlation ID

AI should not directly overwrite track identity or incident state without explicit application logic.

---

# 22. Internal Event Architecture

Initial implementation:

```text
in-process events
```

Examples:

```text
ObservationReceived
DetectionCreated
TrackCreated
TrackUpdated
ClassificationChanged
AssessmentCreated
GeofenceBreached
AlertCreated
IncidentCreated
CommandIssued
CommandAcknowledged
```

Future:

```text
NATS / Kafka / another broker
```

Only if required.

---

# 23. Authentication and Authorization

Initial roles:

```text
operator
supervisor
administrator
```

Potential future role:

```text
analyst
```

Example permissions:

```text
sources.read
observations.read
assets.read
tracks.read
alerts.read
alerts.acknowledge
incidents.create
incidents.update
assessments.create
missions.create
commands.issue
admin.manage
```

Authorization belongs in application logic, not only HTTP middleware.

---

# 24. Observability

Use:

- `log/slog`
- OpenTelemetry traces
- OpenTelemetry metrics

Important measurements:

- observations/sec
- telemetry/sec
- observation processing latency
- track update latency
- geofence evaluation latency
- active WebSocket clients
- alerts generated
- command acknowledgment latency
- stale tracks
- database latency
- scenario status

Future intelligence metrics:

- assessments generated
- conflicting evidence count
- AI assessment latency
- correlation decisions

---

# 25. Testing Strategy

## Unit

Test:

- state transitions
- validation
- classifications
- alert logic
- command lifecycle
- incident transitions

## Integration

Use Testcontainers with real PostGIS.

Test:

- repositories
- migrations
- spatial queries
- observation persistence
- track updates
- transactions

## Scenario Tests

Example:

```text
Given:
- two sources
- one unknown entity
- restricted zone

When:
- both sources observe entity
- entity enters zone

Then:
- observations preserved
- track updated
- provenance retained
- alert created
- realtime event emitted
- audit recorded
```

## Frontend

Vitest:

- components
- stores
- utilities

Playwright:

- operational workflows
- observation drill-down
- alert acknowledgment
- incident creation
- asset assignment

---

# 26. Local Development

Normal workflow:

```text
Host
├── Go C4ISR server
└── React/Vite operator UI

Docker Compose
└── PostgreSQL/PostGIS
```

Optional later services:

```text
OpenTelemetry Collector
AI service
telemetry gateway
```

---

# 27. Future Service Extraction

## Telemetry / Observation Gateway

Extract when:

- device protocols multiply
- ingestion rate is high
- buffering becomes necessary
- source authentication becomes complex
- device connections need independent scaling

## AI / Intelligence Service

Extract when:

- Python/ML runtime is needed
- GPU infrastructure is required
- inference workloads need independent scaling
- AI failures should be isolated

## Media Service

Extract when:

- video ingestion
- transcoding
- streaming
- object storage
- media analysis

become substantial.

---

# 28. Explicitly Deferred Technologies

Do not add initially:

- Kafka
- NATS
- Redis
- Kubernetes
- service mesh
- TimescaleDB
- Elasticsearch
- MQTT
- Rust edge runtime
- MAVLink
- dedicated API gateway
- full sensor fusion engine

Each requires a concrete reason.

---

# 29. First Architecture Milestone

The architecture is proven when this flow works:

```text
Scenario Source
      ↓
Observation
      ↓
Track
      ↓
PostGIS Geofence Evaluation
      ↓
Alert
      ↓
WebSocket
      ↓
Operator UI
      ↓
Incident
      ↓
Asset Assignment
      ↓
Command
      ↓
Audit
```

With these guarantees:

- observation provenance preserved
- track is distinct from evidence
- current state and history separated
- real PostGIS evaluation used
- realtime update delivered
- operator action auditable
- scenario replay deterministic
- no hardware dependency
- no distributed infrastructure dependency

---

# 30. Architecture Decision Summary

```text
Architectural vision      C4ISR
Implementation focus      C2 + Operational Awareness
Architecture              Modular monolith
Backend                   Go
HTTP routing              net/http + Chi
Database                  PostgreSQL + PostGIS
Database access           pgx + sqlc
Browser API               REST + OpenAPI
Realtime                  WebSocket
Machine API               gRPC + Protobuf
Proto tooling             Buf
Frontend                  React + TypeScript + Vite
Routing                   TanStack Router
Server state              TanStack Query
Local UI state            Zustand
Forms                     TanStack Form
Tables                    TanStack Table
UI primitives             shadcn/ui + Base UI
Styling                   Tailwind CSS
Mapping                   MapLibre GL JS
Client geometry           Turf.js
Backend testing           Go + Testcontainers
Frontend testing          Vitest + Playwright
Observability             slog + OpenTelemetry
Local infrastructure      Docker Compose
```

The architecture should remain simple enough to build quickly, while preserving the information model required for future ISR and Intelligence capabilities.
