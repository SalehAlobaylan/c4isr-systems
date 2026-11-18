# EMAD diagram prompts

Copy one text block and send it to a diagram agent. Each block contains the context the agent needs. The repository references below each prompt are optional. The feature baseline is commit `8fe5b1c`, and the status descriptions reflect its documented evidence. For an updated status graphic, supply the latest acceptance record with the prompt.

## System overview and geography

### 01. EMAD at a glance

```text
Create a landscape system architecture diagram titled “EMAD at a glance” for a technical audience new to EMAD. EMAD is a map-centered Public Context and geographic environment preparation application.

Arrange the diagram in three tiers. At the top, put an operator browser with the React workspace, MapLibre Atlas, Environment Library, and Three.js geographic preview. In the middle, put a single Go API boundary containing scoped access, Public Context and analyst queries, geographic preparation and releases, and context-package export. At the bottom, put PostgreSQL/PostGIS for queryable records, metadata and durable jobs, plus object storage for retained source bytes, prepared ZIPs and display derivatives. Outside the API boundary, show admitted public data providers, preparation workers and an authorized receiver. Label the request and data arrows. Show the two HTTP preparation workers reaching storage through the API, and label the separate display-only worker as a distinct path.

Show two paths converging in Atlas and the 3D preview. Public provider reports become scoped evidence. Geographic source assets become prepared terrain and other display layers. Show a verified geographic package or frozen handoff as the boundary for future consumers. Put any future simulation consumer outside the delivered system with a dashed arrow and the label “Future consumer”.

Use solid arrows for implemented data or request paths, and dotted outlines for external actors. Highlight Riyadh as the detailed reference, Saudi Arabia as national discovery, and the Middle East as regional discovery. Deliver editable diagram source, an SVG if export is available, and a short caption.
```

[System architecture](ARCHITECTURE.md) · [Product scope](EMAD_CURRENT_SCOPE.md) · [Worker deployment](docs/implementation/GEOGRAPHIC_OPERATIONS_AND_UPGRADE.md)

### 02. Riyadh, Saudi Arabia and Middle East coverage

```text
Create a layered map or a three-panel geographic coverage diagram titled “From regional discovery to a detailed Riyadh environment”. Use an outline map only if reliable boundary geometry is available. With no geometry, use three nested labeled frames and a side panel.

Read from broad to narrow. The Middle East uses a configured 17-country working profile for discovery and source coverage. Saudi Arabia uses retained national and first-order administrative geography for navigation. The bounded Riyadh study area is the first detailed reference release, with terrain, imagery, retained vectors and local streets. Place Al Ain in the side panel as the second local-frame check, Jubail as a bounded coastal pilot, and Jeddah and Avignon as regression examples.

Distinguish “navigation coverage”, “staged detailed release” and “bounded pilot or regression” in the legend. Show the reviewed data coverage stopping at the study-area boundary. Treat the 17-country set as a working definition and leave precise boundaries or memberships unlabeled unless supplied. Deliver editable diagram source, an SVG if available, and a one-sentence caption about the different coverage levels.
```

