# PRODUCT.md

# C4ISR-Oriented Platform Product Requirements Document

## 1. Product Overview

This product is a **software-first C4ISR-oriented operational platform** designed to evolve toward integrated:

- Command
- Control
- Communications
- Computers
- Intelligence
- Surveillance
- Reconnaissance

The first implementation milestone is **not** a complete C4ISR system.

The first milestone is a strong **C2 + Operational Awareness core** that is architected from the beginning so future ISR and Intelligence capabilities can be added without redesigning the fundamental information model.

The platform is intended to help operators understand, coordinate, and act within a dynamic operational environment.

At a high level, the product should answer:

- What is happening right now?
- What do we control?
- What are we observing?
- Where did this information come from?
- How recent is this information?
- How confident are we in it?
- Which observations belong to the same real-world entity?
- What requires attention?
- What assets are available to respond?
- What commands have been issued?
- What intelligence assessments exist?
- What happened previously?
- Who performed each action?
- Why does the system believe what it currently presents?

The product should begin with synthetic and scenario-driven operational inputs, then evolve toward integration with:

- sensors
- UAVs
- ground vehicles
- cameras
- radar
- external operational systems
- telemetry gateways
- AI services
- simulators
- edge agents
- real hardware

The **platform itself is the product**.

Simulators, AI systems, edge runtimes, sensors, and hardware are external capabilities that connect to it.

---

# 2. Product Vision

Build a modern operational platform that can progressively support the full information-to-decision chain:

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
   ↓
External Effect / Response
```

The product should eventually support:

1. collecting operational information from multiple sources
2. preserving source provenance and observation history
3. maintaining a live operational picture
4. correlating observations into tracks
5. representing uncertainty and confidence
6. generating alerts and incidents
7. supporting operator decision-making
8. coordinating missions and assets
9. issuing and tracking commands
10. creating intelligence assessments
11. integrating AI as an assistive analytical capability
12. integrating external sensors and systems through stable contracts
13. replaying operational scenarios deterministically
14. preserving complete auditability and traceability

The long-term ambition is a reusable C4ISR-oriented foundation for environments such as:

- security operations
- infrastructure monitoring
- emergency response
- search and rescue
- industrial operations
- smart-city operations
- robotics coordination
- field operations
- perimeter monitoring
- large-scale IoT operational awareness

---

# 3. Product Strategy

The product should follow this strategy:

```text
Architectural Vision
       C4ISR
         │
         ▼
First Product Milestone
C2 + Operational Awareness
         │
         ▼
ISR Ingestion
         │
         ▼
Correlation / Fusion
         │
         ▼
Intelligence
         │
         ▼
