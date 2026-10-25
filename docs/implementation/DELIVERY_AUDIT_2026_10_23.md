# EMAD delivery audit — 2026-10-23

**Subsequent recovery:** See [delivery recovery acceptance](DELIVERY_RECOVERY_ACCEPTANCE.md) and the [current ledger](RECOVERY_STATUS.json). This audit remains evidence of the earlier failed delivery; its counts describe that earlier snapshot.

**Conclusion: delivery is incomplete. The earlier blanket conclusion that all 32 tasks plus S4.1 were delivered and ready for review was too broad and should not be used as the current product status.**

There is substantial working implementation. The failure was accepting technical profiles and disposable-fixture demonstrations as sufficient evidence of a configured, usable installation. The audit also confirms a real assessment-review correctness defect and a missing real-terrain preparation/handoff. Changing permissions or adding a Home page does not close those gaps.

This report covers the original 32 tasks, the user-selected S4.1 increment, the two authoritative v2 proposals, the current dirty working tree, and the observed local development database. Earlier Phases 0–21 remain the historical documented baseline; this audit does not recertify that entire older product.

## 1. What can be quantified honestly

- **33 task records reviewed:** corresponding implementation/artifact evidence was located for all 33. This does not mean every requirement is correct or delivered.
- **8 technical foundation/contract/decision records evidenced:** F0.1–F0.5, S0.1–S0.2 and S1.2. This category includes mathematical contracts and a renderer decision, not eight populated user workflows.
- **1 partial default rollout:** F0.6. The recent Home/module navigation fix is present, but the installed account/resources are still legacy-only.
- **24 capability records have profile evidence but are not activated in the local installation.** These include data-facing public and native exercise features. P2.3 and P3.3 additionally share a confirmed review correctness defect.
- **0 of the 2 authoritative end-to-end demonstrations are installed persistently.** The automated profiles are real evidence, but their temporary data is not user content.
- **45 of 53 tables introduced in migrations 19–38 are empty.** The eight populated new tables contain workspace/environment/scope, principal/credential/grant and authorization support.
- **12 findings and 9 recovery packages** are recorded below. Findings overlap; adding their counts is not a measure of independent implementation effort.

No defensible overall completion percentage follows from these counts. A coordinate-library task, an identity migration and a real imagery workflow have different scope and effort. Table population is evidence of activation, not a code-completion metric.

## 2. Observed persistent installation

Database snapshot: 2026-10-23T00:36:20+03:00 (Asia/Riyadh), also retained as 2026-10-22T21:36:20Z UTC. Queries ran with PostgreSQL default_transaction_read_only=on as the local administrative role; row-level filtering therefore did not hide content from the inventory. Schema migration **38** is applied.

- One registered operator, one human principal and one principal grant.
- Three data scopes, all default/local/EXERCISE_OBSERVATION: legacy and two older scenario runs.
- No PUBLIC_EVIDENCE, EXERCISE_INPUT or EXERCISE_TRUTH scope; no source-bound connector or native worker principal.
- No public admissions/evidence/events/reference features, imagery scenes/jobs/artifacts, approved context packages, models/templates/environments/scenes, native runs/exercises, recordings/evaluations/released reports or sensor presets.
- Existing exercise content includes four synthetic sources, 41 observations, one track, two classifications, two geofences, one alert and two scenario runs. These records were preserved; they do not represent the new public/native delivery.

The full non-secret table counts, resource tuples and grant metadata are retained in [the machine-readable audit](../../docs/implementation/evidence/DELIVERY_AUDIT_2026_10_23.json). No credential hashes or bearer tokens are included.

## 3. What actually exists and should be preserved

The work is not empty scaffolding. Implemented paths include persisted scoped identities and RLS; service authorization; provenance and revision-aware ingestion; native provider adapters; real reference sample files; immutable packages; raster preparation and NDVI jobs; assessment revisions; model validation; a Three.js authoring viewport; an independent native kinematic runtime; synthetic perception and virtual tasking; and archive/replay/evaluation code. Recent builds and fixture-backed integration/browser checks exercise many of these paths.

The retained Natural Earth/NOAA subsets are real prepared reference data. Copernicus files contain real recorded catalogue metadata, not retained satellite raster bands. USGS has documented historical live acceptance in a disposable database. FIRMS/moving/report fixtures do not establish live activation. The shipped abstract-wing model is one triangle; the ground tile is two triangles. Their declarations are honest test geometry, not a meaningful aircraft/environment library.

### Fresh behavioral checks

1. **Prepared terrain probe:** both retained artifact hashes matched. Passing elevation.json to the actual scene loader at the declared Jeddah frame failed with an unresolved EGM2008-to-WGS84-ellipsoid conversion. Passing the whole places.json failed the 10 km radius. The audit did not relabel heights or disable validation.
2. **Independent-review probe:** the existing P2 integration test was rerun on disposable PostGIS/S3 through a Go overlay that added only a diagnostic log. It successfully published using source-reviewer and p2-independent-reviewer, both registered to operator-01. The source guard checks principal IDs; it does not enforce distinct registered humans. The test passed while confirming the defect.
3. The preceding startup follow-up passed 80 browser regressions, 16 real-backend browser checks and the production build. Those results are reused as recent evidence; they do not close the above installation/correctness gaps.

## 4. Findings, impact and required correction

### D01 — Technical profile completion was reported as product delivery

