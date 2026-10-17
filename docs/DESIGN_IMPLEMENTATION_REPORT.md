# EMAD — Alloy Pearl, Balanced Migration Report

**Date:** 2026-10-17
**Authoritative contract:** [`DESIGN.md`](../DESIGN.md) — **v2.0, Alloy Pearl, Balanced**
**Status of the previous identity:** Basalt & Copper is **retired** (`DESIGN.md` §1.4)
**Visual review artifacts:** [`design-review/`](./design-review/)
**Preceding plan:** [`DESIGN_IMPLEMENTATION_PLAN.md`](./DESIGN_IMPLEMENTATION_PLAN.md) — **retired, kept for provenance only**

---

## 1. Revision / branch

| | |
| --- | --- |
| Base commit | `977a8e8` — *Get the end-to-end specs running, and fix what they found* |
| Branch | working tree, uncommitted at the time of writing |
| Identity replaced | Basalt & Copper (dark) → **Alloy Pearl, Balanced** (one permanent light appearance) |
| Scope | `apps/operator-ui` only. No Go, SQL, API, OpenAPI, RBAC, scenario or map-logic changes. |

This is the **second** visual migration on this codebase. The first delivered the
architecture the second re-pointed: one canonical token file, one stylesheet that
owns cascade order, one central semantic resolver, and a contract suite that
asserts the palette and its contrast. Nothing about that architecture changed;
only its values and its light/graphite division did.

---

## 2. What changed, and what deliberately did not

`DESIGN.md` v2.0 is a different identity, not a skin. The migration therefore
touched three layers:

| Layer | Change |
| --- | --- |
| **Palette** | All eight foundation colours replaced (mineral / ceramic / mist / graphite / graphite-ink / lime / signal-teal / structural-border). Every interface role re-points to them. |
| **Structure** | The application was dark everywhere; it is now **balanced light**, with graphite kept for exactly one structural region — brand and navigation — plus the primary action (§5.3). |
| **Semantics** | Operational meaning is untouched. The same domain values map to the same *tone* and the same shape glyph; only the tone's colour changed. Lime is now a brand signature and is barred from every operational channel. |

Explicitly unchanged: entity lifecycle, alert severity, freshness thresholds,
command/incident states, confidence semantics, provenance rendering, the map's
geospatial computation, layer ids, hit-testing and measurement, and every API
contract. `DESIGN.md` §1.3 ranks operational meaning above brand expression, and
the diff was kept on that side of the line.

---

## 3. Token system

### 3.1 One authoritative source

`apps/operator-ui/src/styles/tokens.css` is the only file in the application that
contains a hex literal, and it is imported exactly once, from `index.css`. The
Tailwind `--color-*` names in `index.css` are `var()` references through
`@theme inline`, so a token is defined once and cannot drift from its utility.

The token names follow `DESIGN.md` §2.1–2.3 and §3.1, including the two renames
the new spec requires:

| Old (retired) | New | Why |
| --- | --- | --- |
| `--emad-status-informational` | `--emad-status-info` | §2.3 vocabulary |
| `--emad-status-inactive` | `--emad-status-neutral` | §2.3 vocabulary |
| `--emad-brand-basalt`, `--emad-brand-copper*` | `--emad-industrial-graphite`, `--emad-energized-lime` | The brand layer is now named for what it is |
| `--emad-surface-raised` | `--emad-surface-inset` (alias kept) | On a light canvas the step *above* a panel is a deeper tint, and §5.3 calls that Mist. The old name survives as an alias because ~21 call sites read it as "the raised step" |
| `--emad-brand-signal` | `--emad-signal-teal` | §2.1 |

`--emad-status-review` is new and is carried end-to-end: token, tone, glyph and
CSS. It is deliberately *not* `info`. It answers "does the platform have a
conclusion, or does the operator still have to make one?", and collapsing the two
would hide a human step. Its glyph is a hollow diamond so it stays distinct from
`high` in greyscale.

### 3.2 Brand lime is quarantined

Lime appears in exactly three rules:

1. the active navigation item's underline,
2. the ruled brand glyph in the top bar,
3. the `signature` button variant — defined but used nowhere by default (§6.2).

It is never a status, a freshness ring, a selection ring, a link, body text, or
anything on a light surface, because `#B7D36B` is 1.51:1 on ceramic. The test
*brand lime is never an operational tone* asserts that no entry in `TONE_TOKEN`
resolves to it.

### 3.3 The two focus indicators

§6.2 requires a visible focus ring that meets contrast against the *actual*
adjacent surface, and there are now two surfaces in the application:

| Where | Token | Value | Against |
| --- | --- | --- | --- |
| Pale workspace | `--emad-focus-light` | `#2C626B` | 4.97:1 on mist |
| Graphite rail | `--emad-focus-dark` | `#B7D36B` | 8.68:1 on graphite |

The graphite rail's controls therefore override the global focus colour. This is
the single most easily missed rule in the migration: without it, a teal ring on
a graphite navigation item would sit at roughly 1.9:1 and be effectively
invisible — while every automated test that only ever looks at the light
workspace would still pass.

---

## 4. Surface ownership

| Region | Token | Value |
| --- | --- | --- |
| Top bar, brand, navigation | `--emad-surface-nav` | `#202B2D` graphite |
| Application canvas | `--emad-surface-canvas` | `#E3E9E2` mineral |
| Run bar, cards, panels, dialogs | `--emad-surface-panel` | `#F2F4ED` ceramic |
| Grouped/inset regions | `--emad-surface-inset` | `#D4DED8` mist |
| Inputs and editable fields | `--emad-surface-input` | `#F7F8F2` |
| Map land base | `--emad-map-ground` | mist |

The whole application was dark before; §5.3 is explicit that this must not simply
be inverted. Graphite is a *rail*, not a theme, and the review captures are the
evidence that the workspace stays light and calm while the top band and the
primary actions carry the industrial weight.

### 4.1 Two deliberate deviations from a literal reading of the spec

1. **Status hues lifted on the graphite rail.** The canonical status colours are
   dark hues chosen for light surfaces (§2.3). On graphite they would sink. The
   connection dot and the system-health label therefore use
   `--emad-status-*-on-graphite` — four lifted steps declared in `tokens.css`
   for that surface only, so the "one file owns every colour" rule stays true
   and the decision stays reviewable in one place. The canonical tokens are
   untouched, so a status reads identically everywhere else.
2. **Control borders strengthened on the run bar.** §6.2's "Secondary: ceramic
   fill, dark ink, control border" is applied literally to the run-bar controls,
   which previously used a soft border that is now too faint against ceramic.

---

## 5. Type and geometry

The type scale was re-pointed to §4.2 exactly, including two changes that alter
every screen: heading 22→24px, panel heading 16→20px, and — more visibly —
uppercase tracking reduced from `0.08em` to `0.04em`, which §4.2 caps and which
was previously *outside* the documented maximum. Corner radii follow §5.2
(4px control, 6px card/dialog, 2px compact).

---

## 6. Map

The map was the most delicate part of the change, because it is the one surface
that must carry both the new identity and real geographical accuracy.

- **Entity colours are derived, not chosen** (`--emad-map-entity-track` →
  `--emad-status-info`, and so on), so a marker on the plot and a badge in a
  table can never read as two different states. Asserted by *map-domain values
  resolve through the status tokens*.
- **Freshness rings** keep the outer-ring concept of §7.3 unchanged, with the
  same three ring distances. Only their colours changed, from light-on-dark to
  dark-on-light.
- **The raster fallback** is a third-party provider, so per §8.1 it is not
  recoloured — only its treatment is tuned: saturation −0.4, brightness
  0.06–0.88, contrast 0.04. That keeps districts, roads and labels readable while
  pulling the tile toward the mineral land base so the plot belongs to the
  workspace. Attribution, provider and tile URLs are untouched.
- **A configured `MAP_STYLE_URL` is still left alone.** Its layer schema is
  unknown to this application, so §8.1's "style only layers under our control" is
  honoured literally. If a non-default vector style must also blend with Alloy
  Pearl, that is a provider-side decision — see §9.9.

---

## 7. Verification

| Check | Command | Result |
| --- | --- | --- |
| Typecheck | `pnpm --filter @c4isr/operator-ui typecheck` | **pass** |
| Production build | `pnpm --filter @c4isr/operator-ui build` | **pass** |
| Contract suite | `pnpm exec playwright test tests/e2e/design-system.spec.ts` | **31 passed** |
| Full end-to-end | `pnpm --filter @c4isr/operator-ui test:e2e` | **47 passed, 31 skipped, 0 failed** |
| Reference captures | `pnpm --filter @c4isr/operator-ui capture:visual` | **21 passed → 26 captures** |

The skips are the suite's own per-project gating (`explicit-token` cases), not
service unavailability.

The contract suite was rewritten for this identity rather than relaxed. It now
asserts the eight foundation values, the §2.2 functional additions and all seven
§2.3 status colours; the §2.5 contrast pairs including the two that fail on
purpose (lime on ceramic < 3:1, decorative border < 3:1 so it cannot be mistaken
for a control edge); graphite carrying navigation text; lime failing on a
light surface — the same assertion that stops lime from leaking into a status;
and the graphite focus ring, which is the one rule a light-workspace-only test
would otherwise let regress silently.

