# Environment and data foundations — preparation phase

Date: 11 November 2026. Baseline inspected: `aa298d3`. Scope: the user's clarification in this chat and `EMAD_CURRENT_SCOPE.md`.

**Implementation handoff:** [Environment foundation implementation plan](ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md) fixes the defaults, contracts, processing choices, ordered batches, rollout and verification for this roadmap. This document remains the supporting analysis and retained-data baseline.

## Decision and intended deliverable

Build a reusable, inspectable environment and data library before simulation systems are designed. The deliverable is a set of geographic releases with explicit inputs, coverage, resolution, accuracy, semantics, rights, processing recipes and verification evidence, supported by reliable preparation infrastructure. A future consumer must be able to inspect, obtain, validate and rebuild a supported release without depending on a running exercise or asking its original developer to explain it.

This replaces the preceding review's recommendation to start simulation design or a pilot. Importing the new packages into the historical Studio/runtime is not a prerequisite for this phase. A neutral independent reader is the acceptance consumer. No engine, physics model, actors, synthetic sensors, scenario execution or simulation API is selected here.

The existing Go/PostGIS, private object storage, geographic source admissions, acquisition jobs, catalog paging, purpose/rights controls, immutable outputs and Python geospatial tools are the starting point. Extend those modules incrementally. A replacement platform, separate data lake service or generic workflow engine is not justified by this audit.

The foundation is **PARTIAL**. The retained data is reusable, but no installed environment delivery has yet demonstrated the intended complete preparation workflow. Completion of this analysis is not completion of the implementation backlog below.

## Evidence from the actual installation

The stopped retained PostGIS container was started to allow read-only administrative inventory. No migration, source admission, provider retrieval, rights change or exercise was executed. The database is at migration **53**, independently verified. This replaces the earlier uncertainty about the installed schema; it does not prove current API/worker or object-store health.

The new `scripts/data/inventory_environment.py` generated `private/environment-foundation/2026-11-11/inventory.json`. This private snapshot includes exact hashes, source locations with credentials/query strings removed, raster headers, retained vector limitations and read-only database metadata. Its scope is explicit: receipted files in the retained raw directory and the named legacy environment/vector manifests. It does not audit every imagery pixel, all object-store objects or backups.

Observed results:

- **11 receipted raw files, 406,072,169 bytes, 9 distinct content hashes.** All 11 match their recorded hash and length. Duplicate bytes retain separate provenance receipts; nothing was deleted or moved.
- **24 reference datasets**, with **0 populated accuracy records** and **0 original reference records explicitly permitting `geographicEnvironment` output**. Those are separate gaps. Existing retention, analysis or historical simulation-export rights do not establish the newer output permission. Current source policy and original rights both need review; this inventory does not grant either.
- **3 geographic editions and 0 geographic releases.** Installed navigation is the earlier world/Saudi edition path. The configurable 17-country Middle East profile exists in source, but that is not evidence of an installed complete regional release.
- **20 imagery scene records** spanning 1 March 2025 to 24 October 2026. Scene metadata count is not an AOI coverage measurement. Retained raster artifacts exist, but the inventory does not establish usable cloud-free imagery for every chosen area/date.
- **0 environment delivery jobs.** The one installed delivery job is a successful brief. There is no installed T21 environment job/release to inherit.
- The retained Copernicus surface tile is **3,600 × 3,600**, at 1 arc-second spacing, covering approximately 39–40° E and 21–22° N. A separate retained EGM2008 geoid grid is **8,640 × 4,321**, at 2.5 arc-minute spacing. Both have hash-matching source receipts.
- The old Jeddah visual environment covers about **7.46 × 7.97 km**. Its GLB matches its manifest hash. It contains **1,800 buildings and 750 road segments** selected from **2,084 buildings and 3,436 road segments** found by the old extractor. Multipolygon relations were omitted. **1,797 building heights are estimated**; the remaining 3 are unverified source-supplied values.
- The generic normalizer permits only **2–64 nodes per side**. For the approximately 8 km Jeddah area its spacing is about **125 m**, substantially coarser than the retained native DEM. The old 256-node demonstration and new generic 64-node normalizer are different paths. Do not claim that the generic path preserves native 30 m detail.

Key evidence locations: `internal/reference/domain.go`, `internal/environmentprep/prepare.go`, `internal/investigations/outputs.go`, `scripts/data/prepare_portable_geography.py`, `scripts/data/prepare_geography.py`, `scripts/data/prepare_environment.py`, `scripts/data/regional_geography.py`, and the private inventory above.

## The gaps that matter

### 1. Environment identity and discovery

Geography editions, reference datasets, imagery, investigation outputs and historical model assets each have useful identities. There is no demonstrated engine-independent environment library joining them into named, versioned, discoverable releases with a declared preparation profile. The current environment output is coupled to a case revision and its original human author. Shared preparation must not erase that case/source authority or require every environment to be treated as an analytical finding.

Add environment definitions/releases as a narrow domain over existing source IDs, area revisions, immutable artifact IDs, recipes and rights checks. Explicitly decide which outputs can be referenced by a workspace environment and under which composite ACL. Preserve source access at discovery, preparation, preview and download. Do not copy existing case data into a broader registry merely because a worker can read it.

### 2. Source meaning and admissible uses

The current Copernicus input is a **surface model**, including structures and vegetation; it is not surveyed bare-earth terrain. Extruding buildings on such a surface can double-count heights. Roads, coastlines, buildings and imagery have different spatial meanings and temporal validity. Reference scale, pixel size and accuracy are distinct quantities.

Every layer must identify its role and supported uses. Preserve original tags and observed dimensions separately from inferred display dimensions. Unknown building height stays unknown in authoritative geometry; an optional display representation may retain an explicitly labelled estimate. Do not invent material, load-bearing, traversability or sensor properties from a land-cover label. Requirements for those belong to later consumer-specific qualification.

### 3. Resolution, tiling and spatial extent

The existing small JSON-grid/8 MiB ZIP profile is useful for regression and bounded exchange. It cannot be the only storage/preparation architecture for regional terrain and imagery. Increasing its array or response limits would move large work into RAM and HTTP responses.

Keep native retained assets, then prepare bounded raster windows and versioned tiled representations with overviews. Use a declared resolution and resampling policy, preserving nodata, masks, grid registration, seam controls and explicit vertical transformations. Estimate resource budgets from AOI and resolution before admission to the queue. Keep small v1 packages compatible while introducing a larger multi-artifact representation with a bounded manifest.

### 4. Actual coverage and missing inputs

A country boundary or raw-file footprint does not establish detailed coverage. For each area/layer/time, record the requested footprint, valid-data footprint, omissions, source age, resolution and confidence in completeness. Distinguish unavailable, unacquired, unprocessed, expired, policy-blocked and qualified data. Report missing geometry and quality masks rather than filling holes with zero elevation or default buildings.

Terrain/surface, geoid, dated imagery, roads/buildings, coast/water and land-cover are separate layer families. The audited Jeddah assets cover only part of this matrix. Land-cover, a suitable bare-earth DTM, bathymetry, ground materials and volumetric atmospheric fields are not established by the retained manifests. Track them as unsupported capabilities; acquire and qualify them only where a declared environment profile requires them. Existing METAR/report feeds are contextual observations, not a complete 3D weather field.

### 5. Accuracy and independent qualification

The old pipeline records a vertical round-trip check. That demonstrates transform consistency, not independent terrain accuracy. The new exporter requires source accuracy and independent controls, which the installed references currently lack.

Record horizontal/vertical uncertainty with units, confidence/statistic, source, spatial applicability and whether it is supplied, measured, estimated or unknown. Keep transformation error separate from source error and output sampling error. Do not put a guessed number into the current `Accuracy` fields to pass preflight. Where vector accuracy is unknown, design an explicit limited-use metadata state; it must not pass a stricter profile. Sparse controls do not qualify every terrain cell.

### 6. Repeatable processing and storage lifecycle

The generic normalizer consumes manually prepared JSON and local paths; the legacy importer contains Jeddah constants, capped feature selections and display assumptions. Source retention, recipes, normalized outputs and independently checked releases need one reproducible lineage chain. Some original native parents currently remain external dependencies in private directories.

