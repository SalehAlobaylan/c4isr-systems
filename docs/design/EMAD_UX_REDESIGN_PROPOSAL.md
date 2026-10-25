# EMAD UX redesign proposal

Date: 23 October 2026. Status: design proposal; application implementation has not started. Approval applies to the proposed workflow and visual direction, not to claims of completed software.

## The product decision

EMAD needs a new product interface around its two pillars. The existing UI still organizes much of the experience around the earlier operations console. Adding functioning features to that shell has produced an accumulation of panels, forms and links rather than a coherent analytical tool.

The recommended design is a persistent spatial workbench: one product shell, two product areas, task-specific work surfaces, linked selection and progressive detail. Users should see their evidence, geography or scenario immediately, understand its source and time basis, and have an obvious next action.

This proposal follows the [architecture direction](../../EMAD_ARCHITECTURE_AND_DIRECTION_v2.md), especially section 11, and the [data strategy](../../EMAD_FINAL_DATA_STRATEGY_v2.md). Controlled operations remains exercise support. Public observations, synthetic observations, exercise input and privileged truth retain separate server authorization and client lifetimes.

## What was actually inspected

Read the current shell, navigation, workspace switching, map, public filters, evidence inspection, source status, imagery capture/comparison/assessment/export, model registry, scene authoring, operator view and replay/evaluation implementations. Captured nine installed desktop routes plus a phone map, using the real API, private analyst/author and operator credentials, and read-only browser navigation. No new run, assessment, export, ingestion job or review was submitted. The browser did not mock responses. The initial seven-route capture recorded no page errors on the five public routes; it was a UX inspection, not a complete functional or accessibility test suite.

Evidence is in [the dated audit directory](ux-review-2026-10-23/). [The API snapshot](ux-review-2026-10-23/installation-snapshot.json) and [UI observations](ux-review-2026-10-23/current-ui-observations.json) are point-in-time records, not a live feed. Browser interaction can update ordinary local workspace preferences; it did not alter domain content. No credentials are included in these artifacts.

At the snapshot, the real API returned:

- 24 reference dataset versions and 12 source registrations.
- 19 imagery catalogue revisions and four successful capture records. Catalogue revisions are not four or nineteen independent retained image acquisitions.
- 931 retained provider records in readiness, but 16 normalized public events returned by the event endpoint. These have different meanings and must not be presented as interchangeable counts.
- 51 weather/report records and zero external tracks in that authorized query. Zero records does not establish absence or complete provider coverage.
- Three imagery processing jobs; one reproducible NDVI comparison and two preparations.
- Two assessment heads: one real proposal awaiting review and one published development workflow test.
- Seven validated visual assets: six actor models plus the prepared environment; one environment registration and two frozen scenes. One scene is an acceptance exercise.

There are useful capabilities to expose. There are also bounded coverage, historical inputs, unavailable retrieval and future simulation limits to explain honestly.

## Why the current experience fails

