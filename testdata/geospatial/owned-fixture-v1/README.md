# Owned environment fixture v1

This directory is owned synthetic data created for the simulation-preparation
reader. It is redistributable without provider terms. No provider product bytes
are present.

- `prepared.zip` - `3e5593776e4a11b17801617bad79231956271e3b72f5d15f72c5c79dd2cfc2c9` (308491 bytes)
- `manifest.json` - `aaf9f14b027749b5d233018606fb90c81e34808b9522e034fc67b5243c966bdc`
- `answers.json` - expected answers, derived from construction constants
- `topology.json` - separately versioned source-relationship companion artifact
- `field-era5.json` - ERA5-shaped pressure-level pilot field (synthetic values)
- `display-descriptor.json` - purpose-specific visual derivative metadata

Regenerate with `python scripts/data/build_owned_fixtures.py`. Verification never
regenerates the fixture: the reader only reads the committed bytes and compares
answers and digests.

## Derivation

The area is an axis-aligned multipolygon with a hole and a second component. The
surface grid is north-up at 0.001 degrees with a declared scale and offset. The
answers are computed from those constants, not from the reader: for a cell
`(row, column)` the west edge is `west + column*0.001`, the north edge is
`north - row*0.001`, and the scientific value is `raw*scale + offset`.

OSM ways preserve their source node identifiers. Crossing ways that do not share
a node remain distinct in `topology.json`, and two ways sharing node 1302 are
recorded as a junction. Clipped ways and multipart relation footprints are
recorded with the clipping recipe.
