# F01 — Source admission, lineage and retention

[Plan index](../README.md)

**Status** Implemented and qualified in the migration 59 staging copy. Live policy decisions and shared rollout are pending. [Source and lineage evidence](../../docs/implementation/GEOGRAPHIC_SOURCE_QUALITY_AND_STORAGE.md#f01-source-policy-and-lineage)

**Implementation owner** Data platform engineering and data stewardship.

**Dependencies** Existing platform services and repository build baseline.

## Outcome and baseline

A steward can discover every retained input, inspect its origin and permitted uses, and identify which environments would be affected by a policy change. The installed library already carries admissions, source grants and retention. The recorded private installation policies expire on 10 February 2027. Navigation retains three Natural Earth archives and covers 17 countries. Those facts establish the initial inventory and renewal workload.

## Features and contracts

Extend the existing catalog with an authorized dependency view connecting provider edition, acquisition receipt, original object, prepared member, release and display derivative. Store new decisions as admission revisions. Evaluate the applicable original and current permissions on preparation, download and export. Scope every relationship and count by the requesting principal.

Add a source operations view with admitted uses, attribution, retained bytes, policy expiry, missing parents and dependent releases. Keep geographic display, raw redistribution and portable export permissions explicit. Record code licenses separately from dataset and hosted-service terms.

## Implementation sequence

1. Reconcile existing geographic objects and regional archives against retained receipts. Audit retained inputs and rights for the first bounded Riyadh study area and record missing layers, coverage and metadata.
2. Implement dependency queries and policy impact previews with bounded pagination. Reuse the existing source admission and authorization model.
3. Add expiry reminders and a renewal workflow that records reviewer, terms reference, date, intended use and retention. Assign operational owners during commissioning.
4. Expose source lineage in the Environment Library and derivative inspector. Add tombstone and policy revision events to the recovery inventory.

## Acceptance

- A consumer with one source grant cannot enumerate a release requiring another source.
- Identical bytes admitted in two scopes retain distinct authorization and policy bindings.
- Revoking a parent blocks subsequent protected metadata, full downloads and range reads.
- Existing releases preserve their original provenance after renewal. New effective rights are evaluated explicitly.
- Navigation and detailed releases resolve to exact retained parents or an explicit missing state. The Riyadh intake identifies each required input and any gaps before preparation.
- An expiry exercise records the affected releases and successful cleanup without exposing credentials.

## Rollout and recovery

Use additive metadata and indexes. Backfill missing values as unknown and keep existing valid readers working. Ship the dependency view before reminders. Reuse F05 for deletion and F07 for restored-policy reconciliation. A downloaded authorized copy follows its issued retention terms. Online withdrawal affects subsequent server access.

## Deliverable evidence

Save an inventory digest, source-to-release dependency examples, denied-read results and a renewal/expiry walkthrough. Keep credentials and raw protected source documents in private storage. Record remaining source decisions by exact product edition.

## Existing implementation references

- [internal/geoenv/admission.go](../../internal/geoenv/admission.go)
- [internal/geoenv/catalog.go](../../internal/geoenv/catalog.go)
- [internal/publicingest/domain.go](../../internal/publicingest/domain.go)
- [scripts/data/region_profiles/environment-intake-v1.json](../../scripts/data/region_profiles/environment-intake-v1.json)

[Foundation evidence](../../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md) · [Original implementation plan](../../docs/implementation/ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md)