**Class:** Acceptance and reporting. **Delivery priority:** Blocking. **Affected tasks:** F0.6, P1.6, P2.4, S1.3, S3.3.

The architecture requires two end-to-end user demonstrations. Neither exists in the persistent installation. Plan/readiness reports treated fixture-based milestone passes as completion of the product delivery. The recent Home/navigation repair does not supply data, services or these demonstrations.

**Impact:** Migrate/restart instructions reasonably led the user to expect usable new product pillars; normal installation acceptance was not performed.

**Required:** Replace blanket completion with implementation/profile/installed states and require persistent user journeys before closing delivery.

**Evidence:** [EMAD_ARCHITECTURE_AND_DIRECTION_v2.md](../../EMAD_ARCHITECTURE_AND_DIRECTION_v2.md) (line 414); [EMAD_ARCHITECTURE_AND_DIRECTION_v2.md](../../EMAD_ARCHITECTURE_AND_DIRECTION_v2.md) (line 418); [EMAD_IMPLEMENTATION_PLAN_v2.md](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (line 5).

### D02 — No public, authoring, evaluator or worker access setup is installed

**Class:** Unfinished activation. **Delivery priority:** Blocking. **Affected tasks:** F0.2, F0.6, P1.1, S2.1, S3.3.

The inventory contains one human principal and one wildcard legacy exercise grant. All three scopes are EXERCISE_OBSERVATION. There is no public/input/truth scope or connector/runtime/worker principal. The restriction itself is intentional; the explicit rollout is missing.

**Impact:** Normal accounts cannot use new resources and required workers cannot authenticate with their intended scoped identities.

**Required:** Provide explicit trusted role/service setup, including distinct registered humans for independent review; verify effective access without implicit privilege expansion.

**Evidence:** [internal/platform/access/store.go](../../internal/platform/access/store.go) (line 46); [internal/platform/access/catalog.go](../../internal/platform/access/catalog.go) (line 13).

### D03 — Prepared and test data was never installed as persistent product content

**Class:** Unfinished activation. **Delivery priority:** Blocking. **Affected tasks:** P1.1, P1.3, P1.4, P1.5, P1.6, P2.1, P3.3, S1.1, S1.3.

Of 53 tables added by migrations 19â€“38, 45 are empty. The eight populated new tables are identity/scope/authorization support. New public, reference, imagery, context, model/template/scene, native exercise, recording/evaluation and preset content is absent. Older synthetic exercise records remain.

**Impact:** The new UI has no persistent content to show. Migrations and menu changes cannot populate it.

**Required:** Perform idempotent provenance-preserving admissions/imports/captures in the intended persistent workspace and provide a visible import receipt.

**Evidence:** [internal/integration/harness_test.go](../../internal/integration/harness_test.go) (line 65); [scripts/data/import_reference_sample.ps1](../../scripts/data/import_reference_sample.ps1) (line 2).

### D04 — Supplied stacks do not package new storage and workers

**Class:** Missing deployment integration. **Delivery priority:** Blocking. **Affected tasks:** P1.1, P2.1, P2.2, S1.1, S2.1, S3.2.

Development Compose starts PostgreSQL. Staging packages the API/UI/database and observability, but no private artifact store or EMAD worker profiles. The server image contains the API and goose. Separate connector, imagery, model-import, runtime and recording executables require manual credentials/configuration/launch.

**Impact:** Rebuilding API/UI does not activate retrieval, processing, simulation or archiving. Unknown external S3 setup is not assumed absent; no operational handoff was verified.

**Required:** Deliver a supported persistent local profile with private blobs and supervised/on-demand workers, preserving separate identities and exact-build admission.

**Evidence:** [deployments/docker-compose.yml](../../deployments/docker-compose.yml) (line 4); [deployments/staging/docker-compose.yml](../../deployments/staging/docker-compose.yml) (line 41); [deployments/Dockerfile.server](../../deployments/Dockerfile.server) (line 25); [apps/emad-runtime/main.go](../../apps/emad-runtime/main.go) (line 43).

### D05 — The environmental intelligence proof uses synthetic pixels

**Class:** Missing real-data acceptance. **Delivery priority:** High. **Affected tasks:** P2.1, P2.2, P2.3, P2.4.

Real STAC catalogue metadata is retained and NDVI computation/preparation code exists. The end-to-end integration constructs 4Ã—4 synthetic spectral arrays. No real retained raster pair, installed environmental result or real-data reviewed assessment demonstrates the promised analyst workflow. Preparation starts from already retained local bands.

**Impact:** The numerical and lineage mechanics are tested for their declared fixture, but a useful real-image intelligence demonstration remains undelivered.

**Required:** Complete one permitted real-raster pair through masks/comparability, processing, canonical independent review and approved context export.

**Evidence:** [internal/integration/imagery_test.go](../../internal/integration/imagery_test.go) (line 115); [scripts/data/prepare_imagery_subset.py](../../scripts/data/prepare_imagery_subset.py) (line 3); [testdata/imagery/retrieval.json](../../testdata/imagery/retrieval.json) (line 5).

### D06 — Prepared real geography fails the current Studio handoff

**Class:** Missing transformation implementation. **Delivery priority:** High. **Affected tasks:** P1.3, S0.2, S0.3, P2.4, S1.3.

Fresh probes passed the unmodified retained files to GeometryFor at the documented Jeddah frame. NOAA elevation fails the unresolved EGM2008 datum; the full places subset fails the 10 km radius. Export retains whole artifacts without clipping. The acceptance scene used synthetic 3Ã—3 ellipsoidal relief and one manually subset reference point.

**Impact:** The central real geography â†’ approved package â†’ scene path is incomplete. Correct rejection safeguards should stay; the explicit derivative preparation is missing.

**Required:** Add versioned AOI subsetting and validated datum/height conversion, or choose a compatible permitted source; retain original bytes and transformation provenance.

**Evidence:** [internal/simscene/domain.go](../../internal/simscene/domain.go) (line 149); [internal/simscene/domain.go](../../internal/simscene/domain.go) (line 78); [internal/contextpackage/service.go](../../internal/contextpackage/service.go) (line 100); [internal/integration/studio_runtime_test.go](../../internal/integration/studio_runtime_test.go) (line 38).

### D07 — Geographic scene content is far narrower than the proposed world-building inputs

**Class:** Unclosed architecture coverage. **Delivery priority:** Scope gap. **Affected tasks:** P1.3, P2.4, S1.3.

Retained reference content is two generalized populated-place points plus coarse elevation. No OSM road/building import pipeline was located in reviewed paths. Studio skips non-point vectors and retains imagery/analysis as lineage rather than rendering imagery on terrain.

**Impact:** This supports a tiny coordinate proof, not a rich geographic environment. This is a proposal-coverage limitation; a full city engine was not a required initial S1 implementation.

**Required:** Explicitly bound the next useful geographic region/layers/omissions. A displayed basemap must not be described as ingested or scene-ready GIS.

**Evidence:** [EMAD_FINAL_DATA_STRATEGY_v2.md](../../EMAD_FINAL_DATA_STRATEGY_v2.md) (line 62); [internal/simscene/domain.go](../../internal/simscene/domain.go) (line 202); [internal/simscene/domain.go](../../internal/simscene/domain.go) (line 225).

### D08 — 3D authoring is static; live 3D movement/playback and spatial truth review are absent

**Class:** Unclosed product capability. **Delivery priority:** Scope gap. **Affected tasks:** S1.3, S2.3, S3.2, S3.3.

SceneViewport consumes a frozen scene and positions actors from authored input, with no runtime-snapshot input. Runtime lifecycle and the 2D Exercise COP are separate. Recording playback uses a longitude/latitude SVG plot; evaluation offers metrics/numeric release rather than spatial truth overlays. The static model profile rejects animations/skins.

**Impact:** The kinematic runtime genuinely exists, but the visible moving 3D experience does not. Static S1/abstract S2 acceptance cannot stand for the full long-term simulator.

**Required:** Scope a role-safe live/recorded display adapter, usable starter assets and spatial after-action views. Preserve the distinction between authoring, operator perception and privileged truth.

**Evidence:** [apps/operator-ui/src/features/simulation/SceneViewport.tsx](../../apps/operator-ui/src/features/simulation/SceneViewport.tsx) (line 43); [apps/operator-ui/src/features/simulation/SceneViewport.tsx](../../apps/operator-ui/src/features/simulation/SceneViewport.tsx) (line 36); [apps/operator-ui/src/routes/recordings.tsx](../../apps/operator-ui/src/routes/recordings.tsx) (line 38); [apps/emad-asset-validator/validate.mjs](../../apps/emad-asset-validator/validate.mjs) (line 47).

### D09 — Independent assessment review checks principal alias rather than registered human

**Class:** Confirmed correctness defect. **Delivery priority:** Blocking. **Affected tasks:** P2.3, P3.3.

AuthorID stores principal ID and Act rejects only equal principal IDs. Multiple human principals can share operator_id. The existing test creates source-reviewer and p2-independent-reviewer for operator-01 and successfully reviews/publishes. A fresh disposable PostGIS/S3 rerun confirmed this; the overlay added only a diagnostic log.

**Impact:** A second authorized principal of the same registered operator passes the independent-human guard. Appropriate provisioned permissions are required; no ability for an ordinary user to create aliases is asserted.

**Required:** Use canonical registered human identity for authorship/review independence while preserving principal/credential audit identity; deny same-human aliases and accept genuinely different registered humans.

**Evidence:** [internal/assessments/review.go](../../internal/assessments/review.go) (line 156); [internal/assessments/review.go](../../internal/assessments/review.go) (line 268); [internal/integration/imagery_test.go](../../internal/integration/imagery_test.go) (line 317); [internal/platform/access/provision.go](../../internal/platform/access/provision.go) (line 100).

### D10 — Several live integrations remain conditional or unactivated

**Class:** External prerequisites. **Delivery priority:** Conditional. **Affected tasks:** P1.5, P3.2, P3.3.

FIRMS live verification is key-gated. Repository rollout documents leave OpenSky/AISStream access/keys/rights unresolved. METAR/GDACS here have fixture/protocol acceptance. USGS has a documented historical live smoke and should not be misreported as never tested.

**Impact:** Disabled coverage is a delivery limitation, but no external key/permission should be invented or synthetic evidence represented as live data.

**Required:** Track and close each source's prerequisites separately, recheck terms at activation and perform bounded persistent acceptance. This audit does not establish current legal/provider eligibility.

**Evidence:** [internal/integration/firms_live_test.go](../../internal/integration/firms_live_test.go) (line 15); [internal/integration/usgs_live_test.go](../../internal/integration/usgs_live_test.go) (line 10); [docs/implementation/P2_4_P3_1_2_CONTEXT_AND_MOVING.md](../../docs/implementation/P2_4_P3_1_2_CONTEXT_AND_MOVING.md) (line 40).

### D11 — Database health/backups do not cover the new dependency chain

**Class:** Missing operational handoff. **Delivery priority:** High. **Affected tasks:** P1.1, P2.1, P2.2, S2.1, S3.2.

The default health handler pings PostgreSQL only. Feature/job health logic exists, but API health does not establish configured sources, storage or running workers. Bundled backups/restores handle PostgreSQL while imagery/models/recordings require external blobs. Tiny profile measurements do not establish representative long-running operation.

**Impact:** A green connection can coexist with unusable pillars; database-only restore cannot recover missing dependent blobs.

**Required:** Deliver feature-readiness diagnostics, paired database/blob recovery rehearsal and a bounded representative operational workload.

**Evidence:** [internal/app/app.go](../../internal/app/app.go) (line 526); [scripts/ops/backup.sh](../../scripts/ops/backup.sh) (line 20); [docs/implementation/P2_1_3_IMAGERY_INTELLIGENCE.md](../../docs/implementation/P2_1_3_IMAGERY_INTELLIGENCE.md) (line 25).

### D12 — Normal setup and complete workflows remain fragmented

**Class:** Missing user-facing handoff. **Delivery priority:** High. **Affected tasks:** F0.6, P1.1, P2.3, P2.4, S1.3, S2.1, S3.3.

The old default launch hid major new routes. Recent Home/navigation and a single optional analyst setup guide repair discovery, but full setup still spans admissions, service credentials, storage, imports, runtime build hashes/attachment and separate reviewer/evaluator identities. No integrated persistent setup/acceptance path was verified.

**Impact:** Following migrate â†’ dev:api â†’ dev:web still does not deliver the intended analyst-to-exercise experience without additional specialist setup.

**Required:** Provide guided setup or an explicit trusted bootstrap command, persistent starter content and a normal-user walkthrough; a general-purpose IAM administration product is not necessary.

**Evidence:** [docs/implementation/LOCAL_STARTUP.md](../../docs/implementation/LOCAL_STARTUP.md) (line 31); [apps/operator-ui/src/routes/simulation-studio.tsx](../../apps/operator-ui/src/routes/simulation-studio.tsx) (line 54); [apps/operator-ui/src/routes/home.tsx](../../apps/operator-ui/src/routes/home.tsx) (line 53).

## 5. Task-by-task reassessment

The classifications below describe delivery evidence, not weighted completion or estimated effort. The complete original bullet requirements and original completion text are preserved per task in the JSON ledger. A source/test path is traceability evidence, not automatic proof that its assertions are sufficient.

### F0.1 — Verify the baseline and publish the migration inventory

**Current delivery class:** Technical foundation evidenced.

**Implemented evidence:** Baseline inventory, migration audit and executed verification records.

**Missing/limited:** Historical checks did not establish a populated two-pillar installation.

**Delivery exit:** Retain the baseline and add persistent installation acceptance.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 160); [implementation/artifact](../../docs/implementation/F0_1_BASELINE.md); [verification/profile evidence](../../docs/implementation/F0_1_INVENTORY.json).

