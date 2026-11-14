# F09 — Readiness evidence and operational qualification

[Plan index](../README.md)

**Status** The 15:03 Riyadh observation on 14 November ended at 17:54 after 1,710 successful requests. Shared staging recovered on the same migration 58 installation fingerprint and a fresh window started at 23:06. Human release and independent receiver decisions remain open. [Installed readiness evidence](../../docs/implementation/COASTAL_MOVING_AND_READINESS.md#f09--installed-readiness)

**Implementation owner** Quality engineering and receiving team.

**Dependencies** [F03 — Release review and independent handoff](03-release-review-and-handoff.md), [F06 — Portable verification and reproducible rebuild](06-portable-verification-and-rebuild.md), [F07 — Paired backup, restore and forward recovery](07-backup-restore-and-forward-recovery.md), [F08 — Operations, metrics, alerts and commissioning](08-operations-metrics-and-commissioning.md), [F10 — Contracts, CI and upgrade compatibility](10-contracts-ci-and-upgrade-compatibility.md)

## Outcome and baseline

A single evidence record states which preparation capabilities are accepted and which remain open for the exact installed candidate. The source report contains successful rebuild, restore, concurrency and browser checks. Human review, the independent workflow and final 24-hour qualification were open when recorded.

## Features and contracts

Create an acceptance record binding source commit, images, configuration, migrations, toolchain, profile revisions and the exact release IDs and digests. Include Riyadh in the three-release workload used by the current runner. Select the other two releases for workload and geographic coverage, and record each release's role. Link separate results for metadata reads, queue recovery, resource bounds, portability, browser use and human handoff.

Preserve the distinction between a completed prerelease observation and SOAK_PASS. Record monotonic and wall-clock duration, failed requests, p95 latency, sampling gaps, worker restarts and storage trends. Changed behavior invalidates the affected evidence, with the reason recorded.

## Implementation sequence

1. Reconcile recorded results with the actual installation and mark expired or inapplicable evidence.
2. Run the existing volume cases in isolated qualification storage.
3. Observe five metadata readers for 24 real hours against the pinned workload including Riyadh and restart one worker during that window.
4. Capture queue, lease, reserved-byte and storage trends alongside request results.
5. Complete independent review/consumer steps and assemble the readiness report with all unresolved items.

## Acceptance

- Five readers exercise three pinned releases with zero failed requests, no prohibited sampling gaps and metadata p95 at most 1 second.
- The 24-hour run uses real wall time and includes a recorded worker restart.
- Two bounded preparations recover consistently with unloaded pickup within the existing 60-second budget.
- The 10,000 metadata and 100,000 vector workloads stay within their declared time/memory/disk envelopes.
- No acknowledged data loss or unexplained storage/lease growth occurs.
- All three workload releases are approved by completion. An independent receiving operator discovers and verifies the exact Riyadh release through the ordinary workflow.
- Saudi national navigation and Middle East overview report their retained reference editions and geographic and layer coverage. Detailed readiness claims cite the specific prepared areas and layers exercised.
- Failed gates remain visible and prevent a complete readiness claim.

## Rollout and recovery

Use new evidence filenames per attempt and retain failed attempts. Pause only the affected acceptance run when the candidate changes. Run destructive fault fixtures in the dedicated staging/restore scope. Continue useful engineering while human decisions or observation time are pending.

## Deliverable evidence

Publish the machine-readable acceptance ledger and a short human-readable report. Baseline thresholds come from the existing foundation implementation plan and describe engineering qualification. F09 qualifies the preparation foundation. V08 separately qualifies the new viewer.

## Existing implementation references

- [scripts/ops/qualify_environment_library.py](../../scripts/ops/qualify_environment_library.py)
- [scripts/ops/test_qualify_environment_library.py](../../scripts/ops/test_qualify_environment_library.py)
- [scripts/ops/commission_environment_library.py](../../scripts/ops/commission_environment_library.py)
- [docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md](../../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md)

[Foundation evidence](../../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md) · [Original implementation plan](../../docs/implementation/ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md)
