# Implementation records — index and current status

This directory holds the implementation, qualification and evidence records for the EMAD product. It is a **record set**, not a single plan. Records from several delivery eras live here because the project's rule is that historical implementation records and immutable evidence remain intact and inspectable.

**Consolidated on 11 November 2026.** The per-task and per-era documents were merged into the records listed below. The originals are preserved verbatim in git history at the commit that carried this consolidation.

Nothing in this directory is a release approval. Where a record states a status, that status applies to the source revision, installation profile and evidence the record names — not to the product as it stands today.

**Governing scope:** [EMAD_CURRENT_SCOPE.md](../../EMAD_CURRENT_SCOPE.md).

---

## Current status

Product release acceptance remains open for both phases. Bounded implementation and qualification results are recorded against their tested installations and fixtures.

| Phase | Scope | State | Governing record |
| --- | --- | --- | --- |
| Map-first (MAP-6) | Map-centered Public Context & Intelligence for the Middle East and worldwide | **T01–T21 implemented**, **T22–T24 IN_PROGRESS**, release `NOT_QUALIFIED` | [Plan](MAP_FIRST_PLAN.md) · [Backlog](MAP_FIRST_BACKLOG_2026_10_25.json) |
| Environment & data foundations | Reusable geographic packages for later simulation design | **Implemented and staged.** Riyadh is the detailed reference. The paired restore and desktop/compact viewer checks passed. The 24-hour observation, human release decision and independent receiver walkthrough remain open. | [Plan index](../../plans/README.md) · [Implementation record](../../plans/IMPLEMENTATION_PROGRESS.md) · [Operations](ENVIRONMENT_LIBRARY_OPERATIONS.md) |