1. **Three navigation systems compete.** The old Map/Systems/More bar, a second module bar, and the workspace/environment/run bar all occupy the top of ordinary screens. Public imagery and Studio look attached to another product. Sources: `TopBar.tsx`, `ModuleNav.tsx`, `AppShell.tsx`.
2. **The primary evidence starts too far down.** At 1440 × 1000, the Public map canvas starts at y=440 and the Studio canvas at y=668. The map's canvas is 528px high before overlay occlusion. Only 300px of the 438px Studio canvas lies above the footer; the actual environment is smaller again inside it. Replay's viewport begins near the bottom of the first screen. These measurements describe this installation and viewport, not universal dimensions.
3. **Public Context occupies an incidental scrolling box.** `PublicContextPanel` combines navigation, interpretation notes, filters, reference versions, layer visibility, pagination, provider health and selected evidence inside `max-h-64`. Those are the main analytical controls, yet they are subordinate to a legacy picture drawer.
4. **The Public map advertises the wrong objects.** Tracks, controlled assets, zones, telemetry, board and exercise-style freshness remain visually prominent with zero counts. The public evidence inspector is elsewhere. The user sees an empty operations console while retained evidence exists.
5. **Geography lacks a useful reading hierarchy.** Twenty-three selected vector layers, largely similar blue geometry and a national extent do not teach the user where the detailed Jeddah content is. Bounded building geometry, national reference, dataset coverage and reports need different representations and scale behavior.
6. **Imagery is a long implementation form.** Real images now appear, but acquisition, catalogue metadata, prepared inputs, analysis jobs, results, proposal authoring, review and export are one scrolling page. Repeated IDs, hashes and decimal-degree boxes obscure the intended sequence. The page had roughly 17,000 characters of rendered text in the inspection; length alone is not a usability score, but the disconnected sections are observable.
7. **Studio is a document containing an editor.** Version buttons and runtime forms precede the viewport. Actor properties, route editing, perception settings, model selection and freeze controls live farther down the page. Selecting an actor does not put its editing controls alongside the scene. The first listed scene is an acceptance scene rather than the intended normal entry point.
8. **Exercise, playback and evaluation have fragmented context.** A long run hash stands in for a human-readable exercise. Playback transport and archive administration take substantial space before the spatial view. The privileged evaluation disclosure shares the ordinary playback page, making its security meaning harder to understand even though the underlying access gates exist.
9. **The green LIVE label is too broad.** It describes socket/API connectivity while individual source health can be stale, unavailable or partial. At the source snapshot, FIRMS was configured and backfilling; station weather and GDACS were stale; OpenSky's retrieval health was unavailable despite configured credentials. These are observations, not a diagnosis of those retrieval failures. Healthy transport cannot certify fresh evidence.
10. **The phone layout preserves controls at the expense of the map.** At 390 × 844, multiple header bands and the public panel push the map down, then the picture drawer covers most of the remaining map. Avoiding horizontal overflow did not make this workflow usable.
11. **Home explains the product before helping someone use it.** A large slogan and installation counters precede actual work. Several counters describe infrastructure and revisions rather than decisions a user can make. First use should lead to Saudi geography, retained Jeddah imagery or a ready scene.
12. **Trust semantics are verbose but poorly placed.** Important caveats appear as repeated paragraphs, while hashes and raw JSON compete with evidence. Confidence, source freshness, geography coverage, observed time, received time, publication time and simulation time should have distinct, predictable places.

Screenshots: [Public map](ux-review-2026-10-23/current-map.png), [phone map](ux-review-2026-10-23/current-map-mobile.png), [imagery](ux-review-2026-10-23/current-imagery.png), [Studio](ux-review-2026-10-23/current-studio.png), [operator view](ux-review-2026-10-23/current-exercise.png), [replay](ux-review-2026-10-23/current-replay.png), [Home](ux-review-2026-10-23/current-home.png).

## Information architecture

### One global shell

Keep one compact header: EMAD identity; **Public Context / Simulation** area switch; friendly workspace and environment name; scoped search; activity/jobs; account and role. Time basis and mode are persistent in the active workbench. The raw partition and technical IDs remain available in details, not ordinary navigation.

The switch changes the product area and requests the authorized resource. It never grants access, merges partitions or promotes a public track to a controlled actor. Changing identity, resource, run or truth mode retains the current abort, query-client clearing, socket reauthorization and object-URL cleanup behavior.

For an account with a single job, land on its permitted work surface: analyst Explore, reviewer Assessment queue, operator assigned exercises, author Projects/last scene, evaluator authorized evaluation records. Multi-role users resume a saved authorized area; first use offers a compact workspace chooser. Do not infer missing grants or choose truth by default.

### Public Context navigation

- **Explore:** map, layer catalogue, public observations, external tracks, station weather, hazard reports and evidence inspection.
- **Imagery:** scene discovery, retained imagery, before/after comparison, quality and reproducible analysis.
- **Assessments:** drafts, review queue, decision history and published assessments, with development tests clearly separated.
- **Context packages:** geographic/reference selection, reviewed findings, permissions checks, immutable export previews and package versions.

Weather and moving tracks become first-class layer families inside Explore, each with a useful result list and inspector. Existing dedicated routes can remain as deep links and accessible full-list views. They no longer require the user to leave the map to understand their location.

### Simulation navigation

