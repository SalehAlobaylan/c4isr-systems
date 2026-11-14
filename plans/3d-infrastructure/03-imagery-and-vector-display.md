# V03 — Imagery and vector display preparation

[Plan index](../README.md)

**Status** The bounded Riyadh overlays are installed on private staging. The isolated durable worker reproduced all four overlay hashes. Shared worker rollout and broader profile qualification are pending. [Imagery and vector evidence](../../docs/implementation/RIYADH_DURABLE_GEOGRAPHIC_PREVIEW.md#v03-dated-imagery-and-vectors)

**Implementation owner** Imagery, geospatial and frontend engineering.

**Dependencies** [F02 — Geographic profiles, coordinates and quality](../environment-foundation/02-geographic-profiles-and-quality.md), [V01 — Browser asset delivery and derivative publication](01-browser-assets-and-publication.md)

## Outcome and baseline

A viewer can add dated imagery, roads, building footprints and water to the terrain while retaining feature identity and limitations. Existing packages contain GeoTIFF bands and GeoPackage vectors. Use the retained imagery and vectors admitted for the Riyadh release for the first detailed workflow.

## Features and contracts

For imagery, bind source scene, acquisition time, band identifiers, grid, scaling, masks and display color recipe. Reproject display imagery explicitly into the terrain footprint and retain its parent values. Align sampling and UV coordinates with pixel registration.

For vectors, preserve stable source feature ID plus edition, original geometry and tags. Derive viewport-specific GeoJSON or vector tiles and local 3D buffers. Display heights must identify whether they came from source measurements, source tags or an explicit rendering estimate. Carry full data and displayed feature counts separately.

## Implementation sequence

1. Define optical texture and vector display recipes with exact layer and member dependencies.
2. Build imagery textures with valid-data masks and evaluate alignment against retained controls.
3. Build roads/water overlays and footprint picking with holes, multipart features and stable IDs.
4. Add optional building extrusion only when its height origin is visible and inspectable.
5. Measure spatial tiling and simplification using Riyadh imagery and urban vectors. Use the existing heavier fixtures for additional load checks and select resolution levels from the measured viewer budget.

## Acceptance

- Acquisition date, band identity and no-data mask remain available in the inspector.
- Texture-to-terrain alignment matches the declared resampling pipeline.
- Feature selection survives tiling and zoom transitions using stable source IDs.
- Unknown height remains unknown in the canonical record and any display estimate is labeled.
- Simplified geometry preserves required topology or is rejected with a diagnostic.
- Turning a layer off cancels pending reads and releases its resources.
- A missing optional display asset preserves the useful view and states the layer failure.

## Rollout and recovery

Publish each display layer independently under one parent release and derivative recipe. A corrected color recipe or geometry simplification creates a new derivative identity. Existing native members remain available for independent analysis.

## Deliverable evidence

Retain imagery alignment checks, feature identity maps, topology fixtures, displayed and available counts, and byte and GPU resource measurements for Riyadh and each subsequently qualified area.

## Implementation and source references

- [scripts/data/environment_library.py](../../scripts/data/environment_library.py)
- [internal/reference/map.go](../../internal/reference/map.go)
- [internal/imagery/preflight.go](../../internal/imagery/preflight.go)
- [apps/operator-ui/src/features/workspace/AtlasCanvas.tsx](../../apps/operator-ui/src/features/workspace/AtlasCanvas.tsx)
- [World Monitor src/components/DeckGLMap.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/components/DeckGLMap.ts)
- [World Monitor server/worldmonitor/imagery/v1/search-imagery.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/server/worldmonitor/imagery/v1/search-imagery.ts)