The [29 October map-first product evaluation](MAP_FIRST_PRODUCT.md#29-october-2026-product-evaluation) recorded `NOT_QUALIFIED`. The following findings include follow-up measurements and fixes:

- **Read lifecycle blocked ordinary work.** Map refreshes could accumulate temporary reads. The server now supersedes a matching read in the same transaction. The browser releases map and catalog reads on normal page exit, and the map retries after capacity errors. The basemap reads country geometry independently. The installed browser check verified cleanup over 19 consecutive loads in commit `8fe5b1c`.
- **Dense workload responsiveness failed.** Measured on 12 November 2026 against the full declared dataset, 60 s, with the run completing cleanly and the planned worker restart passing:

  | Operation | Cold | Warm | Budget |
  | --- | --- | --- | --- |
  | `catalog_search` | 154.19 ms ✅ | 184.95 ms ✅ | 1000 ms |
  | `geography_search` | **1346.74 ms** ❌ | **1266.97 ms** ❌ | 1000 ms |
  | `weather_query` | **1377.76 ms** ❌ | **5000.51 ms** ❌ (timeout) | 1000 ms |
  | `map_family` | **3744.76 ms** ❌ | 7.62 ms ✅ | 2000 ms |

  Payload budgets pass (GeoJSON 69,163 B of 2 MiB; tile 20,714 B of 512 KiB). The raw runs are in `evidence/map-first/qualification-2026-11-12/` and the earlier 29 October attempts in `evidence/map-first/qualification-2026-10-29/`. An experimental weather rewrite was attempted in October and reverted.
- **Real analytical usefulness is unproven.** Retained inputs are unclassified for content purpose; no reviewed working output exists.
- **Workflow still requires technical assistance.** Raw identifiers and qualification JSON leak into ordinary analyst tasks.
- **Coordinate presentation is ambiguous.** Pointer coordinates and study coordinates are not distinguished in the footer.
- **Operational startup was not running under a continuous supervisor** in the evaluated session.

### What is not done

1. Fix the dense-workload latency failures and re-run the **same** frozen workload. Do not relax the targets.
   Three operations miss. `catalog_search` passes cold and warm, `map_family` warm passes at 7.62 ms, and both payload budgets pass — so the read path and the stored-snapshot path are not the problem; the **cold materialisation** is.
   `map_family` cold and both `weather_query` conditions share one root cause: `internal/publicevents/query.go` applies every selective predicate — phenomenon, quality, observed time, bbox, region — in the `filtered` CTE, which runs **after** the `ranked` window function. A map read for `earthquakes`+`weather` therefore ranks all 100,000 revisions with `row_number() OVER (PARTITION BY source_id, provider_record_id …)` and then discards roughly 98% of them, and joins `observations`, `public_event_current` and the head revision for the whole surviving set before filtering.
   `geography_search` is a different query: `internal/reference/geography.go` evaluates `strpos(emad_geographic_search(g.name||' '||g.aliases), …)` per row over a `DISTINCT ON` of 50,000 regions. The existing index `geographic_names_normalized` covers `emad_geographic_search(name)` only, so it cannot serve the concatenated predicate.
   Both fixes change which revision or region can win, so they must be measured with `EXPLAIN ANALYZE` against the declared dataset and re-run against the same frozen targets. Do not move a filter across the window function on reasoning alone.
2. Run the 24-hour soak with a planned worker restart, process-RSS/object-growth telemetry and freshness telemetry. The harness now implements the restart and the telemetry; the 24-hour window itself has not been run.
3. Execute the 12 declared `requiredRecovery` rehearsals against the assembled installation, including the actual rollback binary.
4. Run the v2 backup CLI against the installed profile and restore it into isolated services.
5. Record genuine observed sessions against the published T23 protocol, with an independent reviewer.
6. Populate the release ledger with per-gap links, per-task source revisions, installed profile and user validation; name owners and evidence reviewers.

For geographic environment acceptance, follow the [environment acceptance sequence](../../plans/README.md#next-acceptance-steps).

### What is now enforced

- **Every declared budget is measured and reported.** `qualify_map_first.py` reports a check for each of the eight budgets with `PASS`, `FAIL` or `NOT_MEASURED` and the reason it could not be measured. A budget outside its target fails the run; an unmeasured budget is never assumed to pass. The soak, concurrency and `requiredRecovery` gates are reported the same way.
- **The planned worker restart is real**, not a `NOT_RUN` stub, and the full-volume path now records process RSS, retained-object growth, queue age and provider freshness alongside the previous heap/database figures.
- **The recovery rehearsals are executable.** `qualify_recovery.py` runs each of the twelve declared `requiredRecovery` faults in isolation and records `PASS`, `FAIL` or `NOT_RUN` with the missing instrument named. `qualify_rollback.py` rehearses a nominated rollback binary and refuses to substitute the retained source archive for one.
- **A recorded failed journey is evidence.** The session validator accepts `PASS`, `FAIL` and `BLOCKED` outcomes, counts them, and measures the A19 unassisted rate and exit conditions. It still cannot assert that a human session happened, and no session has been recorded.

---

## The records

| File | What it is |
| --- | --- |
| [MAP_FIRST_PLAN.md](MAP_FIRST_PLAN.md) | The map-first governing plan: the 18-gap report and MAP-0…MAP-6 gates, the code-backed gap analysis, the 24 tasks with acceptance criteria A01–A24 and the traceability matrix, and the pre-execution plan audit. |
| [MAP_FIRST_DELIVERY_LOG.md](MAP_FIRST_DELIVERY_LOG.md) | The implementation record for every task group: T01–T03 through T22–T24, plus the pre-qualification review and the local access fix. Each task's scope, checks and boundaries are a section. |
| [MAP_FIRST_PRODUCT.md](MAP_FIRST_PRODUCT.md) | The map-first product brief and the 29 October 2026 product evaluation. |
| [MAP_FIRST_OPERATIONS.md](MAP_FIRST_OPERATIONS.md) | Operation and upgrade guide for the map profile, plus the historical startup investigation and the historical recovery operations guide. |
| [HISTORICAL_DELIVERY.md](HISTORICAL_DELIVERY.md) | The pre-map-first v2 delivery sequence (F0–S4) and its acceptance. Code that is still installed behind the explicit `historical-full` profile. |
| [HISTORICAL_RECOVERY.md](HISTORICAL_RECOVERY.md) | The bounded delivery recovery of 23–24 October 2026: the audit, its recovery acceptance, the plan review, review/commit readiness, the provider activation snapshots and the asset-licensing record. |
| [ENVIRONMENT_FOUNDATION_PLAN_2026_11_11.md](ENVIRONMENT_FOUNDATION_PLAN_2026_11_11.md) | Retained-data audit, target data organization, preparation profiles and work packages E01–E10. |
| [ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md](ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md) | Implementation specification for the same phase: fixed defaults, acceptance areas, versioned profiles and the per-increment build order. |
| [ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md](ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md) | Windows and Linux installation evidence, pinned staging manifests, qualification results and remaining gates. |
| [MAP_FIRST_BACKLOG_2026_10_25.json](MAP_FIRST_BACKLOG_2026_10_25.json) | The machine-readable backlog: 24 tasks, 18 gaps, 24 acceptance scenarios, external prerequisites, release-candidate state. Read by `scripts/ops/qualify_map_first.py`. |
| [MAP_FIRST_T14_OBSERVED_SESSION_PROTOCOL.md](MAP_FIRST_T14_OBSERVED_SESSION_PROTOCOL.md) · [MAP_FIRST_T23_OBSERVED_SESSION_PROTOCOL.md](MAP_FIRST_T23_OBSERVED_SESSION_PROTOCOL.md) | Observed-user session protocols. `NOT_RUN` — a protocol, not evidence that sessions happened. The T23 protocol extends T14 with the frozen-brief, geographic-package, case-handoff and offline-recipient journeys, the enforced `workflowChecks` matrix and A19's per-task attempt/assistance accounting. |
| [MAP_FIRST_T23_OBSERVED_SESSIONS.template.json](MAP_FIRST_T23_OBSERVED_SESSIONS.template.json) | Empty template for T23 observed sessions. `NOT_RUN`, `sessions: []`. |
| [MAP_FIRST_CONTENT_PURPOSE_INVENTORY_2026_10_25.json](MAP_FIRST_CONTENT_PURPOSE_INVENTORY_2026_10_25.json) · [MAP_FIRST_PROFILE_POLICY_CALLERS_2026_10_26.json](MAP_FIRST_PROFILE_POLICY_CALLERS_2026_10_26.json) | Content-purpose classification inventory and retrieval-profile policy caller map. |
| [RECOVERY_STATUS.json](RECOVERY_STATUS.json) | Per-task delivery ledger for the 33 original records plus nine recovery stages. Read by `scripts/ops/collect_recovery_evidence.py`. |
| [F0_1_INVENTORY.json](F0_1_INVENTORY.json) · [F0_1_LEGACY_AUDIT.sql](F0_1_LEGACY_AUDIT.sql) | Source inventory and read-only legacy audit from the F0.1 baseline work. |

### Data files kept as separate files

The JSON and SQL files above are machine-read data with fixed paths used by `scripts/`, not narrative documents. They are listed individually rather than merged.

---

## Evidence

Evidence is kept under `evidence/`. It is **hash-verified**: the `verification.json` in each bundle records the SHA-256 of the files in that bundle, so those files must not be renamed, edited or removed. An empty log is evidence that a command produced no output — for example `go-vet.log` with hash `e3b0c442…` (the empty-file hash) records a clean `go vet`.

| Directory | Contents |
| --- | --- |
| `evidence/map-first/t01-t03` … `t13-t15`, `t16-t18`, `t19-t21` | Per-batch browser captures, integration logs, source-state and hash manifests. |
| `evidence/map-first/t22-t24` | First qualification tooling bundle: workload samples, paired-recovery result, verification manifest. |
| `evidence/map-first/t22-t24-review` | Reviewed candidate: **the current release ledger** (`release-ledger.json`), verification and the workload-final run. |
| `evidence/map-first/qualification-2026-10-29` | The 29 October evaluation: `evaluation.json`, integration logs, source state, and the five `full-volume-*` attempts with their measured latencies and failures. |
| `evidence/*.json`, `evidence/map-first/*.json` | Earlier acceptance manifests: plan review, pre-commit, review readiness, engine benchmarks, S1–S4 acceptances. |
| `evidence/firms`, `evidence/opensky` | Provider activation evidence and source-status captures. |
| `evidence/recovery` | The recovery acceptance bundle: setup/restore/graphics/hardware/failure acceptance, walkthrough video and UI captures. |

### Evidence entries that name a consolidated document

Where an evidence manifest recorded a hash for a document that the 11 November 2026 consolidation merged into a larger record, the entry now carries `consolidatedFrom`, `consolidatedSection` and a `hashScope` note. The recorded `sha256` is still the hash of the **original** document at capture time; the original bytes remain recoverable from git history. The recorded hash was never recomputed against the merged record, because the merged record contains more than one document.

### Regenerating the release ledger

```powershell
.venv-recovery/Scripts/python.exe scripts/ops/qualify_map_first.py ledger --output <new-path>
```

The ledger records the source fingerprint, task and gap evidence dimensions and the blockers. **The retained ledgers are pinned to older fingerprints and do not match the current source revision.** A fresh ledger must be generated against a clean revision before it is cited as release evidence; the ledger's file-hash list must then match the tree it describes.

---

## Rules for this directory

1. **Historical records are not deleted.** They are labelled historical and kept inspectable. A record is only removed when it is an unreferenced duplicate of content that still exists elsewhere.
2. **Evidence is not edited.** Bundles are hash-verified; a changed artifact invalidates the manifest that vouches for it.
3. **Failures are retained.** Failed qualification runs stay, with their measured numbers, so a later pass can be compared against the same targets.
4. **Status claims are specific.** A record states the source revision, installation profile and evidence it covers. A pass on one snapshot is not a pass on the product.
5. **One source of truth per set.** New material is added to the merged record that owns its subject, not as a new file per increment.
6. **No AI attribution.** Commits touching this directory are authored by humans only.
