# Simulation input readers and verified examples

## Review a source in the Atlas Environment Library

Open a visible Environment Library release. Each tool processes the reviewed file for the request and returns the release, file and source admission identities. A selected release identifies the working environment. Source evidence keeps its published geographic scope.

The **Riyadh reports and road access replay** uses a package that joins those inputs with reviewed roads, inspections, planned-work revisions and population estimates. The [Riyadh delivery record](riyadh-access-delivery.md) includes the authored acceptance case and technical source-backed controls.

The **Civilian aerial inspection replay** pairs verified imagery and local masks, shows dated mapping assessments and civil aviation context, and exports staged handoff responses for scoring. [Aerial delivery and provider evidence](aerial-review-delivery.md)

**Regional response and coastal evidence** presents SRCA and MOH counts at their reported scopes, runs authored workload and road access comparisons, and shows coastal evidence at a cutoff. **Historical port activity** now reports its sustained return criterion and held-out baseline comparison. [Regional delivery and provider evidence](regional-response-delivery.md)

**Supporting evidence replay** shows source revisions, publication and delivery timing, measurement coverage, and matched humanitarian comparisons. It includes authored report queues and an export for scoring navigation uncertainty controls. [Supporting delivery and provider evidence](supporting-sources-delivery.md)

| Atlas tool | Admission dataset and edition | File and result |
| --- | --- | --- |
| Riyadh 940 workload | `riyadh-940`, `2026-q1` | The pinned municipal JSON. Shows Arabic category counts, daily UTC receipt counts and rows with no usable place label. |
| Historical port activity | `imf-portwatch`, `daily-ports-v1` | PortWatch daily JSON for Jubail or Jazan with its reviewed SHA-256. Shows daily records against matching weekdays from an earlier baseline. |
| Copernicus flood mapping | `copernicus-ems-rapid-mapping`, `emergency-response-v1` | A reviewed, complete response for one public flood activation. Shows available delineation and grading metadata from the supplied versions, along with linked imagery acquisition dates. The cutoff uses product delivery time. [CEMS provider contract](https://mapping.emergency.copernicus.eu/about/how-to-harvest-cems-mapping-data/emergency-response-data/) |
| Historical Internet outage annotations | `cloudflare-radar-annotations`, `v4` | A reviewed Radar annotations response and a past country interval. Shows matching retrospective outage annotations with unknown end times where applicable. The response provides no established publication time for participant-level replay. [Radar provider contract](https://developers.cloudflare.com/api/resources/radar/subresources/annotations/methods/list/) |

Use the existing retained source admission workflow. The policy must allow analysis and explicit internal analytical output. The API rechecks access to the release and the source policy before returning the result. The browser uses the existing authenticated workspace session. [Retained source intake](../../../internal/geoenv/admission.go) · [Environment API](../../../api/openapi/environment-library.yaml)

## Local readers

The local [command-line reader](../../../scripts/data/read_simulation_inputs.py) accepts reviewed files and emits `emad.simulation-input-analysis.v1` JSON with the operation, SHA-256 of the input bytes and the result. Pass `--sha256` with an independently obtained digest to pin the input. The caller is responsible for selecting files permitted for the intended use. Use the existing scoped release and offline kit operations for geographic data. Store provider files, private labels and derived outputs under their respective access terms. [Offline kit guide](../../../docs/implementation/OFFLINE_ENVIRONMENT_KIT_GUIDE.md)

Run `python scripts/data/read_simulation_inputs.py --help` to list operations or `task test:simulation-source-plans` for the repeatable checks. Commands take a local file and work without contacting the provider. Use the existing reader environment for `population-raster`, which requires Rasterio and NumPy.

| Plan | Delivered reader operations | Input and result |
| --- | --- | --- |
| Riyadh reports and access | `municipal`, `road-access`, `population`, `population-raster` | Supply the original Q1 JSON for retrospective counts or request its normalized rows with `--rows`. Access takes a reviewed directed graph with edge distances, evidence IDs and dated inspections. Population accepts reviewed cell counts or reads them from a single-band GeoTIFF with reviewed zone allocations. The result distinguishes known zero, masked cells and known unallocated people. |
| Civilian aerial inspection | `image-pair`, `flight-energy`, `flood-review`, `cems-products` | Supply study-area revision, required asset names and reviewed scenes carrying availability and local usable coverage. Energy reads a completed AirLab CSV with `flight`, `time`, `battery_voltage` and `battery_current`. Flood scoring consumes held-out groups, reviewer decisions and pixel-pair counts. Copernicus EMS reads a complete single-activation flood response and includes only the delivered product versions in that response that meet the cutoff. [AirLab fields](https://www.nature.com/articles/s41597-021-00930-x.pdf) · [CEMS fields](https://mapping.emergency.copernicus.eu/about/how-to-harvest-cems-mapping-data/emergency-response-data/) |
| Regional response and coast | `regional-indicators`, `regional-queue`, `port-activity` | Supply reviewed indicator rows at a common reporting grain to reconcile totals. Supply explicit arrivals, durations and staffed slots to compare authored queue profiles. For daily activity, supply an ArcGIS response scoped to one port and a declared pre-period plus study period. The weekday baseline is fixed on earlier days in the supplied file. Recovery requires consecutive days within the observed range after a below-range day. Missing days interrupt recovery. |
| Supporting sources | `notices`, `delivery-replay`, `radar-outages` and existing native field and public-ingest reads | Use reviewed issuer-keyed civil notices with publication, availability and effective times, or delivery and acknowledgment records to compare knowledge at two exercise desks. The Radar reader filters a retained annotations response by country and historical interval and preserves unknown end times. Use the installed source-specific adapters for their admitted products. [Radar annotation contract](https://developers.cloudflare.com/api/resources/radar/subresources/annotations/methods/list/) · [Adapter inventory](../../../internal/publicingest/capabilities.go) |

## Replay the owned road case

```powershell
python scripts/data/read_simulation_inputs.py road-access testdata/simulation-inputs/civilian-road.json --origin junction-a --known-at 2026-11-21T06:45:00Z
python scripts/data/read_simulation_inputs.py road-access testdata/simulation-inputs/civilian-road.json --origin junction-a --known-at 2026-11-21T07:00:00Z
```

The first result is a verified 230 m detour. At 07:00 UTC the eastbound reopening becomes visible and the verified route falls to 120 m. The westbound edge stays closed until its own inspection supports another change. The case uses authored geometry and observations.

For the GeoTIFF reader, pass `--allocations` with a JSON file containing `gridEdition`, `unit: "people-per-cell"`, `grid` with CRS, affine transform, width and height, and `cells` with native row, column and `zoneFractions`. The result pins both the raster and allocation files and rejects allocation metadata for a different grid.

## Reviewed input formats

| Operation | Fields in a prepared input |
| --- | --- |
| `population` | `gridEdition` and `cells`. Each cell has `id`, nullable `people` and `zoneFractions` keyed by service zone. |
| `image-pair` | `studyAreaRevision`, `requiredAssets` and `scenes`. Each scene has `id`, `sensor`, `acquiredAt`, `availableAt`, `reviewedAreaRevision`, `usableFraction` and `verifiedAssets`. |
| `flood-review` | `developmentGroups` and evaluation `cases` with `id`, `collectionGroup`, `reviewVerdict`, independently judged `floodedRoad`, and `intersection`, `union` and `scoredPixels` for flooded-road masks. Keep inputs and receipts with the evaluation team. |
| `regional-indicators` | `indicators` with `id`, `edition`, `region`, `measure`, `unit`, `populationScope`, `periodFrom`, `periodToExclusive` and nullable `value`. Optional comparisons can be expressed in the `reconciliations` list with `totalId` and `componentIds`. |
| `regional-queue` | `from`, `arrivals` with `id`, `arrivedAt` and `serviceSeconds`, and `profiles` with `id` and `staffedSlots`. Wait quantiles apply to arrivals who started service by the cutoff. The result gives the counts still waiting and in service. |
| `notices` | `notices` with issuer-stable `noticeId`, integer `revision`, `evidenceId`, `issuedAt`, `availableAt`, `effectiveAt`, optional `expiresAt`, `status` and `summary`. |
| `delivery-replay` | `messages` with `id`, `recipient`, `sentAt`, and optional `receivedAt` and `acknowledgedAt`. Use an evidence cutoff to compare when each recipient received a correction. |

Timestamps carry a UTC offset. Date-only provider days and reporting periods retain their original calendar dates. The executable known-answer cases include input examples for each operation. [Reader tests](../../../scripts/data/test_simulation_inputs.py)

## Provider checks on 21 November 2026

- Riyadh Q1 reproduced 6,820 rows at the pinned SHA-256 `b3619c468a0e91c06d718ed7421b7f18deb5c7d78535f54046f7f807b8b60a21`. The summary reader produced its retrospective counts directly from the publisher's attachment. [Riyadh municipal release](https://alriyadh.gov.sa/ar/data-sets/940-service-requests/940-requests-group-q1-2026)
- A bounded PortWatch request for each port and calendar years 2023 and 2024 returned 731 rows per port. The reader produced all 366 dates of 2024 for both Jubail and Jazan, with four Jubail calls and zero Jazan calls on 15 January. This is a retrospective coverage check of those two series. [PortWatch layer](https://services9.arcgis.com/weJ1QsnbMYJlCHdG/arcgis/rest/services/Daily_Ports_Data/FeatureServer/0)
- The CEMS reader returned two visible products for Dresden flood activation EMSR761 at 10:00 UTC on 21 September 2024 and three by 24 September. The current response has version 2 for the first product. Earlier revisions require separately retained files. [Activation response](https://rapidmapping.emergency.copernicus.eu/backend/dashboard-api/public-activations/?code=EMSR761) · [CEMS flood bulletin](https://mapping.emergency.copernicus.eu/news/information-bulletin-173-the-copernicus-emergency-management-service-monitors-floods-in-central-and-eastern-europe/)

## Remaining platform and evidence work

Durable exercise input issuance, provider-specific acquisition and end-to-end exercises remain open. Local Riyadh evaluation needs independently timed inspections and reviewed incident associations. A reviewed OSM or Overture graph, WorldPop cells and service zones, and verified imagery assets must be prepared upstream. Copernicus versioned geospatial product intake still needs work. The Radar endpoint is covered by a provider-shaped fixture and needs an admitted historical capture for real coverage qualification. Historical civil aviation, source-specific notices and optional fields each need a permitted sample. Actual staffing, restoration, aircraft-specific evaluation and local labels require independently supplied records. [Normalization dictionary](normalization.md) · [Evidence gaps](README.md#data-still-needed-to-qualify-the-exercises)