### F0.2 — Define principals, resource context, and contract vocabulary

**Current delivery class:** Technical foundation evidenced.

**Implemented evidence:** Human/service principals, resource grants, credential lifecycle and administrator CLI.

**Missing/limited:** Only one legacy human principal and one exercise grant are installed; no analyst/author/evaluator/worker identities.

**Delivery exit:** Explicitly provision scoped roles and service identities; distinct registered humans are required for independent review.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 172); [implementation/artifact](../../internal/platform/access/store.go); [verification/profile evidence](../../internal/integration/provisioning_cli_test.go).

### F0.3 — Add ownership persistence and backfill safely

**Current delivery class:** Technical foundation evidenced.

**Implemented evidence:** Ownership persistence, backfill/quarantine, referential checks and forced RLS; local schema reaches 38.

**Missing/limited:** Only three legacy exercise scopes exist. Schema migration does not import new content.

**Delivery exit:** Preserve migration history and original evidence; create authorized public/input resources explicitly.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 184); [implementation/artifact](../../db/migrations/00020_resource_ownership.sql); [verification/profile evidence](../../internal/integration/ownership_migration_test.go).

### F0.4 — Enforce boundaries in application services and queries

**Current delivery class:** Technical foundation evidenced.

**Implemented evidence:** Scoped service/query paths and cross-resource negative cases.

