# EMAD map exploration and situational awareness

24 October 2026. Status: adopted and implemented. See [implementation, installed scope and acceptance evidence](EMAD_MAP_IMPLEMENTATION.md). The baseline below records the pre-implementation inspection; it is retained as the rationale for this change. The delivery preserves the clean map, full Systems grid, focused workbenches and Public/Exercise isolation. The rejected layout-study HTML remains historical.

## The product change

The map should answer five questions without requiring a different page:

1. Where am I looking, and what geographic area is selected?
2. What observations and evidence are available here, and when were they observed?
3. What is new, stale, unavailable or outside source coverage?
4. What can I inspect, compare or investigate next?
5. Which EMAD capability can I use on this area or selected object without losing the map?

Every geographically located, authorized record family should have an appropriate map representation. This does not mean opening every detailed form, every reference feature and every label simultaneously. Map graphics, scale-dependent detail and compact context should carry the overview. The existing preview dock and focused workbenches carry deeper examination.

The current work is therefore more than adding zoom controls. It needs geographic scope selection, a multi-layer map model and region/time-aware summaries alongside the visible tools.

## Verified baseline and current limitations

The [installation snapshot](map-exploration/installed-summary.json) was captured through authorized read-only API requests at 11:28 Riyadh time on 24 October 2026. These are workspace query results, not counts within an exact Saudi country polygon:

- 24 reference dataset records, comprising 23 vector records and one elevation record, with 4,006 vector features. Several records are storage chunks of the same geographic theme.
- One installed country feature: Saudi Arabia, from generalized Natural Earth geometry. No province or governorate hierarchy is installed by the current geography preparation path.
- 16 retained earthquake reports, with observation times from March 2025 through 16 October 2026. These are not 16 current incidents.
- 51 returned station-weather records, all classified current by the API at capture time. No official-report records were returned by that particular context query.
- Four successful imagery capture records representing two distinct acquisition times in March 2025.
- Zero returned external tracks. This is a query result, not evidence of no aircraft or vessels.

Source inspection found these gaps:

- `PublicMapWorkspace` offers Saudi Arabia, Coastal Jeddah and a custom bounding box. It has no actual country/region search or administrative drill-down.
- `usePublicContext.mapFeatures` builds reference and public-event geometries. Weather and track previews contribute only a selected-record location; imagery contributes a selected extent. Assessment previews are workspace-wide and do not become their own finding layer.
- `usePublicOverview` enables most record queries according to the single preview family. That is suitable for an inexpensive preview strip, but it cannot drive several simultaneously visible map layers.
- Public report points are clickable; the public interaction path does not provide equivalent hit-testing for countries, reference objects, observation footprints, weather stations, imagery or derived findings.
- Reference polygons, lines and points have very broad shared styling. Roads, borders, building footprints and different observation types do not yet communicate distinct roles effectively.
- The reference viewport endpoint returns 2,000 rows, sorted by dataset and feature IDs. A manual next page is useful for inspection, but incomplete arbitrary pages are unsuitable as the normal cartographic display.
- Reference imports and detail viewports reuse the 400-square-degree bound validation. A world country catalogue cannot simply be dropped into that existing bounded import path. Catalogue listing, per-import feature and selected-dataset limits also need an explicit growth path.
- Country names on the raster basemap are pixels, not selectable, versioned application regions.
- Panning changes reference viewport queries, while evidence uses the saved investigation. The distinction is currently explained in text but is not yet a strong geographic interaction model.

Relevant implementation locations: `features/workspace/PublicMapWorkspace.tsx`, `features/public-context/usePublicContext.ts`, `features/workspace/usePublicOverview.ts`, `components/map/MapCanvas.tsx`, `internal/reference/viewport.go` and `scripts/data/prepare_geography.py`.

## Geographic navigation should be a first-class workflow

Provide one search field for countries, admitted administrative regions, places, saved areas and coordinates. Search results identify their type and parent: a city, a province and a prepared simulation area are different objects even when names are similar. Support retained English/local names and source-backed aliases, including Arabic, without silently rewriting original names.

Use a persistent scope breadcrumb. A typical journey is:

**World → Saudi Arabia → Makkah Region → Jeddah coastal study area.**

The last object in this example is the installed bounded study environment. It must not be presented as an official Jeddah municipal boundary. A city search can locate a place before a city polygon has been admitted; the interface must say which scope it actually selected.

