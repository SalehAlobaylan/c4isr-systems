# EMAD workspace and grid design analysis

Date: 23 October 2026. Status: analysis after rejection of `EMAD_MAP_LAYOUT_STUDY.html`. That study is not an accepted design or an implementation specification. This analysis preserves the earlier useful visual/workflow direction, re-examines the role of grids and proposes a stronger information structure. No application UI changes are made here.

The subsequent [detailed UX/UI specification](EMAD_UX_UI_SPECIFICATION.md) develops this direction into proposed screen, interaction, data, layout and delivery requirements. Prefer its explicit sizing examples and verified API limitations over rough estimates in this earlier analysis; neither document claims final visual approval.

## What went wrong in the previous refinement

The previous proposal treated cleanliness mainly as hiding controls. It removed too much visible information and replaced an analytical product with a generic map plus drawers. The schematic illustration also lost the richer visual hierarchy, real-content previews and useful feature discovery of the earlier concepts. It could demonstrate closing panels but could not demonstrate why EMAD would be valuable to use.

It also reduced Systems to diagnostics and lists. That overlooked the value of a grid: seeing several related capabilities, available content and work requiring attention at the same time. A useful overview should not be replaced by administration screens merely to clear the map.

The correction is to design three complementary surfaces with explicit jobs:

- **Overview grid:** what content/capabilities are available, what changed and what should I open?
- **Spatial workspace:** where is the selected content, how does it relate geographically and over time?
- **Focused workbench:** how do I compare, author, review, export, operate or replay this particular object?

These are views inside the two existing product pillars, not three new product pillars. Their region, time, selection and authorization remain connected.

**Confirmed user decision:** keep and improve the full grid in Systems. Systems remains a first-class overview; the map gets a smaller expandable preview grid linked to it. The two surfaces share selection/data contracts without duplicating every Systems card over the map. This resolves the earlier ambiguity about relocating the full grid; the exact visual design remains to be developed.

## Evidence from the current installation

The [current Systems screenshot](grid-review/systems-current.png) and [measured observations](grid-review/systems-observations.json) were captured from the running application using an authorized analyst account and read-only browser navigation. This is a new snapshot, separate from the earlier data inventory. No ingestion, exercise, review or export action was submitted.

The current Systems screen has six detailed grid cells: sources, observation intake, track correlation, asset readiness, alerts/incidents and command dispatch. Its first row starts at y=354 in a 1440 × 1000 browser. The source inventory makes that row **835px high**, stretching neighboring cells despite their small amount of content. The next three cells begin at y=1205, below the first screen. The scrolling content is 1849px inside an 821px region.

This exposes several specific design problems:

1. The top six-item strip repeats the main figures of the six larger cells below. It consumes space without adding an independent comparison.
2. An unbounded inventory inside one cell dictates the height of adjacent cells and moves other modules below the fold.
3. The grid still follows the old operations pipeline. It does not give geography, retained imagery, analysis, assessments, environments, scene authoring and recordings suitable entry points.
4. Public analysts see zero controlled assets and commands, making a populated Public Context installation look inactive.
5. Source registrations, credential/permission state, retained reference inputs, historical evidence and current retrieval are summarized together. A source-level OFFLINE verdict can dominate the whole source card even when the issue is a permission gate or intentionally retained historical/reference data. This is a presentation/read-model concern, not a diagnosis that all data retrieval has failed.
6. Source names are heavily truncated. The inventory shows only a subset of registered sources; such previews need explicit shown/matching counts and a View all action.

The existing Board is a different grid: an entity matrix with asset/track columns and comparable state/position/freshness rows. It is useful for exercise comparisons. It should not be made into a public imagery/reference grid by pretending every geographic feature or satellite scene is a controlled asset.

The map already has Board chips and a collapsible telemetry tray. They provide useful implementation precedents for a compact dock, but their exercise vocabulary and record types do not provide the new Public Context preview contract automatically.

## The design principle: compact information, not invisible functionality

A map can be visually dominant while showing a useful preview strip. The aim should be low obstruction and high information value, not an arbitrary percentage of empty canvas. A mostly empty screen is not inherently usable.

The content visible beside the map should help with the current investigation: retained imagery for this region, a relevant report, a selected station, a reviewed/proposed finding, or a chosen reference dataset. Generic counters, ingestion internals, empty operational modules and repeated status banners do not earn the same space.

The strongest design uses the richer neutral/graphite shell, typography, source/date labels, thumbnail previews and deliberate state distinctions from the original proposal. The map's working arrangement changes; the product should not be visually reduced to the rejected schematic study.

## A full overview grid

Systems retains the full overview grid, with **Overview** as its normal entry. Source inventories, processing history and technical detail are drill-down destinations or secondary tabs, not a replacement for the overview.

