# Simulation preparation delivery record

**18 November 2026.** This record covers the P01–P08 delivery described in
[SIMULATION_PREPARATION_IMPLEMENTATION_PLAN.md](../../plans/archive/simulation-preparation/SIMULATION_PREPARATION_IMPLEMENTATION_PLAN.md).
It states what is implemented, which evidence proves it, and which claims
remain unqualified.

## Result at a glance

| Workstream | Delivered | Primary evidence |
| --- | --- | --- |
| P01 consumer requirements | Shared Python/Go decision engine, 24 known-answer vectors, release-scoped suitability API, immutable assessments and corrections | `testdata/geospatial/suitability-cases`, `internal/geoenv/suitability.go`, `scripts/data/environment_suitability.py` |
| P02 feature semantics | Source feature identity, clipped parts, topology companion, tag/dimension outcomes | `scripts/data/environment_reader.py`, `testdata/geospatial/owned-fixture-v1/topology.json` |
| P03 native queries | Inventory, cell, point, feature, bounded features, frame and field reads with explicit outcomes | `scripts/data/environment_reader.py`, `scripts/data/test_environment_reader.py` |
| P04 reference validation | Owned fixture and answers, scoped assessment storage, migration 60, reviewer correction flow | `testdata/geospatial/owned-fixture-v1`, `db/migrations/00060_geographic_assessments.sql` |
| P05 consumer kit | Deterministic inventory, trusted digest, atomic publish, resume, local read after expiry checks, operator guide | `scripts/ops/environment_kit.py`, `scripts/ops/receive_environment_release.py`, [operator guide](OFFLINE_ENVIRONMENT_KIT_GUIDE.md) |
| P06 time-aware fields | Exact variable/level/time reads, temporal meaning, below-ground and missing-sample outcomes | `field-era5.json`, `environment_reader.field_read` |
| P07 asset metadata | Display purpose descriptors with parents, recipe, units, axes, origin, bounds, adjustments and limitations | `DisplayPurpose` in `internal/geoenv/display_purpose.go`, `environment-display.yaml` |
| P08 workload | Repeatable measurement of verify/inventory/sample/point/feature/features/field | `scripts/ops/measure_environment_workload.py`, `testdata/geospatial/owned-reader-workload.json` |

## Owned fixture

The fixture is generated from owned OSM XML and raster bytes so source IDs,
node membership, component cuts and dimensions survive preparation. Answers and
digests were written by the generator from the fixture definition, not by the
reader.

| Item | Value |
| --- | --- |
| Fixture | `testdata/geospatial/owned-fixture-v1` |
| Manifest SHA-256 | `aaf9f14b027749b5d233018606fb90c81e34808b9522e034fc67b5243c966bdc` |
| Prepared archive SHA-256 | `3e5593776e4a11b17801617bad79231956271e3b72f5d15f72c5c79dd2cfc2c9` |
| Prepared bytes | 308,491 |
| Toolchain SHA-256 | `81bb4c7dd4155123f75811898a304aae4705fbe80a88a8690c3a3afe393f9d61` |
| Area | 46.700–46.760 E, 24.700–24.760 N with a hole at 46.720–46.730 E, 24.720–24.730 N, plus component B at 46.780–46.800 E, 24.700–24.720 N |
| Shared vectors | 24 cases in `testdata/geospatial/suitability-cases/cases.json` |

The fixture includes a valid zero, a nonzero value, a masked cell, the exact
tile seam, the area hole, a second component, a way clipped across both
components, crossing lines with different node IDs, a shared node, a multipart
footprint with a hole and an unsupported dimension string.

Regenerate with:

```text
.venv-recovery/Scripts/python.exe scripts/data/build_owned_fixtures.py
.venv-recovery/Scripts/python.exe scripts/data/build_suitability_cases.py
```

Tests read only the committed fixture and vectors.

## P01 — suitability and assessments

