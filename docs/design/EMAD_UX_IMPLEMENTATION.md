# EMAD workspace redesign

The subsequent [map exploration implementation](EMAD_MAP_IMPLEMENTATION.md) adds selectable geography, simultaneous spatial evidence and in-map workbenches. It uses forward migration 44; the acceptance below remains the record of the earlier UI increment.

24 October 2026. Implemented in `apps/operator-ui`, using the existing persistent installation. The user authorized implementation of the [workspace specification](EMAD_UX_UI_SPECIFICATION.md). The rejected layout study remains historical material; it is not this implementation.

## Review the application

With `task dev` running, open [Systems](http://127.0.0.1:5173/systems), [Public Context Map](http://127.0.0.1:5173/map) or [Home](http://127.0.0.1:5173/). Use an issued account credential. This frontend delivery requires no migration or content reinstall.

The two product areas share a compact identity header and one task-navigation row. Account changes retain the established cache clearing and authorization checks. The event connection is labeled separately from provider freshness. Workspace and run selection remain available on small screens through More.

## Delivered workflows

- **Systems:** six bounded Public Context domains, actual retained image previews, readable proposals, reference coverage, observations, station weather and external-feed state. Source activation and processing history have separate tabs. Workspace and saved-investigation scope are explicit. Simulation has a role-specific overview; existing exercise diagnostics remain reachable.
- **Map:** a dominant MapLibre canvas with a 152px preview dock. The dock supports expansion, collapse, a side orientation and a focus-map state. It occupies layout space instead of hiding geography. Short landscape screens use the side arrangement; portrait screens retain a compact bottom arrangement.
- **Record interaction:** select highlights a record without moving the camera; Locate fits its point or extent; Show layer changes reference visibility; Inspect opens focused detail; Open workspace continues the relevant task. Selection graphics have their own map layers and never masquerade as provider observations.
- **Continuity:** camera, family, selected record, reference layers and investigation filters are scoped to the account and resource. Public event history retains its existing interpretation. Weather and tracks explicitly report when a historical query is unsupported. A wide viewport retains the last bounded reference query with a zoom instruction rather than claiming complete wide-area coverage.
- **Imagery:** separate comparison, catalogue, analysis, review and context-package workspaces. Actual retained before/after pixels, quality masks, acquisition receipts, reproducible NDVI results and immutable assessment history remain accessible. A bounded investigation can supply the capture AOI. Context export offers investigation/retained-interval shortcuts and UTC date controls.
- **Studio:** scene versions and hierarchy beside the 3D viewport, actor properties alongside it, and admitted runtime controls below. The expandable actor library uses validated model thumbnails. Draft changes preview existing actors and authored routes; Freeze saves a validated version, while Start uses the saved version. Working drafts survive navigation within the authorized input session and can be reverted. Workspace/account changes clear that private session; Freeze is required for durable storage.
- **Asset library:** visual models, authored definitions and import/validation history have separate views. Model detail, rights and template/environment authoring open in a focused sheet.
- **Exercise and replay:** permitted spatial views keep unknown altitude explicit. Replay puts the spatial view before compact transport controls, with events and archive recovery available on demand. The recording library reads ordinary authorized run grants and archive indexes; it does not fetch authoring scenes or truth for previews. It is bounded to 24 grants and describes that limit.
- **Evaluation:** a separate explicit view checks evaluator authorization before requesting truth. Closing evaluation or switching accounts removes the private query data. Operator playback, sanitized environments and released reports retain their existing isolation.

The interface uses neutral reading surfaces, graphite structure, restrained teal selection and the existing semantic status palette. IBM Plex typography, keyboard focus, bounded previews and responsive reflow remain consistent across workspaces. Heavy routes load on demand. Renderer teardown releases GPU contexts, and late map updates tolerate teardown safely.

## Follow-up usability corrections

The subsequent [usability review](EMAD_UX_POLISH_REVIEW.md) identified defects beyond page overflow. Weather rows now retain their content height; observation time in UTC and metric groups lead the selected evidence. Navigation resolves a single active destination, including a return from review to comparison.

Assessment review uses a bounded searchable inbox and a separate reading pane. Context export groups region/time, retained inputs and quality limitations, with searchable layer selection and a desktop summary. The model library uses compact searchable tiles and a recoverable no-match state. Catalogue search operates on retained catalogue records.

Analysis puts the retained result beside configuration on desktop and ahead of configuration on mobile. Mobile Studio exposes Actors/scenes, Properties and Run controls through separate tool selectors. Its viewport stays above those panes. The map's mobile controls occupy two compact rows. Empty external-track queries show provider availability and next actions without leaving an unused inspection column.

Source cards separate configuration, retrieval health and next actions from expandable diagnostics. The layer sheet has a bounded search picker. Template and environment cards expose declared behavior, geographic origins and model inspection. Mapped analysis loads its own styles and reports rendering progress; acceptance waits for rendered cells instead of accepting a blank loading frame.

## Verification

Production TypeScript/Vite build passed. The real-backend browser gate passed all **16 tests**, including exact retained evidence downloads, imagery analysis/review/export, model import, scene freeze/runtime control, virtual tasking, replay, evaluator metrics, workspace isolation and identity revocation.

The presentation suite passed **80 tests** with 54 intentional alternate-profile skips. Its navigation and palette assertions now reflect the adopted interface; legacy exercise inspection tests explicitly open the optional inspector. Test Vite instances use separate dependency caches and configurable ports, preserving the persistent development server.

Persistent-installation acceptance passed **18 tests**: the three existing end-to-end workflows, eight workspace checks and seven follow-up usability checks. They cover real startup/data, runtime pause/task/outcome/archive/replay, evaluator isolation across account switching, WebSocket reconnect, bounded Systems cards, independent map gestures, context restoration, unsupported historical feeds, partial failures, private authoring drafts, keyboard sheets and responsive layouts. The added checks measure record-text containment, unique active navigation, selection-preserving searches, model inspection, rendered analysis placement, mobile Studio pane isolation and source/layer/template interactions. Viewports include 1440×1000, 390×844, 320×700 and 768×390. Test artifacts with session traffic remain in ignored private output directories.

Captured application screens are in [the review evidence gallery](ux-implementation/README.md). These use retained source pixels, reference geometry and validated models from the persistent installation. They are not generated design concepts.

## Acceptance boundaries

Existing provider access conditions, content rights, simulation limits and abstract fidelity remain as declared by the backend. The real imagery proposal still awaits human review. Development accounts and published development records demonstrate workflow behavior; they do not prove review by two actual people.

All authorized scene versions remain available, including clearly named acceptance scenes. The last selected scene is remembered. This UI does not infer authoritative content classifications from record names or silently delete development evidence.

The production build still reports large MapLibre/3D dependency chunks. Those dependencies are separated from ordinary workspace loading; the build warning is not represented as a completed performance benchmark. Final aesthetic approval belongs to the user's review of the running screens.
