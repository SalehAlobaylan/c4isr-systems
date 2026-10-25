# EMAD: a clean map with the full analytical workflow

Date: 23 October 2026. Status: refinement of the [UX redesign proposal](EMAD_UX_REDESIGN_PROPOSAL.md), following the user's request to reduce map obstruction while preserving its design and feature approach. This document changes the proposed default layout and interaction policy. It does not change the running application.

The earlier desktop concepts show the fully expanded analytical arrangement. They should not be interpreted as the normal opening state. The default should be a largely unobstructed map; deeper tools should appear because the user chooses a task.

## Recommendation

Keep the two product areas, the coherent shell, the spatial inspector, the dedicated imagery/review/authoring workflows and the trust boundaries. Replace always-open side panels with a small set of predictable states:

1. **Map:** the ordinary reading/navigating state. All large panels are closed. A compact tool rail makes the capabilities visible.
2. **Inspect:** one selected item and an explicitly opened, docked inspector. The map resizes into the remaining space rather than sitting behind a large opaque panel.
3. **Pinned investigation:** the user deliberately keeps a useful tool open. A second panel is allowed only when the remaining map stays usable.
4. **Dedicated work surface:** imagery comparison, review, context export or scene editing gets its own full workspace when the task needs substantial content.

Systems carries source/retrieval coverage, processing, workers, storage and diagnostics. It complements the map and dedicated workspaces. Putting all evidence and authoring work into Systems would make the map cleaner at the cost of hiding the product's actual purpose under an infrastructure label.

The [interactive layout study](EMAD_MAP_LAYOUT_STUDY.html) demonstrates these states with schematic geography. It is an interaction illustration, not an application build, live source display or new geographic dataset.

## What the repository already supports, and what is missing

The existing `stores/layout.ts` has separate persisted toggles for the picture rail, overlays, layers and telemetry tray. The picture rail currently defaults open at 380px. The persisted store is named `aegis.c2.layout.v1` and is shared rather than scoped to a principal/resource. `TopBar.tsx` documents individual collapse controls. Several pieces of the requested behavior therefore already have working foundations.

However, independent booleans permit combinations that compete for the same viewport. They do not express temporary inspection versus deliberate pinning. In `index.css`, the picture rail is positioned absolutely over the map. Collapsing it helps, but opening it still conceals geography. `PublicContextPanel` remains an above-map scrolling block. These are structural layout problems rather than a missing minimize icon.

`SystemsPage` currently composes the legacy source → observation → track → asset → alert/incident → command subsystems. It does not yet provide the proposed public-source coverage, imagery job, model validation, runtime and recording organization. Reusing the route and its useful summaries is sensible; calling the current page the new Systems workspace would not deliver the intended design.

`usePublicContext` already serializes public filters into the route, but selected evidence, layer IDs, dataset selection and viewport state are local hook state. Preserving these through a move to another work surface needs explicit work. The current default selects all available vector datasets up to its limit; a cleaner layout also needs a more selective and scale-aware layer policy.

## Navigation: preserve the previous direction

Use one compact global header for product area, environment, identity and connection. Keep one local navigation row. Do not add another navigation band for every new concept.

Public Context retains **Map/Explore · Imagery · Assessments · Context packages**, with **Systems** as a secondary destination or utility. Simulation retains **Projects · Studio · Exercises · Replay**, with its relevant Systems detail available through the same utility. The exact label Map versus Explore is a copy decision; it must consistently name the geographic entry point.

There is no need for a new generic Workbench tab in addition to these existing task destinations. Imagery, Assessments and Studio are already the specialized workbenches. A full-record view can also open from an inspector without inventing another major product module.

For a wide desktop, Systems can be a quiet text tab at the end of the local row. On smaller screens it can sit in the existing overflow/utility menu. A coverage or worker badge deep-links directly to the relevant Systems section, so it remains discoverable without occupying a permanent map panel.

## What remains visible on a clean map

The useful minimum is:

- Current product/mode, friendly environment, selected region and current versus historical time basis.
- A compact Layers, Search and Filters entry point; secondary map tools live in a small Tools menu.
- A legible contextual coverage/freshness indicator, separate from transport connectivity.
- Zoom/reset controls, scale/orientation where meaningful, and required map/source attribution.
- A small active-selection or applied-filter indication when it matters, with a clear reset path.
- Essential run state and virtual time in Exercise views; recorded/read-only state in Replay.

The screen does not need a permanent report list, source inventory, actor properties, job log, telemetry table, full legend, empty inspector or installation statistics. Those remain reachable without occupying the main visual field.

A compact line such as **Jeddah · Historical: March 2025 · Coverage partial** carries necessary meaning. It is not a claim that a current backfill reconstructs historical coverage. Detailed source status must still state which time basis it describes.

Do not turn the compact line into ten filter chips. Show two or three significant values, then an explicit **Filters (n)** or **Details** control. The popup exposes the complete applied filter set and clear/reset actions. A hidden applied filter must never become an invisible cause of an empty map.

## Where each feature belongs

- **Layer visibility, opacity and scale rules:** an on-demand Layers tool, with category/legend tabs in the same panel. Full dataset versions/rights belong in the source/dataset record.
- **Place search and evidence search:** a compact search control and temporary result list. Picking a result returns attention to the map. Search is always constrained by the current permissions.
- **Region, time and evidence filters:** a small filter editor; full history semantics appear on demand. The applied region/time remain visible after it closes.
- **Selected observation/contact/reference:** compact preview first, inspector on explicit Details, full record for extensive evidence/revision history. No empty inspector takes space on first load.
- **Sources and coverage:** a short status/coverage preview from the map; full admitted AOI, retrieval history, freshness, pacing, credential state and rights in Systems → Sources & coverage.
- **Evidence lists, weather records and external tracks:** optional result tray or full list linked to the map. Open it for comparison/triage, not automatically on every visit.
- **Imagery scene catalogue, before/after inputs and quality:** Imagery workspace. Compare/swipe/quality controls remain adjacent to the imagery, rather than scattered in Systems.
- **NDVI result summary:** an optional map overlay and selected-result inspector. Processing history and failures link to Systems → Processing. The conclusion and review belong in Assessments.
- **Assessment writing/review:** dedicated Assessment desk, with selected evidence/map preview when useful. Do not shrink a long review form into a narrow map drawer.
- **Context export:** focused Context packages flow, carrying the region and evidence selection. Its immutable preview has room to explain rights and omissions.
- **Models, scene hierarchy, actor properties and waypoints:** Studio library/inspector, minimized when not editing. Their full forms remain in Studio, with a maximized viewport available for scene inspection.
- **Exercise tasking:** a selected controlled-asset inspector or task queue, available only in that run. A contact summary is not an invitation to show hidden actor properties.
- **Recording playback:** spatial viewport and compact transport dock. The event list is optional. Retention, integrity, archiving and recovery belong in recording detail/Systems.
- **Truth/perception evaluation:** separately authorized evaluation workspace. Truth is never a minimized ordinary-map panel loaded in advance.
- **Worker/storage/backup/platform diagnostics:** Systems, visible only as appropriate to the role. No private credentials, logs containing secrets or broad configuration privileges in public tools.

## Panel behavior: a consistent contract

### Default

Start with no large panel open and no bottom result tray. Show a meaningful region, visible layer defaults and recognizable tool entry points. The clean state must be useful, not an empty map waiting for configuration.

### Preview

A single feature selection produces a small preview with a readable name/type, date/source or reference identity, one important limitation and **Details / Open record**. A proposed desktop budget is roughly 280–340px wide and 90–140px high. Position it in a predictable safe corner rather than directly on top of the selected marker or route. Only one preview is open. A dense cluster first offers disambiguation rather than a pile of popups.

Keyboard/list selection gets the same preview; it is not hover-only. The map may highlight the selected item, but a single click does not automatically create two full-height sidebars. If an inspector is already intentionally open, the next selected item updates it in place.

### Inspect

Details opens a docked inspector. Docking allocates actual layout space and resizes the map; it does not put a nearly full-height rectangle over an unchanged map. The map camera keeps the selected geography in the visible safe area. Do not re-fit the entire dataset or change scale unexpectedly every time a panel opens.

