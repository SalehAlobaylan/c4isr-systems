# EMAD UX and UI specification proposal

Date: 23 October 2026. Status: implementation adopted on 24 October 2026; see the [delivery and acceptance record](EMAD_UX_IMPLEMENTATION.md). Final visual approval remains a user review. User decision: **keep and improve the full grid in Systems**, with a smaller expandable preview grid on the map. No skills were used for this analysis. The rejected `EMAD_MAP_LAYOUT_STUDY.html` is not a design reference or implementation target.

This specification turns the [grid analysis](EMAD_WORKSPACE_GRID_DESIGN_ANALYSIS.md) into screen, interaction, data and delivery requirements. It preserves the richer visual direction of the earlier concepts while replacing their always-expanded composition with a deliberate spatial layout. The engineering goal is to let a person discover, inspect and complete useful work without understanding the repository's module structure.

## 1. The experience to build

EMAD has two product areas: Public Context and Simulation. Within each, three complementary views perform different jobs:

- **Systems:** a full grid of meaningful capabilities, available content and work requiring attention.
- **Map/spatial view:** a dominant map or scene, with a compact dock of relevant record previews.
- **Focused workspace:** enough room for imagery comparison, human review, context export, scene editing, exercise operation or replay.

The views share a selected investigation and permitted records. They do not repeat every number, form and diagnostic. Systems stays a prominent destination. The map retains useful visible information. A focused workbench is reached with an explicit action and preserves the return context.

The central task loop is **discover → select → inspect → act → return**. An advanced interface should shorten this loop and preserve its meaning. Adding decorative widgets or hiding everything behind menus does neither.

## 2. Design decisions and open validation

The following are settled requirements: full Systems grid; small expandable map grid; two product pillars; real content previews; independent source/time/trust states; preserved public/exercise/truth boundaries; reuse of working renderers and backend modules.

Recommended defaults to validate with real content: bottom-horizontal map previews on desktop; a visible single preview row on an ordinary analytical map; bounded grid cells; a neutral graphite shell with lighter reading surfaces; one deliberate expanded detail region.

Exact card composition, dock height, vertical alternative, responsive breakpoints and final palette remain proposals. They require matched screen designs and task checks. The user has not approved every pixel or interaction by approving the presence of the grid.

## 3. One coherent application shell

Use one global identity/navigation authority, with a compact contextual task row. A useful desktop starting budget is 56px for the application header and about 40px for workspace navigation/context. Avoid the current three independent navigation bands.

The shell exposes EMAD; Public Context/Simulation; friendly workspace/environment; current account; activity; and a precisely named connection state. Primary task destinations remain readable. For Public Context these include Map, Systems, Imagery, Assessments and Context packages. Simulation retains Projects, Studio, Exercises, Replay and Systems, subject to actual grants.

The contextual row contains the active region, temporal mode and relevant task controls. It does not repeat the brand, account, resource hash or a second competing page hierarchy. A long ID is available through Copy ID/technical details. Global search may be added as a scoped action; it is not permission to fetch everything across partitions.

Layout, layer and preview choices are preferences. Product area, account, environment, partition and run are authorization context. Do not treat switching a visual tab as switching identity or granting access. In particular, a role dropdown must not impersonate another development account or reveal evaluator data.

## 4. Investigation scope must be explicit

There are two different spatial extents: the **investigation region**, which determines the analytical question, and the **map viewport**, which determines what is currently visible. Panning should not silently replace the investigation. Offer Search this area or an explicit Follow map setting.

Systems also needs a clear **Workspace / Investigation** scope choice. Workspace summarizes all authorized installed content; Investigation summarizes content matching the selected region and supported time filters. Do not show a Jeddah label beside a workspace-wide total and imply that all records are in Jeddah.

A card can distinguish inventory and matching content when useful, with named denominators. For example, retained catalogue revisions, successful capture records and independent acquisitions are different quantities. A limited list's length is not an exhaustive total. A source's configured status is not coverage of the current region.

The current API does not support one uniform date slider across all data. Verified code paths show:

- Public events support their existing historical interpretation/filter contract.
- Imagery scene lists accept a geographic bbox and return bounded catalogue metadata; their list handler does not expose a date-range query.
- External-track lists expose bbox/source and current segment state; the list is not an arbitrary historical aircraft-position endpoint.
- Weather/report lists expose kind/bbox; their handler does not expose the same historical query as public events.
- Reference data has version/validity semantics rather than observation freshness.
- Exercises and recordings use a run's virtual time, separately from receipt and audit clocks.

Until the relevant query is implemented, show Not available for this historical view or explicitly separated current context. Do not present current tracks/weather as synchronous with retained March 2025 imagery. Metadata filters over a small complete result may be a bounded transitional implementation, but cannot represent exhaustive filtered search over a truncated result.

## 5. Systems grid: actual content at a useful scale

### Public Context overview

Use a stable six-domain composition. Prioritize useful content and review work in the first row:

1. **Geography & environments:** a real coverage/locator preview; national versus detailed Jeddah extent; available reference families; Open map and inspect versions/rights.
2. **Imagery & analysis:** retained image pair or acquisition preview; clearly labeled dates; catalogue-only versus retained-ready state; Open imagery and inspect the applicable result.
3. **Assessments & context:** a readable real proposal/review item; method/state and author identity where permitted; separate access to context packages. Geography export is not made dependent on a fabricated assessment.
4. **Public observations:** a representative report/type distribution only when supported; region/time scope; observed date/source; retained-history or retrieval limitation; Open reports/map.
5. **Weather & official reports:** a selected station/report with location and observed time; meaningful measurements/title; freshness; no inferred regional weather field.
6. **Aircraft & vessels:** separate feed states, bounded results and source coverage. Zero results, missing permission and stale retrieval have different explanations. External tracks do not gain tasking actions.

A small supporting activity/readiness area can expose processing or source issues. Full source inventories, jobs, platform diagnostics and audit remain secondary Systems destinations. They do not displace the overview or repeat it as another row of the same six counts.

### Simulation overview

Use the same visual rules with different records: environments/models; scene/template readiness; admitted runtime and queue; permitted exercise perception/tasking; recordings; authorized evaluation/released results. The visible set depends on actual permissions. An operator must not fetch an author's frozen scene or truth journal merely to obtain a thumbnail or actor count.

### Anatomy of a cell

Each cell has a short title, one meaningful content preview, at most two or three primary facts, an honest status/limitation and one prominent next action. One smaller detail action may accompany it. Large counts are used only when counting answers a useful question.

The content should vary appropriately: imagery uses real thumbnails; geography uses a coverage locator; reports use readable titles and dates; assessments use a claim excerpt and review state; scene previews use validated assets; source availability uses a compact readable status. No invented sparklines or decorative gauges.

Use a three-column desktop layout when cells remain comfortably readable, moving to two and then one column as space/text size requires. A proposed card height is around 264px at normal desktop text size, achieved by bounded content rather than by clipping essential information. For example, show two representative items plus View all rather than embedding an entire source inventory.

A 1440px-wide view with 24px outer gutters and 16px gaps gives approximately 453px per cell in three columns. Two 264px rows plus a 16px row gap occupy 544px. This allows all six domains to remain discoverable without the current 835px first row. At 200% text zoom, reflow and allow content growth rather than enforcing a fixed height that cuts off actions. No internal scrolling in every default card.

Cards stay in stable domain positions as live data changes. New information updates a cell without moving it under the pointer. A failed card shows an inline failure/retry while successful siblings remain visible. Pending initial queries do not render a zero count before the response exists.

## 6. Map preview dock: bounded richness

The map grid is a **record browser**, not a shrunken Systems overview. Its header selects a record family, such as Saved context, Imagery, Reports, Weather, Tracks or Findings. Visible cells carry actual records and exact revisions where appropriate.

Start the installed Jeddah context with authorized reference coverage, retained imagery and the real comparison/proposal where relevant. Keep acceptance fixtures and published development workflow tests out of normal defaults through an explicit classification, without deleting them. Do not add irrelevant global observations to make a row look busy.

### Default geometry

Use a bottom-horizontal dock as the initial desktop proposal. A comfortable candidate budget is **152px total**: 32px header, 8px gap, 104px preview cells and 8px lower clearance. At 104px, a cell can accommodate an 80px thumbnail beside readable title/date/source/action content. About 260–300px width per cell produces four or five useful previews on a normal desktop, with explicit paging/shown counts.

This refines the earlier 100–132px estimate: a shorter strip is only useful if real thumbnails and text still fit. A compact 120–132px mode can be tested for text-heavy previews; it should not be achieved by shrinking essential type or removing date/source.

With a 96px shell and 24px footer, a 1000px-high desktop has 880px of working height. A 152px dock leaves **728px for the map**. An 800px-high laptop leaves 528px. These are arithmetic layout examples, not application measurements or guarantees of usability.

The dock allocates real layout space at the viewport edge. It should not be an opaque 152px overlay hiding the same map below it. Avoid many borders or floating shadows: a single structural division is sufficient.

### Collapsed, preview and expanded

Collapsed retains a labeled 28–36px handle/header, selected category, meaningful selection/count and Restore previews. Preview shows one useful row. Expanded adds a bounded second row or selected-record detail, with a clear Preview/Collapse control.

Cap expanded height against actual available workspace. A starting rule is no more than roughly 40% of working height and no more than 300px, while preserving a useful map. The exact breakpoint is a usability target, not a universal law. If a task needs more, Open full workspace supplies room instead of endlessly resizing the dock.

Inspection should use the dock's expanded region by default where suitable: records/list on one side, selected summary on the other. This avoids a new full-height sidebar on every click. A deliberately docked right inspector remains available for sustained reading when the center stays useful; it is an explicit working arrangement, not an automatic extra panel.

### Vertical option

Offer a stable right-side single-column dock, around 232–260px wide, as an alternative. At 1440px with a 248px dock, roughly 1192px remains before other chrome. Expanded vertical detail should replace or repurpose that region rather than adding another right sidebar beside it.

Do not show both orientations at once. Preserve category, selection and position when changing orientation. Do not change orientation repeatedly during ordinary browser resizing. On narrow portrait, adapt to one selected preview/temporary sheet; on short landscape, a compact side region may preserve height better than a bottom grid.

### Record anatomy

A cell shows a readable title, record type, meaningful preview, observation/acquisition/validity date, producer/source, availability/limitation and explicit next action. Missing values remain unknown. A thumbnail failure has a usable textual alternative. Exact hashes and long lineage open on demand.

Imagery must distinguish native source-band resolution from the preparation/analysis grid. The installed NDVI comparison uses a 128×128 angular grid; 10m input bands do not make the resulting change grid a 10m product. Its valid-cell denominator and masked cells must remain visible in the analysis, with the limitation that NDVI change alone is not a vegetation-loss or causal finding.

## 7. Interaction contract

Give every gesture one predictable meaning:

- **Select:** synchronize cell highlight, map highlight and existing inspector. Keep the map camera and applied query stable.
- **Locate:** pan/fit the authorized geometry. Explain Outside current view and No geometry. Use point/extent-appropriate zoom rather than the current public camera handle's fixed zoom of 7.
- **Show on map:** explicitly enable a compatible layer/result, preserving source/date and rights. This is distinct from selecting a record.
- **Inspect:** open readable detail in the current dock/detail region. Do not automatically stack multiple large surfaces.
- **Open imagery/review/Studio/replay:** navigate to the relevant focused workspace with the exact object/revision and return context.
- **Keep in previews:** add a record to the user's collection if that persistence feature is implemented. This is not the same action as keeping a layout panel open.
- **Focus map:** collapse auxiliary content while preserving region/time/coverage, selection and jobs. Restore returns to the preceding arrangement.

Use normal click/tap/keyboard activation. Hover can preview or emphasize but cannot carry the only source/date/status information. Double-click should not be the only path to a task. Dragging has buttons/form alternatives.

Selecting another record updates an intentionally open inspector in place. Clicking an empty map should not clear a selection after a drag. Escape dismisses the top transient interaction first; it does not reset filters, discard a draft, clear evidence or stop a run. Closing a record panel and Clear selection are separate actions.

When the applied filter removes a selected record, show why it is outside the current results and offer clear/return, rather than silently replacing it. New items/revisions can show a New items indicator; they should not reorder focused previews or silently change a frozen evidence revision.

## 8. Status must be multidimensional

Avoid a universal Healthy/Offline/Available label for every type. Keep these concepts separate:

- **Availability:** catalogue metadata, acquiring, retained/validated bytes, expired rights or integrity failure.
- **Retrieval:** current, partial/backfill, stale, failed, gated, not queried or not applicable.
- **Human workflow:** draft, proposed, reviewed, published, rejected, development test.
- **Exercise lifecycle:** queued, starting/running, paused, terminal/failure states as supported by the domain.
- **Authority:** public evidence, exercise input, perceived synthetic output, privileged truth; enforced by the backend and reinforced by labels.

Show the most relevant type-specific state plus one compact contextual limitation. A source's configured key is not proof of fresh coverage. Retained historical geography should not be an offline-feed alert. A connected websocket is not a running simulation. Zero results require a completed bounded query before implying that the result is a valid zero.

Show observed/acquired time near content. Put last retrieval/update separately in details or where it affects trust. Name timezone and clock basis. Unknown is a distinct state, not green and not a fabricated zero.

Ordinary network loss retains the last authorized reading with a stale timestamp. Permission revocation clears forbidden content immediately. Rights expiry and integrity failure prevent unavailable bytes from being presented as fresh valid data. Worker failures explain the affected task and supported next action without covering the whole map.

## 9. Visual system

Use the earlier rich concepts as a composition direction, not a source of geographic pixels or fabricated evidence. Real retained image previews, validated model renders and real coverage provide the visual substance. Preserve the actual prepared environment and its declared fidelity; concept artwork does not promise a richer terrain engine.

Propose a restrained neutral system: graphite navigation, quiet near-neutral workspace background, light document/card surfaces, dark readable ink and one consistent selection accent. Shift away from pervasive pale-green panels and numerous tinted boxes. Separate selection from warning/failure. A single brand/interaction accent can span Public Context and Simulation; persistent mode text distinguishes their meaning. Truth needs an explicit privileged label, not merely a color change.

Retain the installed IBM Plex type family. Use approximately 14px body, 13px dense data, 12px supporting metadata and 18–24px meaningful headings at normal scale. Avoid all-capital paragraphs and tiny operational labels. Monospace is useful for exact coordinates/time/units, not every title. Use a 4/8px spacing rhythm, restrained dividers and modest 6–8px control radii. Essential text must not depend on very low-contrast placeholder styles.

Map encodings distinguish subdued geographic context, observations, derived/uncertain extents, external tracks, reviewed findings and selection. Exercise controlled telemetry, perceived contacts and evaluator truth remain distinct. Color is supplemented by shape, line treatment and text. No generic red threat classification inferred from a feed; no decorative uncertainty circles or animated trails unsupported by data.

Avoid masonry as the default Systems arrangement, automatic attention-based card reordering, a widget wall, live animation in every preview, blurred translucent text panels, ornamental gradients and oversized slogans. Stable positions, useful content and legible status make the interface feel advanced.

The existing `DESIGN.md`/`styles/tokens.css` encodes a single older appearance. A new palette/composition must deliberately update that design contract and semantic tokens when implemented, rather than scattering inline overrides. No automatic theme switch or new design library is required to make the redesign work.

## 10. Screen-specific depth

**Imagery:** keep acquisition dates, compatible input pair, quality controls, comparison and selected result together. Scene catalogue and processing attempts open without pushing the comparison below a long form. Distinguish scene-wide cloud cover from the actual common-clear analytical denominator. Preserve Collection 1 versus original-product preparation semantics.

**Assessments:** prioritize readable claim, exact evidence, method, rationale, uncertainty, current revision and human identity. Review controls explain eligibility and conflicts before submission while keeping server enforcement. Separate real proposals from development tests. Source quality and model score are not reviewed confidence.

**Context packages:** provide room for selected geography, versions, rights, omissions and immutable preview. Open in Studio carries a version, not a mutable live feed. Public tracks never become controllable actors through a convenient UI shortcut.

**Studio:** library/hierarchy, selected actor/waypoint inspector and timeline are editing tools. Keep them available while editing; collapse them for spatial inspection. Do not apply the same minimal layout to every job. Explicit datum, draft versus frozen state, validation and specified-version Start remain clear. Unknown/unsupported MSL/AGL conversion cannot become a convenience field without a real tested conversion path.

**Exercise:** the operator sees sanitized environment, perceived contacts and permitted controlled telemetry/task outcomes. Run state and virtual time stay close to the scene. Contacts with unknown altitude retain uncertainty. Hidden routes, sensors and truth do not appear in preview payloads or browser caches.

**Replay:** transport, virtual time and recorded/read-only mode are intrinsic controls, not optional dashboard clutter. Use one bottom dock/timeline system rather than stacking preview cards, telemetry and playback into three bands. Evaluation is a separately authorized route/session with purpose before truth retrieval. Released reports remain sanitized.

## 11. Responsive and accessible operation

At desktop widths, show the full Systems grid and compact map previews. At intermediate widths, reduce columns, not type size; limit large map detail regions. At phone widths, prioritize the selected content and map with an expandable single-preview sheet or full readable list. Landscape has a separate space budget. Do not force users to rotate or arrange tiny desktop cards in a horizontal phone strip.

Plan visible focus, semantic headings/lists, labeled controls and meaningful keyboard order before implementation. Route links use navigation semantics; actual tabs use tab semantics; collapse controls expose expanded state. Slow network requests should not run because keyboard focus merely traverses a tab row. Provide buttons for expand/collapse, paging, locating and changing orientation; do not require precise dragging or edge-hover discovery.

Use generous touch hit areas, target roughly 44px for primary mobile controls, and keep zoom/focus/reduced-motion behavior testable. Respect browser text zoom. A drawing/3D renderer needs an equivalent selected-item list/property path. Closing a modal or sheet restores focus and the prior view. On-screen keyboards must not cover the sole Apply/Close action.

## 12. State, security and performance

Define one investigation context for resource, scope mode, region, temporal mode/range, selected record/revision, visible layer versions, meaningful camera and return target. Keep presentation preferences separate: grid density, preview orientation/size, active record category and deliberately open detail.

URL/state must restore Back/Forward and shared authorized links. Avoid serializing credentials, truth payloads, full drafts or an unbounded list into the URL. Local draft/saved-preview persistence is new work unless explicitly implemented; state that limitation and preserve recoverable edits. Changes in resource/identity/run invalidate prior clients and restricted selection.

The existing WorkspaceProvider already aborts requests, clears query clients and recreates scoped sessions on changes. Preserve that behavior. Add presentation state without creating an unscoped global cache. A hidden evaluator view cannot stay mounted with truth in order to restore quickly.

Use domain-specific data adapters for overview summaries and record previews. Each declares supported filters, count meaning, permission requirements, source/time fields and available actions. Prefer existing endpoints where adequate; add small scoped read models only for verified gaps. Do not create a new microservice or generic search platform for the redesign.

Reuse the current MapLibre/Three.js loaders, private artifact access and realtime policies. The model-thumbnail component already serializes render creation and disposes contexts; preserve it and improve thumbnail reuse/caching if needed. Do not render a new live map or simulation inside every tile. Limit active heavyweight renderers to the visible task, pause hidden rendering, cancel obsolete queries and lazy-load below-fold private previews.

Dock changes need a real renderer resize and careful safe-area handling. Avoid subtracting a dock from the container and padding it again. Locate should account for geometry extent and the actual viewport. Resize must not refit the entire scene or redefine the investigation's region. WebSocket connectivity, source freshness and run time have separate state owners.

## 13. Concrete implementation gaps

The repository inspection identifies these work categories:

1. **Presentation composition:** replace legacy C2-centric Public Systems cells and above-map configuration blocks; build two purpose-specific grid types; preserve exercise Board comparison.
2. **Investigation continuity:** selected public evidence/layers/viewport currently live partly in page-local state. Add validated route/store handoff and camera restore.
3. **Camera behavior:** the current public `focus` helper uses zoom 7. Replace with a geometry-aware Locate contract rather than hard-coding one scale for a country, scene footprint and point.
4. **Count/filter semantics:** several bounded lists return loaded counts as `total` plus truncation. A full summary or filtered grid needs truthful aggregate/has-more fields, not client-invented totals.
5. **Temporal capability:** date/history support differs by domain. Extend supported queries where justified or expose clear limitations; do not use a uniform cosmetic filter over incompatible endpoints.
6. **Safe run/recording discovery:** current recordings are primarily accessed by run ID, and native run lists do not by themselves provide a friendly operator-safe library. Add a scoped catalogue if required; preserve author/operator/evaluator distinctions.
7. **Useful previews:** reusable rights-aware thumbnails, human names, development/test classification and stable selected revision metadata may need small persisted/read-model additions.
8. **Layout preference migration:** `aegis.c2.layout.v1` stores legacy independent flags. Introduce a versioned scoped layout model instead of restoring arbitrary old combinations.
9. **Action eligibility:** explain review independence, export prerequisites and runtime readiness using authoritative state. A disabled button without a reason is insufficient; an enabled button cannot bypass the backend.

These are not claims that underlying imagery, simulation or storage must be rebuilt. They are the missing integration contracts required to expose existing capabilities coherently.

## 14. Delivery plan

Deliver complete paths with real installed content rather than styling all routes superficially:

1. Define record types, investigation/filter capability and permission-safe action contracts. Build shared shell, status, disclosure and preview primitives against those types.
2. Deliver Systems Overview with actual geography, imagery and real review content, plus deliberate empty/gated/failed cases. Remove repeated KPI strips and oversized inventories.
3. Deliver map previews with actual retained records, explicit select/locate/show/inspect behavior, horizontal/vertical/collapsed states and context restoration. Keep Systems linked throughout.
4. Complete Imagery → assessment → context handoff using the same selected evidence and timeline semantics.
5. Complete Studio → runtime → operator perception/tasking with correct scoped previews and specified frozen versions.
6. Complete recording discovery → replay → separately authorized evaluation, reusing the same layout/status conventions where appropriate.
7. Verify complete real-backend journeys, devices, keyboard/text zoom, slow/stale/partial cases, rights expiry, session changes, storage/worker failure and reconnection. Capture matched evidence only after acceptance.

No date estimate is justified without sizing these gaps. Each increment must include interface, data adapter/query, failures, permissions and acceptance; a backend-only completion or a static screen is not a delivered workflow.

## 15. Acceptance scenarios

Use the installed data as dated fixtures, not a claim of perpetually current counts:

- A new analyst identifies the Saudi overview, bounded Jeddah detail, retained March imagery, real pending proposal and provider limits directly from Systems.
- The imagery tile distinguishes catalogue revisions from captured pixels and opens the exact pair. No fabricated capture date or independent-source count appears.
- One record selection is consistent across grid, map and existing inspector; camera movement happens through Locate. Returning from a full workspace restores the region/time/selection.
- A historical view does not silently display current aircraft/weather. Unsupported history is understandable and not represented as a valid zero.
- The preview dock is readable when expanded/collapsed or reoriented and does not hide the selected geometry. No duplicate copy of the full Systems dashboard covers the map.
- Large text, keyboard-only and narrow-screen tasks can locate an item, inspect its evidence and return without losing work. No essential source/date lives only on hover.
- Real proposal review preserves same-human/service restrictions and independent-account development labeling. Export uses only eligible retained inputs.
- An author edits and freezes a bounded scene and starts it without build hashes; an operator sees only perception/controlled telemetry and virtual task outcomes.
- Replay opens retained data after restart and never dispatches commands. Evaluator truth is purpose-gated and absent from ordinary previews, socket messages, released reports and restored client caches.
- Job/provider/storage errors show the affected scope and recovery action while retained authorized content remains usable. Permission revocation clears forbidden content immediately.

Measure task completion, wrong turns, time to find content, lost context, source/time misunderstandings, focus issues and map obstruction. Record hardware and real renderer behavior. Screenshots and passing selectors alone cannot prove that this is the best design for the users.

## 16. Required visual design set

The next design set should show the same real, dated Jeddah investigation across: Systems Overview; map with horizontal previews; vertical alternative; selected/expanded record; imagery comparison and review handoff; Studio editing/inspection; ordinary exercise/replay; and privileged evaluation as a separate authority. Include desktop, short laptop, portrait and landscape behavior.

Use realistic text lengths, real retained thumbnails, accurate zero/gated/historical states and representative permissions. Annotate the actions and data bindings. Do not substitute a generic schematic map for the whole visual design or use generated city geometry as a promise of newly delivered simulation fidelity.

## References and inspected implementation

Primary references for mechanics: [W3C disclosure pattern](https://www.w3.org/WAI/ARIA/apg/patterns/disclosure/), [W3C tabs pattern](https://www.w3.org/WAI/ARIA/apg/patterns/tabs/), [ArcGIS dashboard layouts](https://doc.arcgis.com/en/dashboards/latest/get-started/dashboard-layout.htm), and [MapLibre Map API](https://maplibre.org/maplibre-gl-js/docs/API/classes/Map/). These support accessible controls, distinct responsive arrangements and renderer resizing; they do not prove the proposed EMAD layout's usability.

Inspected local implementation: `routes/systems.tsx`, `routes/map.tsx`, `router.tsx`, `features/public-context/usePublicContext.ts`, `components/WorkspaceProvider.tsx`, `components/map/MapCanvas.tsx`, `features/simulation/ModelThumbnail.tsx`, `stores/layout.ts`, `styles/tokens.css`, the API client, imagery/public-context/external-track/recording HTTP handlers and authenticated readiness. The [earlier API snapshot](ux-review-2026-10-23/installation-snapshot.json) and [actual Systems inspection](grid-review/systems-observations.json) remain dated evidence; no live ingestion or domain mutation was needed for this design work.