Advanced AI + External Systems
```

This distinction is fundamental.

We are **not** building a narrow C2 application and planning to retrofit C4ISR concepts later.

We are designing a C4ISR-aware information model from the beginning, while implementing only the capabilities needed for the first C2 milestone.

---

# 4. Product Positioning

The product should be understood as:

> A real-time operational awareness, intelligence, and command platform that transforms observations from multiple sources into a traceable operational picture that supports human decision-making and coordinated action.

It should not be positioned as:

> A drone application.

It should also not be reduced to:

> A map with moving markers.

The map is one interface into a larger operational information system.

The core value comes from:

- information provenance
- observations
- tracks
- operational state
- geospatial context
- intelligence assessments
- alerts
- incidents
- missions
- commands
- history
- confidence
- traceability

---

# 5. Capability Model

The product should be organized conceptually into five major capability areas.

## 5.1 Command and Control

Responsible for:

- operators
- assets
- missions
- tasks
- commands
- incidents
- response coordination
- command lifecycle
- operational workflows

## 5.2 Operational Awareness

Responsible for:

- tracks
- positions
- geospatial context
- alerts
- events
- operational picture
- current state
- history

This is the bridge between raw information and C2.

## 5.3 ISR

Responsible for:

- sources
- sensors
- observations
- detections
- collection
- reconnaissance tasks
- surveillance feeds
- observation history

The first version will implement only the minimum abstractions required to avoid coupling the product directly to synthetic telemetry.

## 5.4 Intelligence

Responsible for:

- assessments
- classification
- confidence
- provenance
- correlation
- entity relationships
- intelligence products
- analytical conclusions

This capability will remain minimal in the first milestone but must be represented in the product model from the beginning.

## 5.5 Communications and Computing

Responsible for the technical fabric that allows the system to operate:

- REST
- WebSocket
- gRPC
- future MQTT
- external integrations
- data processing
- event transport
- observability
- storage
- security
- future edge communications

The first implementation focuses on software communications, not RF or military communications systems.

---

# 6. Primary Users

## 6.1 Operator

The primary operational user.

Responsibilities:

- monitor the operational picture
- inspect assets and tracks
- review observations
- inspect alerts
- acknowledge alerts
- manage incidents
- assign assets
- follow missions
- issue permitted commands
- inspect recent activity

## 6.2 Supervisor

Higher-level operational user.

Responsibilities:

- oversee multiple incidents
- review operator actions
- inspect priorities
- coordinate responses
- inspect assessments
- manage escalation
- review operational history

## 6.3 Intelligence / Analysis User

A future but important user role.

Responsibilities:

- inspect source observations
- compare conflicting information
- create assessments
- review track classifications
- inspect confidence
- correlate multiple observations
- create analytical conclusions
- review AI-generated analytical suggestions

This role may initially be represented by the supervisor role until the intelligence workspace becomes substantial.

## 6.4 System Administrator

Responsibilities:

- users
- roles
- permissions
- operational areas
- alert rules
- integrations
- scenario configuration
- system health

## 6.5 Developer / Integrator

Responsibilities:

- integrate telemetry sources
- integrate sensors
- define gRPC contracts
- build simulators
- integrate AI services
- validate external contracts
- test failure scenarios

---

# 7. Core Information Model

The product should distinguish between **reality**, **observations**, **system interpretation**, and **operational action**.

This is one of the most important product requirements.

```text
Real World Entity
      │
      ▼
Source / Sensor
      │
      ▼
Observation
      │
      ▼
Detection
      │
      ▼
Track
      │
      ▼
Assessment
      │
      ▼
Operational Picture
      │
      ▼
Decision / Mission / Command
```

These concepts must not be collapsed into one generic object.

---

# 8. Core Product Concepts

## 8.1 Asset

An entity that the organization owns, controls, operates, or coordinates.

Examples:

- UAV
- patrol vehicle
- response team
- vessel
- robot
- camera
- radar
- sensor platform

An asset may include:

- ID
- name
- type
- status
- position
- health
- availability
- connection state
- assigned mission
- capabilities
- last known update

An asset is different from a track.

## 8.2 Source

The origin of operational information.

Examples:

- radar
- camera
- UAV sensor
- GPS device
- external API
- operator report
- simulator
- AI subsystem
- another C2 system

A source should have identity and metadata.

The product should eventually be able to answer:

> Which source produced this information?

## 8.3 Observation

A timestamped statement that a source observed something.

Examples:

- Radar-01 observed an object at a given position.
- Camera-04 reported a vehicle.
- GPS tracker reported an asset location.
- Operator submitted a manual report.

Observation fields should support concepts such as:

- observation ID
- source
- observed time
- received time
- position
- observation type
- raw metadata
- quality
- related media or payload references

Observations should be preserved separately from the current operational state.

## 8.4 Detection

A more specific interpreted observation indicating that something of interest was detected.

Examples:

- vehicle detected
- person detected
- object crossed boundary
- radar return detected
- thermal signature detected

A detection may be created by:

- deterministic algorithms
- sensor software
- external systems
- AI models
- operators

Detections should preserve provenance.

## 8.5 Track

A system representation of a real-world entity being observed over time.

A track may be created from one or many observations.

```text
Observation A ─┐
Observation B ─┼──► Track-42
Observation C ─┘
```

A track may include:

- ID
- current position
- speed
- heading
- classification
- confidence
- first seen time
- last seen time
- status
- supporting observations
- source history

Tracks are not necessarily controlled.

## 8.6 Classification

A hypothesis about what a track or observed entity is.

Examples:

- person
- passenger vehicle
- truck
- vessel
- UAV
- unknown

Classification should not always be modeled as absolute truth.

Future versions should support:

- confidence
- multiple competing classifications
- source attribution
- classification history

## 8.7 Confidence

Represents uncertainty in interpreted information.

Example:

```text
classification:
  vehicle: 0.91
  truck: 0.68
