# Riyadh observation pilot: radar, flood reports and pressure fields

The [versioned pilot](../../scripts/data/observation_pilot.json) names the intended inspection, geographic bounds, time, edition, units and quality criterion. These layers are for preparing and inspecting geographic context. They do not activate a simulation or define its physical inputs.

## D03: Sentinel-1 radar

[Microsoft Planetary Computer](https://planetarycomputer.microsoft.com/api/stac/v1/collections/sentinel-1-rtc) returned two candidates for October 2025 over the Riyadh reference area. Earth Search returned an empty page for this query. NASA CMR returned four OPERA RTC candidates, but the tested download requires Earthdata authorization. The Microsoft catalogue supplied a usable CC BY 4.0 radar product and supports bounded HTTP range reads.

The selected Sentinel-1A acquisition started at 14:57:59.097644 UTC on 24 October 2025. It is ascending, relative orbit 72, IW mode and VV polarization. Catalyst produced the terrain-corrected gamma-zero linear-power layer from Level-1 GRD. The native 10 m EPSG:32638 window contains 414,568 samples. It is retained with its STAC metadata, byte-range window, ETag and SHA-256. The scene does not publish a separate layover or shadow raster. Nodata and nonpositive samples are transparent in display.

The radar capture archive SHA-256 is `25cbdd44549324edf6f0a3929e3a8a0bf1881195e3ce8a95cc3e25c73a04bf18`. The preparation recipe preserved 403,468 positive in-footprint native power samples. The grayscale display uses nearest sampling and clips `10 log10(power)` between −30 and +5 dB. Optical imagery stays separately selectable, with its own acquisition date. This is backscatter for visual inspection, without a flood or change classifier.

## D04: Historical provider reports

The [NASA EONET v3 flood catalogue](https://eonet.gsfc.nasa.gov/docs/v3) returned eight records for the 2025 pilot Middle East box (34–56° E, 16–33° N). There were no returned reports in the exact Riyadh study area. The broader Saudi screening rectangle returned five reports whose cited locations were outside Saudi Arabia. It is a rectangle query, not an authoritative national flood inventory.

All eight regional records cite exact GDACS flood IDs. These reports fill a category gap in the current earthquake-only GDACS connector. They provide shared upstream lineage rather than independent corroboration. USGS earthquakes and FIRMS thermal detections do not report flood occurrence. The exact EONET regional response SHA-256 is `56ccb4b22e135fd915665d1ddcd90c067e64452cef7f28393bf35fc32e2912fd`.

The [bounded adapter](../../internal/publicingest/eonet.go) preserves provider identity, original revision, reported geometry history, cited source links, lifecycle closure and unknown positional accuracy. A capped page or a polygon that cannot be anchored is partial. A provider failure does not become a successful empty result. An empty result or provider closure does not erase retained history. No withdrawal is inferred when a provider does not publish one. Atlas can inspect historical flood context, and the handoff can pin exact report revisions after both original and current rights allow it. As-of selection uses platform publication time. A test-owned fixture verifies reopening the earlier revision after correction without promoting it into the event catalogue. The real regional report set was replayed through a disposable scoped acquisition test. No live provider schedule or release approval was activated.

## D05: Historical pressure fields

The [ERA5 pressure-level product](https://cds.climate.copernicus.eu/stac-browser/collections/reanalysis-era5-pressure-levels), distributed here via [WeatherBench 2](https://weatherbench2.readthedocs.io/en/latest/data-guide.html), supplies a 7 × 7 central Saudi grid for 23 October 2022 at 12:00 UTC. Temperature (K), eastward wind (m/s) and northward wind (m/s) each have 98 values over 850 and 700 hPa. Source surface pressure masks below-ground samples. All 98 selected cells were above ground in this example. The retained hourly grids are sampled without interpolation or geographic height conversion. The 850 hPa temperature range was 295.42–296.71 K. This is historical reanalysis, with no claim of site-level accuracy. Station METAR remains an independent context layer and no pressure-level-to-station equivalence is asserted.

The nine exact source Zarr objects total 327,045,137 bytes under the 350 MB pilot budget. Re-decoding with the pinned object hashes reproduced the 7,697-byte subset SHA-256 `044d375d8552a06f1908e4a7bc2b337c49fb3930f04ba5b4d883f4373e462862`. The slice inspector displays one native pressure grid and its exact selected values. A pinned release manifest or prepared ZIP keeps the values stable through reopening.

## Reproduction and rollout

1. Use [bounded acquisition](../../scripts/data/acquire_observation_pilot.py) with the [pinned acquisition dependencies](../../scripts/data/observation-acquisition-requirements.txt) and retain the receipts before admission. Radar uses a fixed provider and acquisition pin with a short-lived anonymous asset token that is excluded from artifacts. `--retained-weather` selects already retained Zarr objects for an offline rebuild. A new period or variable set requires its own exact source-object list and quality review.
2. Use `reference-observation-v1` with the [versioned preparation worker](../../scripts/data/environment_observation.py). Pin its digest in `C4ISR_ENVIRONMENT_OBSERVATION_TOOLCHAIN` and use the opt-in `environment-observation` Compose profile with a source-scoped credential. Admit radar and weather independently. Use the [scoped publisher](../../scripts/ops/publish_enrichment_displays.py) to publish the exact surface, radar and optionally optical display dependencies.
3. Configure the EONET source with adapter `nasa-eonet-floods-v1`, dataset `eonet-v3-floods`, product version `v3`, content-hash revisions, one page, up to 100 records, at least 900 seconds between requests and at most 24 requests per day. Historical runs use explicit windows no longer than 31 days. A whole-year source comparison is discovery evidence, not one admitted connector run. Record original and current rights independently before allowing export.
4. Verify the exact package and display hashes, source denials and offline rebuild. Keep the staged candidate separate from the installed baseline until independent review and installation gates are satisfied.

## Disposable staging result

Build `geobuild_01m7ahfsvv131by5qxjtgs1wjs` produced staged release `georelease_01m7ahvq3f5adbkjsxe92npx8d` in a separate PostGIS and object-store copy. Its manifest SHA-256 is `175515f785c2ae6cdbbec0303140f6d425ff8e773dc24375a64d02d87ac63ab4`. The prepared package is 7,576,159 bytes with SHA-256 `0e4d4f9caa873f45def679db09d9a4f16c560bb1adb8054e9f53f731bd57c7d0`. The scoped API accepted the reference mesh, radar display and optical display and denied unauthenticated content reads. Chromium inspected both imagery types independently and selected exact weather samples at 1440 × 900 and 390 × 844 with external requests blocked. [API qualification](../../private/environment-foundation/implementation/riyadh-observation-pilot/api-verification.json) · [Browser qualification](../../private/environment-foundation/implementation/riyadh-observation-pilot/browser-verification.json)

The network-disabled rebuild reproduced the manifest and prepared ZIP byte for byte. Regenerating the radar display against the rebuilt package also reproduced its exact SHA-256, `8fbdd4eb21339a316c68d06dbe479ab864dce3c0a9a47cc04e799c41f54859a1`. [Rebuild result](../../private/environment-foundation/implementation/riyadh-observation-pilot/rebuild-verification.json)

The installed baseline stayed pinned. Independent review, commissioning to the installed environment, and export authorization remain open.

The [private evidence directory](../../private/environment-foundation/implementation/riyadh-observation-pilot/) contains the original provider objects and evidence receipts. Source bytes and credentials are outside git.
