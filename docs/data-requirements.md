# Data Requirements

What data the platform needs to become genuinely useful, with particular attention to **publicly available** sources.

Scope: derived from the actual ingest contracts in `internal/`, not from assumption. Written because the answer determines whether this system can ever run on real feeds rather than seeded scenarios.

---

## 1. What the system structurally requires

The core accepts exactly two streams. Everything else is derived.

| Stream | Contract | Produces |
|---|---|---|
| **Source registry** | `name`, `type` (synthetic / operator / external / sensor), `status` (active / degraded / offline), `metadata` | Trust and availability. Registered **before** any data lands. |
| **Observations** | `sourceId`, `type`, `observedAt` + `receivedAt`, `position`, `payload`, `quality`, optional `trackHint`, optional idempotency `id` | Evidence layer → correlated into **tracks** |
| **Telemetry** | `assetId`, `sourceId`, `observedAt` + `receivedAt`, `position`, `speed`, `heading`, `health`, `connectionState`, `messageId` (dedupe) | Projects onto controlled **assets** |
| **Geofences** | geometry + active flag | Rules evaluated on `TrackUpdated` → `GeofenceBreached` / `GeofenceExited` → alerts |

### 1.1 The decisive structural fact

**The alert engine is entirely track-driven.** `internal/geospatial/service.go` derives breach and exit events from real containment checks on track updates; those events become alerts in `internal/alerts/service.go`.

> No track, no alert.

So "what data does it need" has a sharper form than it first appears:

**What can be expressed as a positioned, timestamped thing that gets correlated?**

### 1.2 Two properties that already suit real-world data

- `observedAt` and `receivedAt` are stored separately — correct for satellite and batch feeds with genuine latency
- A 5-minute future-clock-skew guard (`maxClockSkew`) plus per-message deduplication

These are non-trivial foundations for real ingestion, and they are already in place.

---

## 2. Public data inventory

| Data class | Public source | Access reality |
|---|---|---|
| Basemap geometry | OSM tiles / Overpass / Geofabrik | Free (ODbL — attribution obligations), rate-limited |
| Administrative boundaries | OSM, Natural Earth, OCHA HDX (COD-AB) | Free; HDX licensed per dataset |
| Protected areas | WDPA | Free to **view**, **commercially restricted** — the classic trap |
| Critical infrastructure | OSM features (power, water, ports, airfields) | Free, incomplete; the `military` layer is deliberately sparse |
| Aircraft movement | **OpenSky Network** | Free API, non-commercial/academic terms, no SLA; ADS-B Exchange requires a key |
| Maritime traffic | Terradue / AISHub | Raw AIS is public; quality feeds are **paid**, redistribution constrained |
| Weather | Open-Meteo, national met services | Free, well documented, easiest first win |
| Wildfire | NASA FIRMS (near-real-time hotspots) | Free |
| Earthquakes | USGS | Free, no key, high quality |
| Transit | GTFS feeds | Free for public agencies |
| Cameras / street imagery | Mapillary, public webcam networks | Licence-restricted |
| **Airspace / no-fly zones** | — | **Effectively not published as structured open data.** NOTAM reaches consumers through paid providers. |

---

## 3. The two gaps that matter most

### 3.1 Public data alone yields situational awareness, not C2

The platform's differentiator — controlled assets that can be assigned and commanded — is inherently private. Public data cannot supply it.

The correct architecture is therefore a **two-track strategy**:

- **Public context layer** — cheap and plentiful: boundaries, infrastructure, hazards, weather, traffic
- **Private controlled layer** — the actual product: owned fleet telemetry

The moat is the *fusion* of the two. Only the second justifies the command loop.

### 3.2 The data model over-fits to moving objects

Weather, wildfire, and earthquake data describe **events and areas**. They do not naturally become tracks with speed and heading.

Today, feeding them in means forcing ill-fitting data through a track-centric pipeline. The realistic outcomes are nonsense tracks, or data that sits inert because no alert ever derives from it.

Closing this requires an explicit **event/area ingestion path** that raises alerts directly, without pretending to be a tracked object.

---

## 4. Blockers to address before real ingestion

### 4.1 No service identity

`observations.ingest`, `telemetry.ingest`, and `sources.manage` all require the **supervisor** role (`Allows()` in `internal/platform/httpx/middleware.go`). There is no integration role.

An external feed today means handing a human supervisor's bearer token to a gateway — precisely what `PRODUCT.md` §24 defers to "future" (service identity, mTLS, device certificates).

**This gap must close before any third-party feed is realistic.**

### 4.2 Volume ceiling

The current posture is demo-scale: roughly 384 observations in the development dataset, 500-entity client-side fetches, browser-side aggregation of up to 2,000 observations.

Global ADS-B runs at tens of thousands of messages per second. Serving that from Postgres with per-observation rows is a different architecture, not a tuning exercise. Real deployments need bounded-area queries, decimation, or batch ingest at the edge.

---

## 5. Recommended sequence

1. **Start with three free, key-less feeds** — USGS earthquakes, NASA FIRMS fires, Open-Meteo weather. Each is geospatial, genuinely public, and immediately exercises geofencing, naming, and alerting.
2. **Add one bounded-area OpenSky pull** to prove multi-source fusion and conflict handling at realistic volume.
3. **Then build one private ingest** (owned fleet via a minimal gateway) to prove the controlled-asset half.
4. **Add an ingest service role before step 1**, so external feeds never need human credentials.

---

## 6. Cautions

**Licensing.** ODbL and HDX datasets carry obligations that survive into derived products. Check terms before any commercial use — particularly WDPA, which is free to view and restricted commercially.

**Alert fatigue.** Public feeds deliver volume without urgency. FIRMS and weather will bury a genuine intrusion alert unless the rule engine is designed so the queue stays trustworthy.

**Staleness is not optional here.** Public data has *known* latency characteristics. Labelling it honestly in the UI (see `product-evaluation.md` §4.1) is a prerequisite for using these feeds at all — otherwise the operator cannot distinguish a live intrusion from an hour-old fire hotspot.

**Discipline.** Do not start a data-acquisition project. Start from the three questions an operator must answer, then admit only the feeds that change an answer.