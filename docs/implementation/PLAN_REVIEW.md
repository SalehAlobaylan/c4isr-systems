# Implementation plan review

**Recovery follow-up — 2026-10-23:** The [persistent recovery acceptance](DELIVERY_RECOVERY_ACCEPTANCE.md) and [delivery ledger](RECOVERY_STATUS.json) record the bounded installation, canonical review correction, real imagery/geography, exercises, replay and restore evidence. Restricted sources and actual human review retain explicit gates. The review below is historical evidence for its own source snapshot.

**Delivery conclusion superseded, 2026-10-23:** The [delivery audit](DELIVERY_AUDIT_2026_10_23.md) found unactivated persistent workflows, an incomplete real-terrain handoff, and a confirmed same-human principal-alias defect in assessment review. The checks below remain historical evidence for their finite cases; their broad product-readiness conclusion is withdrawn.

Reviewed 2026-10-22 against `EMAD_ARCHITECTURE_AND_DIRECTION_v2.md`, `EMAD_FINAL_DATA_STRATEGY_v2.md` and all 32 F0–S3 tasks, plus the selected S4.1 increment. This covers task traceability, the current working tree's domain and integration paths, contracts, UI behavior, operating instructions and verification evidence. Earlier delivery reports remain historical records of their own versions and fixtures.

The delivered scope follows the two-pillar direction. Public evidence and external track hypotheses remain distinct from exercise actors and controllable assets. Simulation authoring, ordinary perception and evaluator truth have separate authorization paths. Existing infrastructure is reused; physics, raster processing, asset validation and archive construction stay outside synchronous API execution. Three.js is the evaluated first viewport adapter; it does not establish specialist physics fidelity or a universal engine choice.

## Findings fixed in this review

1. **Live authorization checks stopped after the first refresh.** A fresh identity context lost its refresh callback, so a later check could use cached grants. `access.Refresh` now retains the live callback. Unit coverage and actual credential revocation during recording artifact retrieval verify the second check reaches current identity state.
2. **Database failure looked like a rejected credential.** Identity lookup converted every query error to unauthorized, which made workers treat an outage as revocation. Only a missing/inactive credential now produces that result; infrastructure failures remain internal errors and can follow the worker's bounded retry policy. Error responses still conceal database details.
3. **Recorded output could finish after access or retention expired.** Frame, event, checkpoint and evaluation reads now recheck current authority, the exact resource and the archive's read window before returning output and recording the successful read. Post-fetch revocation tests cover frame/events/checkpoint, and a retention-change test covers replay. Failure returns an error response without the archived body.
4. **Evaluation sums depended on map order.** Sorting the final metric names did not stabilize floating-point position-error reduction. Published samples now reduce in sorted identity order. Repeated evaluation over deliberately non-associative floating-point inputs verifies identical serialized reports. Metric definitions remain evaluation v2; existing immutable reports are not rewritten.
5. **Moving-track filters drifted from browser history.** The AOI field now follows URL changes on back/forward navigation, with an assembled-backend browser regression.
6. **Missing cloud cover displayed with a percent sign.** Unknown scene cloud cover now displays as unknown, without suggesting a measurement.
7. **Staging validation failed in Windows Git Bash.** Native host mount paths and container paths are now handled separately. The existing Compose, Prometheus configuration/rules, Alloy and Loki checks pass on the actual Windows host. Temporary validation secrets are removed by their explicit filenames.

8. **The mobile header clipped controls and navigation.** Small screens now keep complete status, identity/help controls and a separate navigation row. The role remains readable in the identity menu. Real-backend mobile assertions check control bounds and role access.

Current status links were also added to the historical architecture/product/phase documents so their earlier pending-task statements cannot be mistaken for the current roadmap. Applied SQL migrations 1–38 and the migration script are unchanged. No new migration, dependency, provider activation or product workstream was introduced.

## Coverage of the plan

### F0 — All six tasks

