# F03 — Release review and independent handoff

[Plan index](../README.md)

**Status** Exact archive checks, scoped decision history and the receiver workflow passed verification. The exact Riyadh release, receiver download and withdrawal also passed in isolation. Installed human release and independent receiving decisions remain open. [Riyadh release workflow](../../docs/implementation/RIYADH_RELEASE_RECOVERY.md#f03-release-review-and-receiving)

**Implementation owner** Environment product engineering and authorized reviewers.

**Dependencies** [F01 — Source admission, lineage and retention](01-source-admission-and-lineage.md), [F02 — Geographic profiles, coordinates and quality](02-geographic-profiles-and-quality.md)

## Outcome and baseline

A reviewer can verify the exact package, release it with an attributable decision, and hand it to an independent consumer. The installation evidence records the three candidate manifests as STAGED. The existing workflow supports STAGED → VERIFIED → RELEASED and withdrawal.

## Features and contracts

Bind every action to release ID, full manifest SHA-256, principal, notes and time. Show coverage, source dates, uncertainty, attribution and missing capabilities before the review action. Treat publication readiness, reviewed geographic meaning and physical accuracy as separate report fields.

Provide an independent consumer workflow for discovery, metadata inspection, permitted download and offline verification. A consumer needs source and download grants but does not need preparation or review authority. Review notes describe an actual performed check. Service workers retain execution-only authority.

## Implementation sequence

1. Prioritize the Riyadh reference release for review and handoff. Re-read each selected candidate's state and compare its exact manifest digest with the recorded preparation evidence.
2. Improve the reviewer checklist using existing manifest fields and attach verification evidence references.
3. Make repeated submissions and concurrent transitions return the established result or a conflict without duplicate decisions.
4. Test a separate consumer identity completing discovery and verification from a clean directory.
5. Record the human walkthrough and release decision for each exact candidate. Preserve unresolved questions as open evidence.

## Acceptance

- Tampered bytes or a changed digest prevent verification.
- A reviewer cannot accidentally approve an older selection after switching releases.
- Retries after a lost response preserve one transition and its original actor.
- A worker cannot verify or release, and a consumer cannot acquire review authority.
- An independent participant finds the package, explains its limits and verifies the exact digest.
- Withdrawal removes new download availability while retaining the authorized audit history.

## Rollout and recovery

Use the existing states and permissions. Reconcile ambiguous client responses by reading the release before retrying. Correct a faulty review through withdrawal and a new attributable decision according to the existing state model. Technical development and staged verification can proceed while genuine human acceptance remains pending.

## Deliverable evidence

For each candidate retain its state history, notes, independent verifier result and participant walkthrough. Completion requires actual human decisions and consumer evidence, which automated credentials cannot substitute.

## Existing implementation references

- [internal/geoenv/releases.go](../../internal/geoenv/releases.go)
- [internal/geoenv/http.go](../../internal/geoenv/http.go)
- [apps/operator-ui/src/features/workspace/EnvironmentLibrary.tsx](../../apps/operator-ui/src/features/workspace/EnvironmentLibrary.tsx)
- [scripts/ops/check_installed_environment_library.py](../../scripts/ops/check_installed_environment_library.py)

[Foundation evidence](../../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md) · [Original implementation plan](../../docs/implementation/ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md)