Both readers implement the same rules:

- `scripts/data/environment_suitability.py` for offline and kit evaluation;
- `internal/geoenv/suitability.go` for the installed API.

The shared file is evaluated by `scripts/data/test_environment_suitability.py`
and `internal/geoenv/suitability_test.go`. Expected decisions come from the
requirement rules written in `build_suitability_cases.py`.

Vocabulary and result rules:

- `supported` only when every required item passes;
- `unsupported` for a proven violation, including a different release,
  withdrawn or expired grant, absent required layer, datum/unit mismatch,
  insufficient mask evidence, area outside coverage or a statistic above the
  declared tolerance;
- `unqualified` when required evidence is missing, including unknown accuracy,
  missing valid-sample evidence, an assessment for another member or area, or
  an unresolved expiry instant;
- optional omissions are returned separately and never change the overall
  result.

The corpus covers native surface and native imagery with optional weather
absent, required ground absent, ground with unknown accuracy, an accepted and an
exceeding statistic, a correction, subarea and member mismatches, mixed-year
background and strict windows, partial area coverage, expired terms, a missing
grant, a withdrawn grant, a visual-only native request, datum/unit mismatches
and resource limits.

The release evidence digest covers `{schema, manifestSha256, areaRevisionId,
area, members, inputs}` and excludes assessments and terms, so a review or an
expiry does not rewrite the release identity. The Go test pins the digest the
offline reader produces for the committed fixture
(`bcf918ab071791821d9da6ceeef802a394c594063bcb3b8d2bc2009d6db4514c`), and both
readers evaluate the same shared vectors.

Spatial predicates use the stored area geometry, including holes, and a fixed
9×9 lattice over the requirement's bounding box. The lattice is a documented
sampling predicate, not a claim of full raster coverage. A boundary coordinate
belongs to the eastern or southern neighbour.

### Assessment records

Migration 60 adds `geographic_environment_assessments` with row-level security,
source visibility, an immutability trigger and a foreign key to the parent
release. Each record keys the release digest, member digest, layer, use, method,
control-set digest, statistic, uncertainty, area, reviewer decision and note.
A correction inserts a new record with `supersedes_id`; the original remains
readable, and `supersededBy` is derived. The suitability decision names the
assessment ID and its pinned `evidenceSha256`.

Routes (also in `api/openapi/environment-library.yaml`):

| Method and path | Permission |
| --- | --- |
| `GET /api/v1/environment-releases/{id}/assessments` | `environments.read` |
| `POST /api/v1/environment-releases/{id}/assessments` | `environments.qualify` |
| `GET /api/v1/environment-assessments/{id}` | `environments.read` |
| `POST /api/v1/environment-assessments/{id}/supersede` | `environments.qualify` |
| `POST /api/v1/environment-releases/{id}/suitability` | `environments.read` |

Every route applies the existing scoped access and current source policy before
returning release metadata or decisions. Online suitability reevaluates current
rights at the request instant; a withdrawn grant prevents the request. Offline
evaluation states the issued terms and expiry it used, and the kit refuses an
expired or ungranted operation before a local read.

## P02/P03 — native reader

`scripts/data/environment_reader.py` opens ZIP members through GDAL
`/vsizip/` and never rebuilds a package. Commands: `inventory`, `sample`,
`point`, `feature`, `features`, `frame`, `field`, `field-times`, `asset`,
`suitability`, `verify-kit`.

Outcome contract highlights:

- a measured zero is `0.0`, never null or omitted;
- mask, outside-area, outside-member, uncovered, absent, unsupported layer,
  unsupported unit, unsupported conversion, conflicting overlap and off-grid
  outcomes are explicit;
- the 0.6 m drape offset exists only in display derivatives and is never applied
  to a native read;
- feature output keeps the modeled centroid height separate from footprint
  dimensions and returns all clipped parts for one source feature;
