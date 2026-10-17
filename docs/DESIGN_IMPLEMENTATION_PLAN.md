# EMAD | Basalt & Copper Design Implementation Plan — **RETIRED**

> ## ⚠️ Superseded — do not execute
>
> **This plan targets a design identity that no longer exists.** It was written
> for "Basalt & Copper", a dark-default operator UI built around a copper accent
> on a basalt canvas. Both are **retired** (see [`DESIGN.md`](../DESIGN.md) §1.4),
> and `DESIGN.md` v2.0 supersedes the earlier identity with **Alloy Pearl,
> Balanced** — one permanent balanced-light appearance with industrial graphite
> anchors and a rare energized-lime signature.
>
> The plan is kept for provenance only, because it documents the migration that
> produced the current architecture: the single-token source, the cascade-order
> fix, the central semantic resolver, the measured map chrome, and the contract
> test suite all came from executing it. Those outcomes remain valid.
>
> **Everything in it that names a colour, a dark default, a copper accent, a
> light/dark toggle or the Basalt & Copper proportion targets is obsolete.** Do
> not use this document to make styling decisions.
>
> | For | Use |
> | --- | --- |
> | The visual contract | [`DESIGN.md`](../DESIGN.md) v2.0 — Alloy Pearl, Balanced |
> | How Alloy Pearl was implemented, and the evidence for it | [`DESIGN_IMPLEMENTATION_REPORT.md`](./DESIGN_IMPLEMENTATION_REPORT.md) |
> | Product, domain and architecture | `PRODUCT.md`, `ARCHITECTURE.md`, `IMPLEMENTATION_PLAN.md` |

**Version:** 1.0 *(retired)*  
**Date:** 2026-10-16  
**Status:** **Retired — superseded by `DESIGN.md` v2.0**  
**Repository:** `https://github.com/SalehAlobaylan/c4isr-systems`  
**Scope:** Existing React/Vite operator interface (`apps/operator-ui`)  
**Authoritative visual contract:** [`DESIGN.md`](../DESIGN.md) — **v2.0, not the v1 document this plan was written against**

> **Assignment to implementation agent:** Implement the approved Basalt & Copper visual system in the existing UI, incrementally and without changing backend contracts or operational behavior. Treat `DESIGN.md` as the styling and UX authority. Preserve current workflows, real-time behavior, map semantics, scenario replay, accessibility, and test coverage. Do not create or invent an official logo.

---

<!-- Everything below documents the retired Basalt & Copper migration. It is
     preserved verbatim as provenance and must not be executed as-is. -->

## 0. Read-first context and execution constraints

Before editing, read:

1. Root `AGENTS.md` and existing repository instructions.
2. Root `PRODUCT.md`, `ARCHITECTURE.md`, `IMPLEMENTATION_PLAN.md` to understand domain and implementation boundaries.
3. Root `DESIGN.md` in full.
4. The current UI styles and component structure, especially:
   - `apps/operator-ui/src/index.css`
   - `apps/operator-ui/src/components/board/board.css`
   - `apps/operator-ui/src/components/map/picture.css`
   - `apps/operator-ui/src/components/map/MapCanvas.tsx`
   - `apps/operator-ui/src/components/map/geojson.ts`
   - `apps/operator-ui/src/components/shared/badges.tsx`
   - `apps/operator-ui/src/components/ui/*`
   - `apps/operator-ui/src/components/layout/{AppShell,TopBar,ScenarioBar}.tsx`
   - `apps/operator-ui/src/routes/{map,board,alerts,incident-detail}.tsx`
   - `apps/operator-ui/src/lib/freshness.ts`
5. Existing operator UI tests in `apps/operator-ui/tests/e2e/`, including `operator-workflow.spec.ts` and `scenarios.spec.ts`.

### Verified repository baseline (inspected 2026-10-16)

