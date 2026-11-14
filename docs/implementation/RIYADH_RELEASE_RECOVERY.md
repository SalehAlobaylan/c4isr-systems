# Riyadh release verification and recovery

The current Riyadh reference is `georelease_01m786xtwrr6d0sm1by5n5r4dc`. Its manifest SHA-256 is `f338cbd833efa47fbf79c58702883e2e52fcd977987e6ede9e10f113f73fbeb6` and its prepared ZIP SHA-256 is `d58ffd5fd45641b3c9aeb53a21321a68de78a85544b6ce0440382d2fbfdeb249`. It was staged when inspected on 13 November. Al Ain and Avignon provide separate recovery checks. [Installed acceptance ledger](../../private/environment-foundation/implementation/coastal-moving-qualification/readiness-ledger-release-recovery.json)

## F03 release review and receiving

The release form binds its draft to the exact release and digest. Before a new verification or release decision, the API streams the prepared object and checks its length and digest. The existing transaction still checks source policy and serializes decisions. An authorized reviewer can read the decision history after withdrawal. The PostGIS test covered a wrong manifest digest, same-length archive corruption, duplicate release submission, a receiver restricted to read and download, and withdrawal with retained history. [Release service](../../internal/geoenv/releases.go) · [Integration checks](../../internal/integration/geographic_environment_test.go)

For a live decision, the reviewer should read **Release quality** and **Source lineage and download rights**, verify the downloaded prepared ZIP against the manifest and archive digests, and record the evidence and limits under **Review evidence**. Physical accuracy remains unknown. Decision history is available to authorized reviewers on migration 58. The installed release and independent operator decisions remain separate from the engineering rehearsal. [Environment operations](ENVIRONMENT_LIBRARY_OPERATIONS.md)

The receiver uses a separate account with `environments.read`, `environments.download`, and grants for the release's sources. After the release appears in the receiver's library, run the following from a clean directory. `receive_environment_release.py` discovers the exact release through the paged catalog, checks the pinned manifest and the range/full transfer, and publishes a verified file and a receipt. Share the complete hashes with the receiver through the release record.

```text
python scripts/ops/receive_environment_release.py riyadh-prepared.zip --api API --workspace WORKSPACE --environment ENVIRONMENT --token-file RECEIVER_TOKEN --release RELEASE_ID --manifest-sha256 MANIFEST_SHA256 --prepared-sha256 PREPARED_SHA256 --display DISPLAY_ID=DISPLAY_SHA256
```

Repeat `--display` for each independently supplied display pin. The receiver checks the display manifest, exact release, length and downloaded bytes before saving the prepared ZIP. The receipt records each verified display. Record the operator's walkthrough separately.

In the isolated copy, a receiver scoped to the exact Riyadh sources discovered the released package, verified the 5,716,535-byte ZIP and received the authorized 379,751,071-byte rebuild bundle. The verifier checked all original inputs in the issued bundle. The receiver could not verify a release, and withdrawal removed the prepared download. The shared candidate remains staged pending its reviewer and independent operator. [Isolated receiver evidence](../../private/environment-foundation/implementation/coastal-moving-qualification/riyadh-receiver-rehearsal.json)

The 14 November review repeated the workflow in a disposable migration 58 copy. The receiver also verified all six pinned Riyadh displays by full SHA-256 and length. The receipt SHA-256 is `f887a154b1c8e27b59c3d3816a583074d47ed62d642370c0a76ca35b6cbd57d2`. [Display verification receipt](../../private/environment-foundation/implementation/coastal-moving-qualification/riyadh-receiver-display-review.json)

## F06 portable verification and rebuild

The portable reader checks the prepared archive, exact manifest, scoped input identities, member integrity, and profile-specific quality checks. It supports the original reference profiles and the enrichment, observation and coastal editions. Seven retained packages passed the compatibility check, including the exact installed Riyadh, Al Ain, and Avignon archives. Wrong manifest, missing input, corrupt input and unsafe or extra members fail before preparation starts. [Portable reader](../../scripts/ops/verify_environment_rebuild.py) · [Compatibility evidence](../../private/environment-foundation/implementation/coastal-moving-qualification/portable-reader-compatibility.json)

