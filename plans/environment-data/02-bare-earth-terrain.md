# D02 — Bare-earth terrain and surface comparison

[Plan index](../README.md)

**Status** Modeled ground and surface comparison implemented for the first Riyadh study area, 13 November 2026. Independent release review and independent physical accuracy checks remain open. [Source, preparation and verification evidence](../../docs/implementation/RIYADH_ENVIRONMENT_ENRICHMENT.md).

**Implementation owner** Geodesy and terrain engineering.

**Dependencies** [F02 — Geographic profiles, coordinates and quality](../environment-foundation/02-geographic-profiles-and-quality.md), [V02 — Terrain mesh generation and coordinate rendering](../3d-infrastructure/02-terrain-and-coordinate-rendering.md)

## Outcome and baseline

An environment can carry a qualified bare-earth terrain alongside its existing surface model. The current Copernicus DSM includes surface features, and the packages report bare-earth replacement as unsupported.

## Features and contracts

Assess Riyadh first for a source that explicitly represents bare-earth elevation and covers the study area. Compare candidate source editions, native spacing, acquisition dates, vertical reference, masks and retained-use rights. Record a capability gap when the available source evidence cannot support a Riyadh bare-earth layer.

Give ground and surface separate semantic identities. Record the native horizontal CRS, vertical reference, registration, units, masks and source accuracy. Pin any required transformation grid and keep transformed outputs as derived representations.

## Implementation sequence

1. Identify a source covering the bounded Riyadh study area and record its quality documentation, retention rights and attribution.
2. Implement the exact supported datum conversion with independent controls before ingestion.
3. Add a bare-earth profile capability and processor/reader support.
4. Prepare terrain and DSM on declared comparison grids and compute spatially bounded differences.
5. Render a toggle/comparison with source dates and known exclusions.
6. Validate any physical accuracy statement using independent controls with a defensible uncertainty model.

## Acceptance

- Native sample meaning and vertical reference survive packaging.
- Unsupported conversion fails preflight and never assumes EGM2008.
- Nodata areas and source exclusions remain visible.
- Comparison differences identify registration, time and semantic differences.
- Numerical checks meet F02, while physical checks use the documented source/control tolerances.
- Existing DSM consumers still resolve their original layer identity.

## Rollout and recovery

Release the ground dataset as an additional capability and source edition. A bad conversion withdraws its derivative and dependent release without rewriting the original observations. Restore exact conversion grids through F07.

## Deliverable evidence

Deliver the exact native subset, conversion controls, comparison report, mask evidence and reviewed accuracy status for the qualified area. Assess coverage and source quality independently for each subsequent Saudi or Middle Eastern area.

## Implementation and source references

- [scripts/data/environment_library.py](../../scripts/data/environment_library.py)
- [internal/geoenv/catalog.go](../../internal/geoenv/catalog.go)
- [internal/simulation/spatial.go](../../internal/simulation/spatial.go)