- **Projects:** available scene/environment combinations, recent work and context-package entry points. Initially a lightweight grouping of existing immutable scene versions, not a mandatory new project domain.
- **Studio:** scene composition, asset library, actor properties, route authoring, sensor presets and validation.
- **Exercises:** run library, runtime state, permitted synthetic perception, controlled tasking and outcomes.
- **Replay:** recording library and playback. Authorized evaluators can enter a visibly separate evaluation route from a recording; operators see released reports only.

Assets/templates/environments are Studio panels and full-library destinations. Incidents, tasks, missions, alerts and audit sit under the selected exercise or relevant scoped utility. Existing board, geofence/area rules, classifications, evidence associations, telemetry and Systems diagnostics retain scoped destinations and their meaningful full-detail views. They are preserved, not discarded or promoted to a third product area.

**Sources & data** and **Platform status** are utilities: source admission, coverage, retrieval history, rights, jobs, workers, storage and backup status. Analysts see usable summaries; configuration and credentials remain privileged server operations. No secret input is exposed in a public map inspector.

## The shared workbench pattern

Desktop has four structural regions: a compact area/navigation shell; a collapsible left layer/library/result rail; a dominant map or 3D surface; a context-sensitive right inspector. A narrow bottom timeline or result tray appears only when the task needs it.

The left rail answers **what can I inspect?** The viewport answers **where is it and how does it relate?** The inspector answers **what is this, how reliable is it, and what can I do?** The timeline answers **when and under which clock?** Each control has one owner.

Initial sizing proposal, to be tested rather than treated as a universal specification: 48–56px global header, 36–44px local toolbar, 240–280px left rail, 300–340px inspector, a 64–112px optional timeline. At 1440px the center should remain at least about 700px wide with both rails open. At narrower laptop sizes, opening the inspector collapses the library automatically, with an explicit restore action. The main surface must not be pushed beneath a page of forms. Target at least 65% of available vertical workspace for the map/scene after the necessary shell and timeline; no permanent exercise widgets on Public Context.

Selection synchronizes map/scene, list, inspector and deep link. Hover is a preview; click/tap/keyboard commits selection. Escape dismisses transient UI and restores focus without discarding unsaved work. A selected report's evidence remains selected when its provenance tab opens. Picking on a drag must not accidentally change selection. Lists and numeric controls provide alternatives to precision viewport interaction.

## Screen designs and workflows

### 1. Explore: geography and evidence in one place

First use opens a **Saudi overview** with country boundary, coastline, major routes and important places, plus a prominent **Jeddah detailed environment** bookmark/coverage outline. The detailed view opens the installed bounded area without pretending equivalent detail exists countrywide.

Layer groups are **Reference geography**, **Public observations**, **Moving tracks**, **Weather & reports**, **Imagery** and **Assessments**. Use meaningful geometries and styles: subdued reference polygons and roads; distinct point shapes for reported phenomena; dashed derived areas; hatched measurement/uncertainty footprints; reviewed finding outlines. A coverage rectangle is never drawn as a physical event extent. External aircraft/vessel symbols never expose command actions.

At national scale simplify reference geometry and aggregate public points only when aggregation is supported by a truthful bounded query. At city scale show major roads and places; at local scale show admitted building/road detail. Scale thresholds are chosen with the actual data; disabled layers say **Zoom in to see buildings**. Do not draw every stored vertex at every zoom or restyle all reference datasets identically.

Map and result list share the same region/time/source filters. Users can choose **Search this area** or explicitly enable follow-map filtering; dragging alone should not silently redefine an investigation. **Saved region**, **Draw area** and coordinate entry are equivalent paths. Coordinate entry remains available as Advanced. Sources outside the viewport appear as coverage, not empty marks or global-feed claims.

Primary time controls are an observed-date range with clear **Current window / Historical** selection and a visible timezone. Advanced history offers **What EMAD had published by…** and **Provider-effective by…**, retaining the existing bitemporal query meanings. Reference versions have their own validity and do not time-travel with event filters. Live transports can update data inside a current window; historical mode never silently resumes live data.

Inspector tabs: **Summary / Evidence / History**. Summary contains type, location, observed time, provider and uncertainty. Evidence contains producer → processor/catalogue → retained record → derived finding, rights, exact source and downloads. History shows revisions/withdrawals, received and published times. Raw JSON, full hashes and credentials references are absent from the default summary; hashes and exact retained bytes remain inspectable/exportable. FIRMS remains a thermal anomaly, not a confirmed fire or perimeter. Station weather is a station sample, not an interpolated field.