Interaction rules:

- Hover highlights an eligible boundary and offers a small name/coverage tooltip. It does not change the investigation or camera.
- Clicking a country or explicitly choosing a search result selects a versioned geographic scope, outlines it, updates the regional summary and fits the area with space for visible controls.
- A country exposes its admitted subdivisions through the breadcrumb and map at appropriate zoom levels. A geographic selection-level control resolves ambiguous nested boundaries.
- Clicking an observation, footprint or selected object takes priority over the boundary beneath it. A point click must not unexpectedly switch countries.
- Touch uses taps and explicit selectors; hover is never required. Keyboard search and an accessible results list provide equivalent navigation.
- Panning moves the camera without silently changing a pinned country. An explicit **Search this view** action or **Follow viewport** setting controls investigation changes.
- A parent breadcrumb returns to the parent scope. Fit selected area, previous view and saved views are distinct actions.
- Changes preserve compatible time/layer choices. An object outside the new scope remains an explicitly pinned outside-scope object or is cleared; its location is never silently moved.

World navigation and evidence coverage are separate. All admitted countries can be selectable even when only Saudi and neighboring-area evidence is configured. Choosing another country must show its actual data availability and must not automatically expand provider subscriptions or worker grants.

Administrative land, offshore context and maritime jurisdictions are also separate. Nearby offshore reports or vessels may be useful to a coastal investigation. Offer a visibly bounded surrounding-context area when needed; do not turn a land boundary or a rectangular envelope into an implied maritime jurisdiction.

## The map needs a shared multi-layer model

Separate three states:

- **Geographic scope:** where the investigation applies.
- **Visible layers:** which admitted geographic and evidence families are drawn together.
- **Preview category:** which records are browsed in the compact dock.

Changing the preview category must not remove otherwise enabled map layers. Fetch only the enabled, permitted layers and the deliberately opened preview/detail requests. Avoid fetching every family merely because the map is open.

Use user-facing themes such as boundaries, places, main roads, local roads, buildings, relief, earthquake reports, thermal detections, weather stations, official bulletins, aircraft, vessels, imagery and findings. Storage chunk numbers and artifact hashes belong in version/provenance detail. Family membership should come from explicit import/catalogue metadata, not a fragile guess from display names.

Give each theme a distinct spatial language:

- **Earthquake reports:** reported locations with magnitude and observation-time detail. A measurement or reported area remains different from an approximate display buffer.
- **FIRMS:** thermal-detection symbols and retained measurement footprints. Labels say thermal detections; they do not automatically assert confirmed ongoing fires.
- **Weather:** station symbols with selective temperature/wind labels at regional/local scales, observed time and freshness. No invented continuous weather field between stations.
- **Official bulletins:** document/report symbols at cited locations or retained geometries. Claimed impact areas remain claims; missing geometry is not replaced with an invented country-centre point.
- **Aircraft and vessels:** different reported-track symbols, age and available heading/course fields. Unknown values stay unknown, stale positions remain distinguishable and external contacts have no control actions.
- **Imagery:** actual capture footprints with acquisition dates. Selecting a compatible local capture can draw its retained raster window at its real extent, with opacity, quality-mask and before/after controls. Never stretch a Jeddah subset into national satellite coverage.
- **Analysis and assessments:** derived finding geometry, proposal/review status and direct supporting-evidence links. A published development workflow record does not become genuine public evidence. Any development/synthetic handling needs canonical metadata, not a name-prefix heuristic.
- **Reference data:** different line weights and visibility ranges for borders, main roads, local roads, coastlines and buildings. Important evidence must remain distinguishable from background reference geometry.

At broad scales, show administrative overviews and type-specific aggregates or clusters. At regional scales, show clusters plus selected important labels. Locally, resolve individual reports, stations, imagery extents and detailed reference geometry. These are adaptive representations of the same map, not unrelated pages.

Do not combine aircraft, weather, earthquakes and thermal detections into one generic count or danger score. Cluster counts count their stated record type; they do not prove distinct real-world incidents. Keep reported severity, analyst confidence, retrieval freshness and display selection in separate visual channels.

## Make the situation readable directly on the canvas

A compact selected-area summary should expose the scope, observation interval, available report categories and coverage limitations. Clicking a category toggles or focuses the corresponding map layer and opens its relevant record preview. Counts and geometries must come from the same query scope.

Hover or tap a marker for a short spatial peek: type/title, observation time, source, freshness, uncertainty and an Inspect action. Detailed revisions, exact downloads and human review remain in the existing inspector/workbench. Overlapping objects open a small object picker; the user must not have to guess which layer received the click.

A compact **Changes since you opened this view** entry can expose newly received or revised records. Distinguish newly received historical evidence from a newly occurring observation. Updates should not recenter the camera, reorder the selected record or continuously pulse every symbol. Respect reduced-motion preferences.

The initial view should show something useful without requiring a tour of menus: country/region context, enabled authorized evidence, meaningful symbols, dates and visible layer/time controls. If recent event results are empty, show dated retained-history availability as an explicit action. Do not silently expand the time window or substitute historical reports under a current label.

Useful presets can be **Situation**, **Imagery** and **Geography**. They apply understandable layer arrangements while retaining scope/time/selection and preserving explicit user overrides. They are map views within Public Context, not additional product pillars.

## Systems and other capabilities inside the map

The map should be a persistent workspace that can host the Systems grid and focused capability workbenches. Module access must provide useful work inside that workspace, as well as an explicit expansion to a full view.

Use three coordinated presentations for each applicable capability:

1. **Its spatial representation:** observations, locations, footprints, findings, admitted coverage or sanitized exercise objects drawn on the canvas.
2. **Its map workbench:** the actual record list, detail, comparison or authorized task in the shared dock, linked to the selected area/object.
3. **Its full workspace:** the expanded version for work that benefits from more space. Existing standalone routes remain valid.

Not every capability has geographic data. Models, worker queues and audit entries must remain selectable without acquiring invented map locations. Source-policy areas can be shown only as their stated coverage/query areas, not as proof of complete observation coverage.

### Systems as the map's capability gateway

Keep a clearly labeled **Systems** entry in the map workspace navigation. It opens the same domain-based grid with a compact map presentation, real previews and meaningful actions. The full Systems grid remains available through Maximize and its existing standalone route.

The grid offers:

- **This area:** geography, imagery, observations, weather, external tracks, located assessments and available prepared environments that match the selected scope, where the relevant query supports that scope.
- **Workspace:** all authorized content, global processing state and records that do not yet support geographic association. This scope is always labeled; unsupported geographic filtering must not be implied.
- **Sources & processing:** access, retained coverage, freshness, failures and job progress, with map footprints only where an admitted coverage geometry exists.

A grid cell separates three actions: enable/focus its spatial layer, open its workbench, and maximize its full workspace. Clicking a capability does not silently recenter the map or enable every layer in its domain. A Locate action deliberately fits a selected record; an explicit **Show on map** action enables the relevant overlay.

This keeps the useful Systems design and gives it a direct role in map-based work. Do not reduce the grid to a list of generic app shortcuts or embed a screenshot of the standalone screen.

### What each capability delivers inside the map

- **Geography:** logical layer management, selected-region/reference detail and feature inspection in the dock; corresponding boundaries/roads/places on the canvas. Exact artifact versions and rights remain inspectable.
- **Imagery:** a scoped acquisition list and date/pair controls in the dock; footprints, real retained rasters, quality masks and comparison overlays on the canvas. Open the complete analysis workbench when more space is needed.
- **Public reports and weather:** scoped lists, source/time detail, exact evidence and retained history beside typed map symbols. Selecting a list row and selecting its map symbol refer to the same record.
- **Aircraft and vessels:** reported-contact lists and evidence history connected to permitted positions/trails. Public contacts remain external observations with no asset or command actions.
- **Assessments:** a scoped inbox when geography is known, selected proposal/revision and review eligibility in the dock; supporting finding geometry on the map. A full review form can expand without changing the selected assessment. Publication still requires the existing human identity, revision and rationale checks.
- **Context packages:** choose/draw an admitted bounded AOI on the map, select eligible input versions in the dock and inspect coverage/omissions. Validation and human approval remain explicit. A country selection does not imply a complete country-sized environment can be exported from the small retained Jeddah subset.
- **Sources and processing:** source-health cards, supported diagnostics and job progress in the dock. Selecting a source can show its declared query/coverage area. Closing the dock does not cancel a server job or stop its worker.
- **Models and actor templates:** a protected authoring-library workbench with actual thumbnails, dimensions, rights and reusable templates. These are workspace resources until deliberately placed in an authorized scene; they receive no arbitrary country-centre marker.
- **Studio and environments:** show authorized prepared-environment locations/availability and enter a specific georeferenced scene. The 3D authoring viewport takes the main canvas when Studio is opened in its input workspace. It is not a Public COP overlay.
- **Exercise operations:** in the selected Exercise workspace, expose permitted controlled assets, missions, virtual commands, telemetry, incidents and audit through workbenches tied to sanitized perceived geometry. Keep Controlled Operations within its approved exercise-support role.
- **Recording and replay:** choose an authorized recorded run and navigate its retained events and spatial view in the same application shell. Its transport uses recorded virtual time, not the Public map observation-time filter. No simulation is rerun by opening replay.
- **Evaluation:** expose released results where allowed and a separately authorized evaluator workspace for privileged spatial comparisons. Neither the launcher, Systems previews nor ordinary map caches may preload truth to make appealing cards.