Public Context could use six principal domains:

- **Geography:** Saudi overview and bounded Jeddah detail, reference families, coverage preview, installed/versioned state and an Open map action.
- **Imagery & analysis:** actual retained scene thumbnails/pair, acquisition dates, catalogue-versus-retained availability, latest applicable analysis and Open imagery.
- **Public observations:** normalized event types, selected region/time, representative report and source limitations. Raw retrieval records are not mislabeled as independent observations.
- **Moving tracks:** aircraft and vessels clearly separated, current/bounded/zero/unavailable/gated states, and Open tracks/map. No fictional traffic when a bounded query has zero records.
- **Weather & official reports:** a useful station/report preview, location and observed time, freshness and Open context. Station observations are not an inferred regional weather field.
- **Assessments & context:** real review work and available context packages, proposed/published/development distinctions, approval/rights prerequisites and deliberate handoff actions. Geographic context export must not require inventing an assessment for every reference layer.

Processing and platform health remain reachable and role-appropriate. They can form a compact supporting band or their own technical overview, rather than occupying every prominent card.

Simulation has a different role-dependent overview: environments/models, scenes/templates, admitted runtime/queue, permitted exercise perception/tasking, recordings and evaluation access. An operator's overview cannot fetch authoring scenes, hidden routes, sensor settings or evaluator truth to populate appealing previews.

Do not relocate the full grid to Home or reduce Systems to a diagnostics/list page. Home may provide starting points, while Systems owns the comprehensive grid. The map preview dock and focused workbenches complement that grid.

## A useful grid cell is not a large number

Each overview cell needs a bounded information hierarchy:

1. A clear capability/content name.
2. One relevant visual or readable content preview.
3. Two or three decision-relevant facts, including source/date/coverage where necessary.
4. A specific state or limitation, not an unqualified green/red summary.
5. One clear primary action and a secondary Open details action if useful.

For imagery, that means real thumbnails and dates, not only a scene count. For geography, a small coverage preview communicates the national-versus-local distinction. For a report, a short readable title, location and observed date beat an internal ID. For a scene, use a retained scene/model thumbnail, version and readiness rather than build hashes.

Use shared structural rules while allowing different content forms. Do not force weather, imagery, tracks and assessments into identical charts or the same confidence field. Distinguish producer quality, analyst confidence, job state and source freshness.

Keep the default cells bounded. A proposed desktop range is approximately 220–300px high, adjusted for actual content. Long inventories, JSON and audit logs open in detail; they do not stretch all neighboring cells. At 1440px, aim to expose the important domains within the first one or two useful rows. Exact dimensions require a real-data design pass, not blind adherence to a six-card template.

An important exception can have a wider cell or supporting row, but the grid should keep stable positions. Do not reorder modules automatically when live counts change or expand cards under the pointer. A failed child request marks that cell without blanking the whole grid. A source count or aggregate needs its scope/denominator, including bounded query limits.

## A first-class map preview grid

The map should have a compact **preview dock**, not another copy of the entire Systems dashboard. It carries records/items the user can inspect spatially, while the overview grid carries capability-level awareness.

The dock has three clear sizes:

- **Collapsed:** one compact handle/header, the selected preview category and a meaningful retained selection/count where available. A label such as Previews remains discoverable; collapsing does not turn every feature into an unlabeled icon.
- **Preview:** one row in horizontal placement, or one column in vertical placement. Real thumbnails or concise item summaries appear. This is the normal analytical map arrangement on a roomy desktop.
- **Expanded:** a bounded second row or larger list/detail area for deliberate comparison, with a clear return to Preview. Opening a full workspace is available when the task needs more room.

Keep category navigation in the dock header: for example **Saved context / Imagery / Events / Weather / Tracks / Findings**. The cells below are actual authorized records, not six generic KPI tiles repeated from Systems. The selected category is explicit. Essential empty/gated information is explained truthfully when that category is opened.

An installed Jeddah saved context can start with the relevant reference coverage, retained before/after imagery and the actual analysis/proposal where allowed. The installed March imagery remains clearly historical. A curated starting collection is new view/persistence work if no existing saved-view contract supports it; it is not a claim that this collection already exists. Do not fill the default row with irrelevant global events simply to make it look populated.

### What a preview cell shows

- Readable title and record type.
- A real thumbnail, small meaningful trace/coverage cue or compact text preview.
- Observed/acquisition date, or reference version validity where appropriate.
- Provider/producer and current-versus-historical/partial status.
- Selection/highlight state and an explicit Inspect/Open action.