A compact source strip always indicates coverage limitations affecting the current view. Clicking opens the Sources tab with source name, collection, admitted AOI outline, last attempt, last completed window, next attempt, records returned, quota if reported and next action. It distinguishes **retained historical data**, **complete bounded zero result**, **initial backfill**, **stale**, **temporarily unavailable**, **credentials/permission needed** and **no admitted coverage**. Configured credentials are an independent state. The map remains useful when a provider fails.

Pagination/budgets remain honest: map/list counts name **matching / loaded / selected** separately; a 2,000-feature cap or 100-report page is visible beside the layer/result count. A next-page operation does not imply exhaustive map coverage. Server-side counts/aggregation or deterministic viewport subdivision may be required before replacing the current page model.

### 2. Imagery: a comparison workbench

Enter with an AOI from Explore or the installed Jeddah bookmark. The viewport is immediately visible; a left scene rail shows date, footprint, producer/collection, scene cloud cover and **Catalogue only / Capturing / Retained / Ready for analysis**. Thumbnail availability must be established; never imply that a catalogue record means its full raster is stored.

A persistent task strip provides **Find scenes → Retain inputs → Compare → Analyze → Draft assessment**. Each stage is a view over the existing capture, preparation and analysis jobs, with durable IDs under details. Jobs keep running when the user navigates away. The activity tray exposes progress, retry, lease/worker problems, rights expiry and cancellation where supported.

Before/after slots use scene thumbnails and clear acquisition dates. Disable incompatible pairs before submission and explain tile/orbit/baseline/grid/collection preparation constraints. Distinguish original-product preparation from Collection 1 scale/offset. Default to retained compatible inputs, not the newest catalogue record without pixels.

Comparison supports synchronized side-by-side and swipe views, reset/fit controls and keyboard-accessible slider alternatives. Quality overlay is adjacent to the image, with a readable legend. Avoid flicker/autoplay by default. Put date, source/collection, processing method, resolution/grid and masked fraction beside the evidence.

Analysis result displays the actual change footprint on the comparison map, a diverging legend centered at zero, transparent/hatched masked cells, threshold and valid denominator. For the installed comparison: **147 of 14,635 common-clear cells meet ΔNDVI ≤ −0.2**. This is approximately 1.00% of common-clear cells; a cell-count percentage is not automatically an area percentage on the angular grid. Show 1,749 masked cells separately. Negative NDVI change does not prove vegetation loss or its cause. Legitimate zero change stays a result, not a failure. A clear **Draft assessment from this result** action carries exact retained evidence IDs.

### 3. Assessments: a review desk

The queue separates **My drafts**, **Needs review**, **Published** and **Development records**. Priority reflects explicit workflow state and dates, not invented AI urgency. Selecting an item opens the human-readable claim, evidence, method, analyst rationale, uncertainties and revision diff. An optional map/image pane preserves spatial context.

A reviewer sees who authored the frozen revision, their registered human identity and the current decision state. Same-human aliases and service accounts cannot review; explain the constraint before the button is pressed while retaining server enforcement. Different development accounts prove account separation only. The real proposal remains pending until an actual authorized human reviews it.

Review, rejection and publication controls are grouped with required rationale/confidence fields and an explicit statement of the action's effect. Do not publish by advancing an imagery stepper. Review conflicts preserve typed comments and reload the latest immutable revision. Full append-only history and historical identity-resolution warnings remain available. A report's extracted claim, AI/model score, source quality and reviewed assessment confidence are visibly distinct.

### 4. Context packages: deliberate handoff to Simulation

Launch **Prepare simulation context** from a saved AOI, reference set or reviewed finding. A guided drawer/full page stages region → included versions → rights and review checks → export preview → immutable package. Show the expected geographic coverage and whether retained raster/geometry bytes are actually available.

Preview distinguishes licensed geography, dated imagery, reviewed assessments and explicitly chosen input assumptions. Current public tracks are not selectable as controllable actors. Explain excluded/unreviewed claims and missing rights inline, with a direct return to the relevant review or dataset. A package can be prepared from eligible geography without fabricating an assessment requirement for every reference dataset.

