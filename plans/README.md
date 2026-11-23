# Geographic environment and 3D infrastructure

The implementation covers 26 systems across environment preparation, the 3D viewer, and geographic data. Riyadh's reference release is staged. Shared staging runs migration 61 and has a new installation-bound F09 observation.

## Geographic priorities

1. **Riyadh** is the first detailed environment and the reference for terrain, imagery, feature inspection and the Atlas round trip. [Riyadh release record](IMPLEMENTATION_PROGRESS.md#riyadh-reference-release)
2. **Saudi Arabia** is the national discovery scope. Select the next detailed study area by user need, source coverage and rights.
3. **The Middle East** is the regional discovery scope. The current working profile covers 17 configured countries. [Regional profile](../scripts/data/region_profiles/middle-east-v1.json)

## Next acceptance steps

1. **Complete the migration 61 observation.** The metadata and storage samplers started at 13:27 Riyadh time on 23 November. The earliest completion is 13:27 on 24 November if the pinned installation stays available. The current ledger records eight passing technical checks. [Readiness record](../docs/implementation/COASTAL_MOVING_AND_READINESS.md#f09--installed-readiness)
2. **Record the release decisions and independent Riyadh walkthrough.** Authorized reviewers verify the three pinned releases. A separate receiver discovers the released Riyadh package, verifies the ZIP and display identities, and records the usable layers and limits. These checks can run during the observation. [Review and receiving procedure](../docs/implementation/RIYADH_RELEASE_RECOVERY.md#f03-release-review-and-receiving)
3. **Qualify real Riyadh use for the selected exercise.** Staging has zero active road-exercise admissions permitting internal analysis and zero accepted independent physical accuracy assessments for the Riyadh reference. Source approval, independently timed inspections for real closures, and offline issuance terms apply to their respective uses. [Riyadh access evidence](archive/simulation-sources/riyadh-access-delivery.md)

## First civilian exercise

The authored Riyadh road-closure prototype passed its three replay cutoffs, scoring controls, offline reader checks, and isolated PostGIS release and receiver tests on 23 November. Its owned input package is sufficient for that prototype. Shared staging retains its separate release and receiving decisions. [Qualification record](../private/environment-foundation/implementation/simulation-readiness-20261123T102606Z/prototype-qualification.json)

## Active source plans

- [Desert Maps place names and geographic reference](DESERT_MAPS_PLACE_REFERENCE.md) covers Arabic place search, thematic map layers, and a versioned handoff for civilian exercises. The first regions are Riyadh, Eastern Province, and Jazan. Native publication depends on publisher permission.

## References

- The [archived simulation source plans](archive/simulation-sources/README.md) record the four delivered workflows, normalization rules and verification cases. [Source admissions and remaining qualification](archive/simulation-sources/delivery.md#remaining-platform-and-evidence-work)
- The delivered [simulation preparation implementation plan](archive/simulation-preparation/SIMULATION_PREPARATION_IMPLEMENTATION_PLAN.md) and [November 15 review](archive/simulation-preparation/SIMULATION_PREPARATION_REVIEW_2026_11_15.md) are archived; the [delivery record](../docs/implementation/SIMULATION_PREPARATION_DELIVERY.md) states what is implemented and what remains unqualified.
- [Implementation record and evidence](IMPLEMENTATION_PROGRESS.md) covers the environment foundation (F01–F10), viewer (V01–V08), and data pilots (D01–D08).
- [Environment operations](../docs/implementation/ENVIRONMENT_LIBRARY_OPERATIONS.md) covers commissioning, recovery, display publication and viewing.
- [Diagram prompts](../Diagram-prompts.md) describe the systems and workflows in prompts ready to share.
- The original 26 plans, implementation sequence and source coverage map are preserved in the [implementation commit](https://github.com/SalehAlobaylan/c4isr-systems/tree/8fe5b1c320f05c93256019f781b5b780f0b505bb/plans).