```

Confidence may apply to:

- classification
- correlation
- identity
- assessment
- anomaly
- source reliability

The first version may use confidence minimally, but the concept should exist from the beginning.

## 8.8 Provenance

Provenance explains where information came from and how it was produced.

The platform should eventually answer questions such as:

- Which sensor produced this observation?
- Which observations support this track?
- Was this classification produced by AI, an operator, or a deterministic rule?
- Which model produced this assessment?
- When was the information observed?
- When did the platform receive it?

Provenance is a core C4ISR requirement.

## 8.9 Assessment

An analytical conclusion derived from one or more pieces of information.

Examples:

- Track-42 is likely the same vehicle observed earlier by Camera-03.
- Movement pattern is unusual.
- The entity is likely approaching a protected area.
- Multiple sources indicate the same object.

Assessment fields may include:

- ID
- type
- conclusion
- confidence
- evidence
- provenance
- created by
- creation method
- timestamp

Creation method may be:

- operator
- rule engine
- algorithm
- AI

Assessments are distinct from observations.

## 8.10 Sensor

A specialized source that collects information from the environment.

Examples:

- radar
- camera
- thermal camera
- acoustic sensor
- GPS receiver

A sensor may belong to an asset or exist independently.

## 8.11 Mission

A coordinated operational objective.

Examples:

- monitor area
- patrol route
- inspect location
- investigate incident
- maintain observation
- search designated zone

Mission fields may include:

- ID
- status
- objective
- priority
- assigned assets
- operational area
- tasks
- start/end
- timeline

## 8.12 Task

A concrete action within a mission.

Examples:

- move to location
- inspect sector
- observe target
- wait at checkpoint
- maintain surveillance

## 8.13 Command

An instruction issued toward an external asset or subsystem.

Lifecycle:

```text
CREATED
QUEUED
SENT
ACKNOWLEDGED
COMPLETED
```

Failure states:

```text
REJECTED
FAILED
TIMED_OUT
CANCELLED
```

Commands must always be auditable.

## 8.14 Alert

A condition requiring operator attention.

Examples:

- geofence violation
- lost connection
- stale telemetry
- asset unavailable
- suspicious movement
- conflicting observations
- sensor offline

Alerts should support:

- severity
- source
- state
- timestamp
- acknowledgment
- evidence
- related entities
- related incident

## 8.15 Incident

A higher-level operational situation.

An incident may contain:

- alerts
- tracks
- assets
- assessments
- observations
- missions
- operator actions
- commands

Example incident:

> Unknown vehicle entered a restricted area and was observed by two independent sources.

Incident states:

```text
OPEN
ACKNOWLEDGED
INVESTIGATING
RESPONDING
RESOLVED
CLOSED
```

## 8.16 Geofence

A geographic rule boundary.

Examples:

- restricted area
- protected site
- patrol zone
- surveillance area
- exclusion zone

Rules may include:

- track entered
- track exited
- asset entered
- asset exited
- object remained too long
- object approached boundary

## 8.17 Operational Event

A timestamped fact representing something that happened.

Examples:

- observation received
- track created
- asset position updated
- classification changed
- assessment created
- geofence breached
- alert created
- incident opened
- command sent

Events support:

- real-time delivery
- audit
- replay
- debugging
- historical reconstruction

---

# 9. Time Model

Operational information must distinguish different notions of time.

Important concepts include:

- `observed_at`
- `received_at`
- `processed_at`
- `effective_at`
- `created_at`

Example:

```text
Sensor observed object       10:00:01
Message received by C4ISR    10:00:03
Processing completed         10:00:04
```

The system must not assume database insertion time equals observation time.

This becomes essential for:

- delayed telemetry
- out-of-order messages
- disconnected assets
- intelligence analysis
- replay

---

# 10. Operational Picture

The platform should maintain a **Common Operational Picture** representing the best currently available understanding of the environment.

The operational picture may contain:

- controlled assets
- observed tracks
- sensors
- geofences
- incidents
- missions
- alerts
- current assessments
- operational areas

The operational picture is a **projection of available information**, not the raw information itself.

Raw observations and evidence must remain accessible.

---

# 11. First Product Milestone: C2 + Operational Awareness

The first milestone should implement:

```text
C2 Core
├── Assets
├── Missions
├── Commands
├── Incidents
└── Operators