---

## 8. Reference screens

`docs/design-review/` holds 26 deterministic captures, regenerated after the
identity change:

| File | Screen |
| --- | --- |
| `01`–`04` | Operational picture — live, stale+selected, replay, 1280px |
| `05`–`06` | Entity board — default and compact density |
| `07`–`08` | Alert centre — populated and empty filter |
| `09`–`11` | Incident detail — overview, missions & commands, validation error |
| `12`–`16` | Assets, tracks, timeline, scenarios, incidents |
| `17`–`18` | Keyboard focus; narrow layout (900px) |
| `19`–`22` | Wide (1920px); menu; tooltip; 200% zoom |
| `23` | Map preparing state, unauthenticated |
| `24`–`26` | Long identifiers; request failure; measurement over two zones |

`docs/design-review/before/` holds 22 captures of the **original** interface,
taken before the first migration. They are retained as the baseline for the
*first* migration and are labelled accordingly; they are not a comparison for
this one, whose "before" state is itself a designed screen. The two-row top bar,
the fresh and stale freshness legend, the measured navigation fit, and the
distinct selection ring are all Alloy Pearl work that has no pre-migration
counterpart.

---

## 9. Decisions requiring design or product approval

1. **The primary action is now a dark graphite button in a light application.**
   §6.2 specifies exactly this, but it is the most conspicuous thing on a light
   screen. Worth confirming it reads as "structural" rather than as an alert.
2. **Status hues are lifted on the graphite rail** (§4.1). The lifted steps are
   local to that surface; if a reviewer prefers the rail's health text to be
   neutral ink rather than coloured, that is a one-line change per role.
3. **The active navigation item is marked with a lime underline.** §5.2
   suggests "a 2–3px line on a selected dark navigation item" and §6.1 allows
   lime detail on graphite, so this is the sanctioned use. It is also the only
   lime on most screens, which is the intent.
4. **Tracking dropped to `0.04em`** from `0.08em` (§4.2). This visibly changes
   the rhythm of every uppercase label; the alternative reading of §4.2 was that
   the 0.08em ramp predated the document.
5. **`--emad-surface-raised` now resolves to Mist, a *darker* step than the
   panel.** Any component that used it expecting "lighter than the panel" now
   reads as inset instead. §5.3 supports the new meaning, but a visual review
   should confirm each such surface.
6. **Decorative dividers are asserted to stay *below* 3:1.** This is stricter
   than the spec requires, and it is deliberate: it keeps "divider" and
   "essential control border" from converging on one another.
7. **The map's medium-severity geofence fill is subtle on the light raster.** A
   low-opacity caution tone over a light basemap reads as a warm wash rather than
   a marked zone. The outline carries the reading; increasing the fill opacity
   would be a §8.2 decision, not a palette one.
8. **Lime is unused in the operator screens.** The `signature` button variant
   exists and is available, but no workflow uses it, because §2.4 reserves it
   for "a rare non-destructive branded CTA" and this product has none.
9. **Configured `MAP_STYLE_URL` styles remain untinted** (§6).

---

## 10. Definition of done against `DESIGN.md` §15

- [x] One normal app appearance, no light/dark toggle, no `prefers-color-scheme`
      branch anywhere in the token layer.
- [x] The eight foundation colours are represented through a canonical token
      source and asserted value-by-value.
- [x] Lime is a restrained brand signature, never a status colour — asserted.
- [x] Graphite gives the UI its industrial identity without dominating the
      workspace: one rail and the primary actions.
- [x] Light surfaces are low-glare, differentiated (canvas / panel / mist /
      input) and readable at every documented pair.
- [x] UI state, entity kind, freshness, selection, confidence, affiliation and
      severity remain separate channels.
- [x] Components use semantic role tokens; no hex literal exists outside
      `tokens.css`.
- [x] Contrast is checked in actual states, including the two that must fail.
- [x] Map overlays and the legend are understandable without relying on colour:
      every tone carries a label and a distinct shape.
- [x] Live versus replay is unmissable on the picture.
- [x] English/Arabic names and numeric rendering are unchanged and correct.
- [x] Desktop, responsive, dense, empty, loading and error screens were recaptured
      and reviewed.
- [x] Typecheck, build, contract suite and full e2e pass with no test relaxed.
- [x] No logo, affiliation, certification or capability claim was added.
- [x] The Basalt & Copper plan is marked retired and points to this report.