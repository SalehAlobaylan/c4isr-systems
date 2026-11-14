# D05 — Historical weather fields and station context

[Plan index](../README.md)

**Status** Central Saudi historical pressure-field pilot implemented, 13 November 2026. Physical-use qualification and independent release review remain open. [Pilot evidence](../../docs/implementation/RIYADH_OBSERVATION_PILOT.md#d05-historical-pressure-fields).

**Implementation owner** Environmental data engineering.

**Dependencies** [F02 — Geographic profiles, coordinates and quality](../environment-foundation/02-geographic-profiles-and-quality.md), [V06 — Context overlays, freshness and coverage](../3d-infrastructure/06-context-overlays-and-freshness.md)

## Outcome and activation

A future app can request dated station observations and, when needed, a declared atmospheric field. Current METAR context supplies station reports. The environment library explicitly marks volumetric weather unsupported. Activate field acquisition after the consumer names variables, area, time, vertical levels and required resolution.

## Features and contracts

Qualify a small historical ERA5 pressure-level subset as a candidate. Preserve native source files and define a bounded subset representation with variable names, units, coordinate arrays, valid times, pressure levels, masks, product edition and uncertainty information. Introduce required formats and validation explicitly in the geographic catalog.

Keep station measurements and gridded reanalysis as distinct layers. Pressure coordinates require their declared interpretation. Any conversion to geometric height or interpolation is a separate versioned operation with its required inputs.

## Implementation sequence

1. Freeze the consumer’s variable/time/level request and byte budget.
2. Admit the exact product and retain the provider request and response identity.
3. Add parser/validator support for the chosen native and subset formats.
4. Create bounded spatial/temporal subsets preserving native samples and masks.
5. Provide slices and point inspection with units and sampling resolution.
6. Compare representative values to source samples and retain any station comparison as separate evidence.

## Acceptance

- Variables, units, dimensions and coordinates survive the subset round trip.
- No field is presented at finer native resolution than its source.
- Missing values and vertical-coordinate ambiguity are explicit.
- Reanalysis edition and data time remain distinct from retrieval time.
- Future forecast adapters preserve issue time and lead/valid time.
- Frozen field bundles reopen at the same exact sample values.
- Resource use stays inside the declared pilot envelope.

## Rollout and recovery

Ship format/reader support before activating a provider profile. Keep fields optional for existing geographic releases. A failed request or unsupported field remains visible as an unavailable capability.

## Deliverable evidence

Deliver one bounded request, retained source and subset, dimension/unit validation, sample comparison and a dated slice preview. Suitability for later weather-dependent physical behavior requires that consumer’s own qualification.

## Implementation and source references

- [internal/publicingest/metar.go](../../internal/publicingest/metar.go)
- [internal/publicingest/observation.go](../../internal/publicingest/observation.go)
- [internal/geoenv/catalog.go](../../internal/geoenv/catalog.go)
- [scripts/data/environment_library.py](../../scripts/data/environment_library.py)
- [Copernicus climate reanalysis](https://climate.copernicus.eu/climate-reanalysis)
- [ERA5 pressure levels](https://cds.climate.copernicus.eu/datasets/reanalysis-era5-pressure-levels)