Operational Awareness
├── Tracks
├── Alerts
├── GIS
├── Events
└── Operational Picture

Minimal ISR Foundation
├── Sources
└── Observations
```

The following should be represented but remain intentionally minimal:

```text
Detections
Assessments
Confidence
Provenance
```

This ensures later C4ISR expansion does not require redesigning the core information model.

---

# 12. MVP Goal

The MVP should prove that the platform can transform synthetic observations into a useful and traceable operational picture.

The MVP should prove:

1. sources can be represented
2. observations can be ingested
3. assets and tracks can be maintained
4. provenance is preserved
5. live positions can be stored in PostGIS
6. operational state can change in real time
7. geographic rules can be evaluated
8. alerts can be generated
9. incidents can be created
10. operators can coordinate a response
11. commands can be modeled
12. operator actions are audited
13. scenarios can be replayed deterministically

---

# 13. MVP Operational Workflow

Example:

```text
Scenario Source
      ↓
Observation
      ↓
Track
      ↓
Operational Picture
      ↓
PostGIS Rule
      ↓
Geofence Breach
      ↓
Alert
      ↓
Operator
      ↓
Incident
      ↓
Asset Assignment
      ↓
Command
      ↓
Audit
```

Detailed flow:

1. Scenario creates Source `simulator-01`.
2. Source produces observations about an unknown moving object.
3. Platform associates observations with Track-01.
4. Track position updates in real time.
5. Controlled Patrol-01 also moves.
6. Track approaches a restricted zone.
7. PostGIS determines that Track-01 entered the zone.
8. Platform creates an alert.
9. Operator receives alert through WebSocket.
10. Operator inspects supporting track and observations.
11. Operator acknowledges alert.
12. Operator creates incident.
13. Operator assigns Patrol-01.
14. Command is created.
15. Scenario source simulates command acknowledgment.
16. Complete activity appears in audit history.
17. Scenario can be replayed identically.

---

# 14. Operator Dashboard Requirements

## 14.1 Operational Map

Display:

- assets
- tracks
- sources/sensors
- geofences
- incidents
- routes
- operational areas
- alerts

Capabilities:

- zoom
- pan
- select
- inspect
- movement direction
- recent path
- filtering
- stale-state visualization
- source/evidence drill-down

## 14.2 Asset Panel

Show:

- identity
- type
- operational status
- position
- speed
- heading
- health
- connectivity
- mission
- latest observation/update
- recent events

## 14.3 Track Panel

Show:

- identity
- current position
- movement
- classification
- confidence
- first seen
- last seen
- supporting observations
- source history
- related alerts
- related assessments

## 14.4 Observation View

The operator or analyst should be able to inspect:

- source
- observed time
- received time
- position
- observation type
- raw metadata
- related track
- related detection

This can be minimal in MVP.

## 14.5 Alert Center

Support:

- severity
- acknowledgment
- filtering
- related entity
- location
- evidence
- incident link

## 14.6 Incident Workspace

Show:

- state
- priority
- related alerts
- assets
- tracks
- observations
- assessments
- missions
- timeline
- commands

## 14.7 Timeline / Audit View

Show:

- observations received
- track changes
- assessments
- alerts
- commands
- acknowledgments
- incident changes
- operator actions

---

# 15. Scenario Engine

The platform should include a deterministic scenario engine early.

The scenario engine is an **external source simulator**, not part of C2 business logic.

Example:

```yaml
scenario: restricted-area-intrusion
seed: 1007