- the frame command reports the local east/north/up result and checks frame
  controls.

The committed tests cover a valid zero, mask, hole, exact seam, conflicting
overlap, clipped feature, axis ordering, unit conversion, missing geoid,
missing ground, serialization of zero and display independence. The canonical
edge convention is documented in the module docstring and repeated in the
operator guide.

## P05 — kit and receipt

`environment_kit.inventory_document` builds a deterministic inventory with
release and area identity, prepared digest/length, members, reader contract,
purpose, operation, selected displays and terms. The trusted digest is supplied
separately; `verify` refuses a mismatch before serving a read. The optional
reader environment is locked in `scripts/data/reader-requirements.txt`.

`publish` writes every file into a private sibling directory, verifies copied
bytes, fsyncs where supported and atomically renames the directory. An
interrupted write leaves no published kit. `receive_environment_release.py
--kit` downloads selected displays, checks their exact identities, verifies the
prepared archive, builds the inventory and proves a native sample and feature
read. The original single-archive receiver path and receipt are unchanged.

Specific failures are tested for a corrupt archive, a missing selected display,
expired terms, unknown schema or reader contract, an interrupted publish and a
changed identity. Each archive verification runs in a fresh process, so two
different profiles cannot share the legacy enrichment binding; the kit test
verifies two inventories concurrently and checks that each keeps its own
purpose and digest.

Display inventories now carry `purpose`, `purposeMeaning` and `limitations` when
the display manifest provides them. An older display without a known descriptor
keeps the original inventory shape.

## P06 — dated fields

The retained ERA5 pilot is read by exact variable, pressure level, valid time
and native cell. Output includes native value and unit, above/below-ground mask
state, source edition, native indices, declared observation/issue/valid/platform
times and temporal meaning. `--known-at` distinguishes a known-by-cutoff
observation from a modeled value valid at an instant. Missing variables,
unavailable hours, off-grid requests, a pressure axis asked as metres and
below-ground samples are explicit outcomes.

The fixture carries two valid times (`2022-10-23T12:00:00Z` and
`2023-01-09T12:00:00Z`), two pressure levels and above-ground missing samples.
P01 background and common-window rules use the declared source times.

## P07 — display purpose

The display manifest now includes a purpose descriptor:

- schema `emad.geographic-derivative.v1`, purpose `visual-reference`;
- parent members and their SHA-256 values;
- recipe, toolchain digest and representation format;
- metre units and `["east","up","south"]` axes;
- geographic origin, local ENU origin and WGS84 bounds;
- rendering adjustments as named `kind`/`value`/`unit`/`meaning` records
  (identity vertical scale for GLB; a documented 0.6 m drape offset for vector
  display layers);
- feature mapping (`edition:sourceId` for vector layers);
- limitations and `computationalUse: false`.

The API derives the descriptor from the reservation's recipe, stored parent
names and digests, frame and bounds. It is available on the reservation and
upload responses, display lists and the display manifest. Older displays use the
same derivation; an unrecognized recipe does not claim a purpose. The offline
asset
inventory re-resolves every parent member against the archive and rejects a
descriptor that claims computational use.

## P08 — workload evidence

`scripts/ops/measure_environment_workload.py` measures the actual receiving
path against a verified package. The committed run used 15 iterations, 2
warmups, a kit assembled in temporary storage with purpose `native-inspection`,
Windows, CPython 3.12.6, local SSD and the GDAL tooling environment.

| Operation | p50 ms | p95 ms | Budget p95 ms |
| --- | ---: | ---: | ---: |
| verify archive (3 samples) | 29.457 | 110.750 | 500 |
| verify kit (3 samples, includes archive in a fresh process) | 629.073 | 634.968 | 2500 |
| inventory | 3.916 | 4.217 | 300 |
| sample | 2.207 | 2.284 | 150 |
| point | 4.851 | 4.943 | 150 |
| feature | 4.578 | 4.980 | 400 |
| bounded features | 4.711 | 5.231 | 300 |
| field | 0.239 | 0.273 | 100 |