**Missing/limited:** No activated local public/native workflow demonstrates these boundaries to the user.

**Delivery exit:** Keep boundaries enabled while validating installed workflows; do not grant implicit global access.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 197); [implementation/artifact](../../internal/platform/db/scoped.go); [verification/profile evidence](../../internal/integration/scope_test.go).

### F0.5 — Separate realtime delivery and protect metadata

**Current delivery class:** Technical foundation evidenced.

**Implemented evidence:** Scoped versioned streaming, payload filtering, revocation and reconnect behavior.

**Missing/limited:** A healthy socket is not evidence that public/native producers are running. A cancelled development handshake was repaired separately.

**Delivery exit:** Demonstrate installed producers, reconnect and revocation with current permissions.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 209); [implementation/artifact](../../internal/realtime/hub.go); [verification/profile evidence](../../internal/integration/realtime_boundaries_test.go).

### F0.6 — Separate workspaces and close the compatibility gate

**Current delivery class:** Partial default rollout.

**Implemented evidence:** Workspace/cache isolation and Public/Exercise selection; Home/direct module links were added after the complaint.

**Missing/limited:** The default account still lacks public and authoring scopes. Earlier new routes were not discoverable.

**Delivery exit:** Verify ordinary startup with explicitly configured accounts and visible, populated workflows.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 221); [implementation/artifact](../../apps/operator-ui/src/components/WorkspaceProvider.tsx); [verification/profile evidence](../../apps/operator-ui/tests/e2e/product-entry.spec.ts).