Keep the ordinary inspector focused on summary/evidence/history. Long forms and multi-record comparison have a visible **Open full workspace** action. Selection, applied filters, camera and return target carry forward.

### One expanded panel by default

Layers, Search, Filters and Results use one tool region. Opening a different unpinned tool replaces the previous tool; it does not accumulate another panel. Their control/draft state can be preserved so reopening is not a restart.

Normally an expanded inspector and an expanded tool panel also share a single-panel budget. Opening Layers can minimize an unpinned inspector while preserving selection. Selecting Details can minimize the unpinned Layers tool. The user sees a consistent active-tool label, not a randomly changing arrangement.

Do not automatically dismiss an editor and discard input. Keep unapplied filter drafts when minimized, distinguish them from applied filters, and move substantial editable work into its dedicated workspace. Confirmation is needed only where an explicit action would actually discard unsaved work.

### Pin

Pin means **keep this tool available while I use another tool**. It is deliberate and visible. It does not change data authority or make the panel into a new browser window.

Allow two docked panels only when sufficient map width remains. A starting desktop minimum is around 720px of usable center width; test it with actual label density and 3D tools. A 1280px laptop should usually allow one panel, not squeeze the center between two. If a second panel would exceed the space budget, offer to switch the active panel or open the full workspace. Explain this at the point of pinning rather than silently closing a pinned panel.

### Minimize, close, clear and clean view

These have different effects:

- **Minimize:** hide the panel's body while preserving its local tab, scroll, draft and selection. Its existing rail/tool button signals that it can be restored; do not add a large row of window tabs.
- **Close:** dismiss that inspection/tool surface. Applied filters, enabled map layers and the selected entity do not disappear as a side effect. Explicit Clear selection / Reset filters actions own those changes.
- **Clean map:** temporarily hide nonessential panels, trays and previews. Keep product identity, time, essential coverage, run/recording state and attribution. A single **Restore workspace** action reinstates the user's previous panel arrangement.
- **Reset layout:** restore a sensible clean default. It does not reset filters, remove content, clear drafts, switch resource or erase camera bookmarks.

Avoid shrinking panels into many floating miniature windows. Fixed rail/tool locations and a single restore action are easier to discover, reach by keyboard and maintain on narrow screens.

## Systems as a useful destination

Rework Systems around the current installation, reusing existing status implementations:

1. **Sources & coverage:** admitted providers, retained reference/imagery collections, configured versus fresh, bounded coverage, last successful query, partial/backfill state, reported quotas, rights and permission prerequisites.
2. **Processing:** imagery captures/preparation/analysis, asset validation, context preparation, recording/archiving; exact domain job states, attempt history and supported recovery actions.
3. **Runtime & storage:** admitted runtime workers, active/queued run state within grants, worker heartbeats, private object storage and relevant readiness. Backup/restore details require the appropriate role.
4. **Activity & audit:** scoped ingestion, job, access and exercise actions, with existing full audit destinations preserved. An authorized operator's events do not expose another run or privileged evaluation purpose/contents.

Use searchable lists, meaningful statuses, selected-record detail and links back to the affected content. A giant grid repeating every count is not sufficient. Preserve commands/alerts/incidents as exercise-specific workflows rather than turning them into universal health cards for public analysts.

A source problem should be two actions away at most: source/coverage badge → relevant source detail. Getting back to the map restores region, time, selection and camera. Current source retrieval diagnostics remain explicitly current even when the map is looking at March 2025 evidence.

## Examples of complete journeys

### Quick regional inspection

Open Saudi overview → choose Jeddah bookmark → turn on a layer in Layers → close the tool → inspect the unobstructed map. Click a feature for a compact preview; open Details only if needed. No catalogue, telemetry or worker panel appears automatically.

### Investigate an observation

Select an actual report → preview source/time → Details for uncertainty and evidence → Open record for original/revisions → return to the same map selection. A reference feature follows the same selection pattern but is labeled reference, not observation. Zero-result/partial/unavailable cases retain different explanations.

