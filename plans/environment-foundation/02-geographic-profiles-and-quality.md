# F02 — Geographic profiles, coordinates and quality

[Plan index](../README.md)

**Status** Implemented and qualified with an exact-member Riyadh rebuild and seven legacy package checks. Shared rollout and independent physical accuracy remain open. [Profile and quality evidence](../../docs/implementation/GEOGRAPHIC_SOURCE_QUALITY_AND_STORAGE.md#f02-profile-and-geographic-quality)

**Implementation owner** Geospatial data engineering.

**Dependencies** [F01 — Source admission, lineage and retention](01-source-admission-and-lineage.md)

## Outcome and baseline

Every environment describes what each layer means, where it is valid and which transformations produced it. Existing profiles preserve native surface, geoid, imagery and vector data. The three staged packages record physical accuracy as UNKNOWN. Numerical coordinate checks and byte integrity are already evidenced.

## Features and contracts

Version profile definitions with required and optional semantic layers, allowed formats, source epochs, supported horizontal and vertical systems, nodata behavior and resource limits. Preserve the existing v1 readers. Add structured capability and quality fields with unknown values when older metadata cannot be interpreted reliably.

A quality report must distinguish numerical transformation agreement, provider accuracy, independently checked physical accuracy and coverage completeness. Store units and confidence/statistic definitions. Resolve the local ENU frame and each member’s native datum separately. The manifest’s ellipsoidal frame does not convert raster samples by itself.

## Implementation sequence

1. Inventory current free-text CRS, resolution, dates and uncertainty. Define an additive structured representation with explicit legacy parsing results.
2. Make profile and recipe revisions immutable and include their digests in the build inputs.
3. Extend the processor and independent verifier together for registered pixel/point grids, masks, multi-part AOIs, seams and vector omissions.
4. Add capability queries and quality explanations used by preflight, release review and the 3D inspector.
5. Introduce new source profiles through the D-series plans with their own tolerances and retained transformation grids.

## Acceptance

- WGS84/ECEF/ENU numerical round trips satisfy the existing 1 mm implementation check using independent controls.
- EGM2008 conversion uses the exact retained grid and declared interpolation. Reapplying it is detected by the representation identity.
- Nodata holes, polygon holes and disjoint components survive preparation and verification.
- DSM, bare-earth terrain and bathymetry have distinct meanings. Unknown dimensions survive export.
- Unsupported datum, polar or date-line preparation fails preflight with an actionable reason.
- Reprocessing a v1 fixture preserves its established logical content or produces a new documented representation identity.

## Rollout and recovery

Introduce schema versions before new producers. Qualify reader compatibility against existing packages. Keep old profile revisions available for reproduction. New accuracy claims require the source and control evidence specified in the profile.

## Deliverable evidence

Retain the compatibility matrix, numerical controls, source-backed tolerance rationale, valid-data coverage results and the first detailed quality report for Riyadh. Attach separate evidence as detailed coverage expands within Saudi Arabia and the Middle East.

## Existing implementation references

- [internal/geoenv/catalog.go](../../internal/geoenv/catalog.go)
- [internal/geoenv/build.go](../../internal/geoenv/build.go)
- [scripts/data/environment_library.py](../../scripts/data/environment_library.py)
- [internal/simulation/spatial.go](../../internal/simulation/spatial.go)

[Foundation evidence](../../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md) · [Original implementation plan](../../docs/implementation/ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md)