Version recipes and pin tool builds, parameters, all parent hashes, clipping rules, coordinate/grid definitions and dependencies. Resolve parents through authorized artifact identities. Make a release declare whether it is self-contained or requires identified external parents; test its promised offline behavior. Parent expiry or rights revocation must invalidate eligibility and prevent illegal resurrection during restore.

### 7. Preparation operations

The prior review found reproducible browser/fixture failures and an OpenSky quota-recovery regression, while focused environment/native tests passed. Repair the applicable map, catalog, intake and preparation checks. A historical exercise browser test is not the preparation gate.

Assembly must prove API/database/private-storage readiness, continuously supervised preparation/delivery workers, per-author authorization behavior, retry/cancellation/fencing, disk/RAM limits, queue age and failed-job diagnostics. Back up the database, parent artifacts, derived assets and exact configuration together; restore and verify the same release offline. Existing T22–T24 remain release obligations, but unrelated public analyst features must not become an unlimited prerequisite to documenting the environment contract.

## Target data organization

Maintain this explicit identity chain:

`source → product → edition → retained artifact → recipe/build → representation → environment release`

An area revision, profile and time selection bind the chain to a use. A quality report and rights decision bind to exact versions, never just a display name. Existing domain records remain authoritative where they already own an identity.

The minimal records are:

1. **Source/product/edition:** producer, product/version, actual data time, retrieval time, coverage, original/current terms and admission. A new file retrieval does not imply new observation time.
2. **Artifact:** hash, length, media type, private storage locator, retention and availability. Deduplicate bytes while preserving separate source/rights identities; shared hash does not grant shared access.
3. **Layer representation:** semantic kind, native and output CRS, vertical datum/epoch where applicable, axis order, units, pixel or feature schema, registration, nodata, native/output resolution, accuracy status and valid coverage. Rendering meshes remain replaceable derived products.
4. **Environment definition:** stable name/key, exact area revision, purpose/profile, required/optional layers, preferred editions and intended time window. Edits create a new revision.
5. **Recipe/build:** frozen inputs, code/tool/dependency versions, parameters, estimated budgets, measured resource usage, attempts, outputs and failures. Idempotency is based on the frozen recipe and inputs; stale attempts cannot publish a release.
6. **Quality report:** structural/integrity checks, coverage and omission tests, independent coordinate controls, layer alignment, topology, source limitations, hardware and performance measurements. Each claim records its method and scope.
7. **Environment release:** immutable manifest binding all selected representations, quality reports and rights decisions. Explicit lifecycle: draft → staged → verified for a named profile → released; failed, blocked, expired and superseded states remain inspectable. A release can be verified for visualization and still unsupported for a stricter future use.

Use object storage for binary assets and PostGIS for indexed metadata/geometry. Organize logical prefixes by original content, recipe results and release manifests, with ACL enforcement independent of path names. Do not rename or delete retained history during catalog adoption. The inventory produced now is a discovery aid, not a substitute for the authoritative artifact/access services.

For interoperability, use COG/GeoTIFF representations for windowed raster access and offer a STAC-compatible discovery projection over existing metadata. STAC describes assets/collections; it does not supply authorization, geodetic accuracy, immutable release policy or a simulation model. Avoid creating a second competing catalog. Keep GeoJSON for bounded vector interchange; preserve complete geometry/topology in the retained vector store. Choose a larger vector interchange format only from measured workloads and independent-reader requirements.

