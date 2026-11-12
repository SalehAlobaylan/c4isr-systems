# Geographic Environment Library

The library prepares reusable geographic reference packages from saved Public Evidence areas. It preserves source resolution, dates, masks, rights, and the original input identities in each manifest. Packages carry WGS84 geometry and a declared local east, north, up frame. The included EGM2008 grid and checked conversion method let consumers convert source orthometric heights when needed.

## Install and provision

1. Apply migration 54 and install the environment worker and Python requirements. The ordinary local setup builds the worker. Staging uses the `environment-library` profile in `deployments/staging/delivery.yml`.
2. Run `python scripts/data/environment_library.py toolchain` using the installed Python interpreter. Configure the resulting digest as `C4ISR_ENVIRONMENT_TOOLCHAIN` on the API and worker. The Windows supervisor sets it when the feature is enabled. Use the staging image's digest for staging.
3. Provision an explicit Public Evidence service identity with `operational.read`, `environments.execute`, and grants for the exact source set. Put its credential in `private/local/credentials/worker-environment.token` locally or the corresponding staging secret. Use `emad-admin` for provisioning. Grant execution on the exact retained profile revision through `POST /sources/{id}/profiles/geographic-retained/execution`.
4. Provision a data steward with `sources.read`, `sources.manage`, `environments.read`, and `environments.manage`. Give authorized users `environments.build`, `environments.qualify`, `environments.release`, and `environments.download` for their work. Every contributing source grant applies to packages and their metadata.
5. Enable `C4ISR_ENVIRONMENT_LIBRARY_ENABLED=true`. For staging, install the API before starting the Compose environment worker profile. To suspend new work, stop the workers and set the feature flag to false while retaining the toolchain configuration and storage. This leaves read access to eligible history.

The API contract includes the scoped discovery, definition revision, preflight, preparation, review, and download operations. [OpenAPI contract](../../api/openapi/environment-library.yaml)

## Admit and prepare data

Register external sources for retained products. Record reviewed terms, attribution, retention, and explicit geographic output permissions using `POST /environment-sources/{id}/admissions`. This creates a dedicated retained profile revision. Raw redistribution needs its own permission because rebuild bundles contain the original bytes.

For each reviewed candidate, create a private bindings file with a display `name` and an `assets` map. Each key is the candidate input ID in `recipe.json`. Each value names the approved `sourceId`, `admissionId` and `retainUntil`. Include `referenceId` or `sceneId` when the exact retained parent matches the original record in that scope.

Use `scripts/data/intake_environment.py` with the candidate directory, bindings file, API origin, credential path and exact workspace and environment. It verifies the existing prepared package and the original hashes, stages the sources, saves the area and returns preflight diagnostics. Existing matching source reservations can be reused.

In Map → Workbenches → Environment Library, select a saved definition or save a bounded area and choose its retained layers. Check the preparation diagnostics before starting a build. To review a staged release, open its pinned link and check the release status, manifest digest, source dates, resolution, attribution and limitations. Download the prepared package and run the verifier below against the displayed manifest SHA-256. Record verification with a note about what you checked, then record release with a separate note. Both actions bind to the displayed digest. Withdrawal ends download availability while preserving the release history.

To reproduce the three-area installation, use `scripts/ops/commission_environment_library.py register` with the scoped steward token, `scripts/data/region_profiles/environment-intake-v1.json` and the private candidate root. Provision the service identity with the exact source IDs returned in the commissioning record. Then run the same command with `stage` and `--worker-principal`. It installs the retained inputs and regional navigation, runs preflight and submits idempotent build requests. Run `scripts/ops/check_installed_environment_library.py` with the completed commissioning record, reviewer credential and API origin to verify the installed manifests and byte ranges and download prepared packages to a private folder.

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

## Qualification and recovery

Use paired backup and isolated restore for the database and private objects. The recovery inventory includes geographic object hashes, lengths, expiry, and tombstones. Reapply any source policy decisions that postdate the backup before enabling the restored worker or releases. Check restored packages through the ordinary scoped API and the offline reader.

The qualification runner `scripts/ops/qualify_environment_library.py` samples five simultaneous metadata readers against three exact release IDs and manifest digests over 24 wall-clock hours. Supply `--allow-staged` to start while the human review is pending. A gap longer than twice the configured cadence plus ten seconds makes the run ineligible. The report distinguishes a completed prerelease observation from `SOAK_PASS`, which requires all three releases to be approved by the end of the window. Record worker restart, queue recovery, storage bounds, load, paired restore and independent human workflow evidence alongside the observation.

For migration 54, forward recovery is the qualified path. Pause new work, quiesce writers and preserve the paired database and object snapshot. Restore into isolated PostGIS and Garage instances, compare the inventory and the authorized package downloads, then resume the recovered queue. The preceding commit `aa298d3` could not be built from its clean tree because it omits the Go coverage service that its server imports. Keep migration 54 and its history in place when recovering this installation.

The three private real-data candidates use Copernicus surface tiles, a retained NGA EGM2008 grid, retained Sentinel-2 bands, and OpenStreetMap vector inputs. Jeddah and Al Ain use the retained regional extract. Avignon uses a buffered OSM API extract after the regional download timed out. Its report carries the extraction limits. [Copernicus public tiles](https://registry.opendata.aws/copernicus-dem/) [OSM attribution and terms](https://www.openstreetmap.org/copyright) [OSM map API](https://wiki.openstreetmap.org/wiki/API_v0.6#Retrieving_map_data:_GET_/api/0.6/map)