After successful export, **Open in Studio** carries the exact package version into a draft; it does not start an exercise. Preparation and heavyweight mesh/terrain conversion stay in workers. Show the pinned environment's surface-model, datum, estimated building-height and mixed-date limitations in a compact fidelity summary with detailed lineage available.

### 5. Projects and assets: visual starting points

Replace slogans/counter walls with **Continue Jeddah**, **Explore Saudi context**, **Inspect retained imagery**, **Review pending assessment** and **Open recording** where those actions are authorized. Show thumbnails, human-readable names, dates and readiness. Acceptance scenes and published development records have a distinct **Development/test** tag and are excluded from normal default selection; preserve them and provide an explicit show-tests filter.

Studio's library separates Models, Environments, Actor templates and Sensor presets. Model tiles show a lazy-loaded thumbnail, class, dimensions, license, validation and fidelity. Clicking opens its details; **Place in scene** uses the selected template and has an accessible form alternative. Upload/admission and full import diagnostics sit in a focused import flow. Full hashes, source files and build provenance remain in technical details.

### 6. Studio: a spatial editor

The viewport owns the screen. Left rail: scene hierarchy and asset library. Right inspector: selected environment/actor/template/waypoint properties. Bottom: spawn/despawn and authored event timeline. Header: scene name, draft versus frozen version, validation, Save/Freeze and Start.

Selecting an actor in the hierarchy, viewport or list selects the same inspector. Add explicit Select, Place, Move, Rotate and Route tools; display an active-tool label and Escape path. A 2D geographic view shares selection with 3D. Waypoint editing works with scene handles, insert/remove/reorder actions and a numeric list; provide click/keyboard alternatives to drag. Preserve exact object dimensions; a selected icon/outline and focus camera makes a small UAV findable without enlarging it to urban scale.

Altitude inputs expose units and **WGS84 ellipsoid** datum. The current backend accepts ellipsoid heights; until a tested conversion contract exists, MSL/AGL entry is disabled with a concise explanation. A ground-placement convenience action must use validated transformed surface values and label their surface-model meaning. Never reinterpret an unknown height or implement an unstated MSL/AGL conversion in the UI.

Keep draft edits separate from the pinned frozen scene. Unsaved changes are visible; Start uses a specified frozen version, with **Freeze changes and start** only if the combined behavior is clearly described. A persisted draft/autosave and undo history are new work if added; do not claim the current immutable scene API already stores drafts. The first increment can use explicit local drafts and Freeze, with clear recovery limitations.

Preflight explains scene limits, available runtime, storage/readiness, observation budget, environment/model integrity and actor errors in human terms. Link errors to the object and field. Keep existing limits: 16 actors, eight models, one-hour virtual duration and admitted synthetic-sample budgets. Runtime build hashes are automatic. Advanced settings hold seed, epoch, runtime build details and clock scale.

### 7. Exercises: operator perception and controlled work

After Start, show a clearly named run and **Queued → Starting → Running / Paused → Stopping → Stopped / Completed / Failed** as supported by the actual runtime states; never invent progress percentages. Keep status and Pause/Resume/Stop adjacent to the viewport, with read-only state when permissions do not allow control.

The ordinary operator surface loads only sanitized environment geometry, perceived contacts and permitted controlled-asset telemetry. It never downloads the authoring scene and hides parts with CSS. Known asset position and uncertain/delayed contacts use different symbols; unknown altitude stays **Unknown**, with an explicitly labeled surface projection or 2D symbol rather than a precise aerial model. Uncertainty ellipses must come from data or a declared representation, not decorative circles.

The right inspector provides contact evidence or permitted asset capabilities. A controlled asset can receive **virtual tasking** such as HOLD/RESUME_ROUTE through a small form; display Sent/Acknowledged/Completed/Failed and the observed result. Correlated public aircraft or vessels never receive these actions. Missions, task queues, incidents and alerts are exercise-context panels/full views. Virtual time and wall-clock receipt are named separately. Socket connection, run progress and the age of the latest perceived state each have their own status.

### 8. Replay and evaluation: one timeline, separate authority

