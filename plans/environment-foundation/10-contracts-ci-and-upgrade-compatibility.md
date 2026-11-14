# F10 — Contracts, CI and upgrade compatibility

[Plan index](../README.md)

**Status** Shared staging passed migration 56-to-58 and exact release readback. The contract, Environment Library browser checks and all 16 real-backend browser cases passed. The frontend follow-up is qualified in the worktree. [Compatibility evidence](../../docs/implementation/GEOGRAPHIC_OPERATIONS_AND_UPGRADE.md#f10-contracts-and-compatibility)

**Implementation owner** Backend, frontend and quality engineering.

**Dependencies** Existing platform services and repository build baseline.

## Outcome and baseline

A clean source export builds the feature, generated clients match the API, and upgrades preserve existing packages and callers. The current OpenAPI lint passes. The source export includes the coverage service that the earlier ignore rule had omitted.

## Features and contracts

Use additive schema migrations and versioned manifests. Check generated API types, runtime request validation, permissions and error shapes together. Retain compatibility fixtures for existing releases and old profile versions. Keep each plan’s new routes explicitly marked proposed until implemented.

## Implementation sequence

1. Reproduce clean-checkout builds and current contract diagnostics at the selected baseline commit. Group lint errors by underlying cause and affected callers.
2. Repair the wider contract incrementally, preserving established response behavior or recording an intentional compatible migration.
3. Add new environment and viewer endpoints to the existing OpenAPI source and regenerate the UI client.
4. Wire focused tests for permissions, corrupted content, lifecycle races and old-package reading into CI.
5. Exercise schema → API → workers → UI rollout and feature suspension in staging.
6. Record the executed checks, skip conditions and failures against the selected candidate.

## Acceptance

- Required source files are tracked and a fresh checkout builds without private workspace files.
- Contract lint has no errors for the shipped contract and warnings have a recorded disposition.
- Client regeneration produces no unexplained drift.
- Existing v1 packages and published API entry points retain their documented behavior.
- A mixed-version rollout fails unsupported work clearly and preserves valid history.
- Targeted Go/integration/race, Python and assembled browser checks pass for the changed paths.
- Browser regressions include 390px portrait, 740px landscape and keyboard operation.

## Rollout and recovery

Use the repository’s existing toolchain and CI commands. Do not manually patch generated clients. Validate the previous binary through F07 before claiming rollback support. Suspend new work using the existing feature control while retaining authorized history.

## Deliverable evidence

Deliver clean-checkout build logs, contract diagnostic disposition, generated-client checks, regression results and upgrade compatibility evidence. Apply these checks to each increment while independent design and fixture preparation proceed.

## Existing implementation references

- [api/openapi/openapi.yaml](../../api/openapi/openapi.yaml)
- [api/openapi/environment-library.yaml](../../api/openapi/environment-library.yaml)
- [apps/operator-ui/package.json](../../apps/operator-ui/package.json)
- [.github/workflows/ci.yml](../../.github/workflows/ci.yml)
- [internal/integration/geographic_environment_test.go](../../internal/integration/geographic_environment_test.go)

[Foundation evidence](../../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md) · [Original implementation plan](../../docs/implementation/ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md)
