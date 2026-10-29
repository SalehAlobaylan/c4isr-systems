# Map-first plan completeness and hardening audit

**Date:** 25 October 2026.  
**Scope:** Second-pass challenge of the [implementation plan](MAP_FIRST_IMPLEMENTATION_PLAN_2026_10_25.md), against the [18-gap report](MAP_FIRST_GAP_REPORT_2026_10_25.md), [governing scope](../../EMAD_CURRENT_SCOPE.md), code and delivery tooling.  
**Implementation baseline:** `a6d1d22f34692faea31db58e4a6318c539c3321a`.  
**Outcome:** Twelve design/closure weaknesses were identified and addressed in the plan. All 18 gaps now have 54 explicit requirement statements, mapped to the existing 24 implementation tasks and 24 acceptance scenarios. These are specified solutions, not verified product fixes.

## What the first validation established, and what it missed

The first pass correctly checked task counts, all-gap coverage, dependency cycles and local links. That could still pass with an incomplete design: a gap can have a task ID without specifying concurrent-read behavior, retention consequences or a review/publication race. The second pass challenged the contracts and execution sequence directly, including existing recovery and packaging code.

This was a direct repository audit. No live provider activity, application migration, operational deletion, deployment, user session or application acceptance run occurred. The added Python harness checks planning artifacts only.

## Findings and resolutions

| Check | Weakness in the earlier plan | Concrete hardening | Implementation / proof |
| --- | --- | --- | --- |
| H01 | Freezing an area/interval and using a cursor did not freeze membership; late commits/corrections could change totals or page contents. | D02 defines bounded materialized read snapshots, exact revision membership, expiry and live access checks, distinct from scope and historical time semantics. | T05, T13, T22; A22 plus A04/A09 |
| H02 | Provenance was preserved, but fact/report/indicator/assessment distinctions and later correction impact were not an explicit end-to-end contract. | D14 carries semantic/quality/geometry distinctions and reverse-dependency validity notices; original approved content remains immutable. | T12, T16, T19; A21 |
| H03 | “Keep original bytes/history” was underspecified when rights expire or bytes are legitimately deleted. | D15 defines permitted retention, composite dependency disposition, tombstones, partially reproducible cases and restore expectations. | T16, T18, T22; A18/A23 |
| H04 | Case states existed, but transition authority, owner departure, exact finding revisions and cross-account draft recovery were incomplete. | D03/D04 define lifecycle permissions, atomic saves, ownership recovery, archival semantics and protected cache/draft handling. | T06, T07, T16; A10/A12/A23 |
| H05 | Frozen output/review were required without a complete render-before-review protocol or handling of the final approval label and output backlinks. | D11 defines draft rendering, content-digest approval and a separate attestation; operational backlinks do not revise their own approved content. Edits/renderer changes invalidate eligibility. Escaping, script-free downloads and bounded assets cover the new HTML surface. | T17–T19; A13/A24 |
| H06 | General coverage states did not force per-adapter operation support or account for rejected/unpublished/truncated retrievals. | D08 requires an adapter capability inventory and receipts that distinguish retrieval from publication and complete from capped windows. | T10–T12; A06/A07 |
| H07 | Durable jobs and shared quotas lacked explicit accounting for crashes after sending a billable request or concurrent storage reservations. | D07 specifies conservative uncertain charges, atomic intent/job registration, storage reservation/reconciliation and bounded terminal retries. | T10, T15, T19, T21, T22; A06/A17/A24 |
| H08 | Map startup independence could pass while packaging and recovery still required simulation executables or missed new writers. | T02 includes package checking and platform paths; upgrade/T22 inventory and quiesce all writers including retention/render/environment jobs. | T02, T22; A01/A18 |
| H09 | T10 could remove the history cap before T13 paging; final user qualification could precede operational qualification. | T10 installs guardrails and retains the cap; T13 depends on T10 and replaces it with paging. Final T23 depends on T22; early formative sessions remain permitted. | T10, T13, T22, T23; A09/A17/A19 |
| H10 | Gate evidence could describe different builds, or mandatory external journeys could be relabeled optional to claim all gaps closed. | Section 10 binds qualification to a candidate, defines evidence invalidation/retest and requires all 24 scenarios. Missing humans/real inputs remain open prerequisites. | T22–T24; A18–A20 |
| H11 | Portable output checks named accuracy and LOD without requiring frozen tolerances or processing resource limits. | T21 requires source-supported accuracy/control criteria and pixel/feature/mesh/disk/RAM/time budgets before execution. | T20/T21; A16/A17 |
| H12 | The successful one-off consistency check was not retained and could not prevent subsequent backlog drift. | Added the maintained validator, source fingerprints, atomic requirements, acceptance records and failure-injection checks. | Planning harness now; T24 maintains it; A20 closes product evidence |

