# Geographic environment and 3D infrastructure

The implementation covers 26 systems across environment preparation, the 3D viewer, and geographic data. Riyadh's reference release is staged. Shared staging remains on migration 58, and migration 59 passed its isolated rehearsal.

## Geographic priorities

1. **Riyadh** is the first detailed environment and the reference for terrain, imagery, feature inspection and the Atlas round trip. [Riyadh release record](IMPLEMENTATION_PROGRESS.md#riyadh-reference-release)
2. **Saudi Arabia** is the national discovery scope. Select the next detailed study area by user need, source coverage and rights.
3. **The Middle East** is the regional discovery scope. The current working profile covers 17 configured countries. [Regional profile](../scripts/data/region_profiles/middle-east-v1.json)

## Next acceptance steps

1. **Restart the F09 observation.** Capture the exact migration 58 installation and API process generation, then record a full 24-hour metadata window, a worker restart and storage trends. The previous window stopped early. [Readiness procedure](../docs/implementation/COASTAL_MOVING_AND_READINESS.md#f09--installed-readiness)
2. **Record the release decisions and independent Riyadh walkthrough.** Authorized reviewers verify the three pinned releases. A separate receiver discovers the released Riyadh package, verifies the ZIP and display identities, and records the usable layers and limits. These checks can run during the observation. [Review and receiving procedure](../docs/implementation/RIYADH_RELEASE_RECOVERY.md#f03-release-review-and-receiving)
3. **Qualify migration 59 on shared staging.** After the pinned window, take a paired backup, roll out the candidate and verify release bytes, queue, storage and the viewer. Keep Riyadh portable issuance, frozen handoff and physical mobile checks tied to their own release, source and device requirements. [Upgrade rehearsal](../docs/implementation/GEOGRAPHIC_SOURCE_QUALITY_AND_STORAGE.md#rollout-and-evidence)

## References

- [Simulation data and feature delivery plan](simulation-sources/README.md) maps the shortlisted providers to features, regional exercises, acquisition steps and acceptance cases.
- The delivered [simulation preparation implementation plan](archive/simulation-preparation/SIMULATION_PREPARATION_IMPLEMENTATION_PLAN.md) and [November 15 review](archive/simulation-preparation/SIMULATION_PREPARATION_REVIEW_2026_11_15.md) are archived; the [delivery record](../docs/implementation/SIMULATION_PREPARATION_DELIVERY.md) states what is implemented and what remains unqualified.
- [Implementation record and evidence](IMPLEMENTATION_PROGRESS.md) covers the environment foundation (F01–F10), viewer (V01–V08), and data pilots (D01–D08).
- [Environment operations](../docs/implementation/ENVIRONMENT_LIBRARY_OPERATIONS.md) covers commissioning, recovery, display publication and viewing.
- [Diagram prompts](../Diagram-prompts.md) describe the systems and workflows in prompts ready to share.
- The original 26 plans, implementation sequence and source coverage map are preserved in the [implementation commit](https://github.com/SalehAlobaylan/c4isr-systems/tree/8fe5b1c320f05c93256019f781b5b780f0b505bb/plans).
