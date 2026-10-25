# EMAD map exploration: implementation and acceptance

The latest [adaptive dock review](EMAD_ADAPTIVE_DOCK_REVIEW.md) implements a 44px toolbar, shallow previews and automatic side placement for detailed desktop work. See its [captured application views](map-dock/index.html) for the current presentation and verification status.

The subsequent [map cleanup review](EMAD_MAP_CLEANUP_REVIEW.md) consolidates summaries, controls, selection and evidence into the existing dock. See its [current screenshots](map-cleanup/index.html). Its presentation supersedes the earlier floating peek and separate evidence drawer described in this record.

24 October 2026. Implemented, installed and exercised against the persistent Windows installation. This records the accepted implementation of [map exploration and situational awareness](EMAD_MAP_EXPLORATION_PLAN.md), extending the earlier workspace redesign. The earlier layout-study HTML is historical.

## Open the delivered application

With the supported supervisor running, open [the live map](http://127.0.0.1:5173/map). The current installation has migration **44**, the navigation editions and retained source archives installed. No additional migration is required on this installation. Other installations use `task local:setup`; it applies forward migrations and reconciles the geographic installation without resetting existing data.

The [screenshot gallery](map-implementation/index.html) shows desktop, phone, imagery, review and Studio workflows from the persistent installation.

1. Search **Saudi Arabia**, or choose Country boundaries in Layers and click its geometry. The breadcrumb and counts use that retained polygon.
2. Search **Makkah**, then choose **Jeddah coastal study area** from Tools → Admitted subdivisions. Place points locate a place; they do not fabricate a city boundary.
3. Keep several evidence layers enabled. Preview category only chooses which records the dock browses. Tap a symbol or footprint for its source, date, reported conditions and Inspect action. Locate deliberately moves the camera.
4. Open **Systems** from the map. Its actual six-domain grid, Sources & access and Processing sections share the same dock as the detailed tools. Collapse, side placement, expansion, Maximize and Focus map remain available.
5. In Layers → Retained imagery on map, select March 1 and March 31, 2025. Compare matching retained windows with opacity and a before/after divider. SCL quality overlays leave RGB visible; amber identifies excluded pixels. Acquisition dates stay on the map after closing the controls.
6. Use Imagery & analysis, Review desk or Context packages inside the dock. The shared business forms retain exact jobs, assessment revisions and human-review rules. A dirty draft stays mounted and pinned when another tool or geographic area is selected. Browser Back and Forward restore typed selections, including cleared filters.
7. Use Tools for bounded polygons, keyboard coordinates, PostGIS geodesic distance/area and nearby context buffers. Save a view to retain its area, camera, dates and layers. Shared links additionally carry authorized selected-record/tool references. Safe layout preferences persist per account/resource; private drafts stay in session memory.
8. Open Simulation & replay for an explicit authorized transition into Studio, exercise operations, recordings or evaluation. In Studio, the real asset library opens beside the 3D scene and preserves unfrozen scene edits. Public navigation does not load hidden actors or truth.

## Installed geography and scope

Three immutable navigation editions retain **196 features**: 177 generalized country features, 13 Saudi first-order regions, five populated-place points and one bounded Jeddah study area. English/Arabic names and source-backed aliases support search and parent navigation. Natural Earth acquisition URLs, original ZIP bytes, hashes, rights and preparation lineage are retained. These generalized source editions are reference snapshots, not certification of current official jurisdictions.

The Jeddah rectangle is explicitly an analyst/prepared-environment study area. Nearby context uses a geodesic buffer, with no implied maritime jurisdiction. Global navigation admission has its own bounded contract; it does not expand feed subscriptions, service grants or the 400-square-degree evidence-query budget.

The [post-restart installation snapshot](map-implementation/installation-acceptance.json) records the actual catalogue and exact Saudi polygon query. At capture it returned five retained earthquake reports, 26 station-weather records, two distinct acquired imagery windows and one derived finding. Thermal detections, official bulletins and moving-contact queries returned zero. These are dated retained-query results; empty queries do not establish the absence of real-world activity. The wider configured workspace has additional evidence outside the Saudi land polygon.

The installation retains the existing 24 reference versions and detailed real Jeddah environment. Logical map themes group explicit importer dataset keys across storage chunks. Bounded, authenticated PostGIS vector tiles draw authorized geometry independently of the capped detail inspector.

## Map and workbench behavior

- A versioned geographic scope, visible layers and preview category are independent. Pan/zoom keeps the investigation pinned; Search this view explicitly applies a bounded viewport polygon. Parent breadcrumbs, Fit area, Previous view and saved views have different effects.
- Weather, earthquakes, thermal detections, official bulletins, aircraft, vessels, capture footprints, source query areas and derived findings have typed representations. Overlapping objects offer an object picker. Unknown locations and uncertainty remain explicit; models and processing jobs receive no invented map positions.
- The server counts the complete authorized result before display limits. Large results become bounded family-specific aggregates; their centres are display summaries. The integration check verifies 2,130 reports and 2,130 weather records without losing counts. MapLibre resolves smaller point collections with separate family clusters.
- Current source health is separate from the WebSocket connection and historical evidence. Known-at/provider-effective semantics remain intact for events. Unsupported historical feeds are labeled unavailable for that query rather than painted under a historical timestamp. Pause display freezes presentation, leaving ingestion and virtual clocks independent.
- Newly retained/revised records can be inspected without recentering the map. The banner distinguishes arrival from observation time. Refresh display reads retained records; provider retrieval remains a separate worker responsibility.
- One large workbench and one small object peek occupy the workspace. The dock allocates layout space, supports bottom/side placement and has collapsed, compact, expanded and full states. Hidden visited workbenches retain drafts. Phone and short-landscape layouts keep explicit map/restore controls.
- Systems, imagery, review, context export, weather and external tracks reuse their standalone business components through a typed host. No iframe, copied form or duplicate authorization model is introduced. Full Systems and existing standalone URLs remain valid. Area-supported queries are labeled separately from workspace-wide catalogue/review/processing figures.
- Imagery uses the retained RGB and quality PNG bytes at their captured geographic extent. Comparison only pairs matching windows; each side is clipped independently, so partially transparent acquisitions are not blended into one another. SCL exclusions remain classification limitations. Findings link to the retained comparison and review workflow; algorithmic geometry never becomes authoritative merely by being drawn.
- Studio library access is deliberately opened in an authorized input resource. Actual validated model bytes render the thumbnails; dimensions, rights, templates, import validation and environment definitions remain accessible. Operations/replay/evaluation retain their proper resources and clocks.

## Contracts and compatibility

Migration `00044_geographic_navigation.sql` adds immutable, scoped navigation editions and geometry. Migrations 1–43 and existing immutable evidence were preserved.

The modular Go API adds:

- `POST /api/v1/geography/editions` and `GET /api/v1/geography/regions` for admission, search, exact IDs and admitted children.
- `GET /api/v1/map/features` for complete scoped counts and bounded typed features/aggregates.
- `GET /api/v1/map/reference/{z}/{x}/{y}.pbf` for authorized PostGIS vector tiles.
- `POST /api/v1/map/measure` and `/api/v1/map/surrounding-area` for validated PostGIS geography operations.
- Compatible optional `region_id` and `geometry` filters on the existing supported event/context/contact/imagery list contracts, applied before limits.

OpenAPI and generated frontend types include the new contracts. Existing run controls, legacy Exercise map behavior, workers and rendering engine remain available. Geographical preparation, raster processing and simulation execution stay outside synchronous API handlers.

Public read paths enforce partition, resource, source and family permissions before summaries, feature selection and tiles. Responses are private/no-store. Tile requests use authenticated headers, never credential URLs. Shared views contain whitelisted references and view settings; recipients need their own grants. Workspace/account transitions retain request cancellation, cache clearing and authorization revalidation. Public launchers do not preload authoring scenes, evaluation truth or hidden routes.

The geographic installer was run, archived and repeated: all three edition IDs remained unchanged. Source archives share content-addressed chunks while retaining each acquisition receipt by its own hash, so identical bytes acquired under different names do not conflict or overwrite historical receipts.

## Acceptance evidence

The following checks passed for this delivery:

- Frontend type checking, OpenAPI type generation and production build.
- The full frontend regression command: **80 passed, 54 intentionally skipped** because those scenarios require their explicitly selected live/integration profiles. These skips are not counted as accepted live workflows.
- Persistent map/workspace/polish checks: **27 passed**. The final navigation/library refinements passed their affected 19 checks; the final map/history/imagery changes passed all 12 map checks again.
- The real-backend production-browser gate: **16 passed**, including traceable imagery analysis and independent review/export, public moving hypotheses, official-report review, input-only authoring, exercise execution, replay/evaluation and revocation. No domain API or WebSocket is mocked by this gate.
- Persistent installation after supported restart: **three passed** for Public/imagery/runtime/recording replay, evaluator/account isolation, populated graphics, mobile operation and real WebSocket reconnect/shutdown. The [renderer measurement](map-implementation/graphics-acceptance.json) identifies ANGLE SwiftShader software rendering and its measurement conditions; it is not a hardware-GPU benchmark.
- `go test ./...` passed, with externally gated integration profiles skipped by that command. Dedicated real-PostGIS checks separately passed `TestGeographicMapExactScopesHistoryAggregatesAndIsolation`, `TestM1NativeFIRMSUSGSReferenceAndBitemporalEvidence`, `TestReferenceCatalogRealRegionTerrainAndIsolation` and `TestP33StationWeatherRevisionsRightsAndNoTracks`. Targeted Go checks and Python compilation passed after final cleanup.
- The geographic integration check covers immutable/idempotent admission, aliases/hierarchy, exact polygons versus neighboring points inside their rectangles, boundary membership, historical revision ordering, withdrawals, complete large-result counts, actual MVT bytes, private caching, service/source/resource/partition isolation, and invalid, missing and date-line measurement input.
- The supported doctor reports available storage/runtime and ready workers on the shared IPv4 API address. The persistent application remains running at the live map link above.

The [installation timing record](map-implementation/installation-acceptance.json) contains five warm loopback samples per family. Median responses in that capture ranged from 6.26 to 30.84 ms. This is a small installed-data check on Windows 11, not a production load test or a promised performance limit.

A consistent backup was taken after stopping the supported API/frontend/workers, then restored into isolated randomly named PostgreSQL/Garage containers. The [restore acceptance](map-implementation/restore-acceptance.json) verifies **306 objects / 676,351,671 bytes**, all three navigation editions and retained navigation sources, 24 reference versions, four capture records, seven model/environment records and two recordings. The retained real imagery comparison reproduced the same analysis hash. The original database and volumes were preserved; the isolated resources were removed. Private credentials and backup bytes remain ignored under `private/local`.

## Explicit boundaries

World selection does not provide world-wide evidence coverage. Detailed terrain/imagery/meshes remain bounded near Jeddah. No city/governorate geometry is fabricated from a place name. Current feed results, freshness and provider restrictions remain visible; zero contacts are not replaced with synthetic observations.

The Public map shows evidence and source context. Studio uses protected inputs; Exercise views use permitted perception; evaluation truth requires separate authorization. Simulation fidelity remains the declared abstract profile. Globe rendering, large-world streaming, advanced physics and calibrated sensors retain their previously deferred status.

The build still reports existing large renderer chunks (MapLibre/GLTF); successful functional acceptance does not remove that bundle-size limitation. Optional external basemap loading does not block retained navigation, imagery or evidence layers.