All capabilities should be discoverable through the map's navigation/gateway. Capability availability is permission-aware; being geographically selected does not grant access to another partition or enable a missing provider.

### One dock with predictable states

The existing compact preview dock becomes the shared host for record previews and the active capability. Opening a workbench replaces the dock's body; it does not add a second large panel beside the first.

- **Collapsed:** retain a slim task label, progress/dirty indicator where appropriate, and Restore. The map remains usable.
- **Compact:** previews or a short record list, approximately the existing 100–152px presentation.
- **Split:** the active Systems grid or workbench has deliberately allocated bottom/side space. An initial desktop bottom range around 280–360px is a design target to verify, with presets and optional resizing; it is not a fixed requirement for every viewport.
- **Maximized:** the same capability uses the available workspace. Keep an explicit Return to map action and restore the prior map and dock layout. If the map is fully covered, suspend unnecessary rendering while retaining safe view state.

Support one active large workbench and at most one small selected-object peek. Reference controls use short-lived popovers; inspectors nest within the active workbench's navigation rather than opening chains of modal drawers. Bottom and side placement are alternatives, not simultaneous permanent obstructions.

On phones, the shared dock uses small, medium and expanded bottom-sheet states. Expansion shows the actual tool with proper focus and scrolling. A visible return-to-map action restores the selected geometry; the workflow must not depend on hover or desktop-sized embedded forms.

### One navigation and one investigation context

Reuse the workspace task navigation as the capability selector. Keep Systems and the active capability discoverable, with a compact searchable selector for additional capabilities. Avoid duplicating the same large tab row in a floating map toolbar. Only one task is marked active, even when its canvas remains a map.

A persistent context holds authorized workspace/partition/run, geographic scope/version, observation-time mode, visible layers, camera, selected typed object and active capability/layout. Context may also include the exact chosen acquisition pair, analysis job or assessment revision. Module query state remains typed; not every tool accepts every context field.

Lists follow the map scope only when their query actually supports it. Frozen analyses, selected immutable revisions, prepared scenes and dirty authoring/export drafts remain bound to their original inputs. If the area changes, identify those as pinned work for the original area. Rebinding a draft must revalidate inputs and invalidate any incompatible export preview; it must not change the meaning of an existing approval.

Scope labels should explicitly distinguish **Selected area**, **Workspace**, **Scene** and **Exercise run**. In particular, the current Systems implementation treats assessments and processing as workspace-wide; simply embedding it must not relabel those figures as country totals.

Browser Back should unwind the opened task/record and previous scopes predictably. Maximize, collapse and return must retain camera, filters and selection. Persist safe layout/view preferences per identity/resource; sensitive drafts stay in authorized session memory. Shared links carry typed references and are reauthorized for the recipient. Tokens, private scene bytes and truth snapshots never become URL or local-storage state.

### Preserve the two product areas during map-based navigation

Public Context capabilities operate in Public Evidence. Studio uses Exercise Input; exercise operations/replay use the selected Exercise Observation run; privileged evaluation uses separate truth authorization. Present these as explicit workspace transitions in the existing product shell, with the correct clock and scope label.