### P1.1 — Source admission, provenance, and connector execution

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Admissions, immutable evidence, provenance, quotas/backoff/checkpoints and bounded execution.

**Missing/limited:** No admission, connector state, connector run or public evidence is installed.

**Delivery exit:** Activate one reviewed source/AOI and its service worker; verify durable evidence, health and recovery.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 242); [implementation/artifact](../../internal/publicingest/service.go); [verification/profile evidence](../../internal/integration/public_ingestion_test.go).

### P1.2 — Separate point events, areas, and moving observations

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Point/event/area/moving semantics, revision projections and spatial rules.

**Missing/limited:** Public event tables are empty; current public behavior is evidenced through fixtures.

**Delivery exit:** Verify persisted reports, corrections/withdrawals, spatial effects and zero fabricated tracks.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 255); [implementation/artifact](../../internal/publicevents/projector.go); [verification/profile evidence](../../internal/integration/public_events_test.go).

### P1.3 — Reference GIS and terrain catalogue

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Reference APIs and retained Natural Earth/NOAA samples with rights, coordinates and hashes.

**Missing/limited:** No reference data is imported. Prepared EGM2008 terrain needs conversion/cropping for Studio; no OSM road/building import pipeline was located.

**Delivery exit:** Import the real reference region and derive a separately versioned scene-ready subset with valid datum/extent.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 268); [implementation/artifact](../../internal/reference/service.go); [verification/profile evidence](../../internal/integration/reference_test.go).

### P1.4 — USGS event integration

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Native USGS adapter and a documented historical five-report live smoke.

**Missing/limited:** The live smoke used disposable PostGIS, not the persistent developer workspace.

**Delivery exit:** Admit and capture bounded USGS reports in the persistent installation and inspect their revisions/originals.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 280); [implementation/artifact](../../internal/publicingest/usgs.go); [verification/profile evidence](../../internal/integration/usgs_live_test.go).

### P1.5 — FIRMS thermal detection integration

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Bounded NOAA-20 VIIRS retrieval, CSV provenance and time/quality semantics.

**Missing/limited:** No installed FIRMS source/data; keyed live acceptance remains conditional.

**Delivery exit:** Obtain authorized configuration/key, activate one product and retain honest detection/coverage semantics.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 291); [implementation/artifact](../../internal/publicingest/firms.go); [verification/profile evidence](../../internal/integration/firms_live_test.go).

### P1.6 — Public evidence drill-down and historical view

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Public filters, source originals/revisions, bookmarks and historical time semantics.

**Missing/limited:** There is no installed public evidence to inspect.

**Delivery exit:** Exercise the ordinary UI on retained reports, including outages, originals and history.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 302); [implementation/artifact](../../apps/operator-ui/src/features/public-context/PublicContextPanel.tsx); [verification/profile evidence](../../apps/operator-ui/tests/integration/public-context.spec.ts).

### S0.1 — Scenario versions, runs, and explicit time

**Current delivery class:** Technical foundation evidenced.

**Implemented evidence:** Frozen scenario/run/clock contracts and deterministic lifecycle tests.

**Missing/limited:** No new simulation version/native run is retained locally; old legacy runs are not the new demonstration.

**Delivery exit:** Retain and execute an installed frozen version with declared seed/build/clock and pause/restart behavior.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 320); [implementation/artifact](../../internal/scenarios/frozen.go); [verification/profile evidence](../../internal/integration/simulation_foundations_test.go).

### S0.2 — Coordinate and fidelity contracts

**Current delivery class:** Technical foundation evidenced.

**Implemented evidence:** WGS84/ECEF/ENU transforms, axis/unit/fidelity checks and unsupported-datum rejection.

**Missing/limited:** A real EGM2008-to-ellipsoid terrain conversion is absent; rejection is correct.

**Delivery exit:** Preserve the contract and add a validated, versioned transformation for the selected real terrain.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 333); [implementation/artifact](../../internal/simulation/spatial.go); [verification/profile evidence](../../internal/simulation/spatial_test.go).

### S0.3 — Context package manifest and integrity

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Immutable manifests, retained bytes, rights, hashes and offline validation.

**Missing/limited:** No installed package. A smaller selected AOI retains whole artifacts rather than clipping them.

**Delivery exit:** Retain an approved real context package and explicitly derive crop/transform/omission metadata.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 344); [implementation/artifact](../../internal/contextpackage/service.go); [verification/profile evidence](../../internal/integration/context_export_test.go).

### S0.4 — Existing runner adapter and minimal synthetic ingress

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Existing runner adapter, synthetic ingress and headless frozen-context fixtures.

**Missing/limited:** No installed context-backed exercise demonstrates this path.

**Delivery exit:** Run retained context with feeds disabled, opaque observation identities and protected mappings.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 356); [implementation/artifact](../../internal/exerciseingest/service.go); [verification/profile evidence](../../internal/integration/headless_context_test.go).