## Code evidence that constrained the changes

- [Public-event queries](../../internal/publicevents/query.go) document publication-time `known_at`, select latest revisions before spatial filters, and currently use `LIMIT/OFFSET`. The new snapshot design must preserve the first two behaviors while handling publication between HTTP reads. A timestamp or maximum sequence is not a commit-consistent snapshot.
- [Public-event projection](../../internal/publicevents/projector.go) already resolves related alerts on provider correction/withdrawal. New case/finding/brief dependencies need equivalent explicit impact handling; this audit does not claim correction support is entirely absent.
- [Artifact lifecycle](../../internal/artifacts/service.go) already blocks use after retention and implements `DELETING`/`DELETED` with retry. Composite output/case design must accommodate this existing behavior rather than promise permanent availability.
- [Scoped database binding](../../internal/platform/db/scoped.go) currently binds resource and source IDs. Principal/canonical-human binding and case RLS remain required new work; a client cache key or membership row cannot substitute for enforcement.
- [Capture jobs](../../internal/imagery/captures.go) already compare request input, fence leases and enforce active/history budgets. New intent/account/storage contracts extend these protections; removing the history cap before the replacement is ready would regress bounded operation.
- [Connector state](../../internal/publicingest/repository.go) has source-level checkpoints, leases and quota counters. New shared-account reservations must coexist with per-profile state and retain uncertainty after a network-side effect.
- [Delivery-package validation](../../scripts/ops/check_delivery_package.py) explicitly runs asset/recording workers and runtime description. It must become profile-aware alongside startup.
- [Paired backup](../../scripts/ops/paired_backup.py) relies on a quiescent system and checks a declared set of restored objects/workflows. New workers and object families must join that inventory; an API-port check alone is not writer fencing.

## Coverage of the original report

The machine-readable [backlog](MAP_FIRST_BACKLOG_2026_10_25.json) is authoritative for the 54 individual requirement mappings. Each gap has three separately reviewable obligations:

| Gap | Obligations checked | Additional challenge |
| --- | --- | --- |
| GAP-01 | Ordinary analyst entry; startup/package independence; preserved historical isolation | H08 |
| GAP-02 | Geography-default inventory; versioned region/world scope; identical selection workflow | Fixture constants must stay explicit |
| GAP-03 | Six area forms and period handoff; deliberate subset; exact revisions across races | H01 |
| GAP-04 | Generic reference pipeline; aliases/rights/accuracy/point meaning; regional/worldwide depth | Partial imports cannot activate mixed releases |
| GAP-05 | Area/time coverage states; approved bounded profiles; no implicit grants | H06/H07 |
| GAP-06 | Monitoring/history; distinct clocks; quiet/unobserved/background meaning | H01/H02 |
| GAP-07 | Aggregates/tiles/drill-down; accurate counts/cursors; geometry extremes | H01/H09 |
| GAP-08 | Map/dock/Systems journey; role actions and durable work; observed navigation | H04 |
| GAP-09 | Complete case content; lifecycle/conflicts/handoff; restart/access/immutable output | H02–H05 |
| GAP-10 | Fresh scoped imagery; honest quality/failure/zero-change; regional/worldwide repetition | H06/H07; indicators remain indicators |
| GAP-11 | Readable content; exact permitted reproducibility; independent review/recipient usefulness | H02/H03/H05 |
| GAP-12 | Purpose versus provenance; ordinary defaults; truthful counts and retained support history | Reclassification cannot manufacture human review |
| GAP-13 | Input/task readiness; recoverable provider/queue gates; durable honest outcomes | H06/H07 |
| GAP-14 | Measured workload; cancellation/history/capacity; faults and complete restore | H01/H03/H07–H10 |
| GAP-15 | Real role grants; international meaning; keyboard/narrow/error usability | H04 and degraded external basemap |
| GAP-16 | Bounded durable preparation; rights/coordinates/controls/lineage; portable non-simulation output | H03/H11 |
| GAP-17 | Independent users; observed usefulness/assistance; blocking interpretation failures | H09/H10 |
| GAP-18 | Current scope precedence; separate versioned evidence; honest mandatory closure | H10/H12 |

## Validation and its limits

Run from the repository root:

```powershell
python scripts/ops/check_map_first_plan.py
python scripts/ops/check_map_first_plan.py --self-test
```

The harness validates plan/backlog titles and mappings, reciprocal gap records, requirement coverage, hardening references, scenario coverage, dependency cycles and ordering, source fingerprints and local document links. It rejects unsubstantiated completed/qualified status. The self-test injects broken relationships and false-completion states in memory and expects rejection; it does not modify the plan.

The source fingerprints force a new review if the governing scope or original report changes. They are not proof that every natural-language requirement was interpreted correctly. Likewise, a requirement with a valid task/scenario link still needs human semantic review and actual implementation proof. The audit's 12 weaknesses are resolved **in the design**; runtime performance, acquisition success, geographic accuracy, access enforcement and usefulness remain unverified until their acceptance work runs.

**Recorded planning checkpoint, before implementation authorization:** the consistency check passed, and all 12 deliberately injected faults were rejected. At that checkpoint every implementation task was `PLANNED` and the release was `NOT_QUALIFIED`. The later [T01–T03 record](MAP_FIRST_T01_T03_IMPLEMENTATION_2026_10_25.md), [T04–T06 record](MAP_FIRST_T04_T06_IMPLEMENTATION_2026_10_26.md) and backlog now carry scoped task evidence. T07–T24 remain planned, all full scenarios retain their qualification gates, and planning-harness success itself never populates application acceptance evidence.

The plan is substantially stronger after this pass. Claiming absolute completeness before implementation and independent qualification would exceed the evidence.

Implementation update, 27 October 2026: the ledger now records T01–T18 as IMPLEMENTED with scoped evidence. T19–T24 remain planned; all full acceptance scenarios and independent-human qualification retain their gates. See [T16–T18 record](MAP_FIRST_T16_T18_IMPLEMENTATION_2026_10_27.md).

T19–T21 update, 27 October 2026: readable briefs and portable geographic preparation have scoped implementation evidence in [the T19–T21 record](MAP_FIRST_T19_T21_IMPLEMENTATION_2026_10_27.md). T01–T21 are IMPLEMENTED, T22–T24 remain planned, and full acceptance/release/observed-human gates remain open.


## T22–T24 qualification tooling, 27 October 2026

The authorized final batch adds workload samples, bounded operation histograms, expanded paired inventory/retention checks, observed-session record validation and active release traceability. T22–T24 remain IN_PROGRESS; all full acceptance and release gates retain their state. See [implementation and remaining gates](MAP_FIRST_T22_T24_IMPLEMENTATION_2026_10_27.md) and [verification](evidence/map-first/t22-t24/verification.json). The harness still checks planning consistency only.