[Geographic priorities](plans/README.md#geographic-priorities) · [Middle East profile](scripts/data/region_profiles/middle-east-v1.json) · [Riyadh profile](scripts/data/region_profiles/riyadh-reference-v1.json)

### 03. How public reports become evidence

```text
Create an annotated data-flow diagram titled “From public report to retained evidence”. Use a fictional regional earthquake report as the example. Show a provider, the approved connector and retrieval profile, retained source material and receipt, parser, versioned evidence record, scoped query, and Atlas or analyst workbench. Include optional analyst review as a separate step after inspection. Label the report geometry, provider identity and revision, observation time, platform publication time, and retrieval time where they enter the record.

Branch acquisition into completed, partial and failed retrievals. Separately, branch the scoped query into matching retained records, zero matching retained records and an unavailable result. Keep provider coverage and query status visible beside the map result. A correction may create a later revision while earlier retained evidence remains available to an authorized as-of query. Indicate that “reported by the provider” and “independently reviewed” are two different evidence states. Keep the fictional example clearly labeled.

Deliver an editable flowchart, an SVG if available, and a plain-language caption for an analyst handbook.
```

[Public ingestion history](docs/implementation/HISTORICAL_DELIVERY.md#p1-1) · [Map operations](docs/implementation/MAP_FIRST_OPERATIONS.md) · [Implementation record](plans/IMPLEMENTATION_PROGRESS.md#context-handoff-and-viewer-qualification)

## Environment foundation and operations

### 04. Source admission, lineage and coordinate quality · F01, F02

```text
Create an entity and data-lineage diagram titled “From source admission to geographic quality”. Use a bounded Riyadh environment as the example. Show Source edition and receipt → Source admission revision and rights → Retained input asset → Build with exact area, profile version and toolchain digest → Geographic release manifest and prepared ZIP → Display derivatives. Label the identifiers or digests that bind each step. Use one-to-many relationships where several retained inputs feed one build or one release has several display artifacts.

Add a parallel quality path for CRS, vertical datum, pixel registration, masks and source footprints, then independent numerical coordinate checks. Keep numerical transform verification and local physical accuracy as separate fields, with “Physical accuracy unknown” for the Riyadh reference. Add a steward path that inspects downstream dependencies and records a new admission revision. Original lineage stays on the release and later reads also check current rights. Show that a stale expected revision is a conflict and a tightened policy can block a subsequent online read.

Deliver an editable diagram with a small legend for immutable identities, versioned policy and derived outputs. Include an SVG if export is available.
```

[Source and quality qualification](docs/implementation/GEOGRAPHIC_SOURCE_QUALITY_AND_STORAGE.md) · [Shared identities](plans/IMPLEMENTATION_PROGRESS.md#shared-contracts)

### 05. Durable preparation workers and publication · F04, V01

```text
Create a sequence diagram titled “Durable geographic preparation and publication”. Use lanes for authorized requester, Go Environment API, PostGIS job state, HTTP preparation worker, and object storage. Show a request with a scoped idempotency key, expected parent manifest digest and recipe or toolchain pin. The API creates or returns a durable job. An eligible worker claims it under a deadline, verifies its exact inputs, processes them in local scratch space, sends a heartbeat, submits output for verification, and completes under its live lease. The API validates size, digest, recipe and parent identity before recording one visible manifest. Label object transfer through the API for the HTTP worker. Show workspace capacity and required source grants at claim time.

Add alternate branches for a worker dying mid-attempt, a lost completion response after commit, and uploaded bytes followed by an expired lease or failed publication. Label the resulting behaviors “retry from durable state”, “return the already-published result”, and “keep unreferenced bytes private for reconciliation”. Show cancellation as a separate exit if there is room. Keep branch labels short and readable.

Deliver editable sequence source and an SVG if available. Give the published derivative its own identity and connect it back to the original parent manifest.
```

[Worker contract](api/openapi/environment-worker.yaml) · [Display contract](api/openapi/environment-display.yaml) · [Worker and publication qualification](docs/implementation/RIYADH_DURABLE_GEOGRAPHIC_PREVIEW.md#v01-durable-publication)

### 06. Geographic release, human review and receiving · F03

```text
Create a state diagram titled “Geographic release lifecycle” with the exact states STAGED, VERIFIED, RELEASED and WITHDRAWN. Label STAGED → VERIFIED with “authorized verification decision” and VERIFIED → RELEASED with “authorized release decision”. Show withdrawal from STAGED, VERIFIED or RELEASED and keep authorized decision history visible after withdrawal. Use a separate annotation for a denied transition due to missing rights, stale state, a changed digest or corrupt bytes.

Beside the states, show a short reviewer and receiver swimlane. The reviewer inspects source lineage, download rights, geographic quality, manifest and prepared ZIP bytes. The independent receiver discovers the permitted released entry, verifies ZIP and display size and SHA-256 against the supplied pins, and records a receipt and walkthrough. Attach two evidence labels: “Riyadh documented as staged on the shared installation” and “Receiver path exercised in an isolated copy”.

Deliver editable state-diagram source and an SVG if possible. Use text labels for every decision and status, including when a download becomes unavailable.
```

[Release and receiver workflow](docs/implementation/RIYADH_RELEASE_RECOVERY.md#f03-release-review-and-receiving) · [Release implementation](internal/geoenv/releases.go)

### 07. Delivery, retention and portable verification · F05, F06, V05

```text
Create a two-panel architecture diagram titled “Delivering and verifying geographic artifacts”. Panel 1 shows the online lifecycle. Draw an authorized request going through scope, parent-release and source-rights checks to a full download or single byte range. Show size, SHA-256 and ETag checks, resume after interruption and restart after an ETag change. Draw storage reservation, retention deadline, purge, and retry after a store fault. Keep online access decisions and physical deletion deadlines in separate lanes.

Panel 2 shows three outputs. The prepared package is the verified processing output. The viewing package contains a constrained set of display bytes and local navigation for offline inspection. The rebuild package supplies permitted exact originals and pinned toolchain instructions for an offline rebuild. Show the receiver verifying the package manifest and member digests. For a rebuild, show the pinned processor running with networking disabled and compare results with the expected prepared identities. Label rights and release eligibility at issuance, and use a clear failure exit for missing or corrupt members.

Deliver editable diagram source and an SVG if available. Use distinct shapes for a database record, byte artifact and policy gate. Keep example hashes symbolic.
```

[Delivery and retention](docs/implementation/GEOGRAPHIC_SOURCE_QUALITY_AND_STORAGE.md#f05-artifact-delivery-and-lifecycle) · [Portable verifier](docs/implementation/RIYADH_RELEASE_RECOVERY.md#f06-portable-verification-and-rebuild) · [Local and offline viewing](plans/IMPLEMENTATION_PROGRESS.md#riyadh-geographic-display-and-offline-evidence)

### 08. Recovery, observability and upgrade readiness · F07–F10

```text
Create an operations diagram titled “Proving the installation is ready”. Use three stacked tracks. The recovery track pairs a quiesced PostgreSQL backup with its object-store inventory, restores both into isolation, and compares exact bytes for Riyadh, Al Ain and Avignon. Show the previous API read check and the return to the candidate API as tests on the restored copy. The operations track shows authenticated metrics from API and workers to Prometheus and Grafana. Label queue age, worker heartbeat and lease, retries, failures and retained storage. Connect warning conditions to an operator runbook.

The acceptance track shows the pinned migration 58 installation → 24-hour metadata observation → worker restart and storage trend evidence → readiness ledger tied to the API process generation. Put the migration 59 rehearsal on a separate branch labeled “Isolated qualification”. From the ledger, draw a gated shared cutover path with paired backup, contract/client checks and post-cutover reads for bytes, queues, storage and the viewer. Display installed, isolated and pending as separate labels. Treat the 24-hour observation, independent release and receiver walkthrough as open unless a newer acceptance record is supplied.

Deliver editable diagram source, an SVG if available, and a short description of what evidence advances each gate.
```

[Recovery rehearsal](docs/implementation/RIYADH_RELEASE_RECOVERY.md#f07-paired-backup-and-recovery) · [Operations and upgrades](docs/implementation/GEOGRAPHIC_OPERATIONS_AND_UPGRADE.md) · [Installed readiness](docs/implementation/COASTAL_MOVING_AND_READINESS.md#f09--installed-readiness)

## 3D preview and analyst workflows

### 09. How a place is built · V02, V03

```text
Create an exploded technical diagram titled “How a Riyadh place is built”. Use a small study-area tile with a native elevation raster at the bottom, then the terrain mesh, dated optical imagery and individual roads, building footprints and water layers. Show the source grid and polygon masks clipping the mesh, with holes and separate area components still visible. Include an inset for the alignment checks on two adjacent elevation tiles: same CRS, datum, spacing and registration, plus agreement on overlapping samples. Send misaligned inputs to an explicit validation failure.

Label the rendering axes “east, up, south” and the map location “WGS84 longitude and latitude”. At a picked terrain point show an optional WGS84 ellipsoidal height from the rendering workflow. Attach separate source date and identity tags to imagery and vectors. Highlight a vector with a partial drape and its inspectable original identity. Show the terrain loading first and each chosen overlay loading and verifying independently. Preserve the distinction between numerical alignment and unknown local physical accuracy.

Deliver an editable exploded diagram, an SVG if available, and concise alt text. Keep text large enough for a technical presentation slide.
```

[Terrain and display pipeline](docs/implementation/RIYADH_DURABLE_GEOGRAPHIC_PREVIEW.md) · [Native terrain results](plans/IMPLEMENTATION_PROGRESS.md#native-grid-terrain-results)

### 10. Atlas to 3D and back · V04, V08

```text
Create a user journey storyboard titled “Inspect Riyadh in Atlas and 3D”. Show five numbered actions: open Atlas or search Riyadh, choose the exact Environment Library release, expand “3D terrain preview” and select “Open terrain”, inspect a terrain coordinate or a retained feature with optional overlays, then return to Atlas and reopen the same selection. Carry a compact identity tag across screens with area, release and manifest digest, display ID, selected layer and feature ID when applicable. Show local PMTiles street detail as a separate optional Atlas view for its retained extent and zooms 12–16.

Include a compact-view callout for keyboard-accessible terrain picking, layer controls and an actionable failure message if a selected artifact cannot be verified. Show desktop, compact portrait and compact landscape as tested browser profiles. Keep “physical mobile evaluation open” separate from those profiles. Use schematic interface panels and the exact action labels given above. No real operator or login data is needed.

Deliver editable storyboard source, an SVG if export is available, and brief alt text describing the round trip.
```

[Shared layers and Atlas handoff](plans/IMPLEMENTATION_PROGRESS.md#3d-viewer) · [Viewer qualification](plans/IMPLEMENTATION_PROGRESS.md#context-handoff-and-viewer-qualification)

### 11. What a quiet map means · V06 and map startup fixes

```text
Create a troubleshooting flowchart titled “When the map is quiet”. Start with two parallel reads. The basemap reads admitted country geography directly. The evidence view reads earthquakes, thermal detections, weather stations, official bulletins and retained imagery with the selected area and time. Draw the basemap independently so country outlines can render while an evidence request retries.

Give the evidence read separate outcomes: results available, zero matching retained records, partial or reduced display, and unavailable or stale. Keep provider coverage, observation time, acquisition attempts, query completeness and renderer reduction visible as distinct signals. Show “Last 24 hours” as a valid example of a successful empty result. Make coverage and retry the corresponding user actions for incomplete or failed results.

Add a small lifecycle inset. An account has a bounded pool of temporary map and catalog reads. Normal reload or page exit releases the handles. After an interrupted cleanup, a handle expires within five minutes. A capacity error triggers a retry. Use “Temporary evidence read limit reached” so the diagram never equates a slot count with a number of open tabs.

Deliver an editable decision diagram, a short alt text and an SVG if available. Label all states in words rather than relying on color.
```

[Context coverage and freshness](plans/IMPLEMENTATION_PROGRESS.md#context-handoff-and-viewer-qualification) · [Atlas implementation](apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx) · [Page lifecycle](apps/operator-ui/src/components/WorkspaceProvider.tsx)

### 12. The report that arrived late and the frozen handoff · V07

```text
Create a two-panel timeline and sequence diagram titled “Freeze what was known by the cutoff”. Use this fictional example: an observation occurs at 08:20 UTC, the platform publishes the original at 09:05 UTC, the analyst chooses a 09:30 UTC known-by cutoff, and the platform publishes a correction at 10:15 UTC. Show the observation window and the publication cutoff as independent filters. At 09:30 the original revision is eligible and the correction is beyond the cutoff.

In the second panel draw Analyst → scoped read → context package API → independent receiver. The analyst selects exact retained revisions and submits a handoff request with the exact release and display digests. The API checks the area revision, interval, cutoff, source grants and original and current export rights. It returns an immutable handoff manifest. On reopening, show either the same pinned membership or an explicit unavailable-dependency response if permission or a dependency has changed. Explain that selected observations are historical inputs a future application can consume. Tag fixture verification separately from Riyadh release and export acceptance.

Deliver editable diagram source, an SVG if available, and an alt-text sentence summarizing which revision is included and why.
```

[Frozen handoff evidence](plans/IMPLEMENTATION_PROGRESS.md#context-handoff-and-viewer-qualification) · [Handoff API](api/openapi/geographic-context.yaml)

## Data pilots and layer limits

### 13. Land cover, modeled ground and building evidence · D01, D02, D08

```text
Create a three-column geographic data diagram titled “Riyadh enrichment layers”. In every column show retained source → preparation → viewer result → quality limit. The first column uses ESA WorldCover 2021 categories with the original grid and nodata, a categorical legend and a dated optical comparison for inspection. The second uses the Copernicus surface model and GEDTM30 modeled ground as separate inputs. Compare them in a common EGM2008 vertical reference, display surface, ground and signed difference independently, and mark physical accuracy unknown. The third keeps OpenStreetMap footprints separate from GHS-OBAT modeled height estimates at their centroids. Show modeled height and unknown height as explicit outcomes. Keep centroid estimates disconnected from footprint extrusion.

Add a shared footer with source edition, date or model period, units, rights and quality evidence. Label the result “Bounded Riyadh enrichment qualified in disposable staging”. Keep the shared-installation rollout and human release decision separate. Deliver editable diagram source plus an SVG if available.
```

[Riyadh enrichment pilot](docs/implementation/RIYADH_ENVIRONMENT_ENRICHMENT.md)

### 14. Radar imagery, flood reports and pressure fields · D03–D05

```text
Create a three-lane data-flow diagram titled “Three kinds of retained observation context”. Use an exact source pin, native subset or revision, authorized display, and known limits in each lane. Lane 1 starts with the bounded Riyadh Sentinel-1 VV acquisition. Show terrain-corrected backscatter, native valid-sample mask and grayscale dB display. Keep dated optical imagery on a separately labeled selector. Lane 2 starts with bounded NASA EONET historical flood reports. Keep original provider revision, reported geometry history and cited sources visible. Show a partial or failed query separately from an empty one. Lane 3 starts with a historical central Saudi ERA5 pressure-level grid and displays temperature and winds at native 850 and 700 hPa samples. Show the below-ground mask. Put METAR station observations in a separate branch as station context.

Use the case-study labels “backscatter inspection”, “provider-reported flood history” and “historical reanalysis”. Keep visual inspection separate from conclusions about flood detection or local weather accuracy. Mark these as bounded pilots and show the review and rollout gates. Deliver editable diagram source and an SVG if available.
```

[Riyadh and central Saudi observation pilots](docs/implementation/RIYADH_OBSERVATION_PILOT.md)

### 15. Jubail coastal geography and bathymetric context · D06

```text
Create a technical cross-section and map inset titled “Jubail coastal context”. Use a schematic shoreline and a small offshore sampling grid. Show two aligned GEBCO inputs, the signed elevation/depth grid and the source-type grid, plus a separately sourced Natural Earth coastline. Trace one native grid cell into the viewer and annotate its location, value and units, source type, and validity. Label source-type fill separately from valid depth. Keep the original grid resolution visible and show where the two inputs and coastline are retained in the prepared release.

Show the bounded pilot passing scoped inspection and an offline rebuild in disposable staging. Place two quality labels adjacent to the result: “Sample consistency checked” and “Physical accuracy unknown”. Keep the schematic independent of navigational, survey, water-level and physical seabed decisions. Deliver an editable vector diagram, an SVG if available, and short alt text.
```

[Jubail coastal pilot](docs/implementation/COASTAL_MOVING_AND_READINESS.md#d06--saudi-gulf-coastal-context)

### 16. Moving observations and civil orbital context · D07

```text
Create a split diagram titled “Retained moving reports and computed orbital context”. On the left, show one aircraft or vessel track in one saved area and a bounded time window. Use reported points and a timeline with gaps and quarantined reports. Label provider revision, report time, platform publication time and association decision. Show the optional known-by cutoff and a handoff pin for selected exact reports. Use fixture data, and label real Saudi moving-history intake as requiring source admission. Leave gaps open instead of drawing interpolated paths.

On the right, show the private civil NOAA-20 example. Draw retained CelesTrak GP elements and epoch → versioned offline SGP4 and Skyfield processing → UTC samples with TEME position and velocity → WGS84 subpoints and ellipsoidal heights. Mark the results “computed subpoints” and show the stale-window rejection. Show physical accuracy and instrument coverage as unqualified, and redistribution as pending review. Use distinct symbols for provider reports and computed samples.

Deliver editable diagram source, an SVG if possible, and short alt text that explains the two kinds of positions.
```

[Moving and civil orbital qualification](docs/implementation/COASTAL_MOVING_AND_READINESS.md#d07--historical-moving-reports-and-civil-orbital-context) · [Moving history API](api/openapi/moving-history.yaml)