### P2.1 — Artifact storage and imagery catalogue

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Private S3 artifacts, STAC metadata and bounded scene discovery.

**Missing/limited:** No installed imagery scene/preparation/artifact. Real catalogue metadata is not downloaded imagery; supplied stacks omit blob storage.

**Delivery exit:** Configure private blobs and retain two eligible scenes with permitted raster inputs.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 375); [implementation/artifact](../../internal/imagery/scenes.go); [verification/profile evidence](../../internal/integration/imagery_test.go).

### P2.2 — Reproducible environmental analysis

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Deterministic raster preparation and asynchronous NDVI with comparability/mask/retry checks.

**Missing/limited:** End-to-end acceptance uses synthetic spectral arrays; no real pair/result is installed. Preparation starts from already retained local bands.

**Delivery exit:** Demonstrate one licensed real-raster comparison with pinned inputs, declared limitations and repeatable worker output.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 386); [implementation/artifact](../../internal/imageryanalysis/ndvi.go); [verification/profile evidence](../../scripts/data/prepare_imagery_subset.py).

### P2.3 — Reviewable assessments and intelligence workspace

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Assessment proposal/revision/review/publish flows and imagery comparison UI.

**Missing/limited:** Confirmed defect: two principals of the same registered operator can review and publish that human's proposal. No local reviewed content exists.

**Delivery exit:** Enforce independence by canonical registered human identity; deny same-human aliases and accept genuinely different humans.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 398); [implementation/artifact](../../internal/assessments/review.go); [verification/profile evidence](../../internal/integration/imagery_test.go).

### P2.4 — Approved public-to-simulation export

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Preview/approval, frozen export, rights checks and offline consumption.

**Missing/limited:** No installed preview/package. Retaining imagery/vector lineage does not render it into a 3D environment.

**Delivery exit:** Export real approved evidence/context and disclose exactly what renders, remains a reference or requires conversion.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 410); [implementation/artifact](../../internal/contextpackage/service.go); [verification/profile evidence](../../internal/integration/context_export_test.go).

### P3.1 — External moving track projection

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** External hypotheses, history/associations, late/jump/gap handling and observational-only identity.

**Missing/limited:** No public moving segments or associations are installed.

**Delivery exit:** Populate a permitted bounded capture and verify freshness/history without control authority.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 429); [implementation/artifact](../../internal/externaltracks/projector.go); [verification/profile evidence](../../internal/integration/moving_test.go).

### P3.2 — One ADS-B provider and AISStream

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** OpenSky/AISStream adapters with protocol/fixture/budget tests.

**Missing/limited:** No live integration is installed; documented agreement/key/rights prerequisites remain unresolved.

**Delivery exit:** Close provider prerequisites and conduct bounded persistent live acceptance; label blocked integrations explicitly.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 438); [implementation/artifact](../../internal/publicingest/opensky.go); [verification/profile evidence](../../docs/implementation/P2_4_P3_1_2_CONTEXT_AND_MOVING.md).

### P3.3 — Weather context and selected reports

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Selected METAR/GDACS natural-hazard pilot, provenance/history and report assessment routes.

**Missing/limited:** No station/report data is installed; review shares the principal-alias independence defect.

**Delivery exit:** Activate the admitted pilot and fix canonical human review before accepting reviewed report delivery.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 447); [implementation/artifact](../../internal/publiccontext/service.go); [verification/profile evidence](../../internal/integration/weather_reports_test.go).

### S1.1 — Model and environment asset registry

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Immutable model registry, independent bounded glTF validation and separate templates/environments.

**Missing/limited:** No model/template/environment is installed. Samples are tiny abstract meshes; animations/skins are rejected by the static profile.

**Delivery exit:** Import a useful permitted starter set through normal validation and declare its visual/fidelity limitations.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 465); [implementation/artifact](../../internal/modelassets/service.go); [verification/profile evidence](../../apps/emad-asset-validator/validate.mjs).

### S1.2 — Engine evaluation

**Current delivery class:** Technical foundation evidenced.

**Implemented evidence:** Measured Three.js/Babylon comparison and scoped initial renderer decision.

**Missing/limited:** Benchmark contains 13 triangles/four draws with synthetic terrain; it cannot establish advanced terrain/physics scale.

**Delivery exit:** Retain the initial decision; re-evaluate representative workloads before larger simulation claims.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 474); [implementation/artifact](../../docs/implementation/S1_2_ENGINE_DECISION.md); [verification/profile evidence](../../apps/operator-ui/scripts/evaluate-engines.mjs).

### S1.3 — First georeferenced scene and Studio shell

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Frozen scenes, static georeferenced viewport, placement/properties and 2D alternative.

**Missing/limited:** No installed scene. Acceptance substituted synthetic relief/one point; unmodified prepared real files fail the loader.

**Delivery exit:** Resolve real datum/cropping derivatives and make a retained starter scene reachable through normal navigation.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 483); [implementation/artifact](../../internal/simscene/domain.go); [verification/profile evidence](../../internal/integration/studio_runtime_test.go).

### S2.1 — Runtime lifecycle and actor behavior

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Independent native kinematic executable, broker/lifecycle, leases/build admission and protected truth.

**Missing/limited:** No native run/service identity is installed. Queueing in UI does not launch/attach the separate process.