The public default-branch snapshot inspected for this plan includes:
- React 19, TypeScript, Vite, Tailwind CSS 4, TanStack Router/Query/Table, Base UI, MapLibre GL JS, Playwright.
- Two overlapping CSS token systems in `src/index.css`: Tailwind `@theme --color-*` and legacy `:root` variables such as `--bg`, `--surface`, `--fg`, `--st-track`, `--st-warn`, `--st-crit`.
- Legacy colors including `#161a26`, `#1f2533`, `#4a6fb5`, `#273c6b`, `#913214`, `#b83a1f`.
- `MapCanvas.tsx` already reads some computed CSS variables, but also declares hardcoded colors and mappings for entities, geofences, areas, and measurement tools.
- Board and badge components use their own status-to-tone conventions.
- Current typography includes DIN-family fallbacks and extremely small `--t-*` values (9–11px).
- Existing primary screens: Board, Picture/Map, Alerts, Incidents, Audit; other screens include Assets, Tracks, Scenarios.

**Re-audit the checked-out revision** before implementing. This baseline may have changed. Do not assume these paths or file contents are frozen; adapt to the actual tree. The purpose is to make the plan concrete, not to override newer code.

### Hard guardrails

- **No backend changes** unless required to repair an unrelated preexisting test failure, and such change must be separately documented and approved.
- **No API, database, auth/RBAC, state machine, telemetry, geospatial business logic, or scenario semantics changes** for aesthetic reasons.
- **Do not adopt a new component library or rewrite the frontend.** Evolve existing UI primitives, CSS, and MapLibre styling.
- **Do not silently change displayed domain data, confidence values, timestamps, or status names.**
- **Do not invent a logo, tactical affiliation semantics, or military standards compliance.**
- Keep migration reviewable and reversible. Prefer cohesive small commits/PRs over a single visual overhaul.
- No unsupported new map provider or font licensing assumption.

---

## 1. Required output

The implementation is complete when the agent produces:

1. A canonical token stylesheet at `apps/operator-ui/src/styles/tokens.css` (or an equally central location justified in the PR).
2. Tailwind v4 integration without competing hardcoded palettes.
3. A tested, centralized semantic status mapping for shared badges and views, with domain-specific mapping intact.
4. A Basalt & Copper application shell and compliant shared components.
5. A redesigned, behavior-preserving Operational Map/COP and Entity Board.
6. Consistent Alert Center and Incident Workspace, then all remaining operator routes.
7. Verified font handling, typography scale, focus styles, reduced-motion behavior, and contrast checks.
8. Before/after visual evidence, a final audit of leftover hardcoded colors, and a migration summary.
9. Updated design documentation if approved implementation details differ from the specification, including a reason for the change.

The agent should not mark work complete based only on building successfully or a single screenshot.

---

## 2. Acceptance-driven workflow

Execute in the phases below. Each phase has a **goal**, **actions**, **deliverables**, and **exit criteria**. Take baseline screenshots first and compare them after each slice. If a phase materially changes interaction behavior, fix the regression before proceeding.

### Phase 0: Baseline and audit

**Goal:** Understand the current UI accurately and capture a reference state.

**Actions:**
- Confirm repository revision and existing setup commands from root `README.md`, `AGENTS.md`, `Taskfile.yml`, and `apps/operator-ui/package.json`.
- Run current UI build, typecheck, and available operator tests using the repository's documented setup. Record environment dependencies and any baseline failures.
- Capture baseline screenshots of `/map`, `/board`, `/alerts`, and an incident detail route with representative data. Use the existing mocks/scenario harness where possible; do not replace deterministic test fixtures with made-up business states.
- Audit literal colors, legacy variables, typography, component variants, map styling, and places where a single color carries multiple meanings.
- Inventory component states: hover, active, focus, disabled, pending, loading, empty, error, disconnected, stale, selected, and replay.
- Identify hardcoded status-to-tone logic across MapCanvas, badges, board, and any route-specific components.

**Useful commands (adapt to the checked-out repo):**

```bash
rg -n --glob '*.{css,ts,tsx}' '#[[:xdigit:]]{3,8}\b|rgba?\(|hsla?\(|--(color-|bg|surface|fg|st-|accent)' apps/operator-ui/src
rg -n --glob '*.{css,ts,tsx}' 'D-DIN|font-size|text-\[|--t-' apps/operator-ui/src
rg -n 'critical|high|medium|low|stale|delayed|unknown|available|assigned' apps/operator-ui/src
pnpm --filter @c4isr/operator-ui typecheck
pnpm --filter @c4isr/operator-ui build
```

