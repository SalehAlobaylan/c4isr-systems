# D07 — Moving observations and orbital context

[Plan index](../README.md)

**Status** Bounded history and geographic handoff verified with test-owned reports. The private NOAA-20 pilot reproduced offline. Real moving history requires an admitted source. [Evidence and limits](../../docs/implementation/COASTAL_MOVING_AND_READINESS.md#d07--historical-moving-reports-and-civil-orbital-context)

**Implementation owner** Public observation and geospatial engineering.

**Dependencies** [F01 — Source admission, lineage and retention](../environment-foundation/01-source-admission-and-lineage.md), [V06 — Context overlays, freshness and coverage](../3d-infrastructure/06-context-overlays-and-freshness.md), [V07 — Frozen environment and observation handoff](../3d-infrastructure/07-frozen-environment-context-handoff.md)

## Outcome and activation

A viewer can inspect authorized historical moving reports and, for an identified civil space use case, dated orbital context. Existing EMAD adapters retain aircraft/vessel reports. World Monitor demonstrates globe trails and catalog-based orbital propagation.

## Features and contracts

For moving reports, preserve provider identity claim, report time, coordinate/altitude reference, speed/heading units, uncertainty, gaps and source revisions. Any interpolated display position records the interpolation method and support interval.

For orbital context, admit an actual catalog product with element epoch, object identifier, reference frame and source revision. Retain the propagator/version, requested time interval and validity policy. Use established library implementations and qualify reference samples. Footprints require a declared geometric meaning and source.

## Implementation sequence

1. Identify a legitimate source and retention/export permissions for a bounded history.
2. Expose existing moving records through V06 and exact revision selection through V07.
3. Add gap-aware trail rendering and clear observed-versus-interpolated labels.
4. If civil orbital context is required, add the catalog adapter and a pinned propagation derivative.
5. Validate time/frame conversions and inspect stale or invalid element handling.
6. Package only source-permitted context and the dependencies needed to reproduce displayed values.

## Acceptance

- Frozen history preserves source revisions and original timestamps.
- Gaps remain visible and interpolation is bounded and labeled.
- Missing altitude datum prevents unsupported 3D altitude claims.
- Orbital samples match the selected reference implementation and documented tolerance.
- Out-of-validity elements produce an explicit unavailable/stale result.
- Display-derived paths or footprints carry their stated geometric limits.
- Hosted-service exclusions and source-specific rights are enforced at acquisition/export.

## Rollout and recovery

Enable each source capability separately. Start with already admitted moving observations. Catalog propagation is optional and depends on a stated consumer need. Corrections create new derivatives and preserve original inputs.

## Deliverable evidence

Deliver one reproducible observation window, gap and timestamp tests, applicable source terms and, when activated, reference-checked orbital samples. The deliverable is environmental context for inspection and future data consumption.

## Implementation and source references

- [internal/publicingest/moving.go](../../internal/publicingest/moving.go)
- [internal/publicingest/opensky.go](../../internal/publicingest/opensky.go)
- [internal/publicingest/ais.go](../../internal/publicingest/ais.go)
- [internal/contextpackage/domain.go](../../internal/contextpackage/domain.go)
- [World Monitor src/services/satellites.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/src/services/satellites.ts)
- [World Monitor docs/orbital-surveillance.mdx](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/docs/orbital-surveillance.mdx)
- [World Monitor docs/license.mdx](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/docs/license.mdx)
- [World Monitor hosted terms](https://www.worldmonitor.app/docs/eula)
