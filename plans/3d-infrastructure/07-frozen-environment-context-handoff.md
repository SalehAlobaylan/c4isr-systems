# V07 — Frozen environment and observation handoff

[Plan index](../README.md)

**Status** Implemented through the context-package API and Environment Library. Correction, cutoff, concurrent retry, withdrawal and the independent reader passed with explicitly permitted fixture data. Riyadh awaits release and source authorization. [Implementation evidence](../IMPLEMENTATION_PROGRESS.md#context-handoff-and-viewer-qualification)

**Implementation owner** Data contracts and backend engineering.

**Dependencies** [F03 — Release review and independent handoff](../environment-foundation/03-release-review-and-handoff.md), [F05 — Artifact delivery, integrity and storage lifecycle](../environment-foundation/05-artifact-delivery-and-lifecycle.md)

## Outcome and baseline

A future application can request an immutable geographic release and an explicit set of contextual observations, then reopen the same permitted data. Existing context bundles and clock contracts provide useful building blocks. Current retained map read handles expire after five minutes.

## Features and contracts

Define a versioned handoff manifest referencing exact geographic and display digests, AOI revision, observation identities/revisions, observation window, availability cutoff, source times, publication times when supplied, retrieval times, usage terms and missing-data policy.

Support explicit time interpretations. Observations occurring within a period and evidence known by a cutoff are different predicates. Retain those predicates and use stable ordering by source time plus a deterministic tie-breaker. Preserve original timestamps even when a consuming application later maps them to its own clock.

## Implementation sequence

1. Extend the existing context bundle contract additively to reference geographic releases and retained reported observations.
2. Materialize selected authorized members from a consistent scoped read before its handle expires.
3. Validate dependency lifetime, source grants, corrections and export permission.
4. Publish an immutable manifest and independent reader command.
5. Reopen the package through a fresh client after new upstream revisions arrive and compare values.

## Acceptance

- Reopening an exact bundle returns its original selected revisions or an explicit unavailable-dependency result.
- A corrected upstream observation does not silently replace a pinned record.
- Known-by-cutoff selection excludes later-arriving evidence and retains the reason.
- Source observation and retrieval time remain distinct.
- A missing or withdrawn dependency fails clearly without substituting a current record.
- Concurrent materialization and retries yield consistent immutable membership.
- Authorized users can inspect the manifest and its limitations without simulation execution.

## Rollout and recovery

Keep existing context schemas readable and add an explicit new version for changed semantics. Backfill no implied timestamps. Restore manifests and member inventories through F07, applying current policy before serving them.

## Deliverable evidence

Deliver a frozen input manifest, correction/cutoff fixtures, an offline reader and repeated-open evidence. This is a data handoff contract. Physics, actor behavior and runtime clock execution remain future consumer design decisions.

## Implementation and source references

- [internal/contextpackage/domain.go](../../internal/contextpackage/domain.go)
- [internal/contextpackage/service.go](../../internal/contextpackage/service.go)
- [internal/scenarios/frozen.go](../../internal/scenarios/frozen.go)
- [internal/analystscope/reads.go](../../internal/analystscope/reads.go)
- [World Monitor src/services/bootstrap.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/services/bootstrap.ts)
- [World Monitor docs/architecture.mdx](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/docs/architecture.mdx)
