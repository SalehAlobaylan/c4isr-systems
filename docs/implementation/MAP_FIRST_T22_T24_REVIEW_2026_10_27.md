# T22–T24 pre-qualification review

Reviewed the qualification runner, recovery inventory/backup/restore paths, observed-session validator, HTTP operation metrics, relevant tests and release claims. This is a focused correctness/regression review of T22–T24, not an exhaustive repository audit or completion of the operational/user gates.

Seven issues were found and fixed before qualification:

1. **P1 — Candidate fingerprints omitted staged changes and deletions.** The earlier inventory read index-relative modified/untracked paths. A staged-only edit could therefore be absent even though HEAD did not describe its bytes. Source capture now uses all changes relative to HEAD plus untracked paths, with explicit deletion records and NUL-delimited paths. A regression checks that staged bytes alter the fingerprint and deleted paths remain represented.
2. **P2 — Incomplete/contradictory samples could receive a sample PASS.** The summarizer required an end marker but did not verify the advertised duration or reject successful timeout flags. It now checks finite positive elapsed/advertised duration, frozen requested duration, strict outcomes, profile-first/completion-last ordering and consistent timeout/status flags. The release remains NOT_QUALIFIED regardless of a sample result.
3. **P2 — Standalone summaries used the current budget file.** Reassessing an old sample after changing targets could silently change its outcome. Standalone summaries now require the original sibling `frozen-input.json`, use its budgets/duration and record the canonical budget content hash. The runner checks its own frozen requested duration too. A regression loosens the current file and confirms that the original target still fails.
4. **P1 — Legacy v1 restores skipped retention policy.** v1 archives lacked a persisted inventory, so the previous restore path copied their objects without the new expiry/tombstone policy. Every restore now derives an inventory from the isolated restored database. v2 also compares its persisted fingerprint; v1 enforces current retention while remaining explicitly unqualified for expanded historical inventory comparison.
5. **P2 — Restored-object counts could become negative.** The excluded-key set can contain expired keys already absent from a new backup. Subtracting that set's size from the backup object count was incorrect. Counts/bytes now use actual manifest entries, and excluded-key counts are reported separately from objects actually omitted.
6. **P2 — Observed records lacked explicit real-workflow/accessibility coverage.** Geography/state labels alone could pass record completeness without the required imagery, geographic-package, handoff, offline brief and accessibility tasks. The profile/template/validator now require those workflow entries, a full candidate fingerprint and consistent participant-to-canonical-human mapping. This remains record validation, not proof that observations actually happened; no real sessions were performed.
7. **P2 — Interrupted/timed-out workload runners could lose their final failure result and leave their test process running.** The runner now owns a process group/tree, stops only that invocation on timeout/interruption, and records a failing result even when samples are incomplete/malformed. The timeout regression runs and stops a real owned process. Docker Testcontainers remain responsible for their disposable-container lifecycle; no shared service is selected for shutdown.

Files changed: `scripts/ops/qualify_map_first.py`, `recovery_inventory.py`, `paired_backup.py`, their qualification tests, the proposed qualification profile and observed-session template. No Go runtime/API behavior, migration, shared installation or source rights were changed in this review.

## Verification and handoff

Exact source identity, commands and results are in [review verification](evidence/map-first/t22-t24-review/verification.json), with the revised [candidate ledger](evidence/map-first/t22-t24-review/release-ledger.json). Earlier T22–T24 evidence is preserved as historical; its source fingerprint is superseded for future qualification by this review's candidate. Prior tests remain evidence of their own recorded code, not fresh acceptance of this candidate.

The focused checks cover regression/negative cases, a fresh five-identity/ten-profile reduced workload, and a real isolated database/private-object restore with the original brief approval/intent/access restrictions. The restore test exercises the common inventory and assembled product; it does not qualify the complete installed backup CLI, every populated geographic/environment dependency, all recovery faults or the nominated older binary. Those remain explicit T22 gates.

No further concrete correctness issue was identified in the reviewed paths after these fixes and scoped checks. T22–T24 remain IN_PROGRESS and the release remains NOT_QUALIFIED. Full workload/browser budgets, 24 actual hours with planned restart and growth/freshness telemetry, complete real geographic journeys and observed independent users are still required.


At the final handoff check, the Docker API was unavailable. Current shared-container health could not be verified; Docker must be available before repeating isolated qualification. The earlier successful test logs are retained. The cause was not established and no shared infrastructure was restarted by this review.


The subsequent final recheck found Docker available, `c4isr-postgres` running and healthy, and `emad-garage` running. The earlier unavailable result is retained separately; no infrastructure restart was performed by this review. See `docker-handoff-final.json` in the review evidence. Shared application deployment/source-profile verification remains a qualification gate.
