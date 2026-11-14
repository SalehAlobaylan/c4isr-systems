# D01 — Categorical land-cover data

[Plan index](../README.md)

**Status** Implemented for the first Riyadh study area, 13 November 2026. Independent release review is pending. [Source, preparation and verification evidence](../../docs/implementation/RIYADH_ENVIRONMENT_ENRICHMENT.md).

**Implementation owner** Geospatial data engineering.

**Dependencies** [F02 — Geographic profiles, coordinates and quality](../environment-foundation/02-geographic-profiles-and-quality.md), [V03 — Imagery and vector display preparation](../3d-infrastructure/03-imagery-and-vector-display.md)

## Outcome and baseline

A consumer can distinguish supported land-cover classes in an enriched environment and inspect their edition, class definitions and limitations. Existing reference packages keep their original capability declarations.

## Features and contracts

Evaluate an ESA WorldCover 2021 v200 subset for the first bounded Riyadh study area. Retain its nominal 10 m categorical grid, class dictionary, original source identity, mask, edition and attribution. Model it as a new semantic layer and optional profile capability.

Retain the classification’s time independently of the surface and imagery dates. The 2020 and 2021 products use different algorithm versions, so comparisons must disclose the algorithm change. Global validation statistics do not establish the local AOI’s accuracy.

## Implementation sequence

1. Select the exact source tile and review retention/display/export rights through F01.
2. Add the layer and categorical metadata to the catalog, profile, processor and verifier together.
3. Crop on the native grid, use nearest-neighbor resampling when required and preserve nodata.
4. Produce a color display derivative with a pinned class legend.
5. Compare the selected area with dated imagery and report sample disagreements as review evidence.
6. Release the new environment revision with the added capability and temporal mismatch visible.

## Acceptance

- Prepared cells contain only documented classes and nodata.
- Reprojection introduces no fractional class values.
- Native and prepared class counts agree for the declared selection.
- All viewers expose the class legend, edition and valid-data mask.
- Classification uncertainty remains distinct from terrain accuracy.
- Labels supply land-cover meaning only. Materials, load capacity and physical behavior require separately qualified inputs.

## Rollout and recovery

Add the capability to an explicit profile revision. Existing reference profiles and releases remain reproducible. A failed or unsuitable source is recorded as unavailable for the selected AOI.

## Deliverable evidence

Deliver one real source receipt, original and cropped hashes, class/count comparison, display legend and independent package verification. Expand to other AOIs after this complete path passes.

## Implementation and source references

- [internal/geoenv/catalog.go](../../internal/geoenv/catalog.go)
- [scripts/data/environment_library.py](../../scripts/data/environment_library.py)
- [Geographic intake sequence](../IMPLEMENTATION_SEQUENCE.md#increment-1--reconcile-the-foundation)
- [ESA WorldCover data and validation](https://esa-worldcover.org/en/data-access)