The same fields do not apply to every record. A reference layer does not need observation freshness; an aircraft without altitude does not get an invented height; an assessment does not inherit the provider's quality class as analyst confidence. Full hashes, raw body, long provenance and processing internals stay in detail.

### Horizontal versus vertical

**Recommend bottom horizontal as the initial desktop arrangement.** It preserves broad east-west geographic relationships and supports scene/image sequences and side-by-side record scanning. A proposed one-row height is roughly 100–132px including its compact header; this must be tested with readable text rather than achieved by shrinking labels. Four to six useful cells can be visible on a typical desktop, depending on content and card width. Explicit Previous/Next and visible paging/shown counts support more records; do not rely on hidden horizontal scrolling.

**Offer a right-side vertical arrangement for users who prefer it**, especially on ultrawide screens or while scanning report titles. A proposed width is about 220–260px. It offers more height for previews but costs map width and can compete with the selected-item inspector. Therefore it replaces the horizontal dock; it does not appear alongside it. The user's orientation preference should be stable rather than changing unexpectedly whenever the browser resizes.

**Expanded mode must have a space budget.** A starting cap of roughly 240–300px for a bottom grid allows deliberate comparison without taking over the map on a large desktop. On a short laptop, use a smaller cap or route to the full grid. A vertical dock should not grow into a wide collection of floating cards. Apply the constraint to the actual available viewport, including playback or authoring timelines.

**On phone portrait, use a compact handle and one selected preview**, expanding to a temporary sheet or full list on request. A six-column strip with tiny cards is not a phone design. On short landscape views, a compact side preview can preserve height better than a tall bottom grid; any responsive relocation must be explicit and preserve state. Do not stack a preview grid, telemetry tray and playback timeline into three competing bottom bands.

### Initial map arrangement

The initial desktop view should show the map, a small useful preview row, compact region/time/coverage, and clearly discoverable tools. No permanent left layer catalogue or full-height right inspector is necessary. A one-action Focus map state can collapse the preview dock and auxiliary tools while preserving essential context.

This is a change from the rejected study: the normal view contains visible, useful material. The user sees what EMAD has, not merely where a menu might be hiding it. Fully expanded investigation and focus-only map are both available, rather than choosing one extreme as the entire design.

## Interaction semantics: eliminate surprising side effects

**Selecting a cell selects a record.** It synchronizes the preview, map highlight and any already-open inspector. It does not silently filter away other records, toggle a layer, jump the camera, change time or launch a job.

**Locate** pans/fits the selected record when its geometry is known and authorized. If it lies outside the current view, say so and offer Locate; do not silently move the investigation. Records with no geometry remain inspectable and explain why they cannot be plotted.

**Show layer/result** explicitly changes visibility. Incompatible region/time or expired rights are explained. A historical image cannot become the background without keeping its date/source visible.

**Inspect** expands the selected record's detail in the dock's existing expanded region where suitable. The default does not immediately add a second large floating inspector. For sustained reading, a deliberate docked right inspector or full record is available. This makes the preview grid a coherent browsing surface, not a launcher for accumulating popups.

**Open full workspace** carries the exact object/revision, selected AOI/time and return location into Imagery, Assessments, Context, Studio or Replay as appropriate. Browsing a source problem can open its full Systems cell/detail. Browser Back and an explicit Return to map both restore the meaningful context.

**Keep in previews** retains an item in the user's chosen collection, if implemented. Give this a different name from pinning an entire layout panel. A bookmark does not freeze data; if a user needs a fixed evidence revision, retain the revision ID and label that meaning.

**New data should not reorder the user's work.** Show a New items indicator and update the list deliberately or preserve focus/scroll/selection while appending. Late revisions, withdrawn records and a changed filter can invalidate a selection; explain that state rather than silently substituting another item.

## Systems, map and workbench: one connected investigation

A realistic public journey is:

1. An overview cell exposes the installed Jeddah imagery and its dated retained state.
2. Open map shows its footprint and a preview row with the selected compatible pair.
3. Open comparison carries those exact inputs into the full Imagery workbench.
4. The result appears as a typed analytical finding with valid denominator and limitations.
5. Draft/review uses the assessment workflow, with canonical human identity enforcement.
6. Return to map preserves the result, region/time and preview selection.

The exercise journey uses the same navigation principle with different authority: environment/models → scene authoring → frozen run → perceived contacts and permitted tasking → recording → privileged evaluation. Authoring and evaluator inputs never become ordinary preview data in an operator scope.

The overview grid provides breadth. The map preview dock supports spatial browsing and comparison. Dedicated workbenches provide depth. Their information has different resolution; they should not repeat every detail or number in all three places.

## Preserving the design quality

