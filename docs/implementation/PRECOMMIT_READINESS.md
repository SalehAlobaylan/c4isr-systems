# Commit readiness

**Recovery follow-up — 2026-10-23:** The [persistent recovery acceptance](DELIVERY_RECOVERY_ACCEPTANCE.md) and [delivery ledger](RECOVERY_STATUS.json) record the bounded installation and its verification. The checks below concern an earlier source snapshot; they do not assert that the present working tree has been committed or re-exported. No commit or push was performed during recovery.

**Readiness conclusion superseded, 2026-10-23:** The [delivery audit](DELIVERY_AUDIT_2026_10_23.md) identifies unresolved correctness and delivery gaps. The successful source/build checks below remain historical evidence and do not establish product completion or resolve those findings.

**2026-10-23 follow-up:** This is the earlier source/build verification snapshot. It did not establish that the default development account could use both product pillars or that an unconfigured local installation exposed their workflows. The [local startup follow-up](LOCAL_STARTUP.md) documents the discovered access, navigation and development WebSocket gaps and the subsequent corrections. Treat source verification and configured product usability as separate requirements.

2026-10-22. This check follows the [full implementation review](PLAN_REVIEW.md) and verifies the files Git would actually commit, including the new modules, fixtures and all 38 migrations. It introduces no application behavior or database migration.

## Final packaging fixes

- `.dockerignore` now excludes the private credential directory, local environment files, the documented Python environment and Python caches. Example environment files remain available. A real Docker context export with harmless marker files verifies both exclusion and example-file inclusion.
- The Protobuf compatibility check in CI and Taskfile now resolves `origin/main` to its immutable commit instead of assuming a local `main` branch. In a disposable detached checkout without local `main`, the original command fails and the corrected command passes.

## Clean-export verification

A temporary Git index captured the complete candidate without changing the user's staging contents or creating a commit. Its archive contains Git-normalized source bytes rather than dependencies or artifacts from the development workspace.

From that clean export:

- Frozen-lockfile installation passed without manifest/lockfile changes.
- Go tests, vet and all executable builds passed; Linux Go tests passed separately.
- All **84 integration cases** passed, with the existing four opt-in live/browser skips. The separate **16-check real-backend browser gate** passed.
- All **76 browser regressions** passed, retaining 50 intentional project skips.
- The production UI build, six model-validator checks and exact generated TypeScript reproduction passed.
- The actual UI Docker image built successfully with the repository's Node 20/Linux/Nginx recipe.

Local Markdown links resolve in the candidate. Its file inventory excludes private/runtime credentials, local environment files, compiled executables, caches and test output; intentional `.example` files and source fixtures remain. Common token/private-key format checks found no matches. All 38 applied migrations, the migration script and the line-ending policy retain their protected hashes. The configured commit identity is present, and no active commit hook, commit template or trailer configuration was found.

The [machine-readable evidence](evidence/PRECOMMIT_ACCEPTANCE.json) records the tested application manifest, packaging fingerprints and checks. Earlier review evidence describes its historical working-tree bytes; this check additionally validates the normalized Git export. Raw test/build logs remain ignored under `tmp/precommit/`.

## Commit status

The earlier conclusion that no implementation or packaging blockers remained is withdrawn by the delivery audit. Preserve the tested source/build evidence, address the open correctness and delivery findings, and include new files in any later reviewed commit. The user's staging contents and commit history were left unchanged. No commit or push was performed.

Deployment rebuild/restart, provider credentials/agreements/admissions and future S4 work retain the limits documented in the [review](PLAN_REVIEW.md). They are separate from committing this tested implementation.