Use the prepared archive or rebuild archive as follows. Pass the full release digest from the authorized record. For a rebuild, use the pinned preparation image recorded with the release and run with networking disabled. The Riyadh, Al Ain and Avignon workloads use image `sha256:8a8213b068eef19ef1c1b8ede5611e26c85faf5df831f13ba37705f01b542ade` with toolchain `06d10f6a3fc5fafe1ac36f929bdcabd3ce398ffff1fbdfb6fafc1a1caae6f66a`. Earlier or specialized profiles keep their original images and pins.

```text
python scripts/ops/verify_environment_rebuild.py verify PREPARED_ZIP --manifest-sha256 MANIFEST_SHA256 --prepared-sha256 PREPARED_SHA256
python scripts/ops/verify_environment_rebuild.py INPUTS_ZIP NEW_PREPARED_ZIP --manifest-sha256 MANIFEST_SHA256 --prepared-sha256 PREPARED_SHA256
python scripts/ops/verify_environment_rebuild.py toolchain PROFILE_ID --image-id IMAGE_SHA256
```

The toolchain report records the declared runtime image ID, computed preparation fingerprint, reader digest and installed library versions. Supply the image ID from the runtime's inspection output. [Reference processor receipt](../../private/environment-foundation/implementation/coastal-moving-qualification/portable-toolchain-reference.json)

The rebuild verifies recipe-to-manifest agreement and hashes every original before preparation. It publishes the output only after the expected checks pass. Using the paired snapshot and the exact worker image, fresh offline containers reproduced the Riyadh, Al Ain and Avignon manifests and prepared ZIPs byte for byte. The same command reproduced the Riyadh enrichment and observation packages and the Jubail coastal package with their own pinned processors. The issued rebuild route requires a released environment and effective permission to distribute the originals. [Installed offline rebuilds](../../private/environment-foundation/implementation/coastal-moving-qualification/portable-paired-rebuilds.json) · [Specialized offline rebuilds](../../private/environment-foundation/implementation/coastal-moving-qualification/portable-specialized-rebuilds.json)

## F07 paired backup and recovery

The 13 November paired backup captured the installed database, 59 eligible objects totaling 2,865,321,183 bytes, and the retained navigation originals. The restore used a new Compose namespace and completed in 101.7 seconds including the previous API and forward recovery checks. It compared 129 durable table fingerprints and 1,599 security definitions, revalidated source and access policy against staging, and fetched the pinned manifests, packages and published displays through scoped API routes. The data cutoff was 20:12:28 Riyadh time. The previous V05 API and the current API both served the restored objects on migration 56. That result covers read compatibility on this recovery set. [Backup manifest](../../private/environment-foundation/implementation/coastal-moving-qualification/paired-current-v2/manifest.json) · [Restore and previous-image result](../../private/environment-foundation/implementation/coastal-moving-qualification/paired-current-final.json)

The capture uses a PostgreSQL snapshot, conditional reads of the object store, and a before-and-after comparison of durable database state and object listings. If a durable table, source policy or object listing changes during the capture, take a fresh snapshot during a quieter period. Worker heartbeats remain in the database dump. Their changing timestamps are excluded from the stable data comparison. The restore rejects a changed backup member and stops before serving the copied database if captured source or access policy is stale. The same procedure restored all 59 objects again after the migration 58 cutover and verified the previous API and forward recovery. [Migration 58 restore](../../private/environment-foundation/implementation/coastal-moving-qualification/paired-installed-58-restore.json)

```text
python scripts/ops/staging_environment_recovery.py capture CANDIDATE_JSON PRIVATE_BACKUP_DIRECTORY
python scripts/ops/staging_environment_recovery.py verify PRIVATE_BACKUP_DIRECTORY --manifest-sha256 BACKUP_SHA256
python scripts/ops/staging_environment_recovery.py restore PRIVATE_BACKUP_DIRECTORY COMPOSE_SETTINGS REVIEWER_TOKEN PRIVATE_RESTORE_REPORT --manifest-sha256 BACKUP_SHA256 --previous-api-image PREVIOUS_IMAGE_SHA256
```

Use the restore evidence alongside the earlier two-build recovery result, which reproduced both prepared ZIPs from queued jobs in a disposable copy. An operator must reconcile current grants and policy again before activating a recovered installation. [Two-build recovery](../../private/environment-foundation/implementation/coastal-moving-qualification/installed-two-builds.json)