The shared shell does not require every mode to mount the Public map component or even use the same route. Current input-only authors have an independent Studio shell and no COP subscription. Preserve that property: its main canvas can be the authorized scene/overview renderer, while the Exercise canvas receives only permitted perception. Reuse the capability-host layout across those modes; do not grant Public/COP access merely to make a tool appear. Public Systems can show authorized public context areas; private authoring environments are queried only through the appropriate authorized transition.

For example, **Prepare this area for simulation** starts with a valid frozen context package and an explicit authoring transition. It does not convert external aircraft into controlled assets or load hidden scene actors into a Public map. Product/workspace switching retains the existing abort, cache-clear and access-revalidation behavior. Safe public map preferences can be restored upon return, without retaining a hidden private workbench.

### Implementation approach

Add a typed capability registry that describes label, required resource/permissions, supported scopes/time modes, lazy workbench, full-view route and valid object actions. It describes supported behavior; it does not replace server authorization. A selected module loads only after the appropriate resource is authorized. The launcher must not fetch every module's private records in advance.

Refactor reusable workbench bodies from route adapters. Current pages call strict route-specific `useSearch` and navigation hooks, so rendering whole pages inside a map drawer would create conflicting route state and duplicated chrome. A workbench should receive explicit context/selection/actions; its standalone route and its map host adapt the same component. Share their scoped query models to avoid duplicate requests and divergent record state. Do not use iframes or copy the same business forms into a second implementation.

A proposed shareable route could be `/map?tool=imagery&layout=split&...`, with validated typed object/context references. The current map route only validates selection and public filters, so this requires deliberate route-contract work. Existing standalone links continue to function. Each workbench has its own loading/error boundary; a failed module must leave the map and other permitted layers usable.

## Controls and visual layout

Keep the neutral/graphite visual system and the existing full Systems grid. The map's controls need predictable locations and short labels.

- **Top:** place/region search, selected-area breadcrumb and a compact observation-time control. The selected interval remains visible when its editor is closed. Existing workspace navigation selects Systems/capabilities without another duplicate full-width navigation stack.
- **Slim context row:** relevant category counts, dated imagery availability and partial/unavailable coverage. It must not become a second Systems dashboard.
- **Left tool rail:** layers, study-area drawing, measurement and saved views. Open one compact tool popover at a time.
- **Right navigation group:** zoom, north reset, fit selected area, previous view and fullscreen. Avoid duplicate zoom controls. Any pitch/relief control must identify available geographic coverage.
- **Lower corner:** a small meaningful legend, scale and coordinate readout. Provider health is separate from the event-connection indicator.
- **Bottom:** the remembered shared preview/workbench dock, expanded deliberately for Systems, records, comparisons or investigation tasks. It occupies layout space and does not hide the selected geography.

Labels and graphics should declutter before the product hides useful evidence. Give selected objects priority, use scale ranges and collision-aware labels, and make layer opacity/order available in an advanced layer popover. A dense streets/buildings layer should not dominate a country overview.

On phones, keep search/scope/time compact and make marker peeks a bounded bottom sheet with small/medium/expanded states. Keep selected areas and markers inside visible safe bounds. Desktop hover conveniences need tap/keyboard equivalents; close, escape and focus restoration remain consistent with current dialogs.

## Time, freshness and coverage need explicit meanings

The ordinary time control filters observation/event time, with presets such as the last 24 hours, seven days or a custom interval. Separate advanced controls retain the existing **known at** and **provider-effective** historical semantics. Receipt/retrieval time is never silently substituted for observation time.

Reference context and acquired imagery have their own version/acquisition dates. A historical raster used as geographic background needs a persistent acquisition-date label when current observations are overlaid on it.

Weather and external tracks do not currently implement the same historical query as public events. In a historical investigation, show their unsupported status or an explicitly separate current-context view. Never paint current positions beneath a historical timestamp as though they were historical observations.

For each region/family distinguish: not configured, not authorized, outside coverage, retrieval failed, retained but stale, historical material available, and a genuinely successful zero-result query. A map-wide green connection dot cannot communicate those states.

**Pause display updates** freezes the user's current public view; it does not pause ingestion or a simulation clock. Resume catches up explicitly. Exercise views continue to use their separately defined virtual time and authorized perceived observations.

## Geographic data and implementation requirements

Create a versioned geographic catalogue with stable region IDs, parent links, optional ISO codes, names/aliases, source geometry, validity/revision, rights and separate simplified display geometry. Search results and map boundaries must reference the same catalogue objects. Basemap labels must not determine the analytical boundary.