**Delivery exit:** Provide a trusted on-demand launch/attach path and demonstrate installed lifecycle/clock/failure behavior.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 501); [implementation/artifact](../../internal/simruntime/broker.go); [verification/profile evidence](../../apps/emad-runtime/main.go).

### S2.2 — Synthetic sensor adapters

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Deterministic abstract sensing with sample/drop/delay/noise settings and controlled telemetry.

**Missing/limited:** No installed native perception run. Scheduled abstract observers do not establish calibrated physical sensing.

**Delivery exit:** Demonstrate configured misses/delays in an installed exercise and preserve truth/perception separation.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 510); [implementation/artifact](../../internal/simruntime/sensors.go); [verification/profile evidence](../../internal/integration/native_perception_test.go).

### S2.3 — Exercise ingestion and operational picture

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Durable scoped native gateway and exercise read-model projections.

**Missing/limited:** No installed native exercise/projection. Studio is not connected to runtime snapshots for live 3D movement.

**Delivery exit:** Show accepted native synthetic observations in the installed COP, with duplicate/reconnect/cancellation behavior.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 519); [implementation/artifact](../../internal/simruntime/projection.go); [verification/profile evidence](../../internal/integration/native_gateway_test.go).

### S3.1 — Exercise-controlled workflows

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Run-bound virtual assets/tasking, ACK/outcomes/idempotency and existing operational audit.

**Missing/limited:** No installed native controlled asset/task dispatch; old legacy fixtures are a different path.

**Delivery exit:** Issue an authorized virtual task, observe a simulated outcome and retain correlated audit.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 538); [implementation/artifact](../../internal/nativeexercise/tasks.go); [verification/profile evidence](../../internal/integration/native_perception_test.go).

### S3.2 — Recording and replay

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Durable operator/truth journals, private archives, checkpoints and read-only historical frames.

**Missing/limited:** No installed recording/archive/worker. User replay is a 2D SVG frame plot rather than animated 3D replay.

**Delivery exit:** Archive/recover an installed run and replay it without side effects; state the 2D/3D scope explicitly.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 548); [implementation/artifact](../../internal/recordings/service.go); [verification/profile evidence](../../internal/integration/recordings_test.go).

### S3.3 — After-action evaluation

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Versioned perception/workflow metrics, audited evaluator reads and selective numeric release.

**Missing/limited:** No installed evaluation/report/evaluator. Broader spatial truth-overlay UX is absent.

**Delivery exit:** Evaluate the retained run using a separately granted evaluator and release only intended summaries.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 560); [implementation/artifact](../../internal/recordings/evaluation.go); [verification/profile evidence](../../apps/operator-ui/src/routes/recordings.tsx).

### S4.1 — Reusable sensor-quality presets

**Current delivery class:** Profile proof exists; not activated locally.

**Implemented evidence:** Immutable reusable sensor-quality presets pinned into frozen scenes.

**Missing/limited:** No installed preset. This one selected increment does not complete the open-ended S4 roadmap.

**Delivery exit:** Create/reuse presets in installed scenes while keeping future S4 expansion explicitly open.

**Traceability:** [active plan](../../EMAD_IMPLEMENTATION_PLAN_v2.md) (captured task line 590); [implementation/artifact](../../internal/simscene/presets.go); [verification/profile evidence](../../docs/implementation/S3_2_S3_3_S4_1_RECORDING_EVALUATION_PRESETS.md).

## 6. Reassessing milestone claims

Previous M0–M6 profile passes are historical test results. They must not be silently promoted into installed product acceptance. This audit does not assert that all seven finite test profiles failed.

- **M0 — Safe foundation:** technical evidence: Substantial isolation/compatibility implementation and negative-test evidence. Installed outcome: Default rollout has only legacy exercise access.
- **M1 — Public evidence workspace:** technical evidence: Reference/event/FIRMS fixture checks and historical USGS live smoke. Installed outcome: No installed public scope, admissions, reference imports or public evidence.
- **M2 — Simulation-ready inputs:** technical evidence: Frozen package/time/coordinates/headless profile evidence. Installed outcome: No installed package/version; real terrain is not scene-ready.
- **M3 — Intelligence demonstration:** technical evidence: Recorded catalogue and synthetic spectral workflow. Installed outcome: No real-raster demonstration; canonical independent-human review is defective.
- **M4 — 3D scene proof:** technical evidence: Tiny static scene with synthetic relief. Installed outcome: No installed scene; retained real inputs fail the loader.
- **M5 — Perception-driven exercise:** technical evidence: Native abstract runtime/sensor/gateway integration tests. Installed outcome: No installed native run or worker identity; no runtime-driven 3D viewer.
- **M6 — Integrated exercise demonstration:** technical evidence: Synthetic tasking, archives, replay and metric flows. Installed outcome: No installed archive/evaluation/report or persistent Demonstration B.

Demonstration A still needs real retained imagery/evidence, a reproducible analysis and genuinely independent human review through the persistent UI. Demonstration B still needs a usable retained real-context scene, launched native exercise, permitted task/outcome, archive/replay and separately granted evaluation in that installation. The static/abstract profiles are valid intermediate engineering steps.

## 7. Recovery sequence before declaring delivery complete

These are proposed acceptance packages, not a claim that nine equal-sized coding tasks will finish the product. Preserve the existing implementation; prioritize handoff/integration and the confirmed correctness defect. R4 is a publication correctness blocker and can proceed alongside setup. Provider-gated work remains explicitly conditional.