### Investigate a provider gap

See **Coverage partial** → Sources detail shows the admitted region, initial backfill or unavailable retrieval → inspect job/retrieval history → return to the unchanged map. A credential gate cannot be disguised by hiding its badge or filling the map with illustrative observations.

### Compare imagery and propose a finding

Select an AOI → open Imagery with that AOI → compare the retained pair and quality → inspect mapped results → draft assessment in Assessments. The long scene list and review form never occupy the ordinary Explore map. Once a result is complete, the user can return to a clean map with its chosen analysis overlay.

### Author, watch and operate an exercise

Studio opens with the selected scene and compact scene/library affordances. An author opens the hierarchy or actor inspector for editing; maximizes the viewport to inspect placement; restores editing tools; freezes and starts a specified version. The operator opens a distinct synthetic perception view with run/virtual-time controls and on-demand contacts/tasking. Authoring routes do not remain hidden in the operator's browser data.

### Replay and evaluate

Replay keeps play/seek/time immediately accessible because they are part of the task. The event list and archive details are optional. Evaluation opens a separate authorized workspace rather than minimizing a truth panel into ordinary replay. Returning to an operator session clears forbidden caches and does not restore a previously pinned evaluator panel.

## Clean geography needs more than closed panels

Keep the basemap subdued and distinguish reference from reported/derived/reviewed content. Default to a small useful layer set, not all 23 vector versions. National context, major roads, local buildings and labels use appropriate scale/coverage behavior. Duplicate versions of a reference layer should not all render by default.

Keep label priority stable: selected item, meaningful places and essential task labels before low-priority reference names. Dense points may aggregate only with honest server/viewport counts; clustering a bounded page must not imply complete coverage. Uncertainty and derived extents remain meaningful, not decorative clutter. Preserve missing altitude and source-specific time/quality semantics.

Use a contextual legend for currently rendered families rather than a permanent catalogue of every possible symbol. A small expandable key can be sufficient, but an unknown symbol or critical uncertainty cannot depend solely on hover. Raster/imagery backgrounds retain their acquisition date and source even when all panels close.

## Laptop, phone and accessibility

Treat the panel budget as available space, not a device-name assumption. Above roughly 1440px, users may deliberately pin two panels if the center remains usable. At narrower widths, default to one. On phone portrait, keep the map above a compact preview; an expanded sheet is temporary and has an obvious Close/Apply return path. Avoid a full-height persistent sheet on load. Landscape can dock a compact side inspector where height permits.

Keyboard users can open Layers/Search/Filters, move through results, request Details, minimize/close and restore the workspace. Escape dismisses the top transient interaction first; it does not stop an exercise, clear filters or discard a draft. Pinned panels have explicit close controls; Clean map can temporarily hide them together. Map/scene precision also has list/numeric alternatives. Focus returns to the initiating control or selected feature's accessible list representation.

Do not rely on hidden edge-hover controls, tiny chevrons, unlabeled icons or auto-hiding navigation. Use small but legible labels/accessible names, visible focus and generous touch targets. Hide visual complexity while preserving discoverability.

## State and technical delivery

Represent active tool, selected entity, inspection state and pinned layout explicitly, rather than allowing many unrelated `open` booleans to form arbitrary combinations. Resolve competing tools through a small deterministic layout controller. A proposed state is active tool; selection; inspector closed/preview/docked; pinned tools; result-tray state; and a temporary clean-view restore snapshot. This is a design description, not a new API contract.

Keep layout preferences separate from investigation data. Scope layout preferences to principal and product/resource class so an evaluator's or author's arrangement does not become an ordinary operator's layout. Persist stable preferences; keep temporary previews/hover/drag and clean-view toggles session-local. Validate available tools on restore and remove inaccessible tool state after role changes. Existing `aegis.c2.layout.v1` preferences need a versioned migration or safe clean default, not accidental reuse.

URL/state should preserve the authorized selection/revision, applied region/time, evidence filters, selected layer versions, selected job/scene/run and meaningful camera/return location. Do not serialize credentials, truth payloads or full draft text into URLs. Returning from Systems or a full record must not depend on remounting a hook with all local defaults.

