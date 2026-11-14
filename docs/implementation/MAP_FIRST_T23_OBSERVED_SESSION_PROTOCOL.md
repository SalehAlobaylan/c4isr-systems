# T23 observed analyst/reviewer session protocol

Status: **NOT_RUN**. Owner: appointed observer and product validation. This document is the **instrument**, not evidence that any session occurred. A19 and T23 stay `NOT_RUN` until real target users complete the journeys below under an appointed observer who attests them, and the owner adjudicates the acceptance scenario. Automated role fixtures, registered synthetic database identities and this document are not observed people.

## 1. Relationship to the T14 protocol

The [T14 protocol](MAP_FIRST_T14_OBSERVED_SESSION_PROTOCOL.md) is the foundation and its content is carried forward unchanged: separate analyst, reviewer, source-administrator and restricted-reader accounts; an independent reviewer who is a different canonical human from the analyst and from every case contributor; consent and anonymity; the analyst map/imagery journey; the reviewer, source-administrator and restricted-reader journeys; the accessibility repetitions; and the rule that failures are reported before any acceptance scenario is updated.

This document adds what T14 never contained:

1. the **frozen-brief**, **geographic-package**, **case-handoff** and **offline-recipient** journeys;
2. the explicit **`workflowChecks` matrix** that `scripts/ops/map_first_qualification_profile.json` freezes and that `qualify_map_first.py sessions` enforces, described entry by entry so that the observer and the validator work to the same instrument;
3. the **per-task attempt and assistance accounting** that A19's exit measurement needs, including how a recorded FAIL or BLOCKED journey is reported rather than rejected.

Where T14 and this document could be read differently, this document governs the T23 record. T14 is retained as the foundation and points here.

## 2. The record, the validator and what the verdict means

Fill a copy of the machine-readable [session template](MAP_FIRST_T23_OBSERVED_SESSIONS.template.json) from real consented observations, then validate it:

```powershell
.venv-recovery/Scripts/python.exe scripts/ops/qualify_map_first.py sessions <record.json> --release-fingerprint <64-hex candidate fingerprint> --output <assessment.json>
```

The assessment's `status` is a **record verdict**: `PASS` means the record is complete, internally consistent and pinned to the candidate. It is never a qualification decision. The assessment always carries `qualification: NOT_QUALIFIED` and `a19Status: NOT_RUN`; only the appointed owner and evidence reviewer may set the acceptance scenario, and only after reading the observer's attestations. Exit code `0` means "the record validates", never "A19 passed" — read `exitMetric` for the exit measurement.

A session whose journey outcome is `FAIL` or `BLOCKED` is legitimate evidence. It is recorded, reported and measured; it is not a defect in the record. The validator reports it under `outcomes`, `unresolvedJourneys` and `exitMetric` and, where A19 requires it, requires the retest evidence to be present.

## 3. Sample, roles and consent

The proposed A19 sample is **at least three target analysts and one independent reviewer**, with the reviewer separate from the case contributors. The validator checks that sample and the reviewer separation; it cannot verify that a named person is a target analyst or that consent was meaningful.

Use separate accounts per role. Never reuse one person's credential for two roles: an alias is the same canonical human. Record for every participant: the consented anonymous identifier, the canonical human identity, the role, the observer, the locale, the timezone, the device with viewport and text enlargement, the input method, accessibility needs and the consent record. Consent (`obtained`, `obtainedAt`, `scope`) is recorded per session. Do not include credentials, personal names or personal case text in the record or its evidence.

The observer is appointed, named in the record and present for the session. The observer does not coach: interventions are counted, not hidden, and a participant's wrong interpretation is recorded rather than corrected mid-task.

## 4. Before the session

Pin the record to the exact candidate fingerprint of the release candidate being observed. A new source change invalidates the session evidence and requires new sessions against the new candidate. Use a reviewed dataset and approved retrieval profiles; record any unavailable feed as unavailable. Prepare the frozen brief, the geographic package and the handoff case **before** the session so that the observer is measuring the product, not the setup.

Where a required capability does not exist on the candidate (a provider without coverage for the requested area, a package that cannot be prepared, a bundle that cannot be frozen), record the journey as `BLOCKED` with the reason and the evidence. An unsupported input is a legitimate negative test, but it cannot replace the required successful real journeys, and it cannot be written up as a success.

## 5. Journeys

### 5.1 Analyst core journey (T14)

