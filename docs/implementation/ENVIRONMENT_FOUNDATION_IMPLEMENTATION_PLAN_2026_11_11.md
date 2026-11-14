# Environment foundation implementation plan

The current delivery priorities are Riyadh for detailed preparation, Saudi Arabia for national coverage and the wider Middle East for regional coverage. The three areas described below record the original installation and its regression cases. [Current geographic priorities](../../plans/README.md#geographic-priorities)

**Date:** 11 November 2026. **Status:** the three reference packages are staged on Windows and Linux. Paired restore passed. Human release review and the 24-hour staging observation are open. [Installation evidence](ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md) [Operating instructions](ENVIRONMENT_LIBRARY_OPERATIONS.md) **Baseline:** `aa298d3` plus the inventory and scope work documented in the [foundation analysis](ENVIRONMENT_FOUNDATION_PLAN_2026_11_11.md).

## 1. Outcome, scope and fixed defaults

Deliver an installed, engine-independent geographic environment library. A new team can select a supported area, inspect source meaning and coverage, prepare a versioned release, validate it offline and reproduce it from retained inputs. The deliverables are data, preparation software, contracts, verified sample releases and operational runbooks. Simulation design, engines, actors, sensors, exercise execution and a historical Studio import adapter are outside this assignment.

Use **local Windows development plus Linux staging**, **public/open data first**, and **regional overview plus detailed representative areas**. These are explicit planning defaults because the clarification questions were not answered. This phase neither purchases data nor deploys a public production service. Retained inputs are reused before new acquisition; missing credentials, purpose-specific rights or source evidence leave the affected layer blocked rather than weakening its acceptance rule.

Install the existing 17-country Middle East navigation profile, retaining global country discovery. Freeze three detailed acceptance areas as data profiles, not code constants:

- Jeddah retained regression area: `[39.144, 21.464, 39.216, 21.536]`.
- Al Ain: `[55.73, 24.20, 55.75, 24.22]`.
- Avignon: `[4.82, 43.95, 4.84, 43.97]`.

Bounds are longitude-first west/south/east/north. The latter two reuse areas with prior isolated imagery evidence; this does not establish their installed terrain/vector eligibility. Every area must go through the same ordinary preparation workflow. Add a multi-tile fixture and hole/disjoint-polygon fixtures for geometry stress without changing the three real-data acceptance areas.

Deliver these versioned profiles:

1. **`overview-v1`:** world countries and regional subdivisions/places/coasts; source cartographic scale and omissions are mandatory. Local engineering accuracy is not claimed.
2. **`reference-surface-v1`:** required source surface raster, native masks, declared source accuracy, georeferencing, explicit vertical method and retained dependencies; independent transformation verification. It identifies DSM versus DTM and source-stated versus independently measured accuracy. Qualification means faithful preparation for reference use, not surveyed ground truth.
3. **`reference-context-v1`:** the surface profile plus dated imagery, roads, building footprints and water/coastal context when applicable. Empty source results need a retrieval/completeness receipt; unacquired data cannot be called empty. Unknown vector positional accuracy and building dimensions are allowed only as explicitly limited reference information. They do not become qualified physical properties.

Land cover, bare-earth DTM replacement, bathymetry, verified building heights, materials and volumetric weather remain named optional capabilities with visible availability states. They are not silently filled or made mandatory for all environments. Future physically demanding profiles require new source/quality requirements. No v1 profile is named “simulation-ready” or “survey-grade.”

Initial engineering limits are versioned configuration, not measured service guarantees: 2,500 km² total preparation area; at most 16 polygon components within ±85°; at most 64 layer bindings, 1,024 retained objects and 4 GiB derived output per release; 1 GiB per geographic object; 8 MiB manifest; 2 GiB worker RSS, 8 GiB scratch and 30 minutes per attempt; three attempts; one concurrent preparation locally and two in staging. Reject impossible estimates before reservation. Input acquisition has its own source quotas and disk reservation. No automatic whole-region detailed downloads.

## 2. Architecture and public contracts

### Catalog and storage

Extend the existing Go/PostGIS/private-S3 architecture. Add a narrow `geographic environment` domain, distinct from the existing IAM `environments` table and historical simulation environment assets. Continue to use existing workspace/environment/partition scope; prepared public geographic data belongs to `PUBLIC_EVIDENCE`.

Add immutable product-edition and representation records linking existing source admissions, exact reference/imagery IDs and retained artifact identities. Metadata must include layer kind, geometry/valid-data footprint, data time and retrieval time, CRS/axis order/units, vertical datum, grid registration, native/output resolution, nodata, uncertainty status/method/statistic/provenance, original/current output rights and parent hashes. Existing documents remain immutable; missing metadata is recorded as unknown. Corrected metadata or rights requires a new reviewed version.

Keep the existing 16 MiB `artifacts.Store` and legacy APIs compatible. Add an optional streaming object-store interface and a separate geographic-asset reservation/content path for larger native and prepared objects. Its database records bind all required source/admission dependencies, exact hash/length, media type, retention, upload state and scope. Reuse existing reservation, uncertain-upload, deletion and expiry patterns rather than increasing legacy buffer limits. Stream/hash uploads, support authenticated single-range reads and bound buffers to 8 MiB. Internal keys and provider credentials never become client-supplied paths. Do not deduplicate across authorization scopes in this increment.

Use retained native originals plus **COG/GeoTIFF raster representations**, **GeoPackage vector layers** and bounded **GeoJSON previews**. Format references: [OGC Cloud Optimized GeoTIFF](https://www.ogc.org/standards/ogc-cloud-optimized-geotiff/) and [OGC GeoPackage](https://www.geopackage.org/). The authoritative data is not a GLB. Preserve native raster grids where possible; explicitly declared reprojection/resampling generates a new representation. Add a read-only [STAC-compatible](https://stacspec.org/) projection of released asset metadata, built from the same records and authorization checks. It is not a second catalog or a new source of rights authority. Pin the adopted schema versions and required extensions in Batch 1; validate against those pinned versions.

### Environment definitions, builds and releases

Add scoped records for profile versions, environment definition revisions, recipe versions, build jobs/attempt events, geographic assets/dependencies, quality reports, releases and release events. Use composite `(id, scope_id)` keys, forced RLS and immutable revision/report/manifest rows. A definition binds a saved area revision and profile; a recipe binds exact inputs, processing parameters, toolchain/build hashes and expected outputs.

Definition edits use expected-revision conflict checks. Build submission uses an owner-scoped request key plus frozen input hash: a repeated key with identical input returns the original build; changed input conflicts. Freeze all input and policy identities before queuing. Never resolve “latest” after a job starts.

Build states: `QUEUED`, `RUNNING`, `SUCCEEDED`, `FAILED`, `CANCELED`; policy/preflight blockers are structured diagnostic results before admission or terminal failure codes during execution. Release events represent `STAGED`, `VERIFIED`, `RELEASED`, `SUPERSEDED`, `WITHDRAWN` and `EXPIRED`. Verification binds the exact manifest digest and named profile. Release requires explicit human authorization; a worker cannot release its own output. Current eligibility can become false without rewriting the frozen manifest or prior verification history.

The manifest schema is `emad.environment-release.v1`. It includes definition/profile/recipe versions, exact area and frame, source/time semantics, artifact hashes and relative names, dependency graph, required/optional layer states, coverage/masks, resolution/uncertainty, rights/attribution, quality-report identities and compatibility information. Offer a self-contained prepared-data bundle and a separate rebuild bundle containing all legally retainable original dependencies. If rights prevent a promised bundle, that bundle is unavailable with an explicit reason; do not describe external private paths as portable.

Keep `emad.portable-geography.v1`, case delivery APIs and historical context bundles unchanged. Register their provenance as legacy representations; do not auto-upgrade them to new qualified releases.

### API and worker boundary

Define OpenAPI contracts before handler/UI work, using existing error envelopes and scoped headers:

- `/api/v1/environment-profiles`: list/read immutable profiles.
- `/api/v1/geographic-environments`: paged catalog, create, read and expected-revision update; per-definition eligibility, preflight and build submission.
- `/api/v1/environment-builds/{id}`: state, attempts, diagnostics, cancel and explicit retry as a new attempt within the existing budget.
- `/api/v1/environment-releases`: scoped search/read; manifest, quality report, authorized release/withdraw actions, prepared-data and rebuild-bundle requests.
- `/api/v1/environment-assets/{id}/content`: authorized streaming/range content; upload reservation/completion are available only to the bound intake/preparation identities.
- `/api/v1/environment-worker`: claim, heartbeat, staged-output registration and completion, each bound to an active lease and immutable input identity.

Catalog filters cover area, profile, layer, data time, quality and availability. Reuse fixed catalog pagination, but repair read replacement/release lifecycle before attaching new screens. Add `environments.read`, `environments.manage`, `environments.build`, `environments.execute`, `environments.qualify`, `environments.release` and `environments.download` to the permission vocabulary. Do not add them to all existing identities. Discovery, counts, download and range reads require the relevant permission plus every contributing source grant and original/current policy.

Introduce `emad-environment` as the independent Go worker/coordinator, invoking the pinned Python raster/vector processor in an isolated work directory. It authenticates with a dedicated source-scoped service identity. Job claim and publication require both worker authority and the submitting human's current authority; neither is sufficient alone. Environment contributors and source administrators remain distinct. Existing case-author `emad-delivery` continues handling case outputs.

The preparation child consumes only hash-pinned retained input files and a frozen job manifest, has no provider credentials, and performs no network retrieval. Provider acquisition remains in the existing admitted connector/acquisition path. Spool files remain beneath the registered attempt directory and count toward disk reservations. Enforce container limits in staging and sampled RSS/disk/time termination with a 30-second shutdown grace locally.

Use a 60-second lease and 10-second heartbeat. Retry transient I/O/process failures at 5 seconds and 30 seconds, with deterministic jitter bounded to 20%, for at most three attempts. Validation, rights and missing-source errors require correction/new submission, not unattended retries. Publication and lease checks commit atomically; partial uploads remain unservable and reconciled under the retained upload ledger.

### Preparation and qualification behavior

Source surface is required at its declared native sampling; derivatives may be coarser only with an explicit output resolution. Use tiled/windowed operations, 512-pixel COG blocks and powers-of-two overviews; continuous numeric data uses declared interpolation, categorical/mask data uses nearest-neighbor. Preserve nodata and reject seam disagreements beyond the source uncertainty. No ocean zeros, inferred bare-earth labels or unlabelled gap filling.

Canonical vector extraction retains complete eligible geometry, stable source IDs, relations and original tags within the bounded AOI. Capture dropped/invalid geometries and reasons. Remove nearest-N truncation from canonical inputs. Display simplification and estimated extrusion heights are separate derived representations. Preserve unknown authoritative heights even when a preview chooses a labelled approximation.

Use longitude-first WGS84 geographic coordinates in manifests; each raster retains its actual CRS/registration and each local frame declares origin and ENU axes. Support ellipsoidal heights and explicitly pinned EGM2008 conversion initially. Reject unconfigured datums/grids before processing. Date-line-crossing preparation and polar preparation are explicitly unsupported in these profiles; global navigation remains usable. Multipart non-date-line areas and polygon holes are supported with per-component outputs and preserved masks.

Quality reports distinguish four checks: byte/schema integrity; coordinate/transform consistency; faithful preservation of source coverage/resolution/semantics; and independently observed physical accuracy where such evidence exists. The first three qualify reference profiles; absence of field/survey evidence remains visible and cannot justify the fourth. Use the existing independent Go ECEF/ENU implementation against PROJ and independent known-coordinate controls. Existing stricter T21 terrain-control gates remain unchanged; do not relabel generated heights as independent survey controls.

Freeze profile tolerances and their evidence before qualification. Record uncertainty statistics as supplied; do not add incompatible confidence statistics into a fabricated combined accuracy. For coordinate implementation checks use a 1 mm numerical tolerance. For terrain/control agreement use the admitted source/grid uncertainties and independent control uncertainty documented in the profile. When no defensible physical-accuracy bound exists, record `UNKNOWN` and forbid the corresponding accuracy claim.

## 3. Ordered implementation batches

### Batch 0 — Baseline and prerequisite repairs (E01 + E08 prerequisites)

Keep the existing inventory work; extend it to imagery parents and object-store availability without scanning credentials. Capture exact baseline source/configuration hashes. Reproduce and fix the catalog read-capacity lifecycle, applicable public-workflow browser failures and acquisition-credit regression. Browser seeding must respect the separate provider-account budget; preserve zero-credit enforcement rather than resetting credits to make a test pass. Run the map-only stack with its normal supervised workers and retain readiness evidence.

**Exit:** reproducible baseline, current map preparation/navigation gates pass, no acknowledged save loss, and a specific unresolved-source list. The prior full suite was interrupted; execute it completely after repairs rather than calling the partial run a pass.

### Batch 1 — Catalog, permissions and streaming assets (E02)

Add additive migrations for catalog metadata, source dependencies and streaming geographic assets. Backfill exact legacy references and raw receipts as unqualified records; do not grant missing accuracy or rights. Add reservation/upload/completion/read/range/expiry operations and scoped catalogs. Stage each native parent with its original receipt and verify private-store round trips. Explicitly provision the data steward, preparation service and consumer permissions in local/staging setup.

**Exit:** all 24 retained references resolve to exact parents or a visible missing-parent state; all available raw inputs pass private-store hash checks; large-file processing remains bounded; source isolation holds for records, counts, bytes and shared hashes.

### Batch 2 — Definitions, manifests and durable build contract (E03 + E06 foundation)

Add the environment domain, profile definitions, recipe/build/quality/release schema and APIs. Ship the OpenAPI-derived browser types. Add the independent worker and lease/publication contract. Implement compatibility readers for legacy representations without automatically releasing them.

**Exit:** immutable revisions coexist; conflicting edits and lost responses recover correctly; access loss, cancellation and stale leases cannot publish; offline manifest validation works with a small explicitly synthetic fixture.

### Batch 3 — Native-resolution processing and quality (E04 + E05)

Generalize retained DEM/geoid/imagery/vector processing, implement windowed COG and GeoPackage outputs, coverage masks and machine-readable source/quality reports. Integrate trusted acquisition for missing area inputs using existing source policies and budgets. Prepare real data into staged outputs for all three acceptance areas. Identify and document source-policy decisions before promoting newly eligible data.

**Exit:** no hardcoded city processing, full canonical extraction within declared limits, reproducible spatial alignment, preserved native detail and explicit unknown/omitted layers. Real preparation failures and unavailable inputs remain recorded.

### Batch 4 — Product workflow and independent consumption (E06 + E07)

Add an Environment Library workbench to the current map: area/profile → eligible layers → cost/coverage preview → preparation → report → release/download. Show readable layer names, quality basis, availability reasons, progress and actionable failures. Ordinary users do not edit raw IDs or qualification JSON. Clear stale selections on area/profile/access changes; switching views releases replaceable read capacity.

Deliver a separate offline reader and CLI for integrity, geometry, masks and compatibility, plus a rebuild command using a pinned recipe/toolchain. Add authenticated STAC projection and prepared/rebuild bundle generation. Byte-identical canonical metadata and normalized values are required for the same pinned build; container compression differences are reported separately, not hidden by replacing hashes. Keep execution timestamps and attempt IDs in the audit envelope outside the reproducible logical content digest. Every physical object retains its exact byte hash; an equivalent rebuild with different bytes receives a new object identity and an explicit equivalence report.

**Exit:** a fresh directory can consume and rebuild a permitted release with provider network disabled, using only retained artifacts and the pinned toolchain. UI and independent reader agree on the exact release and limitations.

### Batch 5 — Installed releases, operational qualification and handoff (E08–E10)

Install the complete regional navigation release and the three qualified reference-context releases. Extend local supervision, staging worker composition, readiness/heartbeat, Prometheus metrics and backup inventory to the new domain. Rehearse paired restoration and verify the same manifests, grants, parents and quality evidence. Finish a 24-hour unattended run against one frozen candidate and observe a consumer completing discovery, preparation, limitation interpretation and offline validation.

**Exit:** all preparation acceptance gates below pass. Publish installed profile/coverage matrices, API/manifest documentation, sample releases, the offline reader, recipes, runbooks and unsupported capabilities. Preserve existing MAP-6 ledger states unless their own exact criteria have separately been fulfilled. No simulation run is a completion requirement.

## 4. Verification and acceptance gates

1. **Integrity/catalog:** tampered bytes, wrong length, missing parents, duplicate byte identities, schema mismatch, unsafe archive paths and extra/missing manifest members are rejected. Changing permissions removes protected metadata and bytes, including range reads and STAC links.
2. **Policy:** original/current rights intersect correctly; missing permission remains blocked; expired/revoked sources cannot build, release or restore forbidden bytes. A service worker cannot qualify or release. A human request cannot borrow worker authority.
3. **Geography:** multiple DEM tiles, nodata strips, holes, disjoint parts, edge registration, source overlap conflicts and differing datum/resolution are covered. Independent coordinate errors stay within 1 mm; real accuracy claims use source-backed thresholds. DSM labels, estimated dimensions and incomplete vectors survive packaging and replay through the reader.
4. **Jobs/storage:** lost submission/completion responses, retries, duplicate work, worker death, stale lease, cancellation during upload, API restart, object failure, full scratch/storage quota and rights revocation during processing leave no successful partial release. Corrupted/uncertain writes remain unservable and reconcilable.
5. **Reproducibility:** fresh offline consumption and rebuild of all three real areas; original byte hashes and manifest identities remain pinned; normalized values/geometry and masks meet the declared equivalence criterion. External provider network must be disabled during this test.
6. **UI:** three real areas use the same workflow; empty versus unavailable is legible; invalid area changes clear selections; keyboard, 390px portrait and 740px landscape retain access to area/time/quality controls. Tests use the assembled backend as well as focused protocol fixtures.
7. **Capacity/soak:** five simultaneous metadata readers, 10,000 asset metadata records, 100,000 vector features and two concurrent bounded staging preparations. Initial budgets: metadata/search p95 ≤1 s, first useful environment preview ≤3 s, unloaded queue pickup ≤60 s, each attempt within its configured memory/disk/time envelope. Retain failed requests/timeouts in statistics. Complete 24 wall-clock hours with a worker restart and no acknowledged data loss or unbounded storage/lease growth. These are engineering acceptance defaults, not a production SLA.
8. **Recovery/rollout:** restore a quiesced paired backup to isolated storage/database; verify all permitted release dependencies, source grants, independent reader results and pending-job recovery. Test the nominated previous binary's behavior with additive schema before claiming rollback support; otherwise retain a documented forward-recovery path.

Run Go unit/integration/race checks for changed services, Python processing/reader tests, complete frontend build and applicable Playwright workflows, contract generation/drift checks and staging validation. Add these new foundation checks to CI. Unrelated historical simulation tests remain regression checks; their failures are recorded and triaged without turning this phase into simulation development.

## 5. Rollout, evidence and completion

Use sequential additive migrations after 53; do not rewrite old migrations or immutable evidence. Deploy new endpoints/UI behind `C4ISR_ENVIRONMENT_LIBRARY_ENABLED=false` until scoped installation checks pass. Roll out schema → API → worker → UI → source/identity provisioning → staged builds → qualified releases. Disable the feature to stop new work while preserving readable history; stop workers before incompatible upgrades. No automatic destructive down migration.

Metric labels are bounded to operation/profile/state/failure class. Record build duration, queue age, retry/lease health, valid-data coverage, reserved/actual bytes, scratch/RSS, expiry cleanup and failed publication. Do not put area geometry, filenames, source secrets or user IDs into metric labels. Health distinguishes database/object availability, worker capability and queue blockage.

Retain an evidence record per batch with source/build/config hashes, migration version, profile/source revisions, actual command results, failures and artifact hashes. A changed implementation invalidates affected evidence; schedule the final soak after processing/storage behavior is stable. E01's private inventory is a baseline, not an approved geographic release.

The phase is complete only when the installed library, all three real releases, independent consumption/rebuild, meaningful workflow checks and operational qualification are evidenced together. If a required source/rights input is unavailable, report the exact unmet layer/profile and keep that release unqualified; optional capabilities may remain explicitly unsupported. Never replace the promised real-data acceptance with synthetic fixtures.

The later simulation team receives a precise answer to what data exists, what it means, where it applies, what is uncertain, how to obtain/reproduce it and how to pin an immutable release. The team retains responsibility for future simulation-specific fidelity and architecture decisions.