sources:
  - id: simulator-01
    type: synthetic

assets:
  - id: patrol-01
    type: vehicle

tracks:
  - id: unknown-01
    type: vehicle

events:
  - after: 10s
    action: observe
    source: simulator-01
    target: unknown-01

  - after: 20s
    action: move
    target: unknown-01
    toward: restricted-zone-a

  - after: 45s
    action: degrade_connection
    target: patrol-01
```

The engine should emit realistic normalized inputs such as:

```text
observation.received
asset.telemetry.received
track.updated
connection.degraded
command.acknowledged
```

---

# 16. Scenario Edge Cases

The engine should support:

## Information Quality

- stale observations
- conflicting observations
- missing fields
- bad coordinates
- duplicate observations
- delayed observations
- out-of-order observations

## Connectivity

- degraded connection
- lost connection
- reconnect
- high latency

## Operational Conditions

- geofence breach
- unavailable asset
- command timeout
- command rejection
- incident escalation

## Intelligence-Oriented Conditions

Future scenarios should support:

- two sources observing the same entity
- two sources disagreeing
- uncertain classification
- incorrect initial correlation
- assessment revised after new evidence

---

# 17. GIS Requirements

PostGIS is a core product dependency.

Required capabilities:

- point storage
- lines
- polygons
- containment
- intersections
- proximity
- routes
- nearest asset
- spatial history

Examples:

- Is Track-22 inside Zone-A?
- Which assets are within 5 km?
- Which observations occurred inside this region?
- Which tracks crossed this boundary?
- Which source observations support activity in this area?

---

# 18. Real-Time Requirements

WebSocket should deliver changes such as:

```text
observation.received

asset.created
asset.updated
asset.position.updated
asset.connection.changed

track.created
track.updated
track.closed

assessment.created
assessment.updated

alert.created
alert.acknowledged
alert.resolved

incident.created
incident.updated

mission.updated

command.status.changed
```

The frontend should not poll continuously for operational state.

---

# 19. Intelligence Requirements

The intelligence capability should evolve in stages.

## Stage 1

Represent:

- provenance
- confidence
- classification
- assessments

Mostly manual or deterministic.

## Stage 2

Add:

- observation correlation
- simple fusion
- entity association
- assessment history
- conflicting evidence

## Stage 3

Add AI-assisted capabilities:

- anomaly suggestions
- classification assistance
- entity resolution
- event correlation
- incident summarization
- analytical search

## Stage 4

Add richer intelligence workflows:

- intelligence products
- analyst workspaces
- confidence revision
- relationship analysis
- historical pattern analysis

---

# 20. AI Strategy

AI should support intelligence and operator awareness.

Potential use cases:

- classify observations
- correlate related observations
- suggest track identity matches
- detect anomalies
- summarize incidents
- identify patterns
- natural-language operational search
- generate test scenarios

AI output must preserve:

- source/model
- timestamp
- confidence where appropriate
- evidence
- correlation/request ID

AI must not silently overwrite operational truth.

Example:

```text
Observations
    ↓
AI Analysis
    ↓
Assessment
    ↓
Human / Rule Review
    ↓