Give the analyst a question requiring an area and a historical period. Observe whether they find a country/subdivision or enter signed coordinates, preserve exact geometry and timezone across tools, identify an unsupported period, resume a saved investigation, reach later catalog pages, inspect one aggregate's source records and recover from an expired read. Repeat a relevant step with Arabic source names and Arabic digits.

For imagery, ask the analyst to distinguish selected area, actual scene footprint and subset; deliberately adopt a valid rectangle; inspect before/after masks; explain an incompatible pair; queue one supported comparison; and retry a deliberately lost response without creating another request. Ask what a zero-change or cloud-masked result does and does not establish. Preserve the exact case revision and processing receipt.

### 5.2 Frozen-brief recipient journey (T23)

The recipient is an analyst who did not author the brief. Give them a brief bundle frozen against an exact case revision, manifest digest and rendered hash, and ask them to:

1. open the frozen bundle and state which case revision, manifest digest and render it is bound to, and how they know;
2. identify the exact area, the exact period (including timezone) and the evidence the brief cites, and say which of those are supported by the pinned revision;
3. separate what the brief establishes from what it does not: coverage versus absence, a provider report versus a derived indicator versus a human assessment, a current view versus a pinned historical one;
4. read the review state and the support-change notices and say what a withdrawn, corrected or expired support changes in their own use of the brief;
5. say what they would still need before acting, and where a cited dependency is unavailable or expired;
6. decide whether the brief can be used for distribution as it stands, and say why not if it cannot.

Record: whether the recipient's area and period matched the frozen values (a mismatch is a wrong-area or wrong-period outcome), whether they explained coverage versus absence and the result/review state, and what they did when the bundle referenced a dependency that was unavailable. A brief that cannot be opened offline is a journey failure, not a broken link to be worked around for the participant.

### 5.3 Geographic-package recipient journey (T23)

Give the recipient a geographic package for an area that has no prepared package, prepared through the ordinary public workflow before the session. Ask them to:

1. read the package manifest and instructions and say which area, CRS/frame/origin, height unit, resolution and source dates the package claims;
2. verify it independently of the authoring tool (for example with `scripts/data/check_portable_environment.py`), including projection/coordinates, control values, polygon containment, masks and hashes;
3. identify the parent dependencies that remain external, and say what that means for their use;
4. state the accuracy the package claims and what it does not certify (for example that it is not a surveyed-accuracy certificate and not a live rights check);
5. detect and report any missing, expired or omitted geometry, nodata or estimated properties, and any source whose rights or purpose do not permit their intended use.

Record: whether the recipient found the package usable for the stated purpose, what they could not verify, and whether they treated the package as portable evidence or as a runtime dependency. A package that cannot be prepared for the candidate area is `BLOCKED` with the reason; it is not an invented successful journey.

### 5.4 Case-handoff journey (T23)

Hand a live case from one analyst to a different analyst. The receiving analyst did not author the case content. Ask them to:

1. open the handed-over case and identify its current revision, its owner and the review state;
2. find the prior findings and the exact evidence revisions they rest on, and distinguish a current view from a superseded revision;
3. save and resume the investigation, and continue from the handed-over state without losing the scope, the period or the pinned evidence;
4. record their own new revision and see the effect on the pending review (for example that an authored revision invalidates the previous approver's eligibility);
5. explain what they would need to take the case further, and what they could not determine from the handoff alone.

Record: whether the receiving analyst found the handoff sufficient, where they needed the prior author, and whether they read any pinned content as current when it was not. A handoff that required the prior author to explain the product is a failure of the journey, recorded as such.

### 5.5 Offline brief recipient journey (T23)

Give the recipient an approved brief bundle on a machine with no network access to the product (an isolated/offline workstation is required, not merely a disconnected browser tab). Ask them to:

1. open the bundle and confirm it renders without any live evidence, tile or API request;
2. state the area, period, coverage and review state the bundle asserts, and the uncertainty it records;
3. distinguish permitted references from protected bytes and confirm that no restricted source, case title, count or job reference leaked into the offline copy;
4. say which claims they can still support offline, and which need a reconnection;
5. describe what they would do about a dependency that the bundle reports as unavailable, expired or partial.

Record: whether the offline bundle was readable and self-explanatory, whether the recipient treated it as complete when it was not, and any leakage of protected content. Offline completeness is never claimed from a bundle that fetches on open.

### 5.6 Reviewer, source administrator and restricted reader (T14)

Ask the reviewer to locate the intended independent-review action and explain its evidence and limits, and to identify the exact case revision, manifest digest and rendered bytes they are attesting to. Ask the source administrator to identify the owner and recovery path for a blocked source, profile or storage condition. Ask the restricted reader to locate permitted evidence without seeing protected case titles, counts or job references.

Repeat navigation and recovery on 320/390px portrait, short landscape, 200% text and keyboard-only input as relevant. Include a real assistive-technology pass with an appropriate user or specialist (section 8). Do not coach users through the primary task; record interventions.

## 6. The `workflowChecks` matrix

The frozen profile in `scripts/ops/map_first_qualification_profile.json` lists twelve workflow checks. The validator requires **every** one of them to appear in the recorded `workflowChecks` matrix **and** to be covered by at least one recorded task attempt with counts. Each check below is therefore a task the observer actually assigns, watches and counts.

### 6.1 `fresh-regional-imagery`

A fresh imagery study in an additional Middle Eastern area of interest, beyond the prepared fixture: discover admitted scene metadata, choose a valid local subset, acquire real windows, inspect masks, select a compatible pair and retain the analysis. Record the area, the acquisition dates actually returned and the exact case revision and processing receipt. Not covered when the candidate has no admitted real imagery for the area — that is recorded as a `BLOCKED` journey with the provider reason, never as a success.

### 6.2 `international-imagery`

The same imagery journey in an international area of interest, so that the product does not silently assume Middle Eastern geography, geometry or coverage. Record the area, the frames actually admitted, the subset the analyst chose and the result they explained. Not covered when the international provider coverage is absent or gated.

### 6.3 `new-geographic-package`

Prepare and independently verify a geographic package for an area that has no prepared package, as in section 5.3. Record the package identity, the manifest and hash verification outcome, and the parent's external dependency list. Not covered when the package could not be prepared for the candidate area.

### 6.4 `case-handoff`

An independent analyst receives a live case from another analyst and continues it, as in section 5.4. Record the receiving analyst's canonical identity, the revisions they read, what they added and whether the handoff alone was sufficient.

### 6.5 `offline-brief-reading`

A recipient reads an approved brief bundle with no network access, as in section 5.5. Record the device and its isolation, whether the bundle opened without a live request, what the recipient understood and any protected content they could see.

### 6.6 `correct-area-period`

The participant confirms the exact area and the exact period of an output, and the observer deliberately presents at least one case where the area or the period is wrong (a mismatched scope, a period that is not supported, a stale investigation restored into the wrong question). Record whether the participant detected the wrong area or wrong period, and what they did about it. A19's exit condition is that **no** wrong-area or wrong-period outcome remains in the sample, so this check is how that outcome is manufactured deliberately and caught.

### 6.7 `coverage-uncertainty`

The participant explains the difference between *coverage* (what was observed or acquired) and *absence* (nothing found, no observation, no grant), and the uncertainty of the result. Use at least one successful-empty, one unobserved and one partial or stale state. Record the participant's explanation verbatim in `explanation`, and whether it was correct. A participant who reads an empty result as "no activity" has failed this check.

### 6.8 `keyboard-only`

The core journey is completed with the keyboard only: no pointer. Record the focus order, whether focus was restored after a dialog and after a failed or expired read, whether every control was reachable and operable, and whether the participant became stuck. A journey completed with a pointer is not coverage of this check.

### 6.9 `portrait-320px`

The core journey at 320 CSS pixels of portrait width. Record the viewport, whether the area, period, coverage and review state remained legible, whether the map or list stayed usable and whether the participant could complete the journey.

### 6.10 `portrait-390px`

The same journey at 390 CSS pixels of portrait width. Record the viewport and the same observations.

### 6.11 `short-landscape`

The same journey in a short landscape viewport (the recorded candidate uses 740x360). Record the viewport, whether the primary task stayed reachable without scrolling away from the result, and whether the participant could complete the journey.

### 6.12 `text-200-percent`

The same journey with text enlarged to 200%. Record the enlargement, whether text was clipped or overlapping, whether the controls stayed operable, and whether the participant could complete the journey.

The same three matrices frozen in the profile (`geography`, `selectionForms`, `evidenceStates`) must also be covered by the record, and the protocol for each is the T14 analyst journey plus the state handling it describes. `geography` covers Saudi Arabia, UAE, Jordan, Egypt, France, Japan, Brazil, cross-border, coastal-water, date-line and high-latitude selections. `selectionForms` covers country, subdivision, named-study, drawn-polygon, viewport, restored-case and point-requires-area. `evidenceStates` covers current, historical, corrected-late, withdrawn, successful-empty, unobserved, partial, stale, missing-credentials, denied-access and processing-failure. Record each label only when it was actually observed; a state that the candidate could not reach is recorded as a gap in the report, not asserted.

## 7. Task attempts, assistance and the unassisted rate

Every session records a `tasks` array. One entry per assigned task attempt-group for that participant:

| Field | Meaning |
| --- | --- |
| `task` | The workflow check id (section 6) or an additional journey id. |
| `outcome` | `PASS`, `FAIL` or `BLOCKED` for that task. |
| `attempts` | Assigned attempts for that participant and task. At least 1. |
| `completedUnassisted` | Attempts that completed with **no** developer assistance. |
| `assistedCompletions` | Attempts that completed only with developer assistance. |
| `assistanceCount` | Observed developer-assistance interventions. |
| `wrongAreaOrWrongPeriod` | Whether any attempt concluded with the wrong area or the wrong period. |
| `coverageVersusAbsenceExplained` | Whether the participant correctly explained coverage versus absence. |
| `resultAndReviewStateExplained` | Whether the participant correctly explained the result state and the review state. |
| `critical` | Whether this is a critical journey under A19. |
| `retested` | Whether the failed journey was repaired and retested. |

The record's unassisted rate is `sum(completedUnassisted) / sum(attempts)` over every recorded task, reported overall and per role. Assistance means an intervention that moved the participant toward the result: a hint, a navigation step, a spoken instruction, a re-run of the task for them. Clarifying the task wording is **not** assistance; correcting a wrong interpretation is assistance and is counted.

The validator rejects a record whose completions exceed its assigned attempts, whose counts are missing or negative, or whose task outcomes are not one of the three permitted values.

## 8. Assistive technology

Include a real assistive-technology pass with an appropriate user or specialist. Record it in the document's `assistiveTechnologyEvidence`: the repository-local redacted observation record with its SHA-256, the participant, the technology used, when it was observed and the observer. The validator checks the file's hash and that the observation names a participant, a technology and a time. A simulated assistive-technology pass, an automated accessibility fixture or a screenshot without a participant is not coverage, and the missing fields are reported as errors rather than assumed.

## 9. Failures, corrections and retests

Record what happened, not what should have happened. A failed or blocked journey is entered with its outcome, its evidence and its reason. Where a critical journey failed, the session records the `corrections` entry (the task, the defect, the repair and the `retestOutcome`) and the `retestEvidence` for the repaired journey, and the task's `retested` flag is set accordingly. A claim of a retest without recorded retest evidence, or a recorded retest that is not claimed, is an inconsistent record and is reported as such.

Report observed success, failure and unresolved issues before updating A19/A20 or any task's user-validation evidence. Absence of a session is `NOT_RUN`, never `PASS`.

## 10. Reviewer independence

The document's `reviewerIndependence` block asserts that the independent reviewer is a different canonical human from every case contributor, from every observed analyst and from every service identity, and it names the reviewers and the observer who attests it. The validator checks the assertion, the named reviewers, that no observed reviewer is a case contributor or an observed analyst, and that every observed reviewer is named in the assertion. An alias of the same person is the same canonical human and does not satisfy independence. Reviewer assignment never grants source access, and a reviewer who authored a revision becomes a contributor.

## 11. The A19 exit measurement

A19 ([MAP_FIRST_PLAN.md](MAP_FIRST_PLAN.md#a19--observed-independent-usefulness)) proposes the exit: no wrong-area or wrong-period outcomes; all participants correctly explain coverage versus absence and the result/review state; **at least 80%** of assigned core task attempts complete without developer assistance, with counts and failures reported; and every critical failed journey is repaired and retested. The thresholds are the plan's, quoted unchanged in the validator; they are never re-derived from an observation and never relaxed after a run.

The validator reports each condition with its observed value and its status (`MET`, `NOT_MET` or `NOT_EVALUABLE`) under `exitMetric.conditions`, the unassisted rate against the 0.80 threshold, and the participant sample against the proposed three-analyst/one-reviewer minimum. A `NOT_MET` condition is a finding to report and repair, not a reason to edit the record.

This small sample establishes a bounded usability result, not statistical proof for all users.

## 12. What this protocol does not do

It does not certify that a session happened, that the observer was faithful, that the participants were the target users, or that the product is qualified. It does not replace the frozen profile's geographic, state and workflow matrices, the release ledger, or the owner's adjudication. The record, this protocol and the validator together make an observed session **expressible and checkable**; only real people, real consent and an appointed observer can make it **true**.