- **F0.1:** The baseline inventory and migration audit remain in `F0_1_*`; the historical phase plan is explicitly distinguished from the adopted v2 execution plan.
- **F0.2:** `platform/access`, HTTP identity resolution and `emad-admin` provide human/service identities, grants, source restrictions, expiry, rotation and explicit truth delegation. This review adds repeated-refresh and infrastructure-failure coverage.
- **F0.3:** Ownership migrations, `db.Scoped` and forced RLS preserve existing identifiers while separating resources and relations. Applied migrations are checked against the earlier recorded hashes.
- **F0.4:** Domain services, scoped SQL and integration negative matrices enforce workspace, partition, run and relation boundaries. Public entities cannot receive exercise commands.
- **F0.5:** Realtime topic policy, resource binding and the presentation allowlist separate subscriptions and omit hidden identities. Revocation/reconnect and serialized-payload cases remain in the regression suite.
- **F0.6:** `WorkspaceProvider`, scoped query keys and workspace UI isolate caches and cancel obsolete requests. Real-backend workspace, authoring and recording navigation remain covered.

### P1 — All six tasks

- **P1.1:** `publicingest` owns reviewed admissions, immutable evidence, trusted retrieval locations, persistent quotas/leases/backoff and recoverable publication. Source identity, rights and coverage remain visible.
- **P1.2:** `publicevents`, observation semantics and PostGIS event rules separate point events, reported/derived areas and moving observations. Revisions and withdrawals preserve history rather than fabricate tracks.
- **P1.3:** `reference` retains versioned reference/terrain metadata, attribution, coordinates, processing and checked artifacts for the bounded region.
- **P1.4:** The native USGS adapter and recorded/live-gated tests preserve provider identity, origin/revision time, quality, corrections and withdrawals.
- **P1.5:** The NOAA-20 VIIRS adapter preserves exact CSV evidence, acquisition time, product and categorical quality. Thermal detections are not labeled fire perimeters.
- **P1.6:** Public evidence drill-down and history preserve originals, revisions, source lineage, provider-effective time and EMAD knowledge/publication time.

### S0 — All four tasks

- **S0.1:** Frozen scenario definitions, version/run identity and explicit virtual-clock contracts preserve restart inputs and separate wall receipt time.
- **S0.2:** WGS84/ECEF/ENU transforms, units, axes, vertical-datum and fidelity validation support scene-space calculations without replacing authoritative PostGIS queries.
- **S0.3:** `contextpackage` validates manifest identity, hashes, rights, bounds, retained bytes and declared external dependencies. Reference-only packages cannot claim offline reproduction.
- **S0.4:** The existing runner adapter and `exerciseingest` retain run-scoped synthetic ingress and sealed actor mappings; retained-context/headless and boundary cases exercise both pillars' independence.

### P2 — All four tasks

- **P2.1:** Private content-addressed artifacts, bounded Copernicus STAC discovery and imagery metadata preserve exact records and current/original rights.
- **P2.2:** The independent imagery worker and bounded raster preparation/NDVI baseline pin source hashes, grids, masks, algorithm/build and limitations. Synthetic acceptance pixels remain explicitly labeled.
- **P2.3:** Assessment revisions and independent human review separate a proposal, model score, analyst confidence, publication and evidence lineage. An AI method label cannot publish a reviewed fact automatically.
- **P2.4:** Preview/approval/export freezes exact selected reference and imagery dependencies with AOI, time, transforms, rights and omissions. The offline bundle validator and headless integration consume the same manifest.

### P3 — All three tasks

- **P3.1:** `externaltracks` records source-bound hypotheses, association decisions, late/jump/gap handling and freshness without asset/command authority.
- **P3.2:** Bounded OpenSky and AISStream adapters have native fixture/protocol, budget, outage and isolation coverage. This is not acceptance of operational live access: the written agreement, server key, reviewed rights and admissions remain required.
- **P3.3:** The selected natural-hazard pilot uses METAR station samples and GDACS earthquake bulletins, with unknown values, cited-location/time uncertainty and shared upstream lineage preserved. Reports use reviewable assessments rather than automatic hazard or identity assertions.

### S1 — All three tasks