Peak RSS 96,133,120 bytes against a 768 MiB budget. Package bytes 308,491;
member bytes 300,761. Cleanup probe removed. Status `PASS`, no failures. The
record is `testdata/geospatial/owned-reader-workload.json`; its kit inventory
digest is `32a0bf0dafa688f1be681f160b9180704ae8a24b40b158154013ebaf67cfd528`.

The kit verification is deliberately measured in a fresh process, so it includes
Python startup and archive verification. The archive-only p95 is reported
separately. This is a baseline for the owned workload, not a frozen acceptance
run for Riyadh.

## Reproduce the checks

```text
# Python reader, fixture, suitability, kit and workload
cd scripts/data
../../.venv-recovery/Scripts/python.exe -m unittest test_owned_fixture test_environment_reader test_environment_suitability
cd ../ops
../../.venv-recovery/Scripts/python.exe -m unittest test_environment_kit test_measure_environment_workload test_receive_environment_release

# Go decision engine and assessment rules
go test ./internal/geoenv

# Generated client contract
pnpm --dir apps/operator-ui contracts:generate
```

The Python suites currently run 42 reader/fixture/suitability tests and 23
kit/workload/receiver tests. The Go suite includes all 24 shared vectors, the
cross-language fixture digest, assessment validation, manifest evidence
decoding and display purpose. The PostGIS integration case additionally covers
assessment recording, correction, suitability before and after correction, and
refusal after current-rights withdrawal; it requires Docker. The geographic
integration case passed against PostgreSQL/PostGIS on 18 November 2026.

## Not delivered or not established

These are recorded as gaps rather than implied claims.

1. **Real Riyadh physical accuracy.** No independent control set has been
   admitted for the Riyadh area in this delivery. The owned fixture exercises
   the assessment workflow; it does not establish a real-world accuracy claim.
   The responsible role is the geospatial reviewer together with data
   stewardship. Until controls are admitted, real independent-accuracy uses
   remain `unqualified`.
2. **Real Riyadh kit issuance.** Current source decisions do not establish
   native offline analytical use for the real release. Fixture kit acceptance
   and real issuance must be recorded separately. The responsible role is the
   source reviewer.
3. **Installed native-query API.** The Go API exposes release metadata,
   suitability, assessments and display purpose. Exact cell, point, feature,
   field and frame reads run in the receiving process through the shared
   reader; this delivery does not add a server-side asynchronous native-query
   job endpoint. A future endpoint should reuse the same reader and recheck
   access on retrieval.
4. **Frozen commissioning budgets.** The P08 record is an owned baseline. The
   Riyadh and Al Ain workloads, paired database/object restore, over-quota,
   permission-loss and interrupted-publication acceptance runs are not
   performed here. Per-operation I/O counters are not separately instrumented;
   the record includes package and member byte bounds and a temporary-disk
   cleanup probe. Peak RSS covers the measuring process; the isolated
   verification subprocess is not separately sampled.
5. **Legacy display purpose ambiguity.** Purpose for older displays is derived
   from the known recipe and exact stored bytes. Facts not established by that
   recipe, such as a legacy rendering adjustment that was not recorded, remain
   absent rather than guessed.
6. **Operator UI.** The new metadata and decisions are available through the
   API, generated client types and kit documents. No new Environment Library
   screen was added in this change.

## Handover contents

Deliver together:

- this record and the [operator guide](OFFLINE_ENVIRONMENT_KIT_GUIDE.md);
- the owned fixture, answers, digests, topology, field and display descriptor;
- the shared suitability vectors and their derivation script;
- the consumer requirement example and the decision output for the fixture;
- the verified prepared archive and, where issued, the kit inventory digest;
- the workload record with machine, reader and query pins;
- the named gaps above and their responsible roles.