Run Playwright with the required backend or mocked setup described by the repository; do not label absent infrastructure as a passing test.

**Deliverables:** audit checklist, baseline screenshots, initial test report.

**Exit criteria:** all four reference workflows can be exercised or the precise external blocker is documented; known preexisting failures are distinguished from introduced regressions.

### Phase 1: Canonical tokens and transition aliases

**Goal:** One source of truth for colors and semantic roles.

**Actions:**
- Implement the exact primitives in `DESIGN.md` §3 and §4 in `src/styles/tokens.css`.
- Import tokens from `src/index.css` in a stable CSS order.
- Adapt existing Tailwind v4 `@theme` names and `var(--...)` utilities to reference canonical tokens, without copying hex literals.
- During migration, keep needed legacy names (`--bg`, `--surface`, `--fg`, `--st-*`, `--color-*`) as aliases to canonical role variables. Do **not** assign the same deprecated variable a contradictory value in two places.
- Create a small semantic color resolver or mapping module if required by MapLibre and badges. Separate `brand`, `surface`, `component`, `domain status`, and `map` concepts.
- Use `getComputedStyle` for MapLibre tokens, or a documented theme-object equivalent. Avoid hardcoding a second palette in JS fallback constants.

**Integration example (illustrative; adapt to Tailwind v4 build):**

```css
/* apps/operator-ui/src/index.css */
@import 'tailwindcss';
@import './styles/tokens.css';

@theme inline {
  --color-bg: var(--emad-surface-canvas);
  --color-panel: var(--emad-surface-panel);
  --color-panel-raised: var(--emad-surface-raised);
  --color-ink: var(--emad-text-primary);
  --color-ink-muted: var(--emad-text-secondary);
  --color-accent: var(--emad-brand-copper);
  --color-edge: var(--emad-border-subtle);
  --color-edge-strong: var(--emad-border-control);
}

/* Deprecation bridge only, delete when callers are migrated. */
:root {
  --bg: var(--emad-surface-canvas);
  --surface: var(--emad-surface-panel);
  --surface-2: var(--emad-surface-raised);
  --fg: var(--emad-text-primary);
  --muted: var(--emad-text-secondary);
  --line: var(--emad-border-subtle);
  --st-ok: var(--emad-status-positive);
  --st-warn: var(--emad-status-caution);
  --st-crit: var(--emad-status-critical);
}
```

Do **not** blindly map the old `--st-track` to brand copper: track status is not brand emphasis. Keep the existing semantic meaning until the new state mapping has been validated. Verify Tailwind variable generation and resulting computed values in the browser.

**Deliverables:** token stylesheet, working aliases, optional tested role resolver.

**Exit criteria:** application builds; resolved computed colors match the design spec; no screen is left with a broken `var()` reference; no functionality changes.

### Phase 2: Typography, geometry, and common states

**Goal:** Establish the premium, engineered design language before moving individual routes.

**Actions:**
- Load approved IBM Plex font families where licensing/assets are verified. Ensure fallbacks prevent flash-of-invisible-text problems. Check Arabic support; do not regress bidirectional text.
- Migrate global typography to `DESIGN.md` §5, including readable labels and tabular-nums for operational values.
- Replace default 9–11px operational labels with defined 12–14px sizes. Where a dense board cannot fit, adjust row/column geometry and overflow instead of silently retaining illegible text.
- Standardize focus rings, keyboard navigation, input borders, scrollbars, reduced motion, and responsive type.
- Create reusable utilities only where repeated design semantics justify them.

**Deliverables:** global type scale and geometry improvements, verified keyboard focus states.

**Exit criteria:** no meaningful operator labels below 12px without documented accessibility exception; fonts render in Latin and Arabic; content remains usable at supported widths and zoom.

### Phase 3: Application shell and shared components

