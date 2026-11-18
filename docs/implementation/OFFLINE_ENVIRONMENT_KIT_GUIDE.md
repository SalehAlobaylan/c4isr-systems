# Offline environment kit: operator guide

This guide is for the receiving analyst or developer. A kit opens without the
viewer, database, network or source rebuild archive. Every answer names the
member, digest and reason code it used, and a measured zero remains the number
`0.0`.

## Install the reader

Use the locked optional environment. It is separate from the server and worker
dependencies.

```text
python -m venv .venv-reader
.venv-reader/Scripts/python -m pip install -r scripts/data/reader-requirements.txt
```

The verified baseline is CPython 3.12.6 with the versions pinned in
`scripts/data/reader-requirements.txt`. The reader also runs on the existing
geospatial tooling environment used by preparation.

## Obtain the trusted pins first

Ask the issuing team for these values through a channel separate from the kit
itself:

- release ID and exact `manifestSha256`;
- exact prepared archive SHA-256 and byte count;
- kit inventory SHA-256;
- issued operation, purpose, terms and expiry;
- any selected display identities and digests.

Never accept an inventory digest that was read from the kit you are verifying.
The receiver requires `--kit-inventory-sha256` for this reason.

## Receive and verify a kit

```text
.venv-reader/Scripts/python scripts/ops/receive_environment_release.py \
  --api https://api.example.internal \
  --workspace <workspace> --environment <environment> \
  --token-file /run/secrets/environment-receiver \
  --release <georelease_...> \
  --manifest-sha256 <64 hex> \
  --prepared-sha256 <64 hex> \
  --kit \
  --kit-inventory-sha256 <64 hex> \
  --kit-purpose native-inspection \
  --kit-terms issued-terms.json \
  --kit-now 2026-11-18T00:00:00Z \
  --kit-sample raster-00-00.tif:10:10 \
  --kit-feature roads:osm-way-1001 \
  /data/received-environment
```

`--kit` writes to private temporary storage and publishes the complete
directory only after the archive, selected displays, inventory and sample
checks pass. A retry against the same trusted identity resumes; a changed pin
or an existing different kit is refused. Kit publication requires issued terms
with an explicit expiry and a `read-native` grant. The same command without
`--kit` retains the original single-archive receipt format.

Without the receiver, verify a delivered directory directly:

```text
.venv-reader/Scripts/python scripts/ops/environment_kit.py verify \
  /data/received-environment --inventory-sha256 <64 hex> \
  --now 2026-11-18T00:00:00Z
```

The result reports `status: VERIFIED_KIT`, the archive verification, every
selected display and the inventory profile/layers. The kit helpers check the
inventory identity, prepared digest and issued terms before every local read;
the full profile archive verification runs during `verify`.

## Read native values

For a kit, serve reads through the kit helper so the inventory identity and
issued terms are checked first:

```text
.venv-reader/Scripts/python scripts/ops/environment_kit.py sample \
  /data/received-environment --inventory-sha256 <64 hex> \
  --now 2026-11-18T00:00:00Z \
  --member raster-00-00.tif --row 10 --column 10
```

The direct reader below operates on a verified prepared archive. It checks the
manifest identity but does not itself establish kit terms; run the kit
verification first when the archive came from a kit. `environment_kit` exposes
the same expiry-checked path programmatically as `native_read` and
`native_point`.

```text
# One exact cell. A valid zero reports value 0.0.
.venv-reader/Scripts/python scripts/data/environment_reader.py sample \
  /data/received-environment/prepared.zip \
  --manifest-sha256 <64 hex> \
  --member raster-00-00.tif --row 10 --column 10

# One point, with the documented seam convention.
.venv-reader/Scripts/python scripts/data/environment_reader.py point \
  /data/received-environment/prepared.zip \
  --manifest-sha256 <64 hex> \
  --layer surface --longitude 46.73 --latitude 24.7055 --component 0

# One source feature, including all clipped parts and topology evidence.
.venv-reader/Scripts/python scripts/data/environment_reader.py feature \
  /data/received-environment/prepared.zip \
  --manifest-sha256 <64 hex> \
  --layer roads --source-id osm-way-1001

# A bounded selection and the local frame.
.venv-reader/Scripts/python scripts/data/environment_reader.py features \
  /data/received-environment/prepared.zip --layer roads \
  --bounds 46.70,24.70,46.76,24.76 --limit 100
.venv-reader/Scripts/python scripts/data/environment_reader.py frame \
  /data/received-environment/prepared.zip --longitude 46.72 --latitude 24.72 \
  --check-controls
```

Outcomes are explicit rather than silently substituted:

| Outcome | Meaning |
| --- | --- |
| `value` | a native value, possibly `0.0`; `maskState` states whether a field cell is above or below ground |
| `masked` | the retained mask marks the cell invalid |
| `outside-area` | the coordinate is outside the release area, including a hole |
| `outside-member` | the cell index is outside that member |
| `uncovered` | no native member covers the coordinate |
| `absent` | the requested feature or variable is absent from the release |
| `unsupported-layer` | the requested native layer is not retained |
| `unsupported-conversion` | a datum/grid conversion is unavailable; see `reason` |
| `unsupported-unit` | the requested unit has no exact conversion; the native value is included |
| `conflicting-overlap` | two retained members disagree at the coordinate |
| `off-grid` | the request is not on a native cell |
| `unavailable-hour` / `unavailable-before-validity` | the requested field time is not retained or not yet valid |

## Read a dated atmospheric field

```text
.venv-reader/Scripts/python scripts/data/environment_reader.py field \
  testdata/geospatial/owned-fixture-v1/field-era5.json \
  --variable temperature --level-hpa 850 \
  --valid-time 2022-10-23T12:00:00Z \
  --longitude 46.5 --latitude 25.25

.venv-reader/Scripts/python scripts/data/environment_reader.py field-times \
  testdata/geospatial/owned-fixture-v1/field-era5.json
```

`field` returns the native value and unit, the pressure level and valid time,
`aboveGround`, the source edition and temporal meaning. A below-ground cell or a
missing sample is a documented outcome, not zero. `--known-at` records the
cutoff for an observation that was known by a particular instant.

## Decide whether a release fits a use

```text
.venv-reader/Scripts/python scripts/data/environment_reader.py suitability \
  /data/received-environment/prepared.zip \
  --requirements requirement.json \
  --assessments assessments.json \
  --terms issued-terms.json \
  --now 2026-11-18T00:00:00Z
```

Use `--evidence evidence.json` instead of a package when a reviewer supplies a
complete evidence document. The result is `supported`, `unsupported` or
`unqualified`. Optional omissions are listed separately and never change the
overall result. Each decision carries a reason code and the release evidence
digest; an assessment decision also names the assessment and its pinned
`evidenceSha256`.

A minimal native-surface requirement looks like this:

```json
{
  "schema": "emad.environment-requirement.v1",
  "requirementId": "example-native-surface",
  "operation": "read-native",
  "release": {"manifestSha256": "<64 hex>", "areaRevisionId": "<area revision>"},
  "resources": {"maxMembers": 64, "maxBytes": 1073741824},
  "representations": [
    {"id": "surface", "layer": "surface", "required": true, "units": "metre",
     "accuracy": {"mode": "unknown-acceptable"}}
  ]
}
```

Require independent accuracy with
`"accuracy": {"mode": "independent-assessment", "use": "<declared use>", "toleranceM": 0.25}`
and add `"area": {"geometry": {...}, "allowPartial": false}` to check spatial
coverage. The same document is accepted by
`POST /api/v1/environment-releases/{id}/suitability`.

Common reason codes:

| Reason | Interpretation |
| --- | --- |
| `release-mismatch` | the requirement names a different manifest |
| `representation-absent` | a required native layer is not retained |
| `visual-only-derivative` | only a display derivative exists |
| `datum-mismatch` / `unit-unsupported` | the requested datum or unit cannot be established exactly |
| `mask-evidence-below-requirement` | retained valid-sample evidence is below the stated fraction |
| `accuracy-unevidenced` | independent accuracy is required but no accepted assessment exists |
| `assessment-member-mismatch` | the assessment names a different member digest |
| `assessment-area-insufficient` | the assessment covers less than the requirement area |
| `accuracy-exceeded` | a proven statistic is above the declared tolerance |
| `terms-expired` / `grant-withdrawn` | the issued terms no longer authorize a new read |
| `area-partially-covered` | part of the requirement is outside the release area |

## Inspect a display derivative safely

A display asset is a visual reference, never a native value source. The
display manifest now carries a purpose descriptor with parent members,
recipe/toolchain, units, axes, local origin, bounds, rendering adjustments,
feature mapping and limitations. For a kit, the display entry repeats the
purpose and limitations.

```text
.venv-reader/Scripts/python scripts/data/environment_reader.py asset \
  /data/received-environment/prepared.zip \
  --descriptor display-descriptor.json
```

The descriptor must declare `purpose: visual-reference`, metre units,
`["east","up","south"]` axes, at least one surface parent member and
`computationalUse: false`. A native-ground request rejects a visual-only
derivative.

## Expiry and deletion

Verify the evaluation instant whenever the kit states an expiry. An elapsed
expiry, an unrecognized instant or a missing grant returns a specific failure
(`kit-expired`, `kit-expiry-unknown`, `kit-terms-missing`) before any local
read. On an early withdrawal, stop serving
reads and delete the kit directory according to the issued terms; a disconnected
copy learns about the withdrawal when it reconnects to the authorized release
record. Do not retain derived copies after the issued retention date.
