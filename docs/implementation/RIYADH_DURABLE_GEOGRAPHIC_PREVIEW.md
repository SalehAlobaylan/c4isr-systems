# Riyadh durable geographic preview

14 November 2026. The V01–V03 candidate was exercised in a disposable copy of the migration 58 staging installation, with migration 59 applied only in the copy. The exact bounded Riyadh release is `georelease_01m786xtwrr6d0sm1by5n5r4dc`, manifest `f338cbd833efa47fbf79c58702883e2e52fcd977987e6ede9e10f113f73fbeb6`. The original release, prepared archive and previously installed display identities were checked before and after upgrading the isolated copy. Al Ain and Avignon were compatibility checks for existing archives and displays.

## V01 Durable publication

An authorized reviewer can queue native terrain, Sentinel-2 optical imagery, roads, buildings or water from the Environment Library or API. The build binds the exact parent manifest and configured processor digest. The optical request also binds the retained scene bytes. The worker verifies the output and exact member bindings before publishing under a live lease. The display manifest and content route remain independently available through scoped metadata, full reads and single byte ranges. The scope, source, and parent release checks also apply to subsequent downloads and STAC discovery.

All five Riyadh jobs succeeded on their first attempts. Duplicate submissions returned the same job, a queued cancellation reached `CANCELED`, a mismatched Sentinel scene was rejected, and full and range reads returned the expected bytes. Anonymous reads were denied. In the assembled PostGIS test, the worker lease was expired after writing verified bytes to the store, before the publication transaction. The object stayed private and the next attempt published one display. [Display API contract](../../api/openapi/environment-display.yaml)

The API and worker image was `sha256:d3206df14352e0e713c4d1b11ba57408898ccf8ccd5b640570e0ead0e4939348`. The layer worker digest was `0061405bef4189707cfe1cda5d300e410464d9c5ef1efca77fe2a37240f3dace`. The UI image was `sha256:a8057d40a283db91a40a3c321b519b0c858e66b1dbbc686d3a3833fab1b384f1`. The Go suite, assembled geographic integration, Python geographic and operations suites, generated OpenAPI comparison and four Chromium workflow tests passed.

## V02 Native terrain and coordinates

Terrain v2 builds on the v1 native-grid recipe. It can join aligned tiles from the same component at their original sample spacing. The mosaic checks CRS, datum, grid alignment, registration and overlap values. It preserves invalid cells and rejects unresolved conflicts. The worker preserves the local east, up and south frame and applies the retained correction grid once where applicable. The verified Riyadh terrain remained byte-identical to the installed 2,236,212-byte GLB with 92,450 triangles.

A controlled terrain fixture around the Riyadh reference verified that triangles join aligned source tiles, leave raster and polygon holes open, and preserve two separate area components. Offset registration, disagreeing overlap and a sample-budget violation were rejected. The viewer can inspect coordinates and WGS84 ellipsoidal height directly on terrain, including with Enter on the canvas, and locate the point in Atlas under the same retained display and release. The launcher and verifier accept both reference terrain recipes. Local physical accuracy remains `UNKNOWN`. [Terrain fixture](../../scripts/data/test_environment_geographic_display.py)

## V03 Dated imagery and vectors

The durable worker reproduced all four installed Riyadh overlay hashes. The 607 × 668 Sentinel-2 texture retained 404,811 valid pixels and the same exact acquisition and scene metadata hash. The display used the same nearest-sample reprojection and original RGB and SCL inputs. Roads drew 3,660 of 3,703 source features, buildings drew 1,934 of 1,978, and water drew all four. Features with incomplete drapes remain inspectable by source ID. The existing alignment check independently matched 25 imagery controls and all 5,685 retained vectors. Street PMTiles remains a separate reviewed publication at zooms 12–16, and the new durable optical and vector recipes retain the bounded single-scene and single-surface limits.

The Riyadh artifact sizes were 2,236,212 bytes for terrain, 1,429,664 for optical, 4,253,893 for roads, 2,077,887 for buildings and 6,825 for water. Terrain loads independently. The viewer fetches each selected overlay separately and verifies its hash before opening it. A portable `terrain-glb-v2` fixture with retained regional navigation verified and rendered in compact Chromium with external requests blocked. Previous Riyadh measurements and the Saudi and Middle East navigation details are recorded in the implementation history. [Riyadh display and navigation evidence](../../plans/IMPLEMENTATION_PROGRESS.md#riyadh-geographic-display-and-offline-evidence)

## Browser qualification

Chromium 153 on the Ryzen 9 7900X workstation passed desktop, compact portrait and compact landscape with external requests blocked. Cold preview startup took 1,411, 939 and 900 ms respectively. Active navigation p95 was 16.8 ms in each profile. The point picked at 46.762021° E, 24.714124° N appeared at the same coordinates in Atlas. A road retained its exact source edition and ID through navigation, Atlas inspection and reopening the 3D preview. The desktop passed 30 repeated round trips with a peak of eight geometries, eight draw calls and one texture including the selected feature highlight.

The desktop run transferred 239,955 bytes of geographic metadata, 152,864 font bytes and 373,830,342 display bytes across cold, warm and 30 repeated views. The ledger counts repeated protected reads. Failed transfer bytes and display retries were zero. The unique Riyadh terrain and overlay payload totals 10,004,481 bytes. The browser fetched only terrain before enabling layers.

## Rollout

Shared staging runs migration 58 during the restarted F09 observation. Complete the observation before scheduling the candidate cutover. Durable generation is qualified against the bounded Riyadh reference. Independent release review, physical mobile measurement, and new detailed areas within Saudi Arabia and the Middle East require their own acceptance. [Operations](ENVIRONMENT_LIBRARY_OPERATIONS.md) · [Foundation observation](COASTAL_MOVING_AND_READINESS.md#f09--installed-readiness)

The private qualification directory is `private/environment-foundation/implementation/coastal-moving-qualification`.

- `geographic-preview-final.json` SHA-256 `f1c0ea8c82d1ec715ce4329dc449e9e307b22fce46f2fbe946f8aaf8b1ab8324`.
- `geographic-preview-final-copy.json` SHA-256 `54ebd8210eee84fe203375b5ff20fc50283a1b86817d71a3058eaa4b72431d2b`.
- `preview-next-browser.json` SHA-256 `0d136c6b248a2bb38caf70dddfda6fa6d972e7f1ce088c515bfb6388e9c485f6`.
- `portable-terrain-v2-browser.json` SHA-256 `bb3d7df0c4eea9752b16217ef9a8a61fecf86c9c22ffad60a18da83ab6e6e409`.
