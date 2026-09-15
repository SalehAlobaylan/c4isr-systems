# IMPLEMENTATION_PLAN.md

# C4ISR-Oriented Platform Implementation Plan

## 1. Objective

This document defines how to build the platform incrementally.

The implementation strategy is:

> C4ISR-aware, C2-first.

The first implementation should produce a working C2 + Operational Awareness platform while preserving the information structures needed for future ISR and Intelligence capabilities.

The development order should prioritize:

- Go backend
- PostGIS
- real-time state
- observations and provenance
- tracks
- alerts
- incidents
- operator workflows
- deterministic scenarios

Advanced intelligence, AI, external sensors, and edge/hardware integration come later.

---

# 2. Development Principles

## 2.1 Build Vertical Slices

Avoid building isolated modules for months.

Prefer:

```text
source
  ↓
observation
  ↓
track
  ↓
map
  ↓
geofence
  ↓
alert
  ↓
operator action
```

Each milestone should produce visible system behavior.

---

## 2.2 Preserve C4ISR Concepts Early

Even if not fully implemented, create room for:

- Sources
- Observations
- Detections
- Provenance
- Confidence
- Assessments

Do not collapse everything into telemetry or tracks.

---

## 2.3 Keep the First Version Deterministic

AI should not be needed for the core system to work.

The scenario engine should produce deterministic operational data.

---

# 3. Phase 0 — Repository and Tooling Foundation

## Goal

Create a clean monorepo.

## Structure

```text
apps/
├── c4isr-server/
└── operator-ui/

internal/
api/
db/
scenarios/
deployments/
```

## Backend Setup

Initialize:

- Go module
- Chi
- pgx
- sqlc
- migration tool
- slog
- configuration loading

## Frontend Setup

Initialize:

- React
- TypeScript
- Vite
- TanStack Router
- TanStack Query
- Zustand
- Tailwind
- shadcn/ui + Base UI
- MapLibre

## Database

Docker Compose:

- PostgreSQL
- PostGIS extension

## Tooling

Add:

- Taskfile
- sqlc
- Buf
- OpenAPI directory
- lint/test scripts
- `.env.example`

## Exit Criteria

- backend boots
- frontend boots
- PostGIS connection works
- first migration works
- `/health` responds
- frontend can call backend
- test command runs

---

# 4. Phase 1 — Modular Application Skeleton

## Goal

Create the C4ISR-aware modular structure.

Initial modules:

```text
sources
observations
assets
tracks
telemetry
geospatial
alerts
realtime
audit
```

Skeleton-only placeholders:

```text
detections
classifications
assessments
```

The placeholders should exist conceptually without large implementations.

## API

Create empty/list endpoints:

```text
GET /api/v1/sources
GET /api/v1/observations
GET /api/v1/assets
GET /api/v1/tracks
GET /api/v1/alerts
```

## Frontend

Create operator shell:

```text
/map
/assets
/tracks
/alerts
```

## Exit Criteria

- modules compile independently
- no domain logic in HTTP handlers
- frontend calls Go API
- module dependencies are explicit

---

# 5. Phase 2 — Sources

## Goal

Represent where operational information comes from.

## Model

```text
id
name
type
status
metadata
created_at
```

Initial source types:

```text
synthetic
operator
external
sensor
```

## API

```text
POST /api/v1/sources
GET  /api/v1/sources
GET  /api/v1/sources/{id}
```

## Exit Criteria

A synthetic scenario source can be registered and retrieved.

---

# 6. Phase 3 — Observations

## Goal

Create the evidence layer.

## Model

```text
id
source_id
observation_type
observed_at
received_at
position?
payload
quality?
```

## Requirements

Handle:

- valid observation
- duplicate observation
- late observation
- invalid coordinates
- unknown source
- missing required data

## API

Development endpoint:

```text
POST /api/v1/observations
```

Read endpoints:

```text
GET /api/v1/observations
GET /api/v1/observations/{id}
```

## Exit Criteria

The platform can ingest observations while preserving source and time provenance.

---

# 7. Phase 4 — Asset Domain

## Goal

Represent controlled entities.

## Model

Asset:

```text
id
name
type
status
capabilities
created_at
```

Asset state:

```text
asset_id
position
speed
heading
health
connection_state
last_seen_at
```

## API

```text
POST /api/v1/assets
GET  /api/v1/assets
GET  /api/v1/assets/{id}
```

## Frontend

- asset list
- details panel
- map marker

## Exit Criteria

A controlled asset appears on the operator map.

---

