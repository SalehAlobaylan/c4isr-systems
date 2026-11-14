# Geographic source, quality and storage qualification

14 November 2026. F01, F02 and F05 are implemented and qualified in a disposable copy of the shared migration 58 installation. The migration 59 candidate used API and worker image `sha256:2412db850e5a9fc278d3c3b95c81bf64d009a5e08fbdd553596fd57e38ab1b4e`. The copied installation identity was `0f02ca2fe496e0fb6aef86782d0bfc3a171ed093b309590935634f2e255572ea`.

## F01 Source policy and lineage

The steward portal and command-line review workflow can inspect a retained source by ID after the source restricts normal release access. They show the original and live policy context, affected releases, display counts, pending uploads, tombstones and expiry. The renewal records the steward, terms, intended use and expected head. A new revision preserves the original policy identity of prior releases. Routine release and export requests enforce the intersection of original and current rights.

The Riyadh copy returned ten retained inputs across four source policies. The earliest effective expiry for each was 10 February 2027. All four impact reads identified the installed Riyadh release and denied the reviewer credential without stewardship permissions. The assembled PostGIS test denied an otherwise authorized steward with a different source grant. It exercised a renewal, rejected a stale revision, checked frozen lineage under tightened rights and verified that a revoked release remained visible for authorized impact review. The expiry fixture retained a tombstone across a storage fault and completed cleanup on retry.

The three retained national and regional navigation parents matched the paired inventory. Riyadh provides the detailed lineage and renewal rehearsal. Additional Saudi and Middle Eastern editions require individual terms, coverage and a recorded owner. Steward approval for live renewals and Riyadh redistribution is pending.

## F02 Profile and geographic quality

New recipes pin the complete versioned geographic profile contract and its SHA-256. Profile discovery and preflight report retained formats, coordinate rules, source footprints, raster registration rules and bounded processing limits. The structured report binds its frame, source uncertainty, sample counts and legacy unknowns to the selected manifest. The portable verifier reads the GeoTIFF registration and checks the ENU controls independently of the stored summary. The pinned NGA EGM2008 grid, interpolation method and existing processor identity remain available for historical reproduction.

The HTTP worker rebuilt the Riyadh reference on its first attempt. The prepared ZIP was 5,716,535 bytes. All ten member hashes and their logical identities matched the installed release. The new manifest SHA-256 is `ea9430b40812e98ec698ca5f90a17013f03ff7b3424b8011cbc4505b764b83bf`. Its profile contract digest is `fa11011894073cfa7a88e0be7f7df2146935a0acdbebcb8ba44096394ab3b437`. Its prepared ZIP SHA-256 is `61862e80c35d7090c0eecfe3495cdb96bdd503b0ae4e0a0c5100492f17f9dde7`.

Seven retained packages passed portable verification, covering four reference-context packages and the enrichment, observation and coastal profiles. Across 21 ENU controls, maximum disagreement was 1.68 × 10⁻⁹ metres. The verifier checked 45 recorded raster registrations. The processor tests covered disjoint areas, holes, nodata masks and seams. The Riyadh physical accuracy field remains `UNKNOWN` while independent local control is pending.

## F05 Artifact delivery and lifecycle

The old API served the installed manifest, prepared and display identities after migration 59. The candidate API returned the same bytes for Riyadh, Al Ain and Avignon. Paired recovery restored and read back 59 objects totaling 2,865,321,183 bytes. The new download client resumed a deliberately cut HTTP stream from its last verified offset, rejected a changed ETag and removed the partial file. Assembled API scenarios checked invalid ranges, corrupt uploads, retries, revocation and access to lifecycle metadata.

Storage capacity includes pending, ready and unpurged objects across all of an owner's source grants. A test filled the 16 GiB allowance with inaccessible unpurged records, verified that another reservation would exceed capacity and confirmed that successful purge returned capacity without exposing the hidden records. When two simultaneous reservations could individually fit the remaining capacity, one succeeded and the other received a conflict. Physical deletion uses the recorded retention deadline and an upload grace window. A failed deletion backs off while other eligible objects can be reclaimed. Active policy changes take effect on the next request. A separately revised physical deletion obligation requires a storage operator action.

An isolated Garage-backed transfer accepted and verified a 1 GiB fixture at the object limit. The scoped full download returned the exact digest, and an exact `If-Range` returned the final 8,192 bytes. The API process high-water RSS stayed at 25.5 MiB across upload and download, within the rehearsal's 128 MiB additional-memory budget. The measurement represents the API process for one transfer. It excludes object-store memory, filesystem cache and concurrent workloads.

The production frontend built as `emad-ui:environment-foundation-59` with image `sha256:4528eb597f0bc215e76f40b829f5791fba14c08a394faff684d1d60d2efaa680`. The generated client matched OpenAPI. Four Chromium tests passed, including the new source, quality and storage panels at 390×844 and 740×360. The Go suite, twelve Python operations tests, eleven geographic data tests and the assembled geographic API integration test passed.

## Rollout and evidence

Shared staging remains on migration 58 during F09. After the current full window is recorded, qualify the migration 59 staging cutover and recheck scoped reads, worker health and queue state. The shared release and independent receiving decisions remain open. [Current F09 record](COASTAL_MOVING_AND_READINESS.md#f09--installed-readiness) · [Operations instructions](ENVIRONMENT_LIBRARY_OPERATIONS.md)

The private qualification directory is `private/environment-foundation/implementation/coastal-moving-qualification`. It holds the commands and reports below. The image and report identities bind this rehearsal to the worktree build.

- `foundation-59-qualification.json` SHA-256 `c1964ff406685a137fd617fa0a40f1ca708fd67269e0e8e0c3ebb8dc8f8b65a3`.
- `foundation-59-copy.json` SHA-256 `34b4a3b26b9362228af72a170473cf0fe31aae5627231873aa13d7cf67a2093e`.
- `portable-coordinate-compatibility-59.json` SHA-256 `60ec3ad9f1df507391de0b934dd82d0407890c55b839bddda8cd42ff68e26d6`.
- `foundation-59-transfer-limit.json` SHA-256 `f571ec6430d0bd17540f668ade83c96cb6ea40adfd50537695e42bfbf9a6f75f`.
