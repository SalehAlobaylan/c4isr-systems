# T16–T18 implementation and verification

This increment implements case findings and save recovery (T16), independent case review (T17), and composite-output rights/storage foundations (T18). These changes belong to the combined uncommitted working tree. Task closure and full application qualification remain separate from scoped automated checks.

## T16: findings, exact evidence and recovery

Cases select exact provider reports, completed analyses, captures, assessment revisions, reference editions and retained artifacts. The server resolves hashes and source lineage again when saving. An analysis citation freezes both its input and completed result; historical `job` links remain readable but block review/output until the exact result is adopted. Evidence is never edited by the case service.

Finding revisions contain authored conclusions, confidence/rationale, uncertainty, purpose, exact citations, the original scope hash, canonical author and previous finding revision. Unchanged findings retain their revision and scope when the case area changes; a notice requires reassessment. Changed conclusions create immutable successors. Unclassified/development/acceptance inputs cannot silently become working findings. Retained artifacts now have an explicit purpose classification path through the existing source-administration catalog.

Case history retains cumulative source dependencies even when current citations/findings are removed. Typed lineage includes original admissions and secondary artifact admissions. Reverse dependency records are indexed; current support notices are recomputed on case open, review and output eligibility, so missed notifications cannot conceal an unavailable input. Current source state, purpose, policy, retention and provider-support notices stay outside the immutable authored content.

Save intents bind the original input to resource, canonical human and request key. Case revision, findings, dependencies and intent commit together. A lost-response retry returns the original revision even after another editor advances the case. Changed payloads with the same key conflict. Live grants are checked before commit and recovery. New revisions still compare the expected case head and finding revision.

The browser persists an account/resource/case-scoped unsent intent and autosaves valid changes after a pause. A reopened recovery copy is submitted to the server for current authorization and conflict checks before its content is displayed. The operator can discard it. Device storage is a recovery buffer, not authoritative case storage or a source grant. Browser profiles/devices need their normal access controls. History comparison shows notes, question, scope, exact citations and authored finding fields. Existing ownership transfer/archive/reopen rules remain in force.

## T17: exact-content independent handoff

Review requests, changes requests, approvals and supersession events are append-only. Requests bind the current case revision and content digest, expected prior workflow event, assigned canonical reviewer and substantive reason. A new content revision supersedes pending review; prior approvals remain historical. The review inbox contains only currently pending, assigned and visible cases.

The reviewer must already be a case member with live review/read grants covering every cumulative source. Credentials for the same human do not establish independence. All case-history authors, finding authors, cited assessment authors, and any output creator/render contributor are excluded. Services cannot perform human review. A reviewer who edits becomes a contributor to the successor. Grant/credential validity is checked at commit; original admission policy heads are locked for positive review decisions. Source problems block approval but permit a request for changes.

Output review requires an existing READY render. The immutable attestation binds case revision, manifest digest, output identity and exact rendered SHA-256. Approval does not rewrite content or rerender bytes. Independent-human observed sessions remain **NOT_RUN**; distinct test credentials prove protocol behavior only.

## T18: explicit rights, frozen manifests and private bytes

Optional `rights.outputPermissions` fields are tri-state: `internalAnalysis`, `citation`, `includedImagery`, `rawDistribution`, `geographicEnvironment`, and `metadataRetention`. Missing/null permissions fail closed. Original and current policy permissions intersect. Admission lineage stays within the original retrieval profile; independent reference imports use their original and current dataset versions. Legacy `allowSimulationExport` remains insufficient for general analytical output or brief distribution. Existing simulation-package permissions are unchanged.

An output freezes the complete case revision, all historical source-policy dependencies, exact selected assets, explicit source uses/omissions, renderer/style versions, intended purpose and retention. Cumulative prose dependencies cannot be removed to avoid output rights. Assets must be explicitly cited, retained, permitted and within their original retention. External references and declared omissions allow a permitted citation without embedding disallowed bytes. All-source case membership/RLS applies; source administration is not blanket case access.

Frozen manifests reserve resource storage capacity, including PostgreSQL JSON document bytes and the bounded rendered-byte reservation. Original/effective rights, current policy identities and evaluation times are recorded in each dependency decision. Content is immutable after staging. Private reads verify exact byte count/hash and recheck live identity, all-source access, original/current rights and retention after I/O. Staging repeats live creation checks after I/O. Distribution additionally requires the exact independent render attestation and distribution grant. Authorized historical draft artifacts remain pinned and labeled superseded; they cannot be distributed as current approved work.

This foundation accepts bounded JSON render plans and PNG figures, at most 8 MiB, with PNG limits of 4096 per side and 4 megapixels in total. It does **not** implement T19's HTML brief renderer. An uploaded rendering is explicitly declared and must itself be reviewed. No automatic screenshot, external network rendering, HTML execution or zip packaging is introduced here.

Expiry uses DELETE_PENDING followed by private-object deletion and a minimal TOMBSTONED manifest containing only case revision ID, content digest and tombstone flag. Deletion failures remain retryable; expired bytes cannot be restaged. Frozen retention cannot be extended by citation. IDs, hashes and append-only audit events remain. Reconciliation is an explicit authorized API/UI operation in this foundation; unattended scheduling and full production recovery/soak qualification are not asserted by these tests.

## Verification and installation boundary

The final commands, results, logs and source fingerprints are recorded under `evidence/map-first/t16-t18/verification.json`. Final checks pass for **39 unique assembled integration cases**, **53 unique browser cases**, all Go package tests, Go vet, generated contracts, the production UI build and final Linux-image startup. Integration evidence combines the broad regression with focused reruns: one alias fixture initially lacked its required source-read grant, was corrected, and passes in the final workflow run. The final composite/shared-quota rerun verifies manifest-byte capacity accounting. Browser evidence combines 52 regression cases with a final 10-case workflow run, including the new save-race case. Test fixtures use isolated migrated PostGIS and private stores. Browser fixtures establish UI/protocol behavior; they do not establish observed independent human review or real publication.

The user reported applying the preceding migrations, assumed through **00051**. This increment adds **00052_case_workflows_and_outputs.sql**. Only isolated test databases are migrated by this work. Apply 00052 through the normal installation process before running the updated application. No shared application migration, deployment, restart, commit or publication is performed by this increment. Rollback must preserve immutable case/review/output history; use forward recovery.

T19–T24, observed independent users, complete A01–A24 journeys and release qualification remain open. T16–T18 are not marked COMPLETE.
