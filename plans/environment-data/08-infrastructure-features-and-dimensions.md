# D08 — Infrastructure features and dimensional evidence

[Plan index](../README.md)

**Status** Typed source dimensions and separate modeled building context implemented for the first Riyadh study area, 13 November 2026. Surveyed dimensions and independent release review remain open. [Source, preparation and verification evidence](../../docs/implementation/RIYADH_ENVIRONMENT_ENRICHMENT.md).

**Implementation owner** Geospatial data engineering and data stewardship.

**Dependencies** [F02 — Geographic profiles, coordinates and quality](../environment-foundation/02-geographic-profiles-and-quality.md), [V03 — Imagery and vector display preparation](../3d-infrastructure/03-imagery-and-vector-display.md)

## Outcome and baseline

Roads, buildings, waterways and selected infrastructure are useful as inspectable reference features with source-backed dimensions where available. Current OSM layers retain footprints and tags, while verified building dimensions and materials remain unsupported.

## Features and contracts

Use source feature ID plus edition as identity. Preserve original tags and topology alongside normalized display attributes. Represent measured dimensions, provider-stated dimensions, inferred display dimensions and unknown values explicitly.

Add source-specific infrastructure datasets only for a declared gap such as a missing footprint or verified building height. Retain provenance, acquisition/validity time, uncertainty and coverage receipts. World Monitor’s source catalog is a discovery reference for candidate providers.

## Implementation sequence

1. Audit footprint completeness and dimension availability for the first Riyadh study area. Extend the audit to other Saudi and Middle Eastern areas as they enter the release plan.
2. Preserve relation geometry, holes and feature identity through the canonical vector pipeline.
3. Add typed dimension fields with value, unit, origin, evidence and uncertainty.
4. Select one bounded source addition for a verified gap and admit its exact edition.
5. Derive display geometries with stable identities and explicit estimated values.
6. Present a feature inspector showing source geometry, tags, dimensions and omissions.

## Acceptance

- Relation geometry and polygon holes survive processing and picking.
- Unknown height never becomes a measured value through display extrusion.
- Unit conversion and dimension origin are independently inspectable.
- Provider completeness claims are bounded by their actual extraction/query receipts.
- A source correction produces a new edition without replacing pinned geometry.
- Simplification preserves canonical geometry and exposes display omissions.
- Any later topology or physical-property consumer receives only the capabilities actually qualified.

## Rollout and recovery

Extend vector schemas additively and retain original properties. Rebuild derived display layers when normalization changes. Existing references continue to resolve their exact editions. Keep additional providers limited to the accepted AOI and use.

## Deliverable evidence

Deliver a feature/dimension coverage report, one admitted improvement where available, topology fixtures, source-backed dimension checks and a consumer inspection walkthrough.

## Implementation and source references

- [scripts/data/environment_library.py](../../scripts/data/environment_library.py)
- [internal/reference/geography.go](../../internal/reference/geography.go)
- [internal/geoenv/catalog.go](../../internal/geoenv/catalog.go)
- [internal/publicingest/receipts.go](../../internal/publicingest/receipts.go)
- [World Monitor docs/data-sources.mdx](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/docs/data-sources.mdx)