Replay opens the retained scene/environment and spatial perception immediately, with transport docked below: play/pause, seek, speed, previous/next event, virtual time and event category lanes. Scene/model versions and recording integrity remain pinned. Event selection selects the relevant contact/task and inspector. Loading a historical frame shows a clear loading indicator and retains the previous labeled frame until replacement; no interpolation is presented as a new observation.

Archive status, retention, retry and integrity diagnosis live in a recording-details panel. The recording library needs a scoped collection read model if no existing authorized list can provide it. Playback never runs a simulator, sends a task, or mutates exercise state. A stopped run is not shown as running just because its websocket is connected.

**Evaluate recording** is a separate route/workspace session for explicitly authorized evaluators. It requires purpose and authorization before truth is fetched. Truth/perception split view or overlay has clear symbols, time alignment, missed detections, delay and position error, plus fidelity limitations. Operator and evaluator pages must not share truth-bearing queries, object URLs or retained store data. Exiting evaluation clears those resources; direct links reauthorize. Released reports expose only approved sanitized metrics, never hidden actors, routes, sensors or spatial truth. Historical restored evidence is not mistaken for a current run.

## Visual language

Use a neutral graphite shell, light or neutral evidence panels and carefully subdued cartographic backgrounds. A dark map/3D mode and a light reading mode should use the same semantic roles. Teal identifies public selection, a restrained secondary accent identifies simulation mode, amber marks partial/stale, red marks failure and green marks a specific successful/ready state. Never use color as the only distinction. Privileged truth needs a persistent textual badge and separate context, not just a different palette.

Use the existing IBM Plex Sans for interface text, a measured size scale around 13–14px body and 16–24px headings, and IBM Plex Mono only for coordinates, time, units and optional identifiers. Avoid all-caps navigation everywhere, oversized slogans, low-contrast tinted fields, repeated nested cards and paragraphs that precede every work surface. Set neutral dividers and a stable spacing scale. Treat long Arabic/English place names and future RTL layout as design constraints; a complete Arabic localization is separate work, not implied by this redesign.

Legend roles are stable: reference line/polygon, reported observation, derived/uncertain extent, external track, reviewed finding, controlled synthetic asset, perceived synthetic contact and privileged truth. Data type, status and selection are separate visual channels. Do not color by combat affiliation or imply threat classification from a feed type.

## Responsive and accessible behavior

Desktop reference: 1440 × 1000; laptop: 1280 × 800; phone portrait: 390 × 844 plus 360px minimum width; phone landscape: 844 × 390. These are design/test targets, not claims of completed verification.

Phone opens with map/scene visible below one compact bar. Layers/results and inspector become dismissible drawers or bottom sheets; they do not become paragraphs stacked above the surface. A compact preview sheet leaves the map visible. An explicit full-screen detail/editor mode has Close/Apply/Cancel and restores the prior selection, viewport and focus. Bottom navigation exposes the current area's tasks, not both pillars' full trees. Landscape uses compact side sheets and a narrow timeline; rotation is optional.

Essential controls use 44px touch targets where practical. Provide click/keyboard alternatives for map pan, route editing, layer ordering and compare dragging. Arrow keys and numeric fields support precision; do not steal text-input shortcuts for global navigation. Provide semantic lists for evidence/contact selection, visible focus, text status, sufficient contrast, labeled sliders, dialog focus management and accessible alternatives to WebGL-only information. Follow WCAG 2.2 AA as an acceptance target, including focus not obscured, minimum target sizing/spacing and alternatives to dragging; measure compliance rather than asserting it from screenshots.

Keep the current labeled evidence visible during reconnection with last update and stale state. On permission revocation, clear forbidden content immediately rather than retaining it as stale. Pause offscreen/hidden rendering, respect reduced motion, avoid autoplay, cap pixel ratio and offer 2D/list fallbacks for unavailable graphics. Mobile data saver can load lower-resolution previews and fewer layers. No camera, geolocation, orientation, notifications or AR permissions are needed for the proposed core journeys.

## State and implementation fit

Retain React/TypeScript, TanStack Router/Query, Zustand, Tailwind, existing UI primitives, MapLibre and Three.js. Reuse working API modules, renderers, validated asset loaders and WebSocket policies. Avoid replacing engines or rebuilding ingestion to achieve a layout change.

