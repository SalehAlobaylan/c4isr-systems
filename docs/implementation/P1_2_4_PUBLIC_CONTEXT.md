# P1.2–P1.4 — Public event semantics, reference snapshots, and USGS

**Subsequent increment:** [S0.4 and P1.5–P1.6](S0_4_P1_5_6_ADAPTER_AND_PUBLIC_HISTORY.md) completes the adapter/M2 and public-history/M1 gates. Current installations require migrations through **27**, and the next task is **P2.1**. The original acceptance details below remain historical; statements about these tasks being pending are superseded.

Implemented 2026-10-21 on the accepted P1.1/F0 foundation. These are three completed tasks, not completion of the P1 workstream. P1.5 FIRMS and P1.6 comprehensive evidence/history views remain queued. [S0.1–S0.3 foundations](S0_1_3_SIMULATION_FOUNDATIONS.md) are now implemented; current installations require migration 25. The next task is S0.4/M2 before returning to P1.5–P1.6.

## P1.2: Reported events do not imply motion

Observations now carry `semanticKind` (`moving`, `point_event`, `area`) and `geometryRole` (`reported_location`, `reported_extent`, `measurement_footprint`, `derived_buffer`, `phenomenon_boundary`). Point geometry alone does not select correlation. Existing moving/scenario observations keep their behavior. Legacy area inputs default to area semantics and a reported extent. Named `event.*`/`detection.*` inputs, earthquakes, and thermal detections take the point-event path when no footprint is supplied. Contradictory motion overrides, non-moving track hints, and point/footprint role mismatches are rejected.

An optional normalized provider statement travels with immutable public evidence. It records the provider's report, rather than publishing an assessment or asserting independent corroboration. `normalizerVersion` identifies the trusted adapter. Original record hashes, provider IDs/revisions, observed time, provider revision time, receipt time, source identity, and the reviewed admission remain distinct. Normalized origin time cannot silently differ from the record's origin time.

Publication projects a report into an immutable event revision and, for a located non-withdrawn report, an observation. The event's current pointer selects the newest retained provider revision, with deterministic handling of equal timestamps. Older late arrivals remain in history without rolling the current pointer backward. Newly accepted corrections link to their previously published revision. Pending publication is ordered by provider revision time. History remains bounded to 200 entries per request with an explicit truncation indicator.

The event projection, current pointer, observation, PostGIS spatial alerts, catalog visibility, and publication acknowledgment commit together. A forced catalog failure was verified to leave no partial observation, event, or alert. Its durable evidence and pending receipt remain retryable without a provider fetch.

Spatial rules use the existing active geofences: `ST_Covers` for reported points, `ST_Intersects` for supplied footprints. A match records the triggering observation/evidence, geometry role, geofence ID, stored rule geometry and update time, and `public.event.spatial.v1`. This is a spatial match, not an inferred earthquake impact area or fire perimeter. A correction resolves the prior revision's affected alerts with a correction/withdrawal reason and successor evidence ID, then evaluates the new reported geometry. Prior evidence and alert references remain available. Each event evaluates at most 100 matched zones; excess matches leave publication pending instead of silently omitting alerts.

Direct point-event ingestion also evaluates PostGIS rules and can retry incomplete processing. The existing area and moving-track pipelines remain covered by their prior tests. The API/PostGIS acceptance includes repeated earthquake/thermal reports, a supplied measurement footprint, out-of-order revisions, a withdrawal, publication rollback/recovery, and resource/source isolation, all with zero event-created tracks.

## P1.3: A bounded reference and terrain catalogue

`internal/reference` owns immutable dataset versions, original subset artifacts, and PostGIS reference features. These records do not create observations, tracks, assets, or controllable entities. Imports require a human with `sources.manage` in `PUBLIC_EVIDENCE`; reads require `sources.read`. A registered source and its selected resource/source permissions are enforced through scoped transactions, composite foreign keys, and FORCE RLS. A rights ID continues to name its original reviewed terms across versions.

Every snapshot records producer, source/version, AOI, CRS, vertical datum, resolution/units, validity, quality limitations, transformation, attribution/rights, retrieval reference/date, artifact byte count and SHA-256. A parent artifact checksum is retained when available. The artifact hash is verified before import. An unchanged retry returns the existing version; changing content or metadata requires a new version. Source artifacts and metadata cannot be updated/deleted by ordinary application access.

