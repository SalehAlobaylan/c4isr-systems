# F05 — Artifact delivery, integrity and storage lifecycle

[Plan index](../README.md)

**Status** Implemented and qualified in the migration 59 staging copy, including an exact upload and download at the 1 GiB object limit. Shared rollout is pending. [Delivery and lifecycle evidence](../../docs/implementation/GEOGRAPHIC_SOURCE_QUALITY_AND_STORAGE.md#f05-artifact-delivery-and-lifecycle)

**Implementation owner** Storage and backend engineering.

**Dependencies** [F01 — Source admission, lineage and retention](01-source-admission-and-lineage.md)

## Outcome and baseline

Originals, packages and future display assets can be uploaded, verified, downloaded and expired with bounded memory and storage. Existing streaming supports full responses and single ranges, and an interrupted rebuild aborts the transfer. The reconciler tombstones expired objects before deletion.

## Features and contracts

Reuse geographic object identities, length and SHA-256 checks, reservations and scoped content routes. Separate an uploaded object’s verified state from its publication eligibility. Track parent and derivative retention dependencies. Authorization applies even when bytes are content-deduplicated.

Define range behavior, conditional requests, download filename handling and resumable client semantics in OpenAPI. A resumed client must bind segments to one ETag and reject changed identities. Keep protected content out of shared public caches. Any explicit offline export follows its granted policy.

## Implementation sequence

1. Audit every path for bounded stream sizes, response headers, temporary files and cancellation.
2. Extend lifecycle records to include display artifacts and their parent dependencies.
3. Implement the required conditional range behavior and atomic client-side finalization for interrupted downloads.
4. Add a bounded reconciliation view for incomplete uploads, failed publications, expired objects and unreclaimed bytes.
5. Measure reserved versus actual bytes and apply deterministic cleanup retries after object-store failure.

## Acceptance

- Full and range responses return the exact expected bytes, length and identity.
- Invalid ranges, corrupt parents and truncated object reads cannot report successful complete downloads.
- Revocation between reads blocks the next request. An active authorized stream follows the documented cancellation behavior.
- A failed upload or publication exposes no completed artifact.
- Retrying deletion after a crash preserves tombstones and does not delete a still-required eligible dependency.
- Concurrent reservations respect quota and release capacity after failed attempts.
- Large-object transfers stay within the configured memory envelope.

## Rollout and recovery

Use additive lifecycle records and retain the existing ZIP download route. Reconcile uncertain writes by identity before retrying. Stop new reservations if capacity accounting cannot be established. F07 includes the expanded object inventory in paired backups.

## Deliverable evidence

Retain range, corruption, quota, revocation, interrupted-download and deletion-retry results with object digests and peak resource measurements. Document effective online and exported-file retention behavior.

## Existing implementation references

- [internal/artifacts/stream.go](../../internal/artifacts/stream.go)
- [internal/geoenv/http.go](../../internal/geoenv/http.go)
- [internal/geoenv/spool.go](../../internal/geoenv/spool.go)
- [internal/geoenv/reconcile.go](../../internal/geoenv/reconcile.go)

[Foundation evidence](../../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md) · [Original implementation plan](../../docs/implementation/ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md)
