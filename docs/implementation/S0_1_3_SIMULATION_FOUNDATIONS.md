# S0.1–S0.3 — Frozen runs, spatial contracts, and context packages

**Subsequent increment:** [S0.4 and P1.5–P1.6](S0_4_P1_5_6_ADAPTER_AND_PUBLIC_HISTORY.md) completes the adapter/M2 and public-history/M1 gates. Current installations require migrations through **27**, and the next task is **P2.1**. The original acceptance details below remain historical; statements about these tasks being pending are superseded.

Implemented 2026-10-21 against the two v2 proposals. Apply migrations through **25** before starting this server. Verification uses disposable PostGIS databases; the developer database has not been migrated or seeded.

These tasks establish simulation foundations in the existing modular monolith. S0.4 and the M2 adapter/isolation demonstration remain pending. No rendering engine, physics service, GPU service, permanent public-feed polling, or hardware bridge is introduced.

## S0.1: authoring, immutable versions, and runs

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

### Time and reproducibility

Elapsed time is integer milliseconds, event identity is `(run_id, sequence)`, and order is `(virtual_time_ms, sequence)`. The initial runner has a 31-day virtual horizon and the existing 20,000-action bound. Scheduled observation and telemetry times come from the frozen epoch and event time. Delayed observations preserve their source generation time; delivery delay and stale offsets are distinct. Receipt, processing, audit, lifecycle timestamps, and WebSocket delivery continue using wall time.

The request/background context carries an application-owned information-clock resolver. Asset/track reads, telemetry response freshness, COP reads, ingestion future-time checks, and the freshness monitor use the selected exercise clock. Live COP time is not clamped to the last recording sample: silent contacts age while the exercise runs, and remain frozen while paused. Historical scrubber requests retain their bounded recording window. Public data and legacy non-versioned fixtures retain wall/public time semantics. No process-global clock or mutable cross-workspace clock is used.

`simulation.TimeMap` defines explicit identity/anchored-offset placement for historical source time, rejecting missing anchors and duration overflow. The initial runner declares `preserve_source_time`: importing context does not silently rewrite public observation timestamps. The mapping library does not implement historical public-feed playback.

**Declared guarantee:** the same frozen version, seed, epoch, runtime build, and scripted inputs produce the same scheduled event ordering, observation source instants, seeded positions/footprints, and modeled telemetry values despite pause/speed changes. Tests compare persisted observations between executions. Database-generated IDs, run-qualified IDs, receipt/processing/audit/lifecycle times, transport timing/IDs, and uncontrolled external commands are excluded. Dynamic operator actions require the same ordered virtual-time input to claim equivalent results. GPU/physics/cross-platform bit identity is not claimed.

**Recorded replay** reads stored evidence through the existing `/picture` endpoints without executing the engine. **Re-simulation** creates a new execution from frozen inputs. The event inspection endpoint is an execution ledger, not a complete recording of every sensor payload. Full recordings, historical control-state reconstruction, checkpoints, and after-action evaluation remain S3 work; the existing scrubber is not promoted to that guarantee.

## S0.2: renderer-independent coordinates and fidelity

`internal/simulation` specifies horizontal WGS84 (`EPSG:4326`) with explicit longitude then latitude, separate height/datum, and WGS84 ECEF to a right-handed local ENU frame: X east, Y north, Z up, metres. The origin, `wgs84-ecef-enu-v1` transform, axes, order, units, and orientation convention are recorded in `SimulationFrame`.

The orientation convention is heading clockwise from north, pitch nose up, and roll right, all in degrees. It is a contract for future adapters, not a physical attitude dynamics model. Local coordinates are bounded to a 500 km radius. A different engine axis system must use an explicit adapter transform.

Coordinate conversion requires a finite height with `WGS84_ELLIPSOID`. Missing height remains null. MSL, EGM2008 orthometric elevation, AGL, unknown datums, feet, unsupported axes/order, and unsupported transform versions are rejected rather than relabeled. In particular, the NOAA sample retains its EGM2008 metadata and cannot silently become an ellipsoidal scene surface. Geoid/terrain-assisted vertical conversion remains a separately versioned future transform. Existing two-dimensional YAML fixtures need no invented altitude or scene origin.

Fixtures verify known equatorial/polar ECEF axes to 1 micrometre, positive east/north/up directions, longitude/latitude round trips within `1e-9` degrees, and height/local round trips within `1e-5` metres, including poles and antimeridian cases. These numerical tolerances describe this transform implementation, not source-data accuracy. Equations follow [ESA Navipedia's ECEF/ENU transformation](https://gssc.esa.int/navipedia/index.php/Transformations_between_ECEF_and_ENU_coordinates).

Every source/sensor, controlled asset, and observed actor template carries a model version, profile, and stated simplifications. Current profiles are `visual_only`, `abstract_behavior`, and `kinematic`. Legacy YAML receives explicit adapter defaults documenting scripted 2D paths and absent physical dynamics/validated sensors. Specialized validated-model claims are rejected in this increment. PostGIS continues owning authoritative spatial database predicates.

## S0.3: one package format and an offline loader

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

## Verification

The gate covers Go tests/vet; spatial/time/package corruption fixtures; restart after author edits/deletion; speed/pause/seed equivalence; exact frozen numeric precision; real PostGIS scope/immutability and reference-package checks; the full existing integration regression suite; Linux race checks for affected services; OpenAPI lint/type generation; production UI build; mocked browser regressions; and the existing three real-backend browser workflows.

The next task is **S0.4**, followed by the M2 gate and return to P1.5–P1.6. Engine selection remains deferred to S1 evaluation.
