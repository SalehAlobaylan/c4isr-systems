# Product Evaluation

A product-minded assessment of the C4ISR platform as it stands: what it is genuinely worth today, where it falls short of its own claims, and what to do next.

Scope: the C2 + Operational Awareness milestone. Written after completing the design-system and operational-picture work, precisely because that work made it clear how much of the remaining risk sits *underneath* the interface.

---

## 1. Verdict

**Yes, it is useful — as a specific and narrower thing than it presents itself as.**

It is a genuinely strong **C2 training and demonstration rig** with production-grade foundations. It is not yet useful as an operational console for real traffic, and it is not yet an intelligence product.

The primary risk is that it continues to be built as the third while being sold as the first.

---

## 2. Verification performed

Before forming a judgment, the core product promise was exercised end to end against a running stack (PostgreSQL/PostGIS + Go API + operator UI) as the default persona `operator-01`, rather than trusting the README:

```
scenario start (50x) -> observations -> track -> geofence breach -> alert
-> acknowledge -> incident -> mission (asset assigned) -> command SENT -> COMPLETED
```

The audit trail recorded every step:

```
command.issued    | command:cmd_01m4jv1wh6dbqe | operator-01
mission.created   | mission:msn_01m4jv1wftg1m7 | operator-01
incident.created  | incident:inc_01m4jv1wder803 | operator-01
alert.acknowledged| alert:alr_01m4jv0y984nxhfpn | operator-01
```

**Conclusion: the loop the PRD promises genuinely works today.**

### 2.1 Corrections made during verification

Two beliefs held during investigation turned out to be false. Both are recorded because either would have corrupted this analysis:

| Initial belief | Reality | Cause |
|---|---|---|
| "Operators cannot start scenarios (403)" | False | My own malformed request URL, not a role restriction. `Allows()` in `internal/platform/httpx/middleware.go` explicitly grants `scenarios.control` to the operator role. |
| "There is no operator action surface" | False | Mission creation and command issuance exist (`features/incidents/MissionFormDialog.tsx`, `features/incidents/CommandConsole.tsx`) and are exposed in the incident workspace. |

Method note: the first error came from trusting an unverified inference. The second came from an incomplete read of `src/features/`. Both were caught by reading code and exercising the API rather than reasoning about it.

---

## 3. Where the value actually is today

The genuinely defensible asset is **deterministic replay**: fixed seed, fixed ordering, and replay through the same application boundaries that real integrations will use.

Most competing products cannot reproduce an incident exactly. That makes the platform immediately valuable for:

- exercise design and after-action review
- regression-testing detection and alert logic
- demonstrating the information model to stakeholders

Supporting strengths, all verified in code:

- A C4ISR-aware information model (`Asset != Track`, provenance mandatory, `observedAt`/`receivedAt`/`processedAt` stored separately)
- PostGIS-authoritative geospatial rules (`ST_Contains`, `ST_DWithin`, `ST_Distance`)
- Route-level RBAC with a single permission matrix shared by `/auth/me` and the middleware
- A complete, queryable audit trail

**The buyer who receives value within six months is training units and integrators — not live operations.**

---

## 4. Gaps, ranked by product pain

### 4.1 Staleness is computed server-side and discarded at the UI — highest risk

`internal/telemetry/service.go` maintains a complete stale-entity set (`ReconcileStale`, `ObserveStaleEntity`). The UI surfaces it in exactly one place: a `· stale` suffix on telemetry rows in `AssetDetailPanel.tsx`.

Nothing on the picture. Nothing on the board.

This directly violates the product's own requirements:

- `PRODUCT.md` §25 — the UI *should* visibly distinguish live / delayed / stale / unavailable / uncertain
- `PRODUCT.md` §14.1 — the map requires stale-state visualization

In a C2 tool, "is this still true?" is the operator's first question. Today a track from three weeks ago renders identically to a live one. This is simultaneously the most dangerous gap and the cheapest to close.

### 4.2 The intelligence pillar is a schema, not a capability

`assessments.create` exists in the permission matrix for `analyst` and `supervisor`. The UI, however, can only *read* assessments — there is no authoring surface anywhere.

The analyst role therefore has permissions and no workspace. The product's most distinctive claim ("why does the system believe what it currently presents?") is answered for machine-generated classifications but not for human judgement.

### 4.3 Time — the product's superpower — is barely used by the interface

Pause, resume, speed change, and replay all exist. There is **no time scrubber**: no way to ask the picture "what did this look like at T+23s, the moment of the breach?"

For training and after-action review this is arguably the single highest-value missing surface in the system, and the underlying machinery is already built.

### 4.4 No coordination primitives for a shared picture

Incidents carry `assignedOperator`, but there is no claim/ownership action, no handoff, no operator presence, and no indication that another operator is currently examining a given entity.

The moment two crews share a console, they work blind. This is a hard blocker for any multi-crew deployment.

### 4.5 Internal identifiers leak into operator-facing language

`internal/alerts/service.go` formats titles directly from IDs:

```go
Title: fmt.Sprintf("Track %s entered %s", ev.TrackID, ev.GeofenceName)
```

So the alert center literally reads `Track trk_01m2z50fzhyrcdb3832nxcpycv entered Restricted Zone A`. The display layer was cleaned up during the design work; the data layer still hands raw identifiers to users as nouns.

Separately, every screenshot captured during the design pass showed a WebGL failure dumping raw JSON internals into the operator's face. For a safety-critical tool, error surfaces that look like debug output destroy trust immediately.

### 4.6 Language mismatch

The product carries Riyadh coordinates, `+03:00` timestamps, and an Arabic brand name — and ships an English-only, LTR-only interface. For the plausible buyer, Arabic and RTL are a first-order requirement, not polish.

### 4.7 Scale posture is demo-grade

The board aggregates up to 2,000 observations and 500 entities client-side in memory; the picture fetches all tracks rather than querying by viewport. Acceptable for a seeded scenario, incorrect for real deployment.

---

## 5. The strategic fork

Three defensible products, requiring very different next six months:

| Position | Fit today | What it demands |
|---|---|---|
| **A. Training / exercise rig** | High — replay is the moat | Exercise authoring, scoring, scrubber, export |
| **B. Live operational console** | Low | Ingest scale, staleness UX, coordination, i18n — 12+ months |
| **C. Integration / reference platform** | Medium — contract-first design fits | Polished SDK, docs, conformance suite |

**Recommendation: sell A now, build toward B, let C fall out as the by-product.**

The riskiest path is drifting into B by accident, because each new schema module feels like progress while no crew ever uses it.

---

## 6. Recommended next moves, in order

1. **Surface freshness everywhere** (map, board, rail) — visually distinguish stale entities and expose a "picture as of" timestamp. Small effort, disproportionate safety value.
2. **Add a time scrubber to the picture** — reuses existing scenario infrastructure and immediately delivers the replay advantage.
3. **Fix operator-facing language** — resolve identifiers into names server-side; replace the WebGL error dump with a plain-language state.
4. **Ship an assessment composer** in the incident workspace — converts the intelligence claim from schema to capability.
5. **Decide the product position explicitly**, and cut whatever does not serve it.

---

## 7. Closing observation

The design-system, picture, and board work completed most recently is the *least* risky part of this project. It looks and feels finished because a session was spent on it.

The gaps that determine whether anyone actually uses this system are all underneath the interface.