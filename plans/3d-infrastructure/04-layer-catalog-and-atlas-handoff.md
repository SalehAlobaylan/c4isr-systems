# V04 — Shared layer catalog and Atlas-to-3D handoff

[Plan index](../README.md)

**Status** Installed in the private staging Environment Library and Atlas. Exact feature and time restoration passed the desktop and compact browser checks. Independent release review remains open. [Implementation evidence](../IMPLEMENTATION_PROGRESS.md#riyadh-geographic-display-and-offline-evidence)

**Implementation owner** Frontend and environment product engineering.

**Dependencies** [V01 — Browser asset delivery and derivative publication](01-browser-assets-and-publication.md), [V02 — Terrain mesh generation and coordinate rendering](02-terrain-and-coordinate-rendering.md), [V03 — Imagery and vector display preparation](03-imagery-and-vector-display.md)

## Outcome and baseline

A user selects an environment in Atlas, opens its approved geography in 3D, inspects a feature and returns to the same context. Atlas already has permission-aware layers, AOI/time state and camera persistence. The existing release detail is the entry point.

## Features and contracts

Extend the current registry with renderer support, geometry/time semantics, required capabilities, source explanation and display limits. Keep runtime layer state separate from immutable definitions.

A handoff descriptor references authorized area revision, geographic release/digest, derivative ID, selected feature/edition, time window and enabled layer IDs. Camera state is renderer-specific presentation data. Resolve all IDs and grants on reload. Use authorized saved IDs in links instead of embedding protected geometries.

## Implementation sequence

1. Extract the existing Atlas layer definitions into the shared registry while preserving current behavior.
2. Add a read-only geographic preview workbench and a lazy Three.js renderer.
3. Implement exact release resolution, forward/return navigation and stable selected-feature mapping.
4. Add layer source/quality explanations and explicit unsupported-capability states.
5. Handle access changes, changed AOI, interrupted fetches and browser back/forward without restoring stale protected content.
6. Preserve useful list/map navigation when WebGL initialization or context recovery fails.

## Acceptance

- Opening, refreshing and returning preserves area, release, time and feature identity.
- A stale network response for the previous area cannot replace the current view.
- Protected selections become unavailable after an access change and are cleared from reusable client caches.
- Unsupported layers identify the missing capability or data.
- Picking agrees with the authorized source feature and can open its provenance.
- Keyboard navigation and compact layouts retain all essential actions.
- Historical Studio entry points retain their established behavior.

## Rollout and recovery

Qualify the preview and Atlas return path on the Riyadh release first. Open it to consumers under F03 release rules and add other Saudi and Middle Eastern study areas as their own releases qualify. Suspend the preview independently of source preparation if a renderer regression appears.

## Deliverable evidence

Record a desktop and compact walkthrough through the actual backend, selection round trips, accessibility checks, stale-response tests and a denied-access case.

## Implementation and source references

- [apps/operator-ui/src/features/workspace/AtlasCanvas.tsx](../../apps/operator-ui/src/features/workspace/AtlasCanvas.tsx)
- [apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx](../../apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx)
- [apps/operator-ui/src/features/workspace/WorkbenchContext.tsx](../../apps/operator-ui/src/features/workspace/WorkbenchContext.tsx)
- [apps/operator-ui/src/features/workspace/EnvironmentLibrary.tsx](../../apps/operator-ui/src/features/workspace/EnvironmentLibrary.tsx)
- [World Monitor src/config/map-layer-definitions.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/config/map-layer-definitions.ts)
- [World Monitor src/components/MapContainer.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/components/MapContainer.ts)
- [World Monitor src/utils/layer-explanation-card.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/utils/layer-explanation-card.ts)
