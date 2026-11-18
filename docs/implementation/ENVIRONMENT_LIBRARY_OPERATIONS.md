# Geographic Environment Library

The library prepares reusable geographic reference packages from saved Public Evidence areas. It preserves source resolution, dates, masks, rights, and the original input identities in each manifest. Packages carry WGS84 geometry and a declared local east, north, up frame. The included EGM2008 grid and checked conversion method let consumers convert source orthometric heights when needed.

## Install and provision

1. Apply migrations through 59 before installing the source-review, quality and lifecycle candidate. The shared staging observation is pinned to migration 58. The ordinary local setup builds the worker. Staging worker profiles are defined in `deployments/staging/delivery.yml`.
2. Run `python scripts/data/environment_library.py toolchain` using the installed Python interpreter. Configure the resulting digest as `C4ISR_ENVIRONMENT_TOOLCHAIN` on the API and worker. The Windows supervisor sets it when the feature is enabled. Use the staging image's digest for staging.
3. Provision an explicit Public Evidence service identity with `operational.read`, `environments.execute`, and grants for the exact source set. Put its credential in `private/local/credentials/worker-environment.token` locally or the corresponding staging secret. Use `emad-admin` for provisioning. Grant execution on the exact retained profile revision through `POST /sources/{id}/profiles/geographic-retained/execution`.
4. Provision a data steward with `sources.read`, `sources.manage`, `environments.read`, and `environments.manage`. Give authorized users `environments.build`, `environments.qualify`, `environments.release`, and `environments.download` for their work. Every contributing source grant applies to packages and their metadata.
5. Enable `C4ISR_ENVIRONMENT_LIBRARY_ENABLED=true`. For staging, install the API before starting the Compose environment worker profile. To suspend new work, stop the workers and set the feature flag to false while retaining the toolchain configuration and storage. This leaves read access to eligible history.

The API contract includes the scoped discovery, definition revision, preflight, preparation, review, and download operations. [OpenAPI contract](../../api/openapi/environment-library.yaml)

### Run a worker through the API

Use the same exact-source service credential and processor digest as the direct worker. Set `C4ISR_ENVIRONMENT_TOOLCHAIN` to the worker processor digest and optionally set `C4ISR_ENVIRONMENT_SPOOL` to a private scratch directory. Point the worker at the API origin and use the token file for its default Public Evidence scope.

```text
emad-environment -api-origin https://api.example.internal -token-file /run/secrets/environment-worker
```

This mode needs API connectivity and the pinned processor dependencies. Use HTTPS across host boundaries. A private lab can use a local HTTP origin. The API keeps database and object-store access and fences each input transfer, heartbeat and publication by the live lease. An exact repeat after lost completion can acknowledge the existing release under the same service and lease. The API stores a digest of that proof. Drain active jobs before changing transports and qualify the selected deployment using the same release and recovery checks as direct workers. [Worker API contract](../../api/openapi/environment-worker.yaml)