Operational Picture
```

---

# 21. External Integration Model

The C4ISR core should consume normalized operational information.

```text
Today

Scenario Runner
      ↓
Normalized Observations
      ↓
C4ISR Core
```

Later:

```text
Radar ───────┐
Camera ──────┤
UAV ─────────┤
External C2 ─┼──► Integration / Telemetry Layer
Operator ────┤                │
AI Service ──┘                ▼
                     Normalized Information
                              │
                              ▼
                         C4ISR Core
```

The core should not directly depend on specific device protocols.

---

# 22. Product Architecture Direction

Initial architecture:

```text
Operator UI
    │
REST + WebSocket
    │
Go C4ISR Core
├── C2
├── Operational Awareness
├── Minimal ISR
└── Intelligence Foundations
    │
PostgreSQL + PostGIS
```

Likely future architecture:

```text
                      Operator UI
                           │
                     C4ISR Core
                 ┌─────────┼─────────┐
                 │         │         │
          Telemetry GW   AI/Intel   Media
                 │
             Edge / Sensors
```

The mature product is expected to become hybrid.

---

# 23. Communications and Computers Scope

The C4 elements in this project are primarily software-oriented.

Initial scope:

- REST
- WebSocket
- gRPC
- event processing
- PostgreSQL/PostGIS
- observability
- authentication
- authorization
- service-to-service contracts

Future scope may include:

- MQTT
- edge networking
- disconnected operation
- buffering
- service identity
- mTLS

Out of initial scope:

- RF engineering
- radio networks
- tactical radio protocols
- satellite communications implementation
- military communications hardware

---

# 24. Security Requirements

Initial:

- authentication
- RBAC
- authorization
- auditable identity
- input validation
- command authorization
- secure secrets handling

Future:

- service identity
- mTLS
- device certificates
- PKI
- key rotation
- command signing
- edge enrollment
- evidence integrity controls

---

# 25. Reliability Requirements

The system must expect imperfect information.

Handle:

- missing observations
- stale observations
- delayed telemetry
- duplicate events
- out-of-order events
- conflicting sources
- disconnected assets
- temporary failures
- WebSocket reconnect

The UI should visibly distinguish:

- live
- delayed
- stale
- unavailable
- uncertain

---

# 26. Observability Requirements

Track:

- observations/sec
- telemetry/sec
- processing latency
- track updates
- alert generation
- active WebSocket clients
- stale assets
- stale tracks
- database latency
- command acknowledgment latency
- scenario status

Long-term intelligence metrics may include:

- correlation decisions
- assessment generation latency
- conflicting evidence count
- AI-assisted assessment rate

---

# 27. MVP Feature Scope

## Must Have

- sources
- observations
- assets
- tracks
- telemetry
- current state
- PostGIS
- geofences
- WebSocket
- operational map
- alerts
- incidents
- audit
- deterministic scenario runner
- scenario replay
- authentication
- roles

## Should Have

- missions
- assignments
- commands
- source provenance
- confidence fields
- classification history
- asset health
- connection state
- observation drill-down

## Could Have

- assessments
- AI-generated scenarios
- incident summaries
- nearest-asset recommendations
- simple observation correlation
- historical playback
- gRPC integration API

---

# 28. Non-Goals for MVP

Do not attempt:

- full C4ISR implementation
- complete sensor fusion
- advanced intelligence analysis
- real drone flight control
- radar processing
- RF systems
- MAVLink
- advanced physics simulation
- real camera streaming
- autonomous command
- large-scale knowledge graph
- Kubernetes
- Kafka
- large-scale distributed deployment
- military-grade communications

---

# 29. Product Phases

## Phase 1 — C2 + Operational Awareness Foundation

Build:

- assets
- sources
- observations
- tracks
- telemetry
- PostGIS
- live map
- WebSocket

Goal:

> Build a traceable live operational picture.

## Phase 2 — Operational Response

Add:

- geofences
- alerts
- incidents
- audit
- missions
- commands

Goal:

> Allow operators to understand and respond to changing situations.

## Phase 3 — ISR Foundation

Add:

- sensor abstraction
- richer observation model
- detections
- source quality
- collection tasks
- observation provenance

Goal:

> Move from synthetic telemetry toward structured multi-source information collection.

## Phase 4 — Intelligence Foundation

Add:

- assessments
- confidence
- correlation
- classification history
- competing interpretations
- evidence relationships

Goal:

> Turn observations into analytical conclusions without losing provenance.

## Phase 5 — AI-Assisted Intelligence

Add:

- anomaly detection assistance
- classification assistance
- correlation suggestions
- summarization
- natural-language search
- AI-generated scenarios

Goal:

> Improve analysis while preserving human control and traceability.

## Phase 6 — External Integration

Add:

- gRPC contracts
- telemetry gateway
- MQTT if justified
- external sensors
- simulator
- other systems

Goal:

> Connect real external information sources without changing core domain logic.

## Phase 7 — Edge and Hardware

Add:

- Rust edge agent
- local buffering
- device identity
- reconnect logic
- hardware protocols
- real devices

Goal:

> Validate architecture against real external systems.

---

# 30. MVP Success Criteria

The first milestone is successful when:

1. a scenario source produces observations
2. observations create/update a track
3. provenance is preserved
4. track moves on the map in real time
5. a controlled asset moves independently
6. PostGIS detects a geofence breach
7. alert is generated
8. operator can inspect supporting information
9. operator acknowledges the alert
10. incident is created
11. asset is assigned
12. command lifecycle begins
13. all important actions are audited
14. scenario can replay deterministically

---

# 31. Product Design Principles

## 31.1 C4ISR-Aware, C2-First

Design for the larger information model.

Implement only the capability needed now.

## 31.2 Evidence Before Interpretation

Observations should remain available even after tracks and assessments are created.

## 31.3 Provenance Is Mandatory

Important information should be traceable to its origin.

## 31.4 Uncertainty Is Real

Not every classification or assessment is absolute truth.

## 31.5 Operational State Is a Projection

The current picture is the platform's best current understanding, not raw reality.

## 31.6 Human Decision-Making Remains Central

AI and algorithms assist.

They do not silently control critical operational outcomes.

## 31.7 Geography Is Core

PostGIS-backed geospatial behavior is authoritative.

## 31.8 Real-Time by Default

Operational changes should reach operators without polling.

## 31.9 External Sources Are Unreliable

Expect delays, duplicates, stale information, disagreement, and disconnects.

## 31.10 Start Modular, Distribute Only When Needed

The initial system remains a modular monolith.

---

# 32. Initial Product Definition

The first product can be summarized as:

> A C4ISR-oriented real-time operational platform built around a Go core, PostGIS, and a TypeScript operator interface. The first implementation delivers C2 and operational-awareness capabilities while preserving C4ISR concepts such as Sources, Observations, Provenance, Tracks, Confidence, and Assessments from the beginning. Synthetic scenarios initially provide the operational inputs, while future sensors, AI services, telemetry gateways, simulators, edge agents, and real hardware can integrate through stable boundaries without redesigning the core information model.

---

# 33. First Build Target

```text
Scenario Source
      ↓
Observation Ingestion
      ↓
Go C4ISR Core
├── Sources
├── Observations
├── Assets
├── Tracks
├── Telemetry
├── Geospatial
├── Alerts
├── Incidents
├── Realtime
└── Audit
      ↓
PostgreSQL + PostGIS
      ↓
TypeScript Operator UI
├── Operational Map
├── Asset Details
├── Track Details
├── Observation Evidence
├── Alert Center
├── Incident Workspace
└── Event Timeline
```

The first implementation should prove that the platform can transform source observations into a traceable operational picture and allow a human operator to coordinate a response.

That is the first step toward the larger C4ISR vision.
