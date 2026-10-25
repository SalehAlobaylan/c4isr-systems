# EMAD map cleanup review

The [adaptive dock review](EMAD_ADAPTIVE_DOCK_REVIEW.md) supersedes this increment's dock sizes and placement. The current toolbar is 44px, desktop previews are 124px, and detailed desktop tools open beside the map. See the [latest captured views](map-dock/index.html); the measurements below describe the earlier increment.

24 October 2026. Implemented and reviewed in the running application. Open [the live map](http://127.0.0.1:5173/map) or the [five-screen walkthrough](map-cleanup/index.html).

## Findings resolved

The previous map repeated navigation and context across a floating situation card, left tool rail, bottom capability strip, record preview dock, floating selection card and modal evidence drawer. Several of these could occupy the screen together. The inspector also repeated the global time filters and layer checklists.

Those surfaces now share the existing dock:

- **Situation and refresh:** compact typed counts, update notices, refresh, previous view and pause controls sit in the dock's summary row.
- **Capability access:** a workspace picker replaces the long permanent capability strip. Existing primary task tabs remain available. Systems retains its actual six-domain grid and Sources/Processing sections.
- **Layers, tools and views:** one dock body hosts the active control panel. Layers group appearance, evidence families, retained imagery and reference geometry. Time filters have one shared form reached from the visible time control above the map.
- **Selection and inspection:** a selected map object appears in the dock, with source, date, uncertainty and Locate/Inspect actions. Exact evidence and reference provenance use the Geography workbench. Opening inspection no longer adds a modal drawer or second panel over the map.
- **Underlying work:** temporarily opening controls hides the active workbench while preserving its mounted state. Closing controls restores it. Selected records, assessment drafts and original inputs remain intact.
- **Layout:** the dock opens compact by default, with previews on request. Layout groups side position, expansion and full view. The full-view menu opens inward so the application header cannot intercept it. Collapsing a control panel and choosing its tab reopens it correctly.
- **Keyboard and narrow screens:** Escape returns from controls and restores focus. Selection receives focus when opened. Phone controls use a bounded two-row layout; hidden fields do not force a horizontal page. Short landscape layouts allocate a side panel when expanded and return to a compact bottom bar when collapsed.

The map canvas retains native navigation, scale/attribution and a small coordinate/acquisition/status line. Temporary search results and actionable error notices remain contextual. Normal browsing places no persistent situation, capability, selection or control card on top of the geography.

## Measured result

This earlier increment recorded **729 of 870 workspace pixels vertically: 83.8%**, with an **80px** default dock. The current [adaptive dock receipt](map-dock/layout-acceptance.json) supersedes those sizes: **765 of 870 pixels, or 87.9%**, and a **44px** toolbar. These measure available map space, not frame-rate performance.

The default layout preference has a new version so existing installations receive the cleaner initial presentation. Account/resource-scoped camera, geography, layers and other safe preferences are preserved. Later dock choices remain remembered. No database migration or content reinstall is required for this increment.

## Verification

- Type checking and production build passed.
- Full frontend regression suite: **80 passed / 54 intentionally skipped** for separately selected profiles.
- Existing live map/workspace/polish checks: **27 passed**.
- Three additional live cleanup checks passed for initial map space, one shared panel with retained underlying work, and keyboard/viewport reachability. The final map-plus-cleanup run passed **15 checks** after focus and panel-restoration refinements.
- Actual controls were checked at **390 × 844**, **320 × 700** and **768 × 390**. The checks verify control bounds, absence of page overflow, focus on opening and focus restoration on Escape.
- Real-backend production-browser compatibility gate: **16 passed**, including exact evidence downloads and historical filters, imagery review/export, public moving contacts, official-report review, protected Studio/assets, runtime/replay, evaluator isolation and revocation. Its domain APIs and WebSockets are not mocked.
- The live cleanup checks report no page exceptions. They verify an actual selected weather record survives temporary control panels and selection/evidence use the same dock. Existing tests preserve unsaved assessment drafts across tool and geographic changes.

The production build retains the previously documented renderer chunk-size warning. This increment changes presentation and panel hosting; provider permissions, provenance, geographic predicates, simulation fidelity and Public/Exercise isolation remain enforced by the existing contracts.

## Primary implementation

- `apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx`: shared dock, workspace picker, allocated controls and selection, compact summary and focus behavior.
- `apps/operator-ui/src/features/public-context/PublicContextPanel.tsx`: embedded inspector presentation removes duplicate global navigation, filters and layer controls while retaining exact evidence/provenance.
- `apps/operator-ui/src/styles/atlas.css`: consolidated map layout, dock states, settings groups and responsive controls.
- `apps/operator-ui/tests/recovery/map-cleanup.spec.ts`: measured space, panel consolidation, state preservation and keyboard/viewport acceptance.

The prior [implementation record](EMAD_MAP_IMPLEMENTATION.md) documents the geographic installation and data contracts. This review supersedes its earlier floating-peek/control-panel presentation.