The optional `environment-http` profile provides two independently credentialed workers. Install their tokens as `worker-environment-http.token` and `worker-environment-http-secondary.token` in the staging secrets directory. The workers use private scratch volumes and cap each processor attempt at 8 GiB. Pause new submissions during cutover and let active leases finish before stopping the direct workers. To retain durable terrain display jobs, run one direct worker with `-display-only` and the configured display digest. [Transport rehearsal](GEOGRAPHIC_OPERATIONS_AND_UPGRADE.md#f04-database-free-preparation-workers)

Set `C4ISR_ENVIRONMENT_LAYER_TOOLCHAIN` on the API and direct display worker to the output of `python scripts/data/prepare_geographic_display.py --toolchain` from the deployed image. Keep `C4ISR_ENVIRONMENT_DISPLAY_TOOLCHAIN` configured while serving queued v1 terrain jobs. The layer worker supports native-grid terrain v2, optical imagery, roads, buildings and water. Reviewers can open **Build a display** in the Environment Library to monitor or cancel the job. Reopening the same release in the same browser account and workspace restores the saved job status. Optical builds take the retained Sentinel-2 scene JSON for the exact admitted acquisition. The worker runs offline and checks the declared scene, bands, radiometry and hashes before publication. The separate reviewed publication routes remain available for specialized profiles and street PMTiles.

### Inspect queue and storage health

Scrape `/api/v1/environment-operations/metrics` with a dedicated service credential carrying `operational.read` and `environments.monitor`. Its default resource and source grants determine the metric coverage. Keep each scrape target scoped to its assigned environment. An environment steward with `operational.read` and `environments.manage` can use the same endpoint.

```yaml
scrape_configs:
  - job_name: c4isr-environment
    metrics_path: /api/v1/environment-operations/metrics
    scheme: https
    authorization:
      credentials_file: /run/secrets/environment-monitor
    static_configs:
      - targets: ["api.example.internal:443"]
```

The gauges distinguish waiting jobs, active leases, expired leases, failed jobs, pending reservation bytes, retained logical bytes and assets awaiting deletion. If an eligible queue waits beyond the pickup budget, check worker readiness and the installed toolchain digest. If leases remain expired, inspect the persisted attempt history and source grants before retrying. Check that the relevant gauge clears when the queue recovers. The [alert rules](../../deployments/observability/alerts.yml) use these scoped scrapes.

The `environment-observability` profile runs a scoped Prometheus with a token read from `environment-metrics.token`. On Linux, make this file and the two worker token files readable by the service UID 65534 while keeping the host secrets directory private. Grafana provisions the **Geographic environment operations** dashboard. The dashboard shows the largest reported scratch usage among fresh worker identities when samples are available. For an object-read alert, check the private store and confirm that the scoped probe returns to one. For a reservation alert, inspect pending uploads and retained objects before expiring data through the normal policy. For source expiry, check the affected admissions through the source catalog and renew only with reviewed terms. For scrape failure, verify the monitor credential, its scope, and API connectivity. [Metrics and alert qualification](GEOGRAPHIC_OPERATIONS_AND_UPGRADE.md#f08-operations-and-alerting)

## Admit and prepare data

Register external sources for retained products. Record reviewed terms, attribution, retention, and explicit geographic output permissions using `POST /environment-sources/{id}/admissions`. This creates a dedicated retained profile revision. Raw redistribution needs its own permission because rebuild bundles contain the original bytes.

### Review and renew a source

In Environment Library, enter a retained source ID or select **Inspect this source policy** from release lineage. The steward view shows the current policy, affected releases, display counts, retained bytes, pending uploads, tombstones and earliest effective expiry. It remains available to stewards with the exact source grant when the current policy blocks routine release access. Review all dependency pages before changing a policy.

Use `scripts/ops/review_environment_source.py --api ORIGIN --token-file FILE --workspace WORKSPACE --environment ENVIRONMENT SOURCE_ID inspect` to capture the scoped impact response. For further pages, pass `inspect --after CURSOR`. Prepare a reviewed admission document with the existing producer, provider, dataset, edition, coverage and area, plus the reviewed `rights`, `enabled`, `intendedUse` and `expectedRevisionId` from the inspected head. Use a new rights ID for changed terms. Replace `inspect` with `renew REVIEWED_POLICY.json` to submit the decision under the reviewing steward's credential. The revision comparison protects against concurrent changes. Record fresh execution grants for authorized workers on the accepted revision.

The original admission and the current head jointly constrain continued use. Renewals preserve the original expiry, attribution and provenance of an installed package. A product edition or a newly authorized use requires the appropriate new admission and intake before a new release can use that permission. Store terms evidence and credentials privately.

### Review geographic quality

For each reviewed candidate, create a private bindings file with a display `name` and an `assets` map. Each key is the candidate input ID in `recipe.json`. Each value names the approved `sourceId`, `admissionId` and `retainUntil`. Include `referenceId` or `sceneId` when the exact retained parent matches the original record in that scope.

Use `scripts/data/intake_environment.py` with the candidate directory, bindings file, API origin, credential path and exact workspace and environment. It verifies the existing prepared package and the original hashes, stages the sources, saves the area and returns preflight diagnostics. Existing matching source reservations can be reused.

In Map → Workbenches → Environment Library, select a saved definition or save a bounded area and choose its retained layers. Check the preparation diagnostics before starting a build. To review a staged release, open its pinned link and check the release status, manifest digest, source dates, resolution, attribution and limitations. Download the prepared package and run the verifier below against the displayed manifest SHA-256. Record verification with a note about what you checked, then record release with a separate note. Both actions bind to the displayed digest. Withdrawal ends download availability while preserving the release history.

The profile endpoint lists native formats, coordinate rules and preparation limits. New build recipes pin the profile contract and its digest. Use **Coordinate and sample checks** to review the recorded numerical error, selected and valid samples, pixel registration and vector counts where available. A legacy manifest can supply sample and coordinate evidence with an empty profile digest. Provider uncertainty comes from the source record. A numerical check within 1 mm measures transformation agreement. Leave physical accuracy `UNKNOWN` until an independent survey or control review supports a claim.

### Recover interrupted transfers and reclaim storage

The original, prepared and display endpoints support a single byte range and use a quoted SHA-256 ETag. `If-Range` must match exactly for partial delivery. A mismatch returns the full object. Protected responses use `Cache-Control: no-store`. The stream opens under the rights checked for that request. The next request rechecks permissions, and a short storage read aborts the stream. Use the digest and advertised length to verify completeness before publishing downloaded bytes. The installed verification client resumes under the exact ETag and publishes the verified temporary file atomically.

Open **Storage and cleanup** with an environment steward credential to inspect waiting uploads, failed-build associations and uncleared tombstones. Original, prepared and display reservations share a 16 GiB limit per owner within the environment resource. Every unpurged reservation consumes capacity. Physical cleanup marks a tombstone, retries failed deletion after 15 to 300 seconds, and returns the capacity after the store accepts deletion. A live upload retains its grace window. The HTTP worker reconciles API upload scratch through its scoped endpoint. If a deployment uses only direct workers, invoke the same reconcile endpoint with an authorized worker to clear abandoned API upload directories.

The stored retention deadline controls routine physical deletion. A later source policy change can block online access before that deadline. A steward should record any revised physical retention obligation and handle it with the storage operator when the existing deadline is later. Authorized offline copies follow the terms recorded at issuance.

To reproduce the three-area installation, use `scripts/ops/commission_environment_library.py register` with the scoped steward token, `scripts/data/region_profiles/environment-intake-v1.json` and the private candidate root. Provision the service identity with the exact source IDs returned in the commissioning record. Then run the same command with `stage` and `--worker-principal`. It installs the retained inputs and regional navigation, runs preflight and submits idempotent build requests. Run `scripts/ops/check_installed_environment_library.py` with the completed commissioning record, reviewer credential and API origin to verify the installed manifests and byte ranges and download prepared packages to a private folder.

Prepare the bounded Riyadh candidate with `prepare_environment_candidates.py`. Point `--retained` at the directory containing `raw/riyadh-copdem30.tif` and its receipt. Supply the retained Sentinel-2 capture directory with `--imagery-capture`. The command uses the retained EGM2008 grid and GCC OpenStreetMap snapshot from the private local source cache.

```text
python scripts/data/prepare_environment_candidates.py riyadh --area-profile scripts/data/region_profiles/riyadh-reference-v1.json --retained <data-directory> --imagery-capture <capture-directory> --output <candidate-root>/riyadh-candidate --rebuild
```

Supply the reviewed intake profile with `--review` and the parent candidate directory with `--candidate-root` when commissioning. The candidate geometry must match the bounds in the intake profile. Use `stage --areas-only` to stage a new detailed release with the installed Saudi and Middle East navigation. [Geographic intake contract](../../plans/IMPLEMENTATION_PROGRESS.md#shared-contracts)

## Verify and rebuild offline

```text
python scripts/data/environment_library.py verify prepared.zip --manifest-sha256 <release-digest>
python scripts/data/environment_library.py rebuild rebuild.zip rebuilt-prepared.zip
```

The verifier checks member paths, sizes, hashes, coordinate controls, native grids, valid-data masks and clipped vectors. The rebuild checks canonical values and metadata against the retained manifest. Use the recorded toolchain digest to select the installation that made the release. Original payloads are stored once per digest inside the rebuild archive.

For large downloads, use a browser with direct file saving or fetch the package with a scoped credential. This PowerShell example takes the API origin, workspace, environment, token file and release ID from the installation record. Use `prepared` in place of `rebuild` for a prepared package.

```powershell
$headers = @{
    Authorization = 'Bearer ' + (Get-Content -LiteralPath $tokenFile -Raw).Trim()
    'X-EMAD-Workspace-ID' = $workspace
    'X-EMAD-Environment-ID' = $environment
    'X-EMAD-Partition' = 'PUBLIC_EVIDENCE'
}
Invoke-WebRequest -Uri "$apiOrigin/api/v1/environment-releases/$releaseId/rebuild" `
    -Headers $headers -MaximumRedirection 0 -OutFile 'rebuild.zip.partial' -ErrorAction Stop
Move-Item -LiteralPath 'rebuild.zip.partial' -Destination 'rebuild.zip'
```

Reference surface values describe the source product. The retained Copernicus product is a digital surface model. Physical accuracy remains unknown without independent control evidence. Vector tags carry source attributes and available dimensions. Empty vector layers require an evidence entry describing the completed bounded extraction. [Copernicus DEM product description](https://dataspace.copernicus.eu/explore-data/data-collections/copernicus-contributing-missions/collections-description/COP-DEM)

## Prepare and publish reference terrain

Use the exact release manifest SHA-256 and its prepared ZIP with the installed geographic Python dependencies. Write both outputs into private scratch storage.

```text
python scripts/data/prepare_display_terrain.py prepared.zip terrain.glb display-request.json --manifest-sha256 <release-digest>
```

The recipe verifies the package offline before opening the native surface and geoid. It keeps fully supported cells within the AOI, converts EGM2008 where applicable and uses the release's local frame. It reports the vertex precision and byte and triangle counts. The first recipe supports bounded north-up EPSG:4326 surface grids. Each AOI component needs its own surface and, for EGM2008, a matching geoid. Physical accuracy follows the parent release.

With a reviewer credential that has `environments.qualify` and the exact source scope, use the scoped headers from the download example. Reservation retries reuse the recorded ID if the intent is identical.

```powershell
$display = Invoke-RestMethod -Method Post -Uri "$apiOrigin/api/v1/environment-releases/$releaseId/displays" `
    -Headers $headers -ContentType 'application/json' -InFile 'display-request.json' -MaximumRedirection 0 -ErrorAction Stop
Invoke-RestMethod -Method Put -Uri "$apiOrigin/api/v1/environment-displays/$($display.id)/content" `
    -Headers $headers -ContentType 'model/gltf-binary' -InFile 'terrain.glb' -MaximumRedirection 0 -ErrorAction Stop
```

Open **3D terrain preview** under the release in the Environment Library. Readers with download access can open terrain for a released parent. Reviewers can also inspect staged and verified parents. Use HTTPS or a local loopback browser origin for SHA-256 verification and WebGL 2 for rendering. Migration 56 adds the display object to paired recovery inventory checks. [Display API contract](../../api/openapi/environment-display.yaml)

For durable terrain generation, run `python scripts/data/prepare_display_terrain.py --toolchain` inside the worker image and set the result as `C4ISR_ENVIRONMENT_DISPLAY_TOOLCHAIN` on the API and database-connected workers. Keep the geographic preparation digest in `C4ISR_ENVIRONMENT_TOOLCHAIN`. A worker with the display digest set verifies its installed terrain processor at startup.

With a reviewer credential, submit `POST /api/v1/environment-releases/{id}/display-builds` with the exact `manifestSha256`, recipe `terrain-glb-v1` and a `requestKey`. Read the returned `Location` or use `GET /api/v1/environment-builds/{id}` and its `/attempts` subroute. A successful build includes its `displayId`. Cancel with `POST /api/v1/environment-display-builds/{id}/cancel`. Transient failures and expired leases use the geographic queue's attempt budget. The direct worker generates from the retained prepared ZIP and verifies the exact terrain dependencies and GLB before committing publication. [Display API contract](../../api/openapi/environment-display.yaml)

## Qualification and recovery

Use paired backup and isolated restore for the database and private objects. The recovery inventory includes geographic object hashes, lengths, expiry, and tombstones. Reapply any source policy decisions that postdate the backup before enabling the restored worker or releases. Check restored packages through the ordinary scoped API and the offline reader.

The qualification runner `scripts/ops/qualify_environment_library.py` samples five simultaneous metadata readers against three exact release IDs and manifest digests over 24 wall-clock hours. Include the Riyadh reference release in the next delivery qualification and record the workload role of each other release. Supply `--allow-staged` to start while the human review is pending. A gap longer than twice the configured cadence plus ten seconds makes the run ineligible. The report distinguishes a completed prerelease observation from `SOAK_PASS`, which requires all three releases to be approved by the end of the window. Record worker restart, queue recovery, storage bounds, load, paired restore and independent human workflow evidence alongside the observation. [F09 acceptance criteria](../../plans/IMPLEMENTATION_PROGRESS.md#environment-foundation)

Run `python scripts/ops/qualify_environment_library.py status soak.json` to check the latest sample. It reports a stale running observation as `INTERRUPTED` and preserves the original evidence. Start a new run with a fresh output file after an interruption.

For migration 54, forward recovery is the qualified path. Pause new work, quiesce writers and preserve the paired database and object snapshot. Restore into isolated PostGIS and Garage instances, compare the inventory and the authorized package downloads, then resume the recovered queue. The preceding commit `aa298d3` could not be built from its clean tree because it omits the Go coverage service that its server imports. Keep migration 54 and its history in place when recovering this installation.

The three installed regression packages use Copernicus surface tiles, a retained NGA EGM2008 grid, retained Sentinel-2 bands, and OpenStreetMap vector inputs. Jeddah and Al Ain use the retained regional extract. Avignon uses a buffered OSM API extract after the regional download timed out. Its report carries the extraction limits. Prepare a new intake profile revision for the first Riyadh study area using its own coverage and source evidence. [Copernicus public tiles](https://registry.opendata.aws/copernicus-dem/) [OSM attribution and terms](https://www.openstreetmap.org/copyright) [OSM map API](https://wiki.openstreetmap.org/wiki/API_v0.6#Retrieving_map_data:_GET_/api/0.6/map)
## Geographic display and portable viewing

Prepare the display files from the verified ZIP and record the parent manifest SHA-256. The optical command also takes the retained Earth Search scene JSON. The recipe verifies its scene, acquisition and band identities against the admitted members. For roads, buildings and water, run the same command with the corresponding layer name and omit `--scene-metadata`.

```sh
python scripts/data/prepare_display_layers.py prepared.zip imagery imagery.json imagery-request.json \
  --manifest-sha256 "$manifest_digest" --scene-metadata retained-scene.json
python scripts/data/prepare_street_basemap.py prepared.zip street.pmtiles street-request.json \
  --manifest-sha256 "$manifest_digest"
```

Use the geographic preparation Python environment for display layers. The street recipe uses GDAL 3.11.4 with the PMTiles driver for the qualified Riyadh archive. Both run on local retained bytes. The street recipe expects one prepared OpenStreetMap roads, buildings and water GeoPackage each. Its first profile uses a bounded extent and zooms 12–16. Credit OpenStreetMap contributors and include the ODbL terms wherever the street archive is displayed or distributed. [OpenStreetMap attribution and license](https://www.openstreetmap.org/copyright)

Using the review identity, submit each `*-request.json` to `POST /api/v1/environment-releases/{id}/displays`. Send the exact generated bytes and `Content-Length` to `PUT /api/v1/environment-displays/{id}/content` using the returned display ID. Verify the manifest, response length and strong ETag through the normal scoped read route. Retries with the same reservation and bytes return the same display. Keep the receipt and original source metadata with the input evidence. Generated content becomes visible after validation and publication. The durable worker recipe is available for terrain.

Set `C4ISR_UI_LOCAL_BASEMAP_ONLY=true` for the credentialed disconnected Atlas deployment. Serve the app and API on the local network under their normal access grants. The profile starts with retained reference geography. In the Environment Library, open a release and use **Open retained street map** to fit the installed detail. Outside its extent and zoom range Atlas continues to use retained regional outlines. Both the 3D preview and PMTiles reads use the same scoped release and source checks. The MapLibre style uses local DOM labels and fonts and has no glyph or sprite URL.

Build the portable viewer and navigation dependencies on a connected preparation workstation or from its retained dependency cache. Natural Earth navigation comes from the three original ZIPs and their SHA-256 receipts. Store the viewer directory, navigation JSON and derivative hashes with the release evidence and paired backup inventory.

```sh
pnpm --dir apps/operator-ui build:geographic-viewer
python scripts/data/prepare_local_navigation.py retained-navigation-originals navigation.json
python scripts/ops/viewing_package.py export viewing.zip \
  --api "$api_origin" --workspace "$workspace_id" --environment "$environment_id" \
  --token-file "$operator_credential_file" --release "$release_id" --manifest-sha256 "$manifest_digest" \
  --viewer apps/operator-ui/dist-geographic-viewer --navigation navigation.json \
  --display "$terrain_display_id" --display "$street_display_id"
```

Repeat `--display` to select an imagery or vector layer from the same release. The caller needs `environments.download`, the release must be `RELEASED`, and each selected dependency needs `geographicViewerExport` in both the original and current admission. Approval of an updated admission alone cannot change the original rights of a retained release. Use the source review workflow for a new eligible release if the existing source terms grant no portable use. The issuance response binds exact display bytes, the viewer dependency manifest and navigation digest. It records applicable terms and expiry. The server audits the issuance ID and receipt hash.

Record the returned `issuanceSha256` with the issuance through the trusted handoff channel. The ZIP contains `launcher/viewing_package.py` and its digest. The recipient can verify the ZIP with a trusted copy of the launcher and open the verified package using the Python standard library. Open the loopback URL printed by `serve`.

```sh
python scripts/ops/viewing_package.py verify viewing.zip --issuance-sha256 "$trusted_issuance_digest"
python scripts/ops/viewing_package.py serve viewing.zip --issuance-sha256 "$trusted_issuance_digest"
```

The verifier checks every file against the issuance and viewer dependency list. It identifies missing dependencies and changed bytes and rejects expired packages. The launcher serves verified bytes over loopback and stops serving at expiry. Source withdrawal stops new online downloads and issuances under the affected rights. Track issued copies by issuance ID. On expiry or withdrawal under the recorded terms, ask the recipients to close the viewer, remove the archive and extracted copies, and confirm disposal in the issuance record. The launcher removes its temporary extraction on normal shutdown.

## Retained overlays and frozen geographic handoff

Open **Retained context and coverage** from the geographic preview to query the release's exact area in the selected Atlas interval. The overlay reports query status, admitted coverage, provider attempts, observation time and capped display counts. **Coverage and acquisition** uses the existing approved-profile scheduler and account limits. Displayed markers use the reference surface and report locations. The inspector preserves original observation times.

After independent release and source review, open **Frozen geographic handoff**. Choose the observation interval in Atlas. Enter an optional known-by timestamp with its time zone, read exact observation revisions, select the members and confirm the missing-data policy. The save operation binds the release digest, enabled display editions, release area revision, query fingerprint and selected evidence IDs. Current and original admissions must permit `geographicContextExport`. An existing release whose original admission lacks this permission requires a new eligible release. A retry with the same intent returns the same handoff after the read handle expires.

To inspect retained aircraft or vessel history, use a saved area and an elapsed Atlas interval of up to six hours. Copy the track ID from **Aircraft & vessels** into the geographic preview or the optional moving track field in **Frozen geographic handoff**. Quarantined reports stay visible in the evidence timeline. The 3D view displays the retained surface locations for the other reports. Select individual revisions when freezing a handoff. The service checks current and original export rights again when the handoff is reopened. [Moving history contract](../../api/openapi/moving-history.yaml)

The coastal worker uses a distinct toolchain pin and credential under the optional `environment-coastal` Compose profile. The Jubail recipe, sample limitations, and installed readiness procedures are documented together. [Coastal and readiness operations](COASTAL_MOVING_AND_READINESS.md)

Consumers can `GET /api/v1/context-packages/{id}/geographic` with normal credentials. Corrections leave pinned revisions unchanged while the original remains eligible. Withdrawal, expiry, changed rights or missing dependencies return an explicit error. Use the authorized download path to obtain the exact release manifest and display bytes. Name each display file with its ID and send the handoff SHA-256 through the trusted handoff channel.

```sh
python scripts/ops/verify_geographic_handoff.py handoff.json --sha256 "$trusted_handoff_sha256" \
  --release-manifest release.json --displays retained-display-files
```

The independent reader verifies the local manifest and display hashes, interval and cutoff predicates, source terms, expiry and deterministic membership order. Online reopening rechecks current rights. The offline reader operates on the supplied bytes and issuance information.

## Geographic viewer qualification

Set `EMAD_VIEWER_HARDWARE` to the actual host description and run `node apps/operator-ui/tests/qualify-geographic-viewer.mjs "$exact_preview_url" evidence.json` against private loopback staging. Use a URL that pins the release, manifest and display in `environment-3d`. The runner records Chromium and its active graphics device, local-only cold and warm startup, per-display response sizes, active-navigation frame samples, resource counts, keyboard return and 30 desktop round trips. The compact viewport runs share the host hardware. Record physical mobile and independent participant results when those baselines are available.

To suspend the geographic preview independently, set `C4ISR_UI_GEOGRAPHIC_PREVIEW_ENABLED=false` and recreate the UI service. The release list and scoped prepared data downloads remain available.