One active map/scene renderer per main workbench is the default; create two synchronized map instances only for an explicit comparison, with teardown on mode change. Do not load a WebGL renderer for every invisible model thumbnail. Use static generated/retained thumbnails where possible, virtualize long lists, cancel old viewport queries and expose bounded results. Heavy raster, mesh and simulation work remains outside the synchronous API.

Route/search state should include permitted region/AOI, view mode, date range/time basis, selected stable entity/revision, active layers/versions, imagery pair/job and run/recording time. Keep hover, animation frame, drag intermediate state, credentials and unsaved assessment text out of URLs. Preserve existing deep links and endpoints; add redirects only with explicit route migration tests. Saved preferences are scoped by principal, workspace, environment, partition and run where appropriate. A shared link never bypasses authorization. Back/Forward restores meaningful selection and filter state without re-submitting a job.

### Backend/read-model work the design must account for

Most proposed screens can compose existing endpoints. The following require deliberate implementation decisions; they are not already delivered just because they appear in the concept:

- A safe exercise/recording library with friendly names and per-run grants, usable by operators without authoring or truth access. Existing author run lists and per-recording endpoints may not satisfy that contract.
- Cross-domain scoped search, saved views, user-owned project organization and server-persisted drafts if included. Start with local scoped preferences and small permitted lists where appropriate; do not create a large platform prematurely.
- A coherent aggregate job/status presentation over capture, preparation, analysis, validation, runtime and archiving. It may initially be a typed frontend adapter; a shared queue facade must not erase domain state or authorization.
- Honest viewport counts, clustering and pagination when current endpoints only return a bounded subset. Do not imply global completeness through client-side clustering of a page.
- Server-derived review eligibility/reasons and context-export prerequisites if the existing session/revision data cannot explain disabled actions reliably.
- Persisted development/test classification for acceptance content if no existing provenance field can identify it reliably. Do not filter by a fragile display-name substring.
- Transactional draft editing, autosave/undo and alternate altitude conversion only if explicitly implemented and verified; these are new capabilities, not cosmetic promises.

## Delivery order and visible acceptance

Deliver vertical journeys, each with desktop/narrow screens and real installed data. This is a proposed implementation sequence, not a second declaration that the recovery is complete.

1. **UX01 — Shell and entry points.** One area switch, local navigation, friendly scope/role/time, compact actionable Home, truthful separated connection/source/run indicators. Preserve all routes and permission boundaries. Exit: an analyst opens installed Saudi/Jeddah content from normal login without a setup note; switching area/account clears old scoped data.
2. **UX02 — Public Explore.** Viewport-first map, grouped scale-aware layers, Jeddah bookmark, coordinated results/inspector, time controls and coverage status. Exit: select a real report from either map or list, inspect retained evidence/history, show only a chosen region/time, and explain a partial/zero-result feed without leaving the workspace.
3. **UX03 — Imagery analysis.** Scene rail, retained/available distinction, AOI tools, comparison and quality controls, durable job tray and mapped results. Exit: inspect the actual retained March pair, reproduce/select the installed analysis, see the valid denominator and limitations, and draft from exact evidence without prepared-JSON upload in the normal path.
4. **UX04 — Review and context handoff.** Dedicated queue, author/reviewer identity, revision diff, rationale/confidence and export builder. Exit: independent-account development review works; same-human/service review fails; real assessment stays pending actual human review; eligible retained geography exports and opens in Studio with an immutable version.
5. **UX05 — Studio and library.** Scene viewport, hierarchy/library, linked actor inspector, route tools, explicit datum, version/validation/preflight and automatic Start. Exit: select/place a recognizable model, edit a waypoint with both pointer and form, freeze a scene and start a bounded run without manual hashes; normal entry excludes acceptance fixtures while keeping them reachable.
6. **UX06 — Exercise operations.** Safe perception viewport, run-state controls, contact/controlled-asset distinction, tasking/outcomes and diagnostics. Exit: movement and pause are visible, virtual HOLD/RESUME_ROUTE produces acknowledged outcomes, and an ordinary operator receives no scene truth through endpoints, sockets, caches or asset downloads.
7. **UX07 — Recording and evaluation.** Scoped library, immediate replay viewport, event timeline, archive/recovery details and isolated evaluator route. Exit: reopen a retained recording after restart, seek to an event without dispatching work, and compare privileged truth only after purpose/authorization; operator released reports remain sanitized.
8. **UX08 — Cross-journey polish and acceptance.** Test real installed desktop, laptop, portrait and landscape flows, keyboard/focus, contrast, slow loading, stale/partial/zero states, worker/storage errors, session expiry and reconnection. Capture matched before/after screenshots and a narrated task walkthrough. Update the delivery ledger only after the corresponding experience succeeds.

