# V05 — Local basemaps and offline viewing packages

[Plan index](../README.md)

**Status** Credentialed local viewing and the Riyadh street archive are installed on private staging. The portable issuance, verifier and reader passed qualification with explicitly authorized fixtures. Riyadh portable issuance remains subject to the release decision and source admission terms. [Implementation evidence](../IMPLEMENTATION_PROGRESS.md#riyadh-geographic-display-and-offline-evidence)

**Implementation owner** Geospatial delivery and platform engineering.

**Dependencies** [F01 — Source admission, lineage and retention](../environment-foundation/01-source-admission-and-lineage.md), [F05 — Artifact delivery, integrity and storage lifecycle](../environment-foundation/05-artifact-delivery-and-lifecycle.md), [V01 — Browser asset delivery and derivative publication](01-browser-assets-and-publication.md)

## Outcome and baseline

Navigation and geographic viewing work with external internet access unavailable. Atlas already serves retained reference vectors through a local protocol, while its street basemap uses public OSM raster tiles.

## Features and contracts

Provide a versioned local navigation asset set containing coverage, zoom limits, tile archive or endpoint, style, glyphs, sprites, labels, attribution and checksums. Use retained reference geography for the initial offline view. Build PMTiles from a permitted extract when richer street navigation is selected. Source extraction follows the dataset’s permission and delivery method.

Define two supported deployments. A disconnected local stack serves protected assets through normal credentials. A portable viewing package contains explicitly permitted assets and a local reader/launcher, including all required styles and fonts. Record the export policy at issuance.

## Implementation sequence

1. Audit renderer dependencies and remove accidental external asset requests from the disconnected profile.
2. Package retained Saudi country and region navigation, the Middle East overview and their local labels.
3. Build and admit a bounded Riyadh PMTiles street basemap with original data attribution.
4. Add local provider selection and meaningful behavior outside the installed extent.
5. Extend export manifests to bind viewer version, assets, policy, expiry and verification commands.
6. Test both modes from a clean profile with no pre-existing browser cache.

## Acceptance

- The local stack loads geography and the selected 3D environment with external hosts blocked.
- Network tracing identifies every expected local request and any failed external dependency.
- Missing fonts, sprites, tiles or derivative members produce an actionable package validation error.
- Basemap extent and zoom limits are visible without fabricating detail.
- Authorized scoped assets remain protected in local-server mode.
- Portable-package verification detects altered bytes and records applicable use/retention terms.
- Online source withdrawal blocks new downloads. Exported copies follow their granted policy and documented operational deletion process.

## Rollout and recovery

Switch providers through configuration with an authorized retained-outline fallback. Keep old basemap editions pinned for reopening existing work. Withdraw a faulty export edition and issue a corrected package. F07 includes local navigation dependencies in backups.

## Deliverable evidence

Deliver a bounded navigation release, dependency manifest, clean-browser network trace and verified portable package. Public OSM tile service bulk-download restrictions require obtaining basemap data through an allowed source.

## Implementation and source references

- [apps/operator-ui/src/features/workspace/AtlasCanvas.tsx](../../apps/operator-ui/src/features/workspace/AtlasCanvas.tsx)
- [internal/reference/map.go](../../internal/reference/map.go)
- [internal/geoenv/http.go](../../internal/geoenv/http.go)
- [deployments/staging/delivery.yml](../../deployments/staging/delivery.yml)
- [World Monitor src/config/basemap-styles.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/config/basemap-styles.ts)
- [PMTiles integration](https://docs.protomaps.com/pmtiles/maplibre)
- [OSM tile use policy](https://operations.osmfoundation.org/policies/tiles/)