These format directions are supported by the [OGC COG standard](https://www.ogc.org/standards/ogc-cloud-optimized-geotiff/) and [STAC specification](https://stacspec.org/). Copernicus documents its DSM semantics in the [product description](https://dataspace.copernicus.eu/explore-data/data-collections/copernicus-contributing-missions/collections-description/COP-DEM). These references do not approve data licences for a particular release.

## Geographic preparation profiles

Define profiles by intended data use, with numeric acceptance budgets frozen before processing:

- **Overview:** global navigation and regional reference; generalized boundaries/places/coasts, explicit source scale and omissions. No local-detail claim.
- **Source surface:** a bounded source-backed surface and transformation grid, aligned imagery when available, original vector semantics, native-resolution preservation and measured coverage. Unknown dimensions remain explicit.
- **Detailed geographic area:** additional qualified footprints/roads/land cover/water and independently checked alignment/terrain. Optional layers are visibly unsupported until obtained and verified. A detailed-area label cannot imply a qualified physical model.

Profiles must declare required versus optional layers, source age/epoch, supported CRS/datum combinations, target spacing, uncertainty requirements, area/feature/byte limits, control-point method and reproducibility criterion. Do not use the same numeric thresholds for cartographic overview, surface reconstruction and a later physics consumer.

The Middle East is the priority working region, with worldwide discoverability and reusable area handling. Jeddah remains a retained regression dataset. Acceptance includes a second non-Jeddah Middle Eastern area and an international area through the same preparation flow, plus supported edge geometry. Exact areas should be chosen by data availability and terrain/urban/coastal coverage diversity, not hardcoded into processing modules.

## Implementation order and evidence gates

Each package gets code, tests appropriate to its contract, an installation check and durable evidence. Names below are proposed work packages; they do not change MAP-6 completion labels.

### E01 — Inventory and baseline (first increment implemented)

Deliver the read-only inventory CLI, a verified retained-input inventory, the corrected preparation scope and this dependency plan. Verify altered bytes, missing parents, duplicate receipts, root containment and credential stripping. Preserve existing assets. The current audit does this for its documented scope; whole-library/object-store inventory remains E02/E08 work.

### E02 — Catalog and eligibility (depends on E01)

Extend `reference`, `artifacts`, content-purpose and source policy with product/layer/representation links and explicit quality/rights states. Migrate additively; backfill exact identities without upgrading unknown rights or accuracy. Resolve every installed reference to its parent and source. Materialize a read-only environment inventory UI/API that states why an input is unavailable or unsuitable. Source-restricted users must not infer hidden records through counts or search.

Acceptance: every installed candidate has an attributable parent or an explicit missing-parent state; no unknown metadata becomes a successful eligibility result; preserved historical hashes; paged catalogs stay usable during ordinary navigation.

### E03 — Environment profiles and immutable manifests (depends on E02)

Implement engine-independent definitions, profile versions, required layer bindings and manifests using existing immutable artifact/output patterns. Separate preparation ownership from a case while preserving composite access. Specify compatibility with `emad.portable-geography.v1` and historical context bundles without requiring runtime import.

Acceptance: two versions coexist; selection of newer data does not alter old releases; a reader can discover exact coverage, limitations, dependencies and current permission state; unsupported profiles fail explicitly.

### E04 — General native-data preparation (depends on E02/E03)

Replace Jeddah-specific intake choices with explicit AOI/product specs. Retain native DEM/imagery resolution, introduce bounded tile/window processing and overviews, preserve masks and datums. Parse vector relations where required and expose all omissions; remove nearest-feature truncation from canonical data. Keep sampling/simplification in explicitly derived representations. Preflight disk, memory and runtime from actual source metadata.

Acceptance: adjoining DEM tiles and disjoint/hole geometry preserve coverage and seams; output spacing is declared and verified; no false bare-earth label on DSM; estimated heights remain separate; original parents remain hash-pinned. Unsupported antimeridian/polar/datum cases reject before expensive work until their profile is implemented.

### E05 — Source-backed quality and coverage (depends on E04)

Version source accuracy evidence and independent controls; implement quantitative coverage and omission reports and cross-layer alignment/topology checks. Distinguish provider-stated uncertainty from independently measured error. Surface the difference between raw footprint, retained data, prepared data and qualified data.

Acceptance: real-source fixtures, controls independent of the transform under test, missing/misaligned layer rejection, no pass through looser post-hoc tolerances, transparent partial coverage. Lack of suitable control evidence remains a specific open gate rather than fabricated values.

### E06 — Durable preparation service and operator flow (depends on E03; integrated qualification depends on E04/E05)

Reuse durable job leases, reservations and immutable publication. Provide area/profile selection → eligible inputs → cost/coverage preflight → submit → progress/failure → inspect → release/download. Default settings come from versioned profiles; ordinary users should not need raw IDs or qualification JSON. Make service versus human contribution ownership explicit rather than running an unrestricted all-author worker.

Acceptance: idempotent submission after a lost response; interruption and cancellation; stale-worker fencing; no partial successful release; live access recheck; separate raw/normalized/derived storage accounting; operator can locate the failed layer and recovery action.

### E07 — Independent consumption and rebuild (depends on E05/E06)

Extend the offline reader to validate the exact file set, schema, hashes, dependencies, coordinates, vertical method, masks, coverage and declared quality results. Distinguish numerical equivalence from byte-identical serialization and state the required reproducibility criterion. Test with a fresh directory and unavailable provider network.

Acceptance: a separate recipient reads a release without application exercise services or developer-local paths; corruption, expired/missing required parents and unsupported schemas fail clearly; rebuilding a pinned recipe produces the declared equivalent result. Visual inspection supplements quantitative checks.

### E08 — Installed supervision, capacity and recovery (depends on E06/E07)

Assemble matching API/UI/workers against migration 53 plus future additive migrations. Qualify representative detailed data volumes, concurrent bounded jobs, storage growth and recovery on recorded hardware. Reconcile catalog lifecycle and acquisition quota regression tests against intended production behavior. Paired backup/restore must include every required parent, recipe, grant and release dependency.

Acceptance: installed health and worker monitoring; measured budgets with failures retained; unattended operation and fault/recovery evidence; authenticated private asset delivery; no resurrection of expired objects; restored independent release verification. Run long-duration qualification after the processing/storage design is stable.

### E09 — Install the geographic library (depends on E02–E08)

Activate a complete regional navigation release and prepare the representative source-backed areas through normal product operations. Record country/detail coverage separately. Obtain the explicit source-purpose/accuracy/control inputs missing from the audit; keep unavailable layers marked. A worldwide detailed dataset is not required, but arbitrary supported areas must follow the same pipeline.

Acceptance: installed, discoverable, reproducible releases for the declared geographic/profile matrix; no Jeddah fallback; no code edits needed per area; clear reasons for absent detail elsewhere. Observe an independent data consumer selecting and validating a release.

### E10 — Design-readiness handoff (depends on E09)

Publish the environment contract, sample released manifests, offline reader, recipe inventory, coverage/quality matrix, installation/runbooks, resource limits and known unsupported capabilities. Retain the old simulation modules as historical regression work.

The phase is ready for later simulation-system design when a separate team can answer: what geography exists; what each layer means; what is measured/estimated/unknown; what resolution/accuracy/time it has; how to access and reproduce it; what uses its rights and validation permit; and how failures/updates affect a pinned release. No simulation run is an exit criterion.

## Immediate work boundary

Completed in this increment: deeper code/data analysis, starting existing PostGIS for read-only verification, the inventory tool and private report, scope correction, and an actionable preparation plan. The new catalog/domain, native-resolution preparation, real qualification, operational soak and installed releases remain implementation work.

The map-first planning harness pins the governing scope by hash. Its fingerprint was updated after reviewing the 11 November clarification against the existing environment-only boundary; no map task, acceptance result or historical release evidence was promoted. The plan consistency check and five new inventory integrity tests pass.

The next engineering batch is E02/E03: catalog eligibility and neutral environment manifests, with E04 native-resolution preparation following. Preserve current permissions and immutable history. Do not implement a historical runtime adapter as a shortcut to declaring the environment ready.

Missing rights/accuracy/control evidence cannot be replaced by code or a blanket user confirmation. Surface the exact source/area requirement, continue independent implementation, and keep the affected release unqualified until adequate evidence is supplied.

## Reproduce this inventory

Use the repository's recovery Python environment; `--inspect-rasters` requires its installed rasterio dependency. Choose a new output path for each snapshot:

```powershell
.venv-recovery/Scripts/python.exe scripts/data/inventory_environment.py --data-root private/local/data --output private/environment-foundation/<new-snapshot>/inventory.json --inspect-rasters --database-container c4isr-postgres
.venv-recovery/Scripts/python.exe -m unittest discover -s scripts/data -p test_inventory_environment.py
```

The optional database query assumes the existing local `c4isr` maintenance identity and migrated schema. It uses a read-only repeatable-read transaction and never prints credential material. Omit it for a pure filesystem inventory. The report stays `INVENTORIED_NOT_QUALIFIED` even when every hash matches; nonzero integrity issues produce a failing CLI exit status.
