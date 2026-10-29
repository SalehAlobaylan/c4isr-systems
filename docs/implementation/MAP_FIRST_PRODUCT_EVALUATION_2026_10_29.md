# Map-first product evaluation — 29 October 2026

**Release status: NOT_QUALIFIED. T22–T24 remain IN_PROGRESS.**

The retained installation supports part of the intended workflow, but it is not yet reliably usable or qualified for analytical delivery. Passing automated checks does not resolve the product findings below. Work crossed the original one-hour evaluation window; this is not a claim that all qualification was completed within an hour.

## Actual workflow and outcome

The evaluator operated the production UI against the retained database and private artifact storage, without mocked API responses. Local map profiles first needed reconciliation with the supported `emad-local -profile map` command: migrations alone had left the maintained legacy analyst without investigation and output permissions. Automatic local analyst entry now works in the evaluation launcher; Account remains available and backend authorization remains enabled. This is trusted loopback access, not public anonymous access or a completed deployment-wide authentication change.

The installation returned 24 reference versions, 177 country boundaries, 3,706 retained evidence revisions, 20 imagery scenes, four captures, two acquisition dates and one comparison. These counts establish retained material, not useful coverage for a particular question. Reference, scene and capture inputs remain unclassified for content purpose; there are no reviewed working outputs.

A deliberate one-kilometre area centred at longitude 46.5, latitude 24.5 was selected. The clearly labelled draft `[Evaluation] Retained evidence near Riyadh` was saved and reopened. Its internal brief rendered through one explicitly executed author-worker attempt and opened in the frozen reader. Saved geometry and rendered bounds agreed. The brief contained no included evidence or findings and correctly reported coverage as unknown. It establishes durable save/render behavior, not a useful completed analysis. Independent review and approved distribution were not performed.

During navigation and reopening, the UI reported **“active read capacity exhausted”** and could not refresh the investigation list. The saved case and output remained durable. Multiple evaluation browser contexts were present, so a single-browser reproduction and exact cause remain unresolved. Read diagnostics showed retained active manifests across catalog families. View replacement, unmount, reload and expiry handling need investigation; increasing quotas would not demonstrate a lifecycle fix.

## Findings and required outcomes

1. **Read lifecycle blocks ordinary work.** Fix and evaluate opening, switching, saving and reopening views across realistic sessions. Successful recovery must preserve the case and restore the list without requiring the analyst to understand snapshot capacity.
2. **Dense workload responsiveness fails.** With 100,000 report revisions, 50,000 detailed reference features, 10,000 weather records and 1,000 history entries coexisting, five analyst identities, ten areas and ten schedules, the fourth full-volume run failed frozen targets. Cold map p95 was 2,854.619 ms against 2,000 ms; warm geography p95 was 1,079.928 ms against 1,000 ms; warm weather requests reached approximately 5,000 ms/timeouts. Investigate actual query and contention costs, retain failures, and repeat the same workload. An experimental weather rewrite worsened performance in the fifth run and was reverted.
3. **Real analytical usefulness is unproven.** Classify retained inputs with appropriate source rights, establish area/time coverage, select suitable real imagery and terrain, and complete evidence-backed studies through independent review. An empty brief or a successful synthetic fixture cannot establish this outcome.
4. **Workflow still requires technical assistance.** Membership and evidence inclusion expose raw identifiers; environment qualification requires JSON. Observe whether analysts can discover colleagues, select evidence and complete tasks without developer intervention. Replace these technical inputs where they obstruct the intended user.
5. **Coordinate presentation is ambiguous.** The footer retained 43.3000/24.7000 while the deliberate study and frozen brief correctly used 46.5/24.5. Clarify pointer versus study coordinates; no incorrect saved geometry was demonstrated.
6. **Operational startup remains incomplete in this evaluation.** One delivery attempt was run explicitly. No continuous shared worker supervisor or provider acquisition was started, so future queued briefs and live refresh cannot be assumed to run automatically. Evaluate the normal assembled deployment and recovery procedure.

The geographic catalog's omission of exact study geometry was corrected. The final broad integration selection passed after this fix and after reverting the experimental weather rewrite. This supports correctness of covered contracts; it does not close the performance or user-workflow findings.

## Evidence and limits

Machine-readable observations, run references, exact artifact identities and verification results are in [evaluation.json](evidence/map-first/qualification-2026-10-29/evaluation.json). Full-volume attempts retain their own source/profile records and raw failures; the fourth run represents the final runtime query design. Existing browser checks and build checks passed. No additional tests are needed merely to increase the passing count.

No independent participant observer record has been received. The automated role-entry checks and evaluator-operated journey are not human usability sessions. The 24-hour soak, complete real imagery/environment journeys, nominated rollback-binary rehearsal and full installed backup-CLI rehearsal remain incomplete. Paired recovery integration evidence does not substitute for those installation exercises. Raster concurrency and actual browser latency under the full-volume workload were not established.

Shared changes were supported local map-profile provisioning and one labelled evaluation case/internal brief. No shared migrations or provider fetches were run. No human approval was fabricated.

## Next evaluation sequence

First resolve the navigation/read-capacity blocker and establish normal worker startup. Then repeat ordinary analyst journeys on the actual installation and the unchanged dense workload, retaining failures and assistance required. Complete real evidence-backed studies and independent observed sessions. Finally execute the long-running and deployment recovery gates. Release qualification requires these outcomes together; implementation completion alone is insufficient.