# 8. Phase 5 — Telemetry

## Goal

Update asset state continuously.

## Model

Normalized telemetry:

```text
message_id
asset_id
source_id
observed_at
received_at
position
speed
heading
health
connection_state
```

## Requirements

Handle:

- stale update
- duplicate update
- out-of-order update
- invalid coordinates
- unknown asset

## Persistence

Separate:

```text
asset_state
asset_telemetry
```

## Exit Criteria

Asset state updates correctly while history is preserved.

---

# 9. Phase 6 — WebSocket Realtime

## Goal

Push operational changes to the UI.

## Backend

Create:

```text
GET /api/v1/realtime
```

Initial events:

```text
observation.received
asset.created
asset.updated
asset.position.updated
asset.connection.changed
```

## Frontend

On realtime event:

- update/invalidate TanStack Query state
- move map marker
- refresh active detail view

## Exit Criteria

An asset moves on the map without polling.

---

# 10. Phase 7 — Scenario Runner v1

## Goal

Provide deterministic real-time inputs.

## Features

- YAML scenario file
- fixed seed
- timed events
- synthetic sources
- asset movement
- observation generation
- playback speed

## Example

```yaml
scenario: patrol
seed: 10

sources:
  - id: sim-source
    type: synthetic

assets:
  - id: patrol-01
    type: vehicle

events:
  - after: 1s
    action: move
    target: patrol-01
```

## Command

```bash
task scenario:run NAME=patrol
```

## Exit Criteria

Starting a scenario causes a moving asset to appear in real time.

---

# 11. Phase 8 — Tracks

## Goal

Represent observed entities separately from assets.

## Model

Track:

```text
id
status
first_seen_at
last_seen_at
```

Track state:

```text
track_id
position
speed
heading
```

Relationships:

```text
track_observations
```

## Behavior

Initial correlation can be simple:

- scenario explicitly provides track ID
- observation is attached to known track

Do not build full sensor fusion yet.

## API

```text
GET /api/v1/tracks
GET /api/v1/tracks/{id}
```

## Frontend

- visually distinguish tracks from assets
- track details
- source/evidence list

## Exit Criteria

Multiple observations can support a track and provenance remains visible.

---

# 12. Phase 9 — Minimal Classification and Confidence

## Goal

Avoid hard-coding tracks as absolute truth.

## Model

Classification:

```text
id
track_id
label
confidence?
method
source_reference?
created_at
```

Methods:

```text
SCENARIO
OPERATOR
RULE
AI
```

AI is not implemented yet.

## Frontend

Track panel displays:

- current classification
- confidence if available
- classification source

## Exit Criteria

A track can have an explicit classification record rather than only a string field.

---

# 13. Phase 10 — Geofences and PostGIS Rules

## Goal

Make geography operationally meaningful.

## Model

```text
id
name
type
geometry
severity
active
```

## API

```text
POST /api/v1/geofences
GET  /api/v1/geofences
```

## Spatial Operations

Implement:

- contains
- intersects
- distance
- nearest asset

## Frontend

- display geofences
- inspect geofence
- optional drawing

## Exit Criteria

Track entry into a geofence is detected by PostGIS.

---

# 14. Phase 11 — Alerts

## Goal

Turn operational conditions into operator attention.

## Model

```text
id
type
severity
state
source_reference
track_id?
asset_id?
geofence_id?
created_at
acknowledged_at?
acknowledged_by?
```

## First Rule

```text
Track enters restricted geofence
        ↓
Alert created
```

## API

```text
GET  /api/v1/alerts
POST /api/v1/alerts/{id}/acknowledge
POST /api/v1/alerts/{id}/resolve
```

## Realtime

```text
alert.created
alert.acknowledged
alert.resolved
```

## Exit Criteria

A scenario-driven geofence breach produces a real-time alert.

---

# 15. Phase 12 — Audit and Provenance View

## Goal

Make system behavior traceable.

## Record

At minimum:

- source created
- observation received
- track created
- classification created
- geofence breached
- alert created
- alert acknowledged
- operator action

## API

```text
GET /api/v1/audit
```

## Frontend

Create timeline and evidence drill-down.

## Exit Criteria

An operator can trace a generated alert back to the supporting track and observations.

---

# 16. Phase 13 — Incidents

## Goal

Create a coordinated operational response model.

## Model

```text
id
title
description
priority
status
created_at
assigned_operator?
```

Relationships:

```text
incident_alerts
incident_tracks
incident_assets
incident_observations
incident_assessments
```

## State Machine