The initial importer accepts EPSG:4326 GeoJSON and a regular JSON elevation grid. Vectors are validated and clipped with PostGIS; contradictory embedded geometry CRS is rejected rather than relabelled. Features retain provider identifiers and properties. Elevation grids declare point-sample registration, west/south origin, positive steps, south-to-north rows, west-to-east columns, metre heights relative to the declared datum, and null samples for no data. Grid spacing must agree with the stated angular resolution. No height-to-ellipsoid conversion is implied.

Limits are 512 KiB per artifact, 1 MiB per import request, 1,000 features/20,000 vertices per vector snapshot, 256×256 elevation samples, 64 versions/32 MiB per source, and 64 catalog entries per bounded list. Stored artifacts use PostgreSQL `bytea` at this scale; no object-storage service or API raster worker is needed. Larger formats and workloads require a later explicit extension.

The retained sample is the one-degree Jeddah/Makkah region `[39,21,40,22]`:

- **Natural Earth v5.1.2:** Jeddah and Makkah point features at 1:50 million scale. The source is generalized geographic context, with no building or current infrastructure accuracy claim. The subset preserves original coordinates/properties and records the full pinned-source checksum. Natural Earth supplies public-domain data and optional attribution. [Terms](https://www.naturalearthdata.com/about/terms-of-use/), [pinned source](https://raw.githubusercontent.com/nvkelso/natural-earth-vector/v5.1.2/geojson/ne_50m_populated_places_simple.geojson).
- **NOAA ETOPO 2022 v1:** a 60×60 sample of the 60-arc-second bedrock model, using its cell centres as point samples. Heights remain relative to EGM2008, positive up, with bathymetry retained. This is coarse regional relief, about 1.85 km north-south spacing; it is not local engineering terrain. [Product](https://www.ncei.noaa.gov/products/etopo-global-relief-model), [datum guide](https://www.ngdc.noaa.gov/mgg/global/relief/ETOPO2022/docs/1.2%20ETOPO%202022%20User%20Guide.pdf), [distribution terms/metadata](https://coastwatch.pfeg.noaa.gov/erddap/griddap/ETOPO_2022_v1_15s.html).

The prepared vector artifact is 1,436 bytes with SHA-256 `632504397c0c7602b7c925af429f20fa0fb8e19cb8f339e6c77806ba2aab71f7`. The terrain artifact is 62,381 bytes with SHA-256 `d6e4cb7298bf06bab6d4ae8701723616a95c29938a0c61c732e4942449c97fc7`. They are retained in `testdata/reference/jeddah-makkah`. The preparation script retrieved 377,871 elevation-source bytes using checked HTTP ranges, without downloading the approximately 478 MB global TIFF.

`scripts/data/prepare_reference_sample.py` rebuilds these inputs from fixed versioned URLs. It is an offline preparation tool requiring NumPy, tifffile and imagecodecs; the tested versions are pinned in `scripts/data/reference-preparation-requirements.txt`. None runs inside the Go API. Rebuilding produces a new retrieval timestamp; review a new snapshot version rather than changing an existing imported version. The original derived artifacts remain reproducible and downloadable independently of upstream service availability. The initial attempted ERDDAP subset endpoint timed out; the retained data was successfully extracted by byte ranges from NOAA's versioned TIFF instead.

The catalogue's `eligible=true` view exposes exact immutable version IDs/checksums and current rights eligibility for S0. Eligibility does not create an exercise package or grant export authority. S0 will define and authorize the frozen context manifest; this task introduces no competing package format or 3D engine.

## P1.4: A native bounded USGS adapter

The trusted adapter name is `usgs-earthquakes-v1`, restricted to `https://earthquake.usgs.gov/fdsnws/event/1/query`. It uses the existing human-reviewed admission, source-bound service principal, credential refresh, quotas, backoff, byte/time/page limits, exact reference rules, and transactional evidence/publication path. Provider credentials remain server-side; no provider token is needed for this public query.

Queries carry the admitted rectangular AOI, frozen origin-time interval, `eventtype=earthquake`, `format=geojson`, `orderby=time-asc`, one-based offset, bounded limit, `includedeleted=true`, and explicit no-data behavior. A partial cursor preserves the same query interval. A completed query starts the next interval at offset one. The upstream catalogue can change between pages, so its query is not a frozen database snapshot; overlapping bounded queries and revision deduplication allow reconciliation. Offset saturation is rejected instead of skipping remaining results. [Verified native parameters](https://earthquake.usgs.gov/fdsnws/event/1/).

Each feature's ID is retained; `properties.updated` in epoch milliseconds identifies its provider revision. Origin time, reported point, magnitude/type, depth in kilometres, network/code, provider review status, and quality indicators are preserved with the original feature. Depth remains a positive-down provider quantity whose reference may vary by network; it is not terrain or scene altitude. A provider's reviewed status is not an EMAD-reviewed assessment. Native query metadata may omit the summary feed's `count`; the parser handles both forms and validates any count supplied. [GeoJSON contract](https://earthquake.usgs.gov/earthquakes/feed/v1.0/geojson.php).

Explicit `status=deleted` produces a withdrawal revision, preserving older evidence and resolving its prior spatial alerts. It creates no fabricated location/observation for a coordinate-free tombstone. A record missing from a response does not imply deletion. Withdrawals/corrections outside the admitted rolling origin-time window may not be received: this is a bounded catalogue, not a claim to complete global ComCat history. Returned invalid/null geometry for a non-withdrawn record, invalid times, mismatched coverage, malformed JSON, or an outage cannot become a successful empty-world observation.

Recorded fixture data comes from a bounded USGS summary response retrieved on 2026-10-21; its first feature is retained with collection count adjusted to one. Corrected, withdrawn and invalid variants are deliberate test mutations, clearly identified in `internal/publicingest/testdata/usgs/retrieval.json`, not claims about actual historical revisions. An additional recorded native query covers the metadata difference discovered during live acceptance.

The authorized live smoke passed on 2026-10-21 using a temporary human-reviewed admission, exact-source service credential, production restricted HTTPS fetcher, and disposable PostGIS. One Puerto Rico query over the preceding day returned **five reports → five evidence revisions → five event projections → zero tracks**, with no pending publication. This validates the adapter without enabling a permanent source or schedule in the developer installation. ComCat's contributing network remains in each report; review the actual provider/source rights before deployment. [USGS credits policy](https://www.usgs.gov/information-policies-and-instructions/copyrights-and-credits).

## Map and inspection behavior

The existing MapLibre instance owns the new layers. Geographic context uses neutral reference points/lines/outlines; reported events use distinct, screen-stable points and supplied footprint fills. Elevation selection displays dataset coverage, not invented terrain geometry. Dataset selection names an immutable version and preserves its attribution, datum, resolution, quality, eligibility, and checksum. A reference-region action centres the camera. Inputs remain bounded to one selected reference version and at most 200 current retained event revisions; overlaps remain individually inspectable through the report list.

The public panel supplies keyboard/touch controls and a textual alternative to map picking. It shows source coverage/pending publication, supports revision inspection and original provider fields/quality, and downloads exact retained originals or reference artifacts over authenticated APIs. Measurement footprints have dotted outlines, derived buffers dashed outlines, and provider-reported extents/boundaries solid outlines, with explicit role labels. Original hashes are verified by the real-backend browser test. The panel sits outside floating map instruments and has its own bounded scroll area on desktop/mobile. Map or basemap failures leave source facts inspectable. Errors are distinct from a successful empty bounded view. Normalized provider statements are limited to 32 KiB and supplied footprints to 512 vertices, in addition to the ingestion/page and displayed-record limits.

Queries/selection use the established workspace cancellation, cache, permission, and remount lifetimes. The new views poll every 15 seconds for reports/health; they do not add a truth-visible WebSocket topic. Reference/report selection is local to the active workspace lifetime; this increment does not add persistent saved views. The existing track replay control does not reconstruct event history; public overlays explicitly identify that limitation during replay. P1.6 still owns comprehensive temporal filters and effective-time versus known-time views.

## Interfaces, setup, and rollout

New scoped APIs:

- `GET /api/v1/public-events`, `GET /api/v1/public-events/{evidenceId}`, and `GET /api/v1/public-events/{evidenceId}/revisions` require `observations.read` in Public Context. Lists include explicit withdrawals; location overlays omit them.
- `GET/POST /api/v1/reference-datasets`, plus version detail, `/features`, and `/artifact`. Reads use `sources.read`; import uses human `sources.manage`. The `eligible` query restricts versions to current simulation-use eligibility.
- P1.1 source admission, evidence, original-download, run and publication-retry APIs remain the execution and provenance boundary. OpenAPI and generated TypeScript include the additive contracts.

Apply migrations **00023_public_event_semantics.sql** and **00024_reference_catalog.sql** after version 22. Startup now requires schema 24. New event/reference tables enforce public resource/source ownership; retained revisions/artifacts are immutable. Down refuses destructive rollback; use the documented forward recovery/backup procedure. Existing observation IDs, moving scenarios, exercise assets/commands, and earlier evidence remain compatible. No developer database was migrated or populated during implementation.

To load the retained reference sample into an authorized installation:

1. Apply migrations through 24 and provision a human public reviewer with `operational.read`, `sources.read`, and `sources.manage` using the established F0 procedure.
2. Register separate external reference sources for Natural Earth and NOAA using that public resource. Retain their returned canonical IDs; a source is not an authenticated principal.
3. Run the import helper explicitly, using a human credential file and those IDs:

```powershell
./scripts/data/import_reference_sample.ps1 `
  -TokenFile ./private/reviewer-token.txt `
  -WorkspaceId public-context -EnvironmentId world `
  -PlacesSourceId RETURNED_NATURAL_EARTH_ID `
  -ElevationSourceId RETURNED_NOAA_ID
```

The helper sends the checked-in artifacts to the existing API, prints returned IDs/versions/checksums, and never prints the credential. Select Public Context → Map to inspect the imported versions. It does not fetch arbitrary provider URLs or grant an exercise access.

To activate USGS, register a separate external source; provision an exact-source service credential with `operational.read`, `sources.read`, and `connectors.execute`; add a trusted location using the adapter/URL above and an explicit server quota; then approve its producer, product, rights, AOI and retrieval policy through P1.1 admission. Invoke `connector-runs` with the service credential. The manifest remains unset by default, and no polling service or long-running public feed is enabled automatically. See [P1.1 operation](P1_1_PUBLIC_INGESTION.md) for the manifest, retry and suspension contracts.

For explicit live verification after a build with Docker available:

```powershell
$env:C4ISR_TEST_INTEGRATION = '1'
$env:C4ISR_TEST_USGS_LIVE = '1'
go test -count=1 -run TestUSGSAuthorizedLiveSmoke -v ./internal/integration
```

## Verification

Executed checks cover semantics, raw provider parsing, duplicate/revision/withdrawal handling, exact hashes, PostGIS spatial matching, publication rollback/recovery, reference metadata/geometry/terrain import, immutable snapshots, quotas and resource/source isolation. Existing scenario/area/spatial/workflow regressions remain in the full suite. The production browser gate now includes the public report/reference/original-download workflow in addition to scenario-to-command/audit and truth/revocation checks.

Verification used native Go 1.25, Node 24/pnpm 10, Chromium, PostgreSQL 17/PostGIS 3.5 Testcontainers, and Linux Go 1.25 for the race detector. The developer database was not changed.

- `go test ./...` and `go vet ./...` passed. The ordinary run intentionally skips opt-in database/browser/live cases, which were run separately.
- Full PostGIS suite: **41 top-level tests and 24 subtests passed in 240.902 seconds**. The live and browser gates were skipped in that invocation and passed independently. The final focused public/reference regression run also passed after tightening import validation and normalized-statement budgets.
- Mocked browser suite: **70 passed, 50 intentional project skips**, including the new public inspection, reference selection, original download, layer readiness, and narrow-screen interaction. Existing alternate Vite ports 25173/25174 were used because the default port was occupied.
- Production-UI/real-Go/PostGIS gate: **3 browser flows passed** (12.6 seconds of browser execution; 23.252 seconds for the Go invocation). This includes an exact-original SHA-256 comparison and retained elevation version inspection, plus the prior scenario-to-audit and truth/revocation flows. No API/WS responses are mocked in this gate.
- The authorized native USGS live smoke passed: **1 request, 5 reports/evidence/event projections, 0 tracks**, with no pending publication. It creates and removes a disposable database, rather than a permanent provider schedule.
- Race checks passed for observations, tracks, public ingestion, access and HTTP authorization. Event rules, public event projection and reference packages compiled under the detector; their transaction behavior is exercised in PostGIS tests. CI/Taskfile race targets include the new modules.
- Production TypeScript/Vite build, regenerated OpenAPI types, Redocly lint, sqlc generation, canonical Go formatting, whitespace, Python preparation syntax, and PowerShell import-helper parsing passed. The prior Vite bundle-size advisory and mocked socket shutdown/cancellation messages remain non-failing diagnostics.

Raw local evidence is in ignored `tmp/p1-2-4-*` and `tmp/p1-4-live.log` files. Desktop/mobile inspection images are `tmp/p1-2-4-public-context-desktop.png` and `tmp/p1-2-4-public-context-mobile.png`. F0 compatibility remains accepted; P1/M1 remains incomplete until P1.5–P1.6 pass their own gates.