Return to the richer professional visual direction of the earlier desktop concepts: strong spatial imagery, precise typography, quiet chrome, real content previews, restrained status accents and readable hierarchy. Refine the composition of that design rather than substituting a generic minimal wireframe.

Keep the map visually calm through styled layers, selected-label priority and explicit scale behavior. The preview dock should look integrated with the application boundary, not like a stack of floating translucent windows. Use modest dividers and a stable selected state; avoid heavy card framing, flashing alerts, ornamental graphs and repeated boilerplate caveats. Dark map/3D surfaces and lighter reading surfaces can coexist within the same semantic system.

Scope, date, source, uncertainty and permission are evidence-bearing interface elements. They remain readable when the preview grid collapses. A pleasant layout cannot justify false precision, assumed altitude, unlabeled synthetic observations, copied source confidence or fake activity.

## Implementation consequences

- Separate **overview module summaries**, **record preview items** and the existing **exercise entity matrix**. Shared visual primitives do not imply shared domain semantics.
- Design a typed preview item with stable record/revision identity, content kind, source/time meaning, optional geometry/thumbnail, availability, limitation and permitted next actions. Do not flatten all data into arbitrary JSON or a universal confidence score.
- Use real aggregates/counts from authorized query scope. Loaded page length is not automatically total count; reference versions and retrieval revisions are not independent observations.
- Add a small layout controller for dock size/orientation, selected preview/category and deliberate inspector state. Reuse existing collapse/density foundations where compatible; avoid an unrestricted floating-window manager.
- Preserve URL/back navigation for region, applied time/history meaning, selected revision, imagery pair/job, scene/run and relevant camera. Local hook state currently does not guarantee this round trip.
- Scope preferences and retained preview collections to principal, workspace/environment and partition/run. Purge truth-bearing data on authorization/resource changes; no CSS-only hiding of restricted tiles.
- Use static retained/generated thumbnails or inexpensive summaries for grid previews. Do not instantiate one active map/Three.js renderer per grid cell. Lazy-load below-fold content and dispose private object URLs correctly.
- Resize the real map when the dock's allocated space changes; keep selected geometry in its safe visible region without re-fitting all content. Docking cannot silently redefine the query AOI.
- Let durable worker jobs update job/preview state independently of whether a card is open. Do not auto-expand a grid or navigate on completion.
- Add true development/test classification where needed. Acceptance scenes and workflow-test assessments stay accessible but do not dominate normal defaults; avoid brittle name-based filtering.

## Tradeoffs and decisions to validate

The bottom preview row costs some map height but makes the installed content discoverable. The collapsed/focus option recovers that space for pure spatial reading. The vertical variant preserves height but costs horizontal geography. The full overview gives breadth without forcing every preview into the map. Additional switches are useful only if they preserve state and have a clear purpose.

The user confirmed keeping and improving the Systems grid and rejected the previous minimal layout. The exact card style and best default preview orientation across real tasks remain design decisions to validate with coherent user journeys and matched visual alternatives. The confirmation of the grid's location is not approval of every proposed screen or interaction.

Test the proposal against representative tasks: discover installed content, compare the retained March pair, inspect a real report's provenance, understand an empty bounded feed, review a real proposal, author a route, observe permitted synthetic task outcomes, reopen a recording and enter/leave evaluation safely. Capture desktop, short laptop and narrow screens with real content. Measure task success, navigation loss, map obstruction and misinterpretation; visual attractiveness and passing clicks alone are insufficient.

## Recommended next design deliverable

Produce one coherent set showing the full overview grid, the same selected content on the map with a horizontal preview row, the vertical alternative, the expanded selected-record state and its dedicated workbench. Use the same region/time/record and the earlier professional visual language in every state. This gives a fair comparison and makes the continuity visible. The rejected standalone layout study should not be patched into that deliverable.

## External references

These support interaction mechanics, not a claim that EMAD should copy another product:

- [ArcGIS dashboard layouts](https://doc.arcgis.com/en/dashboards/latest/get-started/dashboard-layout.htm) describe grouped/resized layouts and stacked tabbed elements. EMAD needs its own bounded grid and role/time semantics.
- [ArcGIS dashboard actions](https://doc.arcgis.com/en/dashboards/latest/create-and-share/actions.htm) describe linked selection, filters and map adjustments. In EMAD, those different actions should be explicit rather than all triggered by one ambiguous card click.
- [W3C disclosure pattern](https://www.w3.org/WAI/ARIA/apg/patterns/disclosure/) provides keyboard and expanded-state semantics for expandable controls. The preview dock should expose those semantics and offer click/keyboard alternatives to dragging.

No skills, image generation, application implementation, commits or external messages were used for this analysis. The screenshot is an actual installed page; the proposed grid/dock behaviors remain design work.