For every increment, maintain a capability checklist covering existing routes/modules so consolidation does not silently remove working features. Prefer one completed analyst or author/operator journey over touching every page superficially in one commit. Keep destructive actions explicit; ordinary selection, navigation and reversible edits should not trigger approval dialogs.

## Acceptance measures

Proposed usability targets require validation with real users; they are not measured improvements yet:

- First screen shows the main spatial evidence, correct mode, region and time without page scrolling at desktop/laptop targets.
- From normal login, installed Saudi overview, Jeddah detailed scene and retained imagery each have a visible starting action. Typical tasks need at most one detail-opening action from a selected item.
- A user can answer where/when/from whom/how current/what limitation for a selected item without raw JSON or a developer guide.
- An empty regional query, an unavailable provider and a historical capture have visibly different explanations.
- All eight visible workflows above can be completed with existing permitted accounts; same-human review and truth-isolation tests still pass.
- Keyboard-only routes exist for selecting evidence, comparing imagery, editing a route, starting/pausing an authorized run, seeking replay and leaving privileged mode.
- Desktop and phone screenshots preserve the same source/date/uncertainty meaning; the map is reachable immediately and inspector controls can be closed without losing context.
- Measure time and missteps for representative analyst/reviewer/author/operator/evaluator tasks before and after. Screenshots, passing selectors and visual polish alone do not prove usability.

## Reference principles and boundaries

These are workflow references, not new dependencies or layouts to copy:

- [Carbon UI shell guidance](https://v10.carbondesignsystem.com/components/UI-shell-left-panel/usage/) supports separating persistent global structure from product navigation. EMAD's two-area architecture determines the actual navigation.
- [ArcGIS Pro imagery appearance](https://pro.arcgis.com/en/pro-app/3.6/help/data/imagery/raster-display-ribbon.htm) documents swipe comparison of overlapping rasters and scale-dependent visibility. EMAD still needs accessible alternatives, retained-input provenance and its own analysis constraints.
- [WCAG 2.2](https://www.w3.org/TR/WCAG22/) provides the accessibility acceptance basis; [W3C's changes summary](https://www.w3.org/WAI/standards-guidelines/wcag/new-in-22/) explains dragging alternatives, target size and focus visibility. The 44px preferred mobile target here is a design choice stronger than the general 24px AA minimum with its exceptions.

Concept images are illustrative layout references, not fabricated installed screenshots. Geographic textures/marks in those images are schematic. Exact evidence, labels, rights, values and uncertainty will remain real data-bound application elements. Desktop, portrait and landscape direction is pending user review. Production UI code has not been changed by this analysis.

## Visual concepts for review

- [Public Context desktop](ux-review-2026-10-23/concepts/public-context-desktop.png): map, grouped layers, source state and selected context.
- [Imagery desktop](ux-review-2026-10-23/concepts/imagery-desktop.png): comparison workbench, retained pair, quality and an exact-result handoff.
- [Simulation Studio desktop](ux-review-2026-10-23/concepts/simulation-studio-desktop.png): hierarchy, library, scene, actor inspector and version-aware start.
- [Phone portrait and landscape](ux-review-2026-10-23/concepts/responsive-concepts.png): map/imagery with sheets and synthetic recorded operator replay.

The [generation record](ux-review-2026-10-23/concepts/generation.json) retains prompts, corrective revisions and the pending review status. Corrections removed invented image dates/counts, corrected FIRMS to thermal detections and configured backfill, kept stale weather distinct from connectivity, and removed unsupported calibrated sensor/endurance/range implications. Artist-rendered preview geography, object scale, timestamps, icons and placement are schematic; the existing validated terrain, models and observation uncertainty remain authoritative for implementation. Selecting this layout would not authorize replacing retained geography with the schematic concept art or adding advanced simulation fidelity.
