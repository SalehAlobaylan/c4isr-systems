# Implementation readiness for review

2026-10-22. The original 32 tasks through S3.3 and the selected S4.1 sensor-quality preset increment are delivered within their documented acceptance scopes. This closeout fixes concrete reliability and reporting gaps in that delivery. Further S4 capabilities remain a scenario-driven backlog.

The subsequent [full implementation-plan review](PLAN_REVIEW.md) records additional authorization/replay/reproducibility and UI corrections with new evidence. The checks below remain the historical closeout results.

## Recording worker reliability

The long-running `emad-recordings` process now survives individual archive failures and transient queue/database errors. Queue outages use cancellable exponential backoff from one to 30 seconds. Credential expiry, revocation or lost execution authority stops the process; backend error details, credentials and artifact keys are never printed. `-once` still performs at most one eligible attempt and returns failure when that attempt fails.

Archive retries are durable queue state: the first failure waits 20 seconds, the second waits 60 seconds, and the third exhausts the existing attempt budget. A waiting or exhausted job cannot prevent later eligible jobs from running. Discovery and claiming both recheck the retry time and retention window. Work has a nine-minute deadline inside the existing ten-minute lease, with publication still checking the generation, lease and current worker authority. This uses the existing `lease_until` column; no schema change is required.

The authorized recording index now includes `attempts`, `maxAttempts` and an optional `retryAt`. Replay keeps polling while an automatic retry remains possible, then stops at the limit. The UI explains the final failure and disables the ineffective repeated archive request. Repeating the request does not reset an exhausted budget or overwrite a published archive.

Replay now labels its clock as recorded and read-only, while the healthy connection indicator says CONNECTED. Authoring identities can open and reload an authorized recording URL without being redirected to Studio; their return link leads to Studio rather than a COP view unavailable in the authoring shell.

## Evaluation correction and compatibility

New reports use `emad.abstract-evaluation.v2`. A mean's numerator now contains the measured sum, including fractional distances, and its denominator contains the sample count. Together they reproduce the displayed value. Fraction metrics retain their count operands. The previous implementation calculated mean values correctly but labeled the sample count as the numerator.

Existing immutable v1 evaluations and released reports remain readable with their original version and operands; they are neither rewritten nor silently relabeled. The API accepts numeric numerators for both versions. The position-error limitation now accurately states that an active truth snapshot must match the exact sample millisecond; no interpolation is inferred.

## Build and verification commands

`task build:all` builds the API, provisioning tools and independent Go workers/runtimes into `bin/`. `task simulation:recordings:build` builds just the archive worker. The local race target now covers the delivered public-context, model, scene, runtime, gateway and recording packages, including the worker loop, in line with CI.

Verification results and hashes are recorded in [the closeout evidence](evidence/REVIEW_READINESS_ACCEPTANCE.json). This supplements the earlier milestone evidence, which retains its original v1 measurements. The closeout exercises actual disposable PostgreSQL/PostGIS and private S3, the assembled backend, compiled native runtime and real browser workflows, alongside the existing regression suites.

Passed: 80 integration cases (56 top-level and 24 subcases), 16 real-backend browser checks, 76 browser regressions, Go tests/vet, affected-package Linux race checks, all Go executable builds, the production UI build, OpenAPI lint and reproducible generated TypeScript/database models. Three credential-gated live-provider checks were intentionally skipped; the browser integration gate skipped in the main run passed separately. The mocked browser projects retain 50 intentional skips. Desktop/mobile replay and evaluation screenshots were visually inspected. The existing UI bundle-size advisory remains.

## Rollout and remaining scope

The user reports migrations 37 and 38 are applied. No additional migration is introduced, and applied migrations 1–38 are unchanged. Rebuild/restart the API, UI and recording worker to pick up this closeout. Preserve the existing private object-store configuration and environment-bound worker credential; the worker's startup remains `emad-recordings -token-file <private-file>`. The previous native runtime admission and fresh-run capture requirements still apply. No production database or live provider was modified during this verification.

The software is ready for review within the accepted public/reference, synthetic-terrain and abstract simulation profiles after the checks recorded in the evidence pass. Provider credentials, permitted datasets, admissions and deployed process supervision remain environment configuration. Broader S4 modeling, physical sensor fidelity, global terrain streaming, federation and additional scale are future increments, not unfinished tasks in the delivered 32-task plan. The [prior rollout and limits](S3_2_S3_3_S4_1_RECORDING_EVALUATION_PRESETS.md) continue to apply.