```text
OPEN
ACKNOWLEDGED
INVESTIGATING
RESPONDING
RESOLVED
CLOSED
```

## Exit Criteria

An operator can turn an alert into an incident and inspect its evidence.

---

# 17. Phase 14 — Missions and Assignments

## Goal

Allow response coordination.

## Model

Mission:

```text
id
name
objective
priority
status
```

Task:

```text
id
mission_id
type
status
target_location?
```

Relationships:

```text
mission_assets
mission_incidents
```

## Exit Criteria

An asset can be assigned to respond to an incident.

---

# 18. Phase 15 — Command Lifecycle

## Goal

Model control before real hardware exists.

## Command

```text
id
asset_id
type
payload
state
created_by
created_at
sent_at?
acknowledged_at?
completed_at?
failure_reason?
```

## Scenario Support

Simulate:

- acknowledgment
- delay
- rejection
- timeout
- failure

## Exit Criteria

A mission-triggered command moves through an explicit lifecycle.

---

# 19. Phase 16 — Minimal Assessment Model

## Goal

Introduce Intelligence concepts without building a full intelligence platform.

## Model

```text
id
subject_type
subject_id
type
conclusion
confidence?
method
created_by?
created_at
```

Evidence relation:

```text
assessment_evidence
```

Methods:

```text
OPERATOR
RULE
ALGORITHM
AI
```

## First Assessment Example

```text
"Track-42 likely represents the same entity observed by Source-A and Source-B."
```

Initially this may be manually created.

## Exit Criteria

The system can store an assessment separately from raw observations and track state.

---

# 20. Phase 17 — Authentication and RBAC

## Goal

Attribute and authorize operator activity.

## Roles

```text
operator
supervisor
administrator
```

Future:

```text
analyst
```

## Permissions

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

## Exit Criteria

Restricted actions are blocked and actor identity is recorded.

---

# 21. Phase 18 — Scenario Runner v2

## Goal

Turn scenarios into a serious C4ISR test harness.

## Add

Information quality:

- duplicate observations
- delayed observations
- stale observations
- conflicting observations
- missing data
- bad coordinates

Connectivity:

- degraded link
- disconnect
- reconnect

Operational failures:

- asset unavailable
- command timeout
- command rejection
- incident escalation

Intelligence-oriented cases:

- two sources observe same entity
- sources disagree
- classification changes after new evidence
- assessment revised
- incorrect initial correlation

## Controls

- pause
- resume
- restart
- speed multiplier
- seed
- event inspection

## Exit Criteria

The system can replay complex multi-source conditions deterministically.

---

# 22. Phase 19 — Contract Formalization

## Browser Contract

Use OpenAPI.

Generate:

- TypeScript types
- client functions

Enforce in CI.

## Machine Contract

Introduce protobuf for stable external interfaces.

Likely first services:

```text
ObservationIngestionService
TelemetryIngestionService
```

Use:

- Buf lint
- Buf breaking checks

## Exit Criteria

External systems can integrate through versioned contracts.

---

# 23. Phase 20 — Observability

## Logging

Use `slog`.

Include:

- request ID
- operator ID
- source ID
- observation ID
- track ID
- scenario ID
- correlation ID

## Metrics

Track:

- observations/sec
- telemetry/sec
- rejected observations
- track update latency
- geofence evaluation duration
- WebSocket publish latency
- stale entities
- alert generation rate
- command acknowledgment latency
- DB latency

## Tracing

Example trace:

```text
observation received
      ↓
track updated
      ↓
geofence evaluated
      ↓
alert created
      ↓
websocket published
```

## Exit Criteria

Latency and failures can be diagnosed without manual print debugging.

---

# 24. Phase 21 — Test Hardening

## Unit

Cover:

- state machines
- validation
- classifications
- alert rules
- command lifecycle
- incident transitions
- assessment validation

## Integration

Use real PostGIS with Testcontainers.

Test:

- observation persistence
- spatial rules
- repositories
- migrations
- transactions
- track/evidence relationships

## Scenario Acceptance Tests

Example:

```text
Given:
- Source-A
- Source-B
- Track-X
- Restricted Zone

When:
- Source-A observes Track-X
- Source-B observes Track-X
- Track-X enters zone

Then:
- observations remain distinct
- track contains evidence links
- alert is created
- operator sees alert
- audit preserves sequence
```

## End-to-End

Playwright:

```text
scenario starts
track appears
evidence visible
alert appears
operator acknowledges
incident created
asset assigned
command issued
```

## Exit Criteria

