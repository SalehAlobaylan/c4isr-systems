# UX review of the implemented map-first plan — 29 October 2026

The intended journey is sound: select an area and period, understand coverage, investigate using exact evidence, hand off to an independent reviewer, and deliver a readable result. The implementation still makes users assemble parts of that journey from technical forms. This review corrected verified workflow and recovery problems; it does not establish complete usability or release qualification.

## Review scope

Reviewed plan acceptance scenarios A01–A24 and the UI implementing scope, investigation, evidence, acquisition, review and delivery. Operated the retained installation at `http://127.0.0.1:5198/map`, inspected the maintained reviewer account without submitting a review, and checked the corresponding source code. Evidence is strongest for investigations, recovery, evidence discovery and current brief approval presentation. Real provider acquisition, independently qualified terrain and an actual independent reviewer session were not completed.

## Confirmed issues corrected

1. **Browser storage could block a server save — high impact, A10/T07/T16.** Recovery storage was written inside the save's main try block. A storage exception prevented the API request. It is now best effort; a server save remains possible. A clearly labelled case saved successfully against the actual installation with recovery storage deliberately unavailable.

2. **Recovery looked like an error during normal typing and could lose an earlier intent — high impact, A10/A23.** The device recovery banner appeared for each edit. Draft copies and submitted intents now carry distinct markers. Reopening an unresolved save protects its original request before allowing more edits. Recovery can retain an original intent in memory when device storage fails. Automatic refresh preserves the recovery acknowledgement. Device storage is still best effort and cannot guarantee recovery across browser closure when storage is unavailable.

3. **Draft discard was unsafe or incomplete — high impact, A10.** Reloading a saved revision discarded edits without confirmation; deliberate discard could leave a stale device recovery copy. Reload now asks, cancellation preserves the draft, and confirmed discard clears the recovery copy. New-investigation discard follows the same cleanup rule.

4. **Temporary network failures were treated as loss of authorization — high impact, A09/A17/A23.** A failed background workspace check invalidated the whole session. Temporary errors now keep the existing draft and show a connection/staleness warning. Reconnect preserves the draft. Explicit authorization denials continue to clear protected state. Case refresh errors similarly distinguish denied/not-found content from transient refresh failures.

5. **The investigation journey was an undifferentiated form — medium impact, A10/A12/A13.** Added direct navigation between question, evidence/findings, review and delivery. Unsaved cases explain that review and delivery require a server save. A saved scope differing from the current map is explicitly identified. Raw identifier attachment is an advanced path rather than the primary evidence workflow.

6. **Evidence discovery omitted important states — medium impact, A07/A09/A15/T16.** The picker now distinguishes loading, failed discovery, no matching evidence and no loaded search matches. Loaded records can be searched by title/source/identifier, attached items are identified and cannot be attached again, and exact identifiers remain inspectable. Reference editions are explicitly workspace-wide: the earlier generic claim that every picker category used area/time filtering was inaccurate. The first-100 report limit remains disclosed.

7. **Disabled handoff controls lacked a next step — medium impact, A12/T17.** The review panel explains missing membership, unsaved changes and the minimum review rationale. Owners can jump to membership management. A listed member is no longer described as proven eligible; the server still checks independent human identity, review authority and source access on submission.

8. **Delivery presentation implied readiness before approval — medium impact, A13/T19.** The main delivery panel explains unsaved revisions and briefs with no cited support. Approved-download controls require matching current case/output/render approval before becoming available. Current source rights remain server-checked on download; the UI check is not authorization enforcement. The advanced output inspector remains a technical workflow.

## Remaining work, in order

1. **Read lifecycle and capacity — high impact, A09/A17/T13/T22.** The live investigation catalog hit active-read exhaustion during this review. A readable error does not solve it. Implement explicit view ownership/release and coherent monitoring refresh, preserving pinned reads and durable citations. Verify switching, closing and reopening views across multiple tabs without raising quotas or deleting active user manifests.

2. **Human and figure discovery — medium impact, A12/A13/A15.** Membership still requires registered human identifiers; reviewer choices do not prove live eligibility. Brief figures still require asset IDs. Provide authorized, labelled people and cited-figure selectors, with reasons when choices are unavailable. Preserve canonical-human separation and exact artifact identity.

3. **Geographic qualification workflow — medium impact, A16/T20/T21.** Users still supply qualification JSON, origin coordinates and specialist controls. Replace the default technical input with structured validated fields and approved saved profiles, while retaining JSON as an expert import path. Required controls, datum applicability and independent accuracy evidence must remain explicit; never fabricate values to make a package build.

4. **Coverage and imagery task guidance — medium impact, A07/A11/T10–T15.** Coverage presents many provider states and profile keys with equal emphasis. Lead with whether the selected task can proceed, the blocking input and its recovery owner. Keep source receipts and raw profile details available on demand. Validate the complete acquisition-to-comparison journey with suitable real scenes, rather than treating metadata discovery as completion.

5. **Real role and international sessions — unverified, A15/A19/T23.** Complete observed analyst/reviewer sessions with assistance, failures and task outcomes recorded. Include keyboard, mixed Arabic/English content, text zoom and long-running sessions. The maintained reviewer account's reachable empty inbox is account behavior, not independent human validation.

## Verification and shared effects

A labelled case, `[UX review] Save and recovery — 29 October`, remains in the installation as `case_01m64greaj7027hzbxek4b7vja`, revision 3. It contains evaluation notes, no findings and no retained citations. One durable response was intentionally interrupted and recovered using the same request key; revision 3 was not duplicated. Reference attachment/search and draft-discard probes were not saved as analytical content. No review, approval, distribution, migration or provider retrieval was performed.

Live checks verified unavailable browser storage, ordinary typing, response-loss recovery, discard cancellation/confirmation, recovery-copy cleanup, 24 loaded reference editions, duplicate attachment prevention, reviewer membership guidance, a reviewer inbox, mobile step navigation, disabled unapproved brief downloads and connection interruption/reconnect with the draft preserved.

Production TypeScript/build checks passed. The 13 existing investigation/profile cases passed; final recovery changes and workspace access-denial/connection-lifecycle paths received targeted reruns. One recovery-message regression found during the final change was corrected and the affected checks passed. Existing confirmation handling was updated for intentional discard protection; no new test suite was added. These checks support covered behavior and do not close the remaining UX findings.

Private screenshots, initial/final regression artifacts and redacted observations are retained under `private/qualification/plan-ux-review-2026-10-29/`. Earlier failure artifacts remain available.