### R1 — Correct completion states and acceptance

**Addresses:** D01, D12.

Separate implementation, fixture/profile and persistent delivery; expose unresolved guarantees.

**Exit evidence:** No blanket completion claim; all blocked/incomplete outcomes are explicit.

### R2 — Supported deployment and explicit role setup

**Addresses:** D02, D04, D11, D12.

Persistent private blobs, scoped roles, distinct humans, worker launch and runtime-build admission.

**Exit evidence:** A clean supported setup reaches feature readiness without permissive grants.

### R3 — Persist a usable Public Context region

**Addresses:** D03, D10.

Import real reference data and activate reviewed bounded USGS/FIRMS captures when authorized.

**Exit evidence:** Normal UI exposes retained reports/revisions/provenance and survives restart.

### R4 — Fix canonical review independence

**Addresses:** D09.

Use stable registered human identity; retain principal audit identity; test aliases and different humans.

**Exit evidence:** Same-human aliases cannot review/publish; genuinely different registered humans can.

### R5 — Complete real imagery Demonstration A

**Addresses:** D05, D03.

Retain a permitted real raster pair, process it reproducibly, independently review and approve export.

**Exit evidence:** A real-data intelligence workflow works in the persistent installation.

### R6 — Complete real context-to-scene preparation

**Addresses:** D06, D07.

Derive AOI/height datum/transform/layers while retaining source and omissions.

**Exit evidence:** Actual selected real terrain loads; synthetic substitution cannot pass that acceptance.

### R7 — Deliver a usable native 3D exercise entry

**Addresses:** D03, D08, D12.

Install starter assets/environment/scene; coordinate trusted launch/attach; scope role-safe moving 3D display.

**Exit evidence:** An author starts an installed exercise and viewers see only their declared permitted state.

### R8 — Finish persistent archive and evaluation handoff

**Addresses:** D03, D08, D11.

Run recording, retain/recover blobs, use a separate evaluator and deliberately release reports; scope spatial/3D review explicitly.

**Exit evidence:** Demonstration B completes task/outcome/replay/evaluation and survives restart.

### R9 — Verify recovery, budgets and normal-user operation

**Addresses:** D01, D11, D12.

Rehearse database+blob restore and bounded realistic workload/outage/restart scenarios.

**Exit evidence:** A normal-user walkthrough and receipts prove both pillars independently and together.

## 8. What is legitimately future work

High-fidelity aircraft/UAV/vessel dynamics, physically validated radar/sensors, a broad animated asset library, large-world streaming/3D Tiles, federation, GPU clusters and specialist physics were explicitly deferred or conditional. Real hardware and engagement control are outside the approved roadmap. They are not being counted as missing implementations from the original 32-task scope.

The architecture permits an initial deterministic environmental baseline rather than mandatory AI, a web viewport rather than an immediate Unreal/Godot/Unity project, and separate on-demand executables rather than continuously running expensive services. The Three.js renderer decision itself is not the delivery failure. The failure is the gap between bounded technical evidence, unclosed proposal capabilities and the broad completion statements.

## 9. Audit limits and reproducibility

- This is a task-by-task delivery audit of the dirty working tree, targeted source paths, recent evidence and the observed persistent installation. It does not certify every line of code or the absence of all defects.
- The preceding follow-up executed 80 browser regressions and 16 real-backend browser checks. Those recent results are reused, not rerun wholesale; they prove their cases rather than installation activation.
- The review-alias path was freshly rerun with a Go overlay adding one diagnostic log. Original production/test source was unchanged; the green test confirmed the wrong independence assumption.
- The terrain probes used original retained bytes and the unmodified loader; both source artifact hashes match. No source datum was relabeled to make the probes pass.
- No local application data, access grants, source configuration, applied migration, provider activation, commit or push was changed by the audit.
- Unknown external S3 configuration and current provider permissions/keys were not inspected or revalidated. Bundled-profile omissions and database contents are observed facts.
- High-fidelity aircraft dynamics, physical radar, broad model libraries, federation, GPU clusters and real hardware were explicitly future/deferred/out of scope and are not counted as missed initial tasks.
- Mandatory ML/LLM automation, Kafka/Kubernetes and a particular heavyweight engine were not requirements. A deterministic baseline and a Three.js initial viewport remain compatible with the v2 direction.

Evidence contains 80 reviewed source/artifact fingerprints, captured task requirements, exact read-only table counts, original file-loader errors and the same-human review diagnostic. Raw probe/build/browser logs remain ignored under tmp/delivery-audit-2026-10-23 and tmp/runtime-review. No runtime implementation, user data, grants or migrations were modified by the audit.

**Current decision:** keep the useful technical work, withdraw the blanket product-completion claim, fix canonical review identity, and complete persistent acceptance of both demonstrations. No further S4 expansion should be used to substitute for that handoff.

## 10. Corrections and artifact verification

The active implementation plan, README, full-plan review and precommit record now explicitly withdraw the broad delivery conclusion and link this audit. Original per-task profile claims are retained as historical evidence; the new ledger is the current delivery reassessment. The review-identity defect and the implementation/integration gaps are open; no runtime fix was applied during this analysis.

All 33 task records, 80 source/verification paths, anchored evidence and report links were checked. The interactive canvas type-checks against the installed SDK declarations; its native host rendering was not independently exercised. All 38 applied migrations, the migration script and line-ending policy retain their protected hashes. Staging and commit history remain unchanged.