Primary workflow runs in CI.

---

# 25. First Product Milestone

Before advanced Intelligence, AI, external sensors, or hardware, the platform should support:

```text
1. Start system
2. Start scenario
3. Synthetic source appears
4. Source emits observation
5. Observation creates/updates track
6. Supporting provenance is preserved
7. Controlled asset appears
8. Track moves toward restricted zone
9. PostGIS detects breach
10. Alert is generated
11. Operator receives alert in real time
12. Operator inspects supporting observations
13. Operator acknowledges alert
14. Incident is created
15. Asset is assigned
16. Command is issued
17. Scenario acknowledges command
18. Audit reconstructs the full sequence
19. Scenario replays identically
```

If this works correctly, the platform has a valid C4ISR-oriented foundation.

---

# 26. Phase 22 — ISR Expansion

Only after the first milestone.

Add:

- richer source types
- sensor metadata
- detections
- observation quality
- collection tasks
- sensor tasking abstractions
- richer observation payloads
- source health

Goal:

> Move from synthetic inputs toward structured multi-source collection.

---

# 27. Phase 23 — Intelligence Expansion

Add:

- correlation
- multiple supporting observations
- competing classifications
- confidence revision
- assessment history
- conflicting evidence
- entity relationships

Do not build a full knowledge graph unless real use cases justify it.

---

# 28. Phase 24 — AI-Assisted Intelligence

First useful AI capabilities:

1. scenario generation
2. incident summarization
3. classification suggestions
4. anomaly suggestions
5. observation correlation suggestions
6. natural-language operational search

AI outputs become:

```text
assessment
classification suggestion
correlation suggestion
summary
```

not silent state mutation.

---

# 29. Phase 25 — External Integration

Add:

- gRPC observation ingestion
- telemetry gateway if required
- external simulator
- external sensors
- external C2/C4ISR integration

Potential future MQTT adoption only when device connectivity requires it.

---

# 30. Phase 26 — Edge and Hardware

Only when the core is valuable enough to justify it.

Potential Rust edge agent responsibilities:

- protocol translation
- local buffering
- device identity
- connectivity
- command acknowledgments
- telemetry filtering
- future MQTT/gRPC
- future MAVLink

---

# 31. Explicitly Deferred

Do not implement early:

- Kafka
- NATS
- Redis
- Kubernetes
- TimescaleDB
- Elasticsearch
- MQTT
- MAVLink
- full sensor fusion
- knowledge graph
- real drone flight control
- radar processing
- video analytics
- service mesh
- multi-region deployment

---

# 32. Work Order Summary

```text
0. Repository + tooling
1. Modular skeleton
2. Sources
3. Observations
4. Assets
5. Telemetry
6. WebSocket
7. Scenario Runner v1
8. Tracks
9. Classification + confidence
10. PostGIS geofences
11. Alerts
12. Audit + provenance
13. Incidents
14. Missions
15. Commands
16. Assessments
17. Authentication/RBAC
18. Scenario Runner v2
19. OpenAPI + gRPC contracts
20. Observability
21. Test hardening

Then:

22. ISR expansion
23. Intelligence expansion
24. AI assistance
25. External integrations
26. Edge/hardware
```

---

# 33. Implementation Rules

1. Do not model the system around drones.
2. Keep assets and tracks distinct.
3. Keep observations and tracks distinct.
4. Preserve provenance.
5. Keep current state separate from evidence/history.
6. Do not treat classification as absolute truth by default.
7. Do not put business logic in HTTP handlers.
8. Do not make client-side GIS authoritative.
9. Do not duplicate backend state in Zustand.
10. Do not add microservices without a concrete reason.
11. Do not use AI for deterministic core state transitions.
12. Do not introduce a broker before it is required.
13. Make important operator actions auditable.
14. Make scenarios reproducible.
15. Keep external protocols outside core domain logic.

---

# 34. Definition of a Strong Foundation

The system is ready for serious ISR, Intelligence, AI, and hardware work when:

- Sources and Observations are modeled explicitly
- provenance is preserved
- assets and tracks are distinct
- track state is backed by evidence
- spatial rules are tested with real PostGIS
- telemetry handles stale/duplicate/out-of-order data
- realtime updates are reliable
- alerts and incidents have explicit lifecycles
- commands are auditable
- assessments are separate from observations
- scenarios are deterministic
- primary workflow is covered end-to-end
- external sources can be replaced without changing core domain logic

At that point, the system is not merely a C2 demo.

It is a credible C4ISR-oriented software foundation.