**Goal:** Apply the identity once at the component foundation.

**Actions:**
- Restyle `AppShell`, `TopBar`, `ScenarioBar`, `Page`, footer and navigation.
- Update components in `src/components/ui/` for buttons, cards, inputs, dialogs, sheets, select, tabs, menus, tooltips, toast, table, skeleton, separator, labels, and badges as needed.
- Preserve component APIs and keyboard/ARIA behavior from Base UI and current wrappers.
- Ensure Copper emphasizes selection or the principal action only, while warning, critical, health, and live/replay states remain semantically separate.
- Review responsive stacking and overflow. Don't allow top chrome to cover map/replay controls.
- Make focus/hover/selected/disabled/loading/error states coherent and accessible.

**Deliverables:** shared UI primitives and shell with consistent tokens.

**Exit criteria:** all shared component states render correctly on real screens; no regression in navigation, keyboard focus, forms, or scenario controls.

### Phase 4: Operational semantics and map

**Goal:** Establish a reliable visual grammar for operational information.

**Actions:**
1. Inventory API-defined states, current shared `badges.tsx`, `board.tsx` status mappings, and `MapCanvas.tsx` expressions.
2. Implement a centralized semantic mapping (for example, `src/lib/operationalPresentation.ts`) that maps known **domain status and severity** to role tokens, readable labels, and optional icons. Unknown values must remain legible and neutral, never silently positive.
3. Keep alert **severity** separate from alert **lifecycle**, and track **status** separate from track **freshness**.
4. Preserve the current *outer freshness ring* concept and enrich its legibility with labels/patterns. Avoid encoding several independent dimensions in the same marker hue.
5. Replace MapLibre hardcoded areas/hazard/measurement colors with appropriate documented map-domain tokens. Do not alter geospatial data, expression truth conditions, PostGIS rules, marker position, or event handlers.
6. Update basemap-adjacent styles to blend with basalt surfaces, but preserve configured style URL, attribution, raster fallback, and existing map controls.
7. Ensure selected marker ring is visible above other overlays. Test marker collisions, translucent geofences, layers, and focus of detail panels.
8. Preserve replay/live state visually and behaviorally. `Return to live` must remain prominent and actionable.
9. Avoid claiming APP-6/2525 compliance; do not invent friendly/hostile classification if unavailable.

**Scope examples:**
- `components/map/MapCanvas.tsx`
- `components/map/picture.css`
- `components/map/AssetDetailPanel.tsx`
- `components/map/TrackDetailPanel.tsx`
- `components/map/Scrubber.tsx`
- `components/shared/badges.tsx`
- `lib/freshness.ts`
- `routes/map.tsx`

**Deliverables:** operational semantic contract used by map and status components, map design with legend, interaction-preserving styles.

**Exit criteria:** a scenario can demonstrate active track, assigned asset, geofence breach, delayed/stale entity, alert and selected entity without color-based ambiguity. Map attribution, inspection, selection, and replay remain intact.

### Phase 5: Four reference workspaces

**Goal:** Prove Basalt & Copper works for both visual overview and dense operation.

**Order and specific checks:**

**A. Operational Picture (`/map`)**
- Validate map contrast, geofences, entity markers, legends, selected panels, view controls, overlays, and replay.
- Dense activity must stay legible without oversaturating the map.

**B. Entity Board (`/board`)**
- Restyle board matrix with readable typography, sticky gutter, selected cells, live/stale differentiation, and horizontal overflow.
- Keep compact/default density behavior and avoid turning data cells into large decorative cards.

**C. Alerts (`/alerts`)**
- Distinguish severity, state, age, evidence, filter controls and acknowledgment behavior.
- Test empty, loading, error, and active-alert modes. No color-only severity.

**D. Incident Detail (`/incidents/:id`, existing route path as implemented)**
- Validate all tabs, evidence/relations, mission/command forms, action permissions, source details, and changing lifecycle states.
- No hidden or altered commands. No Copper mistaken for warning or success.

**Deliverables:** four polished reference screens, before/after screenshots at 1440px desktop plus at least one compact width, documented adjustments for shared components.