Give global navigation geometry an admitted, size-bounded preparation/storage path that is distinct from provider retrieval and bounded evidence-detail queries. Do not relax the shared `publicingest.Bounds.Validate` limit globally to make a world map work. Retain feed budgets and rights restrictions, add catalogue pagination and serve only scale-appropriate geometry/aggregates to the browser. Source/partition filtering must run before aggregation as well as before record detail.

For source evaluation, Natural Earth has a generalized [Admin-1 collection](https://www.naturalearthdata.com/downloads/10m-cultural-vectors/10m-admin-1-states-provinces/), but warns that administrative divisions are difficult to keep current. The geoBoundaries Saudi [ADM1 metadata](https://www.geoboundaries.org/api/current/gbOpen/SAU/ADM1/) describes 13 units representing 2017; its [ADM2 metadata](https://www.geoboundaries.org/api/current/gbOpen/SAU/ADM2/) describes 147 units representing 2021. Their reported original licenses differ. Evaluate source dates, quality, hierarchy and applicable rights before admission; do not advertise these snapshots as current official divisions merely because they were downloaded recently.

Country/province membership needs authoritative PostGIS predicates against admitted polygons. Bounding boxes can narrow candidates, but should not decide membership. For point observations, [ST_Covers](https://postgis.net/docs/ST_Covers.html) includes boundary points; area observations require an explicit intersection policy. Offshore and cross-boundary geometries need stated inclusion rules. Counts are not necessarily additive across intersected regions, and tied boundary membership needs a defined treatment.

Introduce a scoped map read model, with proposed contracts for:

- Geographic search, region detail and admitted children.
- A layer catalogue describing family, geometry, coverage, supported time modes, freshness, rights, legend and availability.
- Region/time summaries with complete query counts, count meaning, source state, data revision and query fingerprint.
- Bounded typed features/aggregates and scale-appropriate reference tiles.
- A detail resolver for the selected typed object and its exact evidence/version.

These extend the modular monolith and existing APIs. They are not a request for a new GIS service, queue or rendering engine. Preserve existing endpoints and immutable documents; use forward migrations for any new persistent catalogue/read-model state.

The same region/version, filters, observation interval, knowledge-time mode and authorization scope must key summaries and feature queries. Cancel obsolete requests on scope changes. A country header must not display the previous country's totals while new features load. Public map endpoints and tile caches must preserve partition, identity/source restrictions and revocation behavior.

Use complete server summaries and scale-dependent tiles/aggregates instead of counting a capped client list. [PostGIS ST_AsMVT](https://postgis.net/docs/ST_AsMVT.html) can produce vector tiles from query rows. Keep the existing bounded inspection query for evidence drill-down; do not solve world navigation by lifting its feature limits and downloading every raw object.

MapLibre already provides relevant [clustering](https://maplibre.org/maplibre-gl-js/docs/examples/create-and-style-clusters/) and [hover-state](https://maplibre.org/maplibre-gl-js/docs/examples/create-a-hover-effect/) patterns. Validate the implementation against the installed 5.24 dependency; these public documentation pages are not a reason to adopt their newer example CDN version. Reuse shared camera/control utilities and add typed public layers while protecting the established Exercise map path.

Do not run geographic preparation, imagery processing or simulation physics in synchronous API handlers. Do not introduce a globe, global high-resolution imagery or large-world 3D streaming as a dependency of country selection. Relief and 2.5D extrusions can follow where retained elevation and declared building heights actually support them; they are distinct from native Simulation Studio.

## Delivery sequence

1. **Persistent map workspace and Systems.** Establish the shared context, capability registry, typed tool routing and collapsible/maximizable workbench host. Reuse the Systems grid with truthful selected-area/workspace scope. Exit: open Systems from the map, choose a permitted capability and return without losing the map or opening duplicate panels.
2. **Geographic scope and country/regional overview.** Admit global country navigation and evaluated Saudi subdivisions, add catalogue/search/parent selection, explicit country-versus-viewport scope, polygon-aware summaries and an overview representation that works beyond the existing bounded-detail query. Exit: selecting Saudi Arabia and an admitted subdivision produces matching boundaries, Systems context and query scope.
3. **Visible situation layers and evidence workbenches.** Render authorized weather, earthquake/thermal reports, available official bulletins and external tracks simultaneously with typed symbols, scale-dependent clustering, marker/footprint inspection and visible freshness/coverage. Decouple visibility from preview category. Connect actual report/weather/contact workbench bodies to the same selection. Exit: available station observations and dated reports can be understood and inspected from the map without first opening their dedicated pages.
4. **Imagery, assessments and context preparation.** Add acquisition footprints, actual bounded rasters, date selection, quality overlays, before/after comparison and derived/assessed finding geometry linked to exact inputs and review status. Integrate their workbenches and bounded context-package preparation. Exit: the real Jeddah pair can be examined in place and followed to review/context validation, with dates, rights and interpretation limitations retained.
5. **Simulation and retained-run capabilities.** Add the authorized environment/model/Studio entry paths and run-scoped operations, replay and evaluator workbenches in their proper workspaces. Exit: a deliberate context-package-to-scene transition and an authorized recording can be opened through map navigation without public/private data or clock mixing.
6. **Investigation tools and persistence.** Finish drawing bounded study areas, distance/area measurement, surrounding context, useful basemap choices, logical layer controls, bookmarks, shareable authorized views and time/arrival navigation. Exit: users can save and resume the same scope, layers, dates, selected evidence and capability layout.
7. **Cross-scale acceptance and polish.** Integrate large-result behavior, international/date-line cases, mobile/keyboard operation, provider failures and restart/revocation tests. Each earlier increment includes its interface and error states; this step completes the combined acceptance rather than deferring correctness until the end.

The first reviewable delivery should complete the geographic selection-to-evidence journey. Country selection, regional counts, visible station/report symbols and inspection belong together. An isolated toolbar or a country dropdown without matching data behavior does not satisfy that exit.

The Systems/workbench host is part of that journey. A menu that routes away to disconnected pages does not satisfy the requirement to use capabilities inside the map.

## Acceptance scenarios

- Select a country by search and map click; confirm one selected versioned scope, correct parent/child navigation and matching summary/features. Test keyboard and touch equivalents.
- Select an administrative polygon with neighbors inside its bounding rectangle. Neighboring land observations must not be counted as inside the selected jurisdiction. Test coastal/nearby context separately.
- Show several evidence families together, switch the preview category and confirm the enabled map layers remain visible. Turn one layer off and confirm its requests stop without disabling others.
- Open Systems inside the map, inspect its real grid, choose Weather or Imagery and confirm the working tool and its map geometry share the selected context. Collapse, maximize and return; camera, layer choices and selection remain consistent.
- Enter the same capability through its existing standalone route. Confirm identical business rules, authorized data and actions, with clear region/workspace/run scope labels and no duplicated page chrome.
- Change geography while an immutable result or dirty draft is pinned. Confirm its original inputs remain explicit and an incompatible export preview cannot be approved under a new region label.
- Confirm every returned, display-eligible station/report in a complete bounded result is represented or explicitly aggregated. Summary counts must not become the length of the first capped feature page.
- Inspect overlapping objects, footprints and an unlocated report. No synthetic coordinate, invented heading or arbitrary confidence appears.
- Select the real March 2025 imagery pair in Jeddah, compare it on-map and follow a finding to its retained analysis and review history. No national imagery coverage is implied.
- Enter a recent empty period and expose retained-history availability separately. Disable a provider and distinguish the outage from a successful zero response.
- Pan/zoom, switch scope quickly and replay browser history. Old responses, hidden layers and outside-scope selections must not silently change the new investigation.
- Test a world overview and date-line/multipart country geometries without duplicate wrapped-world counts or silently retained stale regional geometry.
- Confirm ordinary Public/Exercise operators cannot obtain authoring scenes, privileged truth, hidden routes or evaluator geometry through new features, tiles, summaries, links, caches or WebSockets.
- Verify narrow/short viewports, keyboard drawing alternatives, focus, readable legends, reduced motion, restart persistence and measured behavior on the existing installation. Set performance budgets from observed hardware and data volume; do not present targets as completed benchmarks.

## Recommendation

Keep Systems as the full overview and expose its grid inside the persistent map workspace. Make Map the place to explore geography, read evidence and open the relevant capability directly. Prioritize selectable geography, simultaneous spatial data, truthful regional context and one shared workbench host. Reuse detailed analysis, review and authoring workbenches with explicit full-view expansion and authorized workspace transitions.