Notify MapLibre/Three.js when the actual viewport dimensions change. Keep camera continuity and safe-area padding distinct from a full fit-to-content operation; do not double-count dock widths as both container shrink and map padding. Hidden tabs/panels do not justify running extra renderers. Pause invisible rendering and cancel obsolete queries. Durable worker jobs continue independently; their completion updates an activity badge without opening a panel or moving the camera.

Preserve current abort/query/socket isolation across principals, environments, partitions and runs. In particular, do not keep an evaluator renderer or truth-bearing DOM mounted invisibly to make panel restoration faster. Permission revocation clears unauthorized data immediately; ordinary reconnection can retain last-known authorized data with a stale indicator.

## Refinement to the delivery sequence

1. Implement one shell and the clean-map default together with the layout controller, minimize/restore behavior and honest persistent context.
2. Wire scale-aware layers, search, filters and selection through the single-tool/inspector policy. Carry context across routes before moving content out of the map.
3. Rework Systems into scoped sources, processing, runtime/storage and activity detail. Wire direct contextual links in both directions.
4. Build the previously proposed Imagery, Assessment, Context, Studio, Exercise and Replay journeys in their appropriate workspaces. Use collapsible editing panels where useful, rather than imposing the same layout on every task.
5. Verify keyboard/mobile and failure states across the complete task paths. Capture the clean default as well as inspection, pinned investigation and full-workspace states.

## Acceptance targets

- Normal desktop Map opens with no expanded sidebars, result tray, empty inspector or above-map configuration form. Necessary scope/time/coverage and attribution remain legible.
- The map occupies at least about 90% of the content region in the default state, excluding the compact application header/navigation. Treat this as a design target to measure, not a current product claim. Essential overlays and their actual occluded footprint must be counted.
- One feature click provides a compact preview; one Details action opens a full inspector. Closing it does not clear the user's region, filters or selection.
- Opening a normal unpinned tool never creates a second competing expanded panel. Two pinned panels are an explicit choice subject to the center-width budget.
- Opening/minimizing/docking does not hide the selected point behind a panel, unexpectedly re-fit the entire map or change the investigation's AOI/time.
- Clean map / Restore workspace is reversible, keyboard-accessible and does not stop jobs, reset data controls or discard edits.
- Detailed source/job/evidence work is discoverable in one or two purposeful actions. A user does not need to guess that assessment review is inside Systems.
- A round trip to Imagery, a full record or Systems restores the same authorized map context. Historical source diagnostics do not masquerade as historical coverage reconstruction.
- New runtime/recording/truth boundaries and existing human-review safeguards remain intact regardless of which panels are minimized.
- Test all states at desktop, laptop, portrait and landscape sizes with real installed content. Good screenshots alone are not usability acceptance.

The intended result is a clean spatial surface for reading, a fast path for inspection, and enough room for sustained analytical work when the user asks for it. The earlier design's capabilities, semantic hierarchy and workflow direction remain; its fully expanded arrangement becomes an intentional working state rather than the default.

## Reference checks and study verification

The implementation detail around container changes should follow the [MapLibre Map API](https://maplibre.org/maplibre-gl-js/docs/API/classes/Map/#resize): the real canvas must be resized when its allocated container changes. This supports deliberate docking; it does not by itself preserve a selected point or investigation state.

Use navigation links for route destinations, and the [W3C tab pattern](https://www.w3.org/WAI/ARIA/apg/patterns/tabs/) for actual in-page tabs. That distinction matters for Back/Forward behavior, keyboard focus and panels that take time to load. Do not make slow network/worker work start merely because keyboard focus moves across a tab.

The [layout-study check record](clean-map-study/CHECK.json) records desktop/narrow/phone interaction checks, including docking without overlap, single-panel behavior, minimized layer/search state, deliberate pinning, clean/restore, Systems return context and absence of external network requests. It verifies the standalone illustration only. It is not acceptance evidence for the future application implementation. The actual map renderer and backend were not exercised in this check.