**Exit criteria:** product owner can visually review one cohesive identity; every reference screen remains usable with realistic information density, keyboard navigation and state variations.

### Phase 6: Complete the remaining UI

**Goal:** Remove style fragmentation across the whole app.

**Actions:**
- Apply shared patterns to Assets, Tracks, Scenarios, Audit/Timeline, list/detail screens, dialogs and remaining forms.
- Unify notifications, detail lists, state badges, panels, table headers, and selected navigation.
- Audit all `*.css`, `*.tsx` for stray legacy hex values and semantic mismatches.
- Preserve user preferences/layout state; do not reset stores or URLs due to styling.
- Check typical views at 1280, 1440, and 1920px and a compact/narrow layout supported by existing CSS.

**Deliverables:** complete operator UI visual migration.

**Exit criteria:** no routes visibly use the previous blue/purple identity; no new inconsistent status colors; common components feel and behave the same across routes.

### Phase 7: Hardening, tests, accessibility, documentation

**Goal:** Prove the migration is safe and maintainable.

**Actions:**
- Run typecheck/build and the repository's existing Playwright tests using their documented setup.
- Confirm test coverage for map, board, alerts and incident workflows; add focused tests for token/status resolvers and for accessibility-relevant states.
- Add repeatable visual screenshots/snapshot comparisons where deterministic data/scenarios support them. Keep screenshots consistent in viewport, fonts, time, fixture and animation state.
- Audit actual contrast with a programmatic checker and manual inspection over the rendered map; test focus boundaries, error states, tooltips, overlays and banners.
- Review tab order, form labels, ARIA attributes, keyboard selection, reduced motion, zoom and color-blind differentiation.
- Review performance: ensure map styling does not cause excessive rerenders or style reloads, and avoid unnecessary animations.
- Retire compatibility aliases **only after confirming no references**. If remaining aliases are essential and safe, document them in a single deprecation table with ownership.
- Align design documentation and code, annotate deliberate exceptions.

**Deliverables:** test report, visual comparisons, token audit, accessibility audit, implementation notes, updated screenshots where appropriate.

**Exit criteria:** all acceptance criteria in `DESIGN.md` §12 pass, or documented exceptions are explicitly reviewed. No introduced backend or browser-workflow regressions.

---

## 3. Required state test matrix

At minimum, validate the following states in the relevant screens, with **text plus non-color encoding**:

| Area | Test cases |
| --- | --- |
| Assets | available, assigned, maintenance, unavailable, offline |
| Tracks | active, lost, closed, selected, unselected |
| Freshness | live, delayed, stale, unavailable, unknown |
| Alerts | low/medium/high/critical independently from ACTIVE/ACKNOWLEDGED/RESOLVED |
| Connection | connected, degraded, disconnected, recent vs stale timestamp |
| Incident | open/investigating/responding/resolved/closed, when supported by API |
| Commands | created, queued, sent, acknowledged, completed, failed, rejected, timed out, cancelled, when supported |
| Scenarios | running, paused, stopped, replay vs live |
| UI primitives | default, hover, focus-visible, selected, disabled, loading, empty, error |
| GIS | one marker, dense overlaps, multiple geofences, area observations, measurement, selected marker, basemap fallback |

**Important:** These are target *coverage categories*, not instructions to fabricate data or introduce missing states. If a state isn't supported by the checked-out implementation, mark it `not applicable` and explain why.

---

## 4. Automation and verification commands

Use repository-specific commands as authoritative. Current `apps/operator-ui/package.json` exposes:

```bash
pnpm --filter @c4isr/operator-ui typecheck
pnpm --filter @c4isr/operator-ui build
pnpm --filter @c4isr/operator-ui test:e2e
```

The E2E suite may depend on external/mocked backend setup and test fixtures; follow `playwright.config.ts` and root task documentation. Do not claim test success if required services are not available. The implementation must not relax or disable tests to make the visual migration pass.

A useful final inventory:

