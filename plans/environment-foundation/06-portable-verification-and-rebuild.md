# F06 — Portable verification and reproducible rebuild

[Plan index](../README.md)

**Status** The portable reader verifies the original and newer profile packages. The Riyadh, Al Ain and Avignon archives rebuilt byte for byte from the current paired snapshot. Installed rebuild issuance depends on release. [Portable verification evidence](../../docs/implementation/RIYADH_RELEASE_RECOVERY.md#f06-portable-verification-and-rebuild)

**Implementation owner** Data tooling engineering.

**Dependencies** [F02 — Geographic profiles, coordinates and quality](02-geographic-profiles-and-quality.md), [F05 — Artifact delivery, integrity and storage lifecycle](05-artifact-delivery-and-lifecycle.md)

## Outcome and baseline

An independent reader can validate a package and rebuild it from retained inputs without provider network access. All three Linux candidates already rebuilt byte-for-byte in fresh containers with networking disabled. Windows and Linux retain distinct toolchain and scoped input identities.

## Features and contracts

Maintain a versioned offline manifest reader, a rebuild bundle and a pinned processor distribution. Report byte identity, normalized content agreement, geographic validity and rights metadata separately. The caller supplies the expected manifest digest from an authorized release record.

Include schema version, recipe, profile revision, processor image or executable digest, library versions and all retained dependencies. Preserve the timestamp-free reproducible content identity and keep execution timestamps in the audit report. A matching content hash alone does not grant access.

## Implementation sequence

1. Package the existing reader with documented supported schema and toolchain versions.
2. Add compatibility fixtures for current Windows and Linux releases and future derivative manifests.
3. Resolve missing dependencies before expensive reconstruction and provide exact diagnostic identities.
4. Run reconstruction in a clean workspace with provider networking disabled and bounded disk use.
5. Add a machine-readable comparison report and a concise consumer-facing verification summary.

## Acceptance

- All three pinned Linux candidates reproduce their recorded manifest and ZIP identities.
- The Riyadh reference release passes offline verification and reproduces its recorded manifest and ZIP from the pinned toolchain and retained inputs.
- Wrong manifest, extra archive member, unsafe path, corrupt bytes and missing inputs fail verification.
- Old packages remain readable after additive schema extensions.
- A changed toolchain produces a recorded new identity or explicit equivalence result.
- Reader output preserves masks, source times, native datums and unknown physical accuracy.
- The independent consumer can complete the process from the delivered instructions.

## Rollout and recovery

Publish the reader and toolchain before a new producer schema. Keep prior supported verifier versions accessible with their hashes. Failed rebuild output remains temporary and can be removed safely without touching retained parents.

## Deliverable evidence

Deliver a portable verification command, pinned toolchain manifest, offline rebuild recipe and exact comparison results. Use the existing verify and rebuild commands as normal entry points. F03 owns human interpretation and release.

## Existing implementation references

- [scripts/data/environment_library.py](../../scripts/data/environment_library.py)
- [scripts/ops/check_installed_environment_library.py](../../scripts/ops/check_installed_environment_library.py)
- [internal/geoenv/releases.go](../../internal/geoenv/releases.go)

[Foundation evidence](../../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md) · [Original implementation plan](../../docs/implementation/ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md)
