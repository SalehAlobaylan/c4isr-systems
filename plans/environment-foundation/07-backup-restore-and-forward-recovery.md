# F07 — Paired backup, restore and forward recovery

[Plan index](../README.md)

**Status** The current installed paired snapshot restored in isolation, including Riyadh displays and navigation originals. Previous and current API read compatibility passed on migration 56. [Recovery evidence](../../docs/implementation/RIYADH_RELEASE_RECOVERY.md#f07-paired-backup-and-recovery)

**Implementation owner** Platform operations.

**Dependencies** [F05 — Artifact delivery, integrity and storage lifecycle](05-artifact-delivery-and-lifecycle.md)

## Outcome and baseline

An isolated restore recovers the permitted geographic library, its exact bytes and pending preparation work. Recorded evidence compared 129 tables, 1,588 security definitions and 33 objects totaling 1,938,693,939 bytes. Navigation originals were verified separately. Those counts are a historical baseline and must be regenerated for later backups.

## Features and contracts

Treat database state, object inventory, source decisions, migration version, executable/image identity and toolchain as one recovery set. Bind each inventory to the quiescence window and backup hashes. Include newly added browser assets, basemap dependencies and job reservations.

Record measured recovery time and recoverable data cutoff. Use these measurements to agree an operational recovery objective with the owner. Retention and revocation decisions made after the backup must be reconciled before exposing restored data.

## Implementation sequence

1. Consolidate the navigation and geographic inventories into the normal backup procedure.
2. Quiesce relevant writers, capture the paired set and verify required objects against hashes.
3. Restore into an isolated database and object namespace and reapply current effective access policy.
4. Verify release discovery, scoped downloads, offline integrity and pending-job recovery.
5. Test the exact nominated previous binary against additive schema in isolation. Record whether binary rollback is supported or forward recovery is required.

## Acceptance

- The restored inventory matches the captured eligible dependencies and security definitions.
- Corrupt, missing or revoked dependencies keep affected releases unavailable.
- Pending work resumes with valid fencing and one publication.
- A clean consumer retrieves and verifies the same exact release through normal API paths.
- An unauthorized identity receives no protected metadata or content.
- Recovery duration and data cutoff are measured, including reconciliation time.
- Destructive rollback is absent from the qualified recovery procedure.

## Rollout and recovery

Retain the current forward-recovery path until the previous binary passes explicit compatibility checks. Preserve the failed environment for diagnosis where retention allows. Run restorations into new namespaces and keep activation as an explicit operational action.

## Deliverable evidence

Produce backup and restore manifests, current-policy reconciliation results, consumer verification and an executable recovery runbook. Store secrets outside the evidence bundle. F09 uses the rehearsal as one readiness gate.

## Existing implementation references

- [scripts/ops/recovery_inventory.py](../../scripts/ops/recovery_inventory.py)
- [scripts/ops/qualify_recovery.py](../../scripts/ops/qualify_recovery.py)
- [scripts/ops/qualify_rollback.py](../../scripts/ops/qualify_rollback.py)
- [docs/implementation/ENVIRONMENT_LIBRARY_OPERATIONS.md](../../docs/implementation/ENVIRONMENT_LIBRARY_OPERATIONS.md)

[Foundation evidence](../../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md) · [Original implementation plan](../../docs/implementation/ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md)