```bash
# Look for old hardcoded theme colors and legacy styling.
rg -n --glob '*.{css,ts,tsx}' '#161a26|#1f2533|#273c6b|#4a6fb5|#913214|#b83a1f' apps/operator-ui/src

# Find any remaining raw colors and approve only truly domain-specific uses.
rg -n --glob '*.{css,ts,tsx}' '#[[:xdigit:]]{3,8}\b|rgba?\(' apps/operator-ui/src

# Check legacy variable consumers before deleting aliases.
rg -n --glob '*.{css,ts,tsx}' 'var\(--(bg|surface|fg|st-|accent|line)' apps/operator-ui/src
```

A raw color is not automatically a defect: transparent and data-driven map expressions may need carefully documented values. But every remaining raw color should have a reason.

---

## 5. Visual review requirements

Capture and review:

- `/map` normal live, selected track, geofence alert, stale/delayed marker, and replay state.
- `/board` default density, compact density, long identifiers, selected column, unavailable data.
- `/alerts` mixed severities and lifecycle states, filter interactions, acknowledgement, and no-data/error state.
- Incident detail tabs with actions, commands, evidence, validation and state changes.
- Representative form/dialog, notification, menu, tooltip and loading/empty state.
- Desktop at 1440px minimum and narrower supported layout; extra 1280/1920px captures where layout changes.

Compare against:
- **Semantic correctness:** does the same status mean the same thing across screens?
- **Accessibility:** text, focus, inputs and symbols remain legible.
- **Hierarchy:** primary action and current state are immediately apparent.
- **Brand restraint:** basalt dominates, copper is a deliberate accent, cyan is not confused with alert state.
- **Professional quality:** no unrelated neon, muddy overlays, cramped labels or excessively ornamental styling.

Visual approval is required **before** mass-propagating the chosen treatment to non-reference routes. This is a review checkpoint, not a request for the coding agent to invent alternative identities.

---

## 6. Suggested implementation PR/commit slices

1. `design: establish EMAD tokens and type foundations`
2. `design: migrate shell and shared UI components`
3. `design: formalize operational state presentation and map overlays`
4. `design: refine board, alert center and incident workspace`
5. `design: complete remaining routes and responsive behavior`
6. `test: harden visual, semantic and accessibility regressions`

If working in one coding-agent session, retain these as review checkpoints rather than producing an unreviewable monolithic diff.

---

## 7. Definition of done

The agent must report with concrete evidence:

- [ ] Visual tokens centralized and documented; Tailwind and CSS agree.
- [ ] Legacy token mappings either removed or explicitly listed as transitional aliases.
- [ ] Typography uses approved families or a documented, verified fallback; Arabic and numeric text remain readable.
- [ ] Copper is never used as a domain-state classification.
- [ ] Shared badges, board, and map use consistent domain-specific semantic mappings.
- [ ] Meaningful icons, focus states, and controls pass required contrast checks.
- [ ] Live/replay and freshness are visibly distinct.
- [ ] Map and overlays preserve attribution, existing data, and operational interactions.
- [ ] Four reference screens reviewed with before/after evidence and state coverage.
- [ ] Remaining routes consistently migrated.
- [ ] Supported responsive layouts remain usable with dense information.
- [ ] Build/typecheck and relevant Playwright tests pass or have honestly documented environmental blockers.
- [ ] Any failing preexisting tests are separated from migration regressions.
- [ ] No backend contract, auth, or lifecycle change introduced.
- [ ] No fictional/unfinished official logo introduced.
- [ ] All accepted deviations documented with rationale and affected file list.

### Final handoff report format

The implementation agent must return:

```text
EMAD BASALT & COPPER MIGRATION REPORT

1. Revision / commit(s) / working branch
2. Completed phases and affected files
3. Implemented token system and semantic mapping
4. Four reference-screen screenshots or test artifact paths
5. Build/typecheck/E2E/accessibility results (pass/fail/not run + reason)
6. Remaining raw colors, aliases and exceptions
7. Known issues or regressions
8. Decisions requiring design/product approval
```

The migration is done only when the resulting application is **recognizably EMAD**, operationally unambiguous, accessible, and functionally unchanged.