- **S1.1:** `modelassets`, the separate Go/Node validator and the portable glTF profile retain licenses, source/engine artifacts, immutable models and separate actor/environment definitions. Import does not instantiate actors.
- **S1.2:** The retained Three.js/Babylon comparison and engine decision establish the initial bounded viewport choice. Hardware numbers remain measurements of their recorded fixture and machine, not a new scale claim.
- **S1.3:** Frozen georeferenced Studio scenes, 3D and accessible geographic/property views, scoped model loading and resource disposal preserve protected authoring and coordinate/fidelity declarations.

### S2 — All three tasks

- **S2.1:** The compiled runtime owns the world/clock loop; the broker owns lifecycle, bounded service leases, immutable configuration and protected snapshots. The API does not run simulation ticks.
- **S2.2:** Abstract scheduled observers and controlled self-reporting preserve configured drops, delays, noise, quality, actor lifecycle and deterministic sample identity. They do not claim physical sensing accuracy.
- **S2.3:** The durable native gateway validates run recipients and publishes synthetic observations/telemetry through existing services into the Exercise COP, with deduplication and private mappings.

### S3 and S4.1 — All four delivered tasks

- **S3.1:** Existing assets, missions/tasks, commands and audit support permitted virtual tasking, ACK/outcomes, cancellation, deadlines and idempotency inside one run. There is no live actuator route.
- **S3.2:** Separate operator/truth journals, verified private artifacts, immutable archive indexes, checkpoints and virtual-time/sequence playback preserve historical state independently of the live COP. This review closes the delayed-read authorization/retention gap.
- **S3.3:** Evaluation v2 defines perception/workflow operands, missing denominators, limitations and clock basis over sealed recordings. Explicit evaluator grants and deliberate numeric release remain separate from ordinary playback. This review stabilizes aggregate byte reproducibility.
- **S4.1:** Immutable sensor-quality presets retain exact numeric settings and limitations in frozen scenes; custom edits clear attribution and version changes do not rewrite earlier scenes.

## Verification and operating limits

The [review evidence](evidence/PLAN_REVIEW_ACCEPTANCE.json) records the final executed checks, counts, source fingerprints and unchanged migration hashes. It supplements the earlier [delivery readiness record](IMPLEMENTATION_REVIEW_READINESS.md). Verification includes the existing Go tests/vet, real disposable PostGIS/S3 integration suite, assembled-backend browser workflows, browser regressions, affected-package Linux race checks, API/tool/runtime builds, production UI build, model-validator/no-GPU/raster checks, contract generation/lint and staging configuration validation.

Final results: **84 integration cases** (56 top-level and 28 subcases), **16 real-backend browser checks**, **76 browser regressions**, six model-validator checks and two raster-preparation checks passed. Both no-GPU scene adapters completed their 20,000-step fixtures. Go tests/vet, Linux race checks, executable/UI builds, generated-code reproduction, OpenAPI/Protobuf lint, Protobuf compatibility against the committed baseline, shell syntax and Windows staging configuration checks passed. All 38 applied migrations and the two other protected baseline files retain their recorded hashes.

Live-provider tests requiring credentials are opt-in; skips do not establish live-provider acceptance. The separate real-browser gate is run explicitly. The existing UI bundle-size advisory remains. Scale, runtime/fidelity and archive/retention ceilings remain those of the accepted profiles; the checks do not certify production load or physical-model accuracy.

Rebuild/restart the API, UI and recording worker to take up the fixes. Other workers using the shared identity package should also be rebuilt/restarted so repeated live checks and infrastructure-error handling use the corrected code; `task build:all` builds all Go executables. Preserve the exact-build admission policy whenever replacing the native runtime binary. No production database, live provider or deployed process was changed by this review.

The original 32-task scope and S4.1 have passed the final checks and are ready for user review within their documented acceptance profiles. Future S4 expansion, deployment credentials/admissions/supervision, additional terrain/asset scale and specialist-model validation remain explicit follow-on work. There is no claim that a review or test suite proves the absence of every defect.
