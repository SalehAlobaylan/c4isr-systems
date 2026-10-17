# EMAD | Visual Identity & Operator Interface Design System

**Version:** 2.0  
**Date:** 2026-10-17  
**Status:** Approved design direction and implementation specification  
**Identity:** **Alloy Pearl, Balanced**  
**Product:** EMAD / عِماد  
**Application:** `apps/operator-ui`  
**Appearance policy:** **One permanent balanced-light mode. No light/dark appearance toggle.**

> **Coding-agent contract:** This document is EMAD's current design source of truth. Build the specified system, not an interpretation of the mood board. Keep every operational state and interaction correct. If a visual treatment conflicts with accessibility, geospatial correctness, domain meaning, or the real API, prefer correctness and accessibility, document the trade-off, and do not invent product behavior.
>
> **Supersession:** This file **replaces the previous Basalt & Copper `DESIGN.md`**. Any older `DESIGN_IMPLEMENTATION_PLAN.md` referencing Basalt & Copper, copper accents, or a dark-default operator UI is **outdated and must not be executed as-is**. Reconcile or regenerate that plan before using it for implementation. The existing `PRODUCT.md`, `ARCHITECTURE.md`, and `IMPLEMENTATION_PLAN.md` remain authoritative for product, domain, and architecture decisions.

---

## 1. Intent and design principles

### 1.1 EMAD's positioning

EMAD is a software-first, C4ISR-oriented operational information platform, currently implementing **C2 and Operational Awareness**. Its future scope includes richer ISR, intelligence, and external integrations. EMAD's visual character should feel appropriate for a **premium Saudi defense-technology company** without implying corporate affiliation with SAMI, SAMI-AEC, or any other organization.

Its promise is **clarity, control, evidence, and engineering confidence**. The interface is not a consumer social application, a game HUD, or a military-themed decoration layer. It is an operator tool for reading evolving, sometimes uncertain information and making accountable decisions.

**Identity adjectives:** engineered, precise, assured, modern, energetic, disciplined, composed, technically credible.

**Not:** generic enterprise blue, sterile white SaaS, old-style olive military software, neon sci-fi, glossy luxury, pseudo-tactical ornament, or a gloomy charcoal-only UI.

The name **عِماد** suggests a structural support or pillar. Build visual confidence through hierarchy, geometry, spacing, and consistency, not by forcing literal pillar graphics onto every surface.

### 1.2 The defining decision: single balanced-light identity

**Alloy Pearl, Balanced** is **not** a standard white light theme and **not** a dark theme with lighter cards. It is one intentionally composed material system:

1. **Mineral canvas:** soft, grey-green surfaces that remain comfortable in long sessions.
2. **Industrial graphite:** stable dark anchors, navigation, strong actions, prominent brand moments.
3. **Energized lime:** a limited, memorable signature, most effective against graphite.
4. **Signal teal:** cool engineering/data accent that does not compete with lime.
5. **Deep readable ink:** the principal means of communicating information.

No theme toggle, no parallel dark-token family, and no automatic `prefers-color-scheme` appearance switch. **Keep the same identity in every viewport.** Responsive layout changes are expected; arbitrary color-mode changes are not.

This is a decision about EMAD's **normal application appearance**, not permission to disregard accessibility. A specialized accessibility/high-contrast adaptation may be evaluated later as a separate requirement; do not invent it in this migration.

### 1.3 Priority when requirements conflict

**Operational meaning > accessibility and legibility > accurate hierarchy > consistency > brand expression > decoration.**

No appearance change may silently alter roles, permissions, backend workflows, operational state machines, track/asset provenance, confidence values, timestamps, or user actions.

### 1.4 What is approved and what is not

| Item | Decision |
| --- | --- |
| Color direction | **Approved:** Alloy Pearl, Balanced |
| Default appearance | **Approved:** permanent balanced-light workspace |
| Brand anchor | **Approved:** industrial graphite + energized lime |
| Supporting analytical color | **Approved:** signal teal |
| Existing Basalt & Copper | **Retired** for the new identity |
| Normal light/dark switch | **Not permitted** |
| Logo/symbol | **Not approved. Do not create or replace.** |
| Font system | Proposed below, contingent on actual font files/licenses and glyph testing |
| Tactical symbology standards | No APP-6/MIL-STD-2525 compliance claim without separate standards work |
| Third-party brand inspiration | Inspiration only, **do not copy SAMI/AEC logos, lockups, layout elements, or trademarked brand assets** |

---

## 2. Canonical color system

### 2.1 Permanent foundation tokens

Use these exact initial values. Primitive colors are source values; components must consume **role tokens** rather than scattering hex literals.

| Primitive | Hex | Role |
| --- | --- | --- |
| `--emad-mineral-canvas` | `#E3E9E2` | Default application canvas, non-white and low-glare |
| `--emad-ceramic-surface` | `#F2F4ED` | Main cards, tables, inspector panels, dialogs |
| `--emad-mist-surface` | `#D4DED8` | Recessed groups, map land base, secondary bands |
| `--emad-industrial-graphite` | `#202B2D` | Structural brand anchor, navigation, dark controls |
| `--emad-graphite-ink` | `#1D3030` | Primary readable text and data |
| `--emad-energized-lime` | `#B7D36B` | Reserved brand signature on dark surfaces |
| `--emad-signal-teal` | `#3C7780` | Data/technical accent, strokes, selected chart emphasis |
| `--emad-structural-border` | `#B4C4B9` | Subtle, decorative separators |

### 2.2 Functional additions needed to implement the palette

These are intentional companions to the eight foundation colors, not a competing color identity.

| Role token | Hex | Correct use |
| --- | --- | --- |
| `--emad-text-primary` | `#1D3030` | Primary text on light surfaces |
| `--emad-text-secondary` | `#4B6159` | Descriptions and supporting content, including mist surface |
| `--emad-text-on-dark` | `#E9F2EA` | Text on graphite navigation/actions |
| `--emad-text-on-lime` | `#202B2D` | Readable labels on lime fills |
| `--emad-link` | `#2C626B` | Small actionable text on all core light surfaces |
| `--emad-lime-on-light` | `#355A3F` | Legible green-toned *text* if a brand-related label is necessary |
| `--emad-border-control` | `#607F77` | Essential control borders, not decorative dividers |
| `--emad-surface-hover` | `#E8EEE7` | Hover over the default light card surface |
| `--emad-surface-selected` | `#C7D9C8` | Selected background, with additional selection cue |
| `--emad-surface-input` | `#F7F8F2` | Inputs and editable fields |
| `--emad-graphite-raised` | `#304044` | Secondary items on the dark navigation rail |
| `--emad-graphite-muted` | `#B7CAC4` | Readable secondary navigation text |
| `--emad-focus-light` | `#2C626B` | Focus indicator on light surfaces |
| `--emad-focus-dark` | `#B7D36B` | Focus indicator on graphite surfaces |

**Important:** `#B7D36B` is not readable as small text against mineral/ceramic/mist. Use `#355A3F` for green-flavored text on light surfaces, or graphite text on a filled lime control. `#3C7780` is useful for graphics, but use darker `#2C626B` for regular small links and labels.

### 2.3 Operational semantic tokens

Operational states are **not brand colors**. The following colors have been chosen for usable text contrast against the three core light surfaces. These are starting semantics for the current product, not tactical affiliation colors.

| Token | Hex | Reserved meaning |
| --- | --- | --- |
| `--emad-status-positive` | `#276645` | Successful/healthy/available **when the specific field means this** |
| `--emad-status-info` | `#2A5B66` | Informational, active/acknowledged where semantically appropriate |
| `--emad-status-caution` | `#7E5319` | Medium concern, degraded, delayed |
| `--emad-status-high` | `#91451F` | High-severity attention |
| `--emad-status-critical` | `#9A372F` | Critical severity, failed actions or explicit errors |
| `--emad-status-neutral` | `#4B6159` | Closed, not applicable, neutral/unknown where supported by text |
| `--emad-status-review` | `#2C626B` | A provisional analytical assessment that needs review, accompanied by wording |

Do not infer *confidence*, *affiliation*, *threat*, *health*, *connection*, *freshness*, or *selection* from one another. All are separate dimensions. Positive health is not evidence validity. An unknown classification is not automatically hostile. An offline asset is not necessarily a critical incident.

### 2.4 Recommended proportion and distribution

For **overall brand-facing compositions** use as a guide, never a forced pixel rule:

- **55%** mineral canvas / quiet base.
- **22%** ceramic panels and lighter surfaces.
- **16%** graphite structural anchors.
- **5%** signal teal for technical emphasis.
- **2%** energized lime as a signature.

Operational status colors are exempt: alert visibility and information clarity always win over palette quotas.

Examples of correct lime usage: logo-independent brand underline, selected navigation detail on graphite, small structured indicator, carefully constrained CTA fill against graphite, and corporate identity accent.

Examples of incorrect lime usage: all cards, large page backgrounds, warning/critical alerts, all map pins, colored rows full of lime, normal body text on mineral, or every primary button.

### 2.5 Accessibility pairings

Ratios below are calculated using WCAG relative luminance for the specified **opaque foreground/background pair**, rounded to two decimals. Alpha/translucency, hover states, gradients, map imagery and real overlays must be tested separately.

| Pair | Ratio (approx.) | Use |
| --- | ---: | --- |
| Graphite Ink `#1D3030` on Mineral Canvas `#E3E9E2` | **11.21:1** | Primary text |
| Graphite Ink on Ceramic `#F2F4ED` | **12.48:1** | Primary text |
| Graphite Ink on Mist `#D4DED8` | **10.04:1** | Primary text |
| Secondary `#4B6159` on Mist | **4.83:1** | Secondary text |
| Link `#2C626B` on Mist | **4.97:1** | Link text |
| Lime `#B7D36B` on Graphite `#202B2D` | **8.68:1** | Lime text/accent on graphite |
| Graphite on Lime | **8.68:1** | Text in lime-filled controls |
| Lime on Ceramic | **1.51:1** | **Fails for text, avoid** |
| Signal Teal `#3C7780` on Canvas | **4.11:1** | Not suitable for normal body text here |
| Positive `#276645` on Mist | **4.96:1** | Positive text + icon |
| Caution `#7E5319` on Mist | **4.86:1** | Warning text + icon |
| High `#91451F` on Mist | **4.96:1** | High attention text + icon |
| Critical `#9A372F` on Mist | **5.15:1** | Critical text + icon |
| Control border `#607F77` on Mist | **3.18:1** | Necessary control edges, verify adjacent colors |

Conformance goals: **WCAG 2.2 AA**, including 4.5:1 normal text; 3:1 large text; 3:1 meaningful non-text contrast against **relevant adjacent colors**; visible keyboard focus; no color-only communication. The ratio of a border to its *interior and exterior* matters if both are visually adjacent. Do not assume a token-level pass guarantees component-level compliance.

Reduced glare is a visual design goal, **not** a measured medical or ergonomic guarantee. Verify readability on ordinary laptop displays, bright monitors, and different map styles.

---

## 3. Implementation token contract

### 3.1 Single authoritative source

Create or centralize `apps/operator-ui/src/styles/tokens.css` and import it **once** from `apps/operator-ui/src/index.css`. The token file should contain only canonical primitive and role mappings, plus a limited set of typography/layout tokens. Tailwind v4 aliases are adapters to these role tokens, not a separate palette.

**Reference CSS (adapt to existing organization without creating duplicate declarations):**

```css
:root {
  /* Foundation */
  --emad-mineral-canvas: #e3e9e2;
  --emad-ceramic-surface: #f2f4ed;
  --emad-mist-surface: #d4ded8;
  --emad-industrial-graphite: #202b2d;
  --emad-graphite-ink: #1d3030;
  --emad-energized-lime: #b7d36b;
  --emad-signal-teal: #3c7780;
  --emad-structural-border: #b4c4b9;

  /* Semantic interface roles */
  --emad-surface-canvas: var(--emad-mineral-canvas);
  --emad-surface-panel: var(--emad-ceramic-surface);
  --emad-surface-inset: var(--emad-mist-surface);
  --emad-surface-hover: #e8eee7;
  --emad-surface-selected: #c7d9c8;
  --emad-surface-input: #f7f8f2;
  --emad-surface-nav: var(--emad-industrial-graphite);
  --emad-surface-nav-raised: #304044;

  --emad-text-primary: var(--emad-graphite-ink);
  --emad-text-secondary: #4b6159;
  --emad-text-on-dark: #e9f2ea;
  --emad-text-on-lime: var(--emad-industrial-graphite);
  --emad-text-nav-muted: #b7cac4;
  --emad-link: #2c626b;
  --emad-lime-on-light: #355a3f;

  --emad-border-subtle: var(--emad-structural-border);
  --emad-border-control: #607f77;
  --emad-focus-light: #2c626b;
  --emad-focus-dark: var(--emad-energized-lime);

  /* Operational statuses, never synonymous with brand lime */
  --emad-status-positive: #276645;
  --emad-status-info: #2a5b66;
  --emad-status-caution: #7e5319;
  --emad-status-high: #91451f;
  --emad-status-critical: #9a372f;
  --emad-status-neutral: #4b6159;
  --emad-status-review: #2c626b;
}

/* Tailwind 4 role aliases, keep only one actual source of hex values */
@theme inline {
  --color-bg: var(--emad-surface-canvas);
  --color-panel: var(--emad-surface-panel);
  --color-panel-raised: var(--emad-surface-inset);
  --color-edge: var(--emad-border-subtle);
  --color-edge-strong: var(--emad-border-control);
  --color-ink: var(--emad-text-primary);
  --color-ink-muted: var(--emad-text-secondary);
  --color-accent: var(--emad-link);
  --color-danger: var(--emad-status-critical);
  --color-warn: var(--emad-status-caution);
  --color-ok: var(--emad-status-positive);
}
```

The project's Tailwind/CSS import ordering must be confirmed so `@theme inline` is processed correctly. Do not paste a second `@theme` block that leaves competing values in `index.css`.

### 3.2 Legacy compatibility, carefully scoped

The current UI, as reviewed, uses both Tailwind `--color-*` and older CSS vars (`--bg`, `--surface`, `--fg`, `--st-track`, `--st-warn`, etc.), along with direct colors in MapLibre expressions and some button classes. Migrate gradually, but never keep two independent color authorities.

- Temporary aliases can point old CSS variables to the new canonical role tokens.
- Delete hardcoded Basalt/Copper or old navy/orange values as their consumers migrate.
- Remove the old semantic mistake `--accent: var(--st-track)`. UI selection must not inherit track meaning.
- Avoid replacing every color with `--emad-energized-lime`; some colors are operational, others are decorative.
- An agent may reorganize styling files, but must keep React state, API contracts, routing, interactions, map logic, and persistent UI preferences working.
- Do not modify backend types, database, WebSocket protocols, or state-machine values as part of rebranding.

### 3.3 Keep token names semantic

Preferred: `--emad-status-caution`, `--emad-surface-panel`, `--emad-text-on-dark`.  
Avoid: `--yellow3`, `--green-button`, `--enemy-red`, or `--lime-threat`.

Never use `brand-lime` as a synonym for `available`, `healthy`, `friendly`, `selected-track`, or `acknowledged`. It must retain its **brand** meaning.

---

## 4. Typography

### 4.1 Family roles

| Role | Preferred family | Fallback |
| --- | --- | --- |
| UI English | IBM Plex Sans | `system-ui, sans-serif` |
| UI Arabic | IBM Plex Sans Arabic | Arabic-capable system sans |
| Coordinates / telemetry / identifiers / timestamps | IBM Plex Mono | `ui-monospace, monospace` |
| Corporate wordmark | Custom, **deferred** | Clean text-only `EMAD` / `عِماد` |

Do not fabricate a new EMAD logo by typesetting a special symbol. Verify actual available font weights, language coverage and licenses before bundling fonts. Use local assets if available and permitted; graceful system fallback must remain readable when web fonts fail. Do not distribute restricted font binaries.

### 4.2 Type sizes and hierarchy

| Token | Size / line-height | Typical usage |
| --- | --- | --- |
| Display | `30px / 36px`, 600 | Rare, corporate headings or major dashboard title |
| H1 | `24px / 30px`, 600 | Primary workspace heading |
| H2 | `20px / 26px`, 600 | Section heading |
| H3 | `16px / 22px`, 600 | Card and panel titles |
| Body | `14px / 21px`, 400 | Lists, labels, forms, descriptions |
| Compact | `13px / 18px`, 400/500 | Dense entity tables and metadata |
| Small | `12px / 17px`, 400/500 | Supporting noncritical information |
| Numeric KPI | `24–30px`, tabular numerals | Metrics and measurements |

Avoid 9–11px information-critical labels, which appear in legacy prototype styles. A tiny decorative overline can exist only if it is nonessential and tested at realistic screen densities. Operational time, status and confidence must remain readable without zooming.

- Use `font-variant-numeric: tabular-nums` for aligned changing values.
- Use mono for IDs, coordinates, machine timestamps, compact readings and technical values, not paragraphs.
- Limit uppercase tracking to short English overlines, typically `0.04em` or less. Do not letter-space Arabic.
- Support mixed `EMAD / عِماد`, LTR coordinates, bidirectional strings and copy/paste accurately.
- Labels should use clear words (`DELAYED`, `REPLAY`, `ACKNOWLEDGED`) rather than decorative abbreviations.

---

## 5. Layout, surfaces, and geometry

### 5.1 Spacing

Use a 4px baseline: `4, 8, 12, 16, 20, 24, 32, 40, 48`. Dense operational views may use 4/8px internally; normal panels should mostly use 12–20px. Consistent spacing is more important than ornamental separators.

### 5.2 Shape and borders

- Corner radius: 4px by default; 6px for cards/dialogs; 2px for compact data controls. Avoid large, pill-like cards everywhere.
- Keep large page areas calm and open; do not turn every data group into a nested card.
- Use `#B4C4B9` for visual dividers and `#607F77` when the border is necessary to recognize a control.
- Use shadows rarely; hierarchy should come primarily from surface contrast, whitespace, typography, and grid alignment.
- Use lime accent rules sparingly, for example a 2–3px line on a selected dark navigation item. Avoid decorative strips on every card.

### 5.3 Surface ownership

| Surface | Default |
| --- | --- |
| App workspace background | Mineral Canvas `#E3E9E2` |
| Cards / main panels | Ceramic `#F2F4ED` |
| Secondary region / grouped content | Mist `#D4DED8` |
| Dark brand navigation | Industrial Graphite `#202B2D` |
| Dark navigation item hover | Graphite Raised `#304044` |
| Light panel hover | `#E8EEE7` |
| Selected light row / tab | `#C7D9C8` plus visible structural cue |
| Inputs | `#F7F8F2`, control border `#607F77` |
| Modals and dialogs | Ceramic, with solid backdrop for spatial separation |

One dark graphite structural rail/header is an intentional feature; **do not make the whole application graphite**. Conversely, do not convert every graphite region to pale grey, or the identity will lose its industrial character.

### 5.4 Responsive behavior

- Preserve map-as-primary reading where relevant; collapse side inspectors rather than shrinking map markers or text below usability.
- Horizontal board scrolling is acceptable when accompanied by stable headings and clear sticky context. Avoid hiding required state columns.
- No horizontal overflow from the top navigation or dialog controls at supported widths.
- Touch controls target at least WCAG 2.2 AA 24×24 CSS px (with proper spacing where applicable), and prefer ~40×40px for primary touch interactions.
- Breakpoints should respond to actual component constraints, not arbitrary device marketing categories.

---

## 6. Component specifications

### 6.1 Navigation and application shell

- Dark graphite brand/nav region, mineral main workspace.
- Current route indicated by **label emphasis + visible selected background/line**, optionally lime detail on graphite. Do not rely on hue alone.
- Navigation labels should be legible and at least 12–13px where layout permits.
- The real-time connection, scenario mode and operator identity retain their existing semantics.
- Do not mix **live** and **replayed** operational pictures visually. `REPLAY` must remain persistently obvious even when the interface is otherwise quiet.
- Do not invent a new logo or pretend the platform is officially affiliated with SAMI/AEC.

### 6.2 Buttons

| Variant | Specification |
| --- | --- |
| Primary | Graphite fill, `#E9F2EA` text; clear hover/pressed/focus states |
| Secondary | Ceramic fill, dark ink, control border |
| Outline | Clear control border, transparent/ceramic fill, ink label |
| Ghost | No default border, but visible hover and focus treatment |
| Signature (exceptional) | Lime fill with graphite text for a rare non-destructive branded CTA, **not** default for every workflow |
| Destructive | Critical color with explicit label and icon, never lime |
| Disabled | Reduced prominence but still decipherable; native disabled semantics |

Keyboard focus: a clearly visible focus outline/ring that meets contrast against the **actual adjacent surfaces**. Use the light focus token in pale areas, lime focus token on graphite, and a separated ring/outline if background contrast is insufficient. Never remove focus without replacement.

### 6.3 Inputs and forms

- Input fill: `#F7F8F2`; control border: `#607F77`; normal content text: `#1D3030`.
- Keep labels visible and external to input; placeholders are hints, not replacements.
- Invalid field: critical outline + icon + error text; do not signal through color only.
- Readonly and disabled must look different, and must remain programmatically distinguished.
- Labels, units, coordinate precision and required indicators remain unchanged in meaning.

### 6.4 Cards, panels and dialogs

- Ceramic panel on mineral canvas, generally 1px subtle border.
- Use hierarchy within the panel: title, supporting metadata, body, actions.
- Avoid heavy elevated shadow stacks and blurred glass overlays in operational workspaces.
- Modal focus management, keyboard closure and ARIA behavior must survive restyling.
- Inspector panels may use Mist bands to separate evidence from assessments, but should not imply one is more truthful merely because it is visually prominent.

### 6.5 Tables and Entity Board

- Sticky identifiers/column headings stay legible over scrolling content.
- Numeric values use tabular numerals and explicit units.
- Hover background is a convenience, **not** selection state.
- Selected row uses selected surface + 2–3px structural cue or clear outline.
- Keep state indicators short, explicit and labeled. Do not replace meaningful words with colored dots.
- Compact mode may reduce padding, never compress important text into unreadable sizes.
- Handle dense data, empty data, stale data, long names and truncated identifiers deliberately.

### 6.6 Badges, toasts and alert panels

- Badges pair status **label + icon/pattern where useful + appropriate status color**.
- Each severity must remain different in text and shape treatment, not just green/yellow/red.
- Do not use lime as an alert green; it belongs to corporate expression.
- Toast colors follow message type but must not obscure navigation/map interaction or disappear before users can understand consequential events.
- Alerts retain severity, source, timestamp, evidence, acknowledgement and resolution cues.

### 6.7 Empty, loading and failure states

Empty, loading, error, offline, stale and unauthorized are distinct states. Use precise terminology, actionable recovery options and accessible status descriptions. Keep skeletons subtle; do not create lime shimmering/loading bars. Avoid suggesting data is live while API or WebSocket state is unknown.

---

## 7. Operational semantics, independent visual channels

**This section is more important than brand consistency during an incident.** Define mapping centrally and base it on **real domain values**, not labels invented to fill a palette.

### 7.1 Mandatory separation of dimensions

| Dimension | What it answers | Preferred visual channel |
| --- | --- | --- |
| Entity kind | Asset, observed track, source, area | Symbol/shape + label |
| Affiliation | Friendly, hostile, neutral, unknown when actually modeled | Standards-informed symbol/shape/colors only after explicit domain work |
| Entity lifecycle | Active, lost, closed; assigned, available, etc. | Text + relevant status cue |
| Connection health | Connected, degraded, disconnected | Labeled connection icon/status |
| Freshness | Live, delayed, stale, unavailable, unknown | Ring/hatch + timestamp + word |
| Alert severity | Low, medium, high, critical | Distinct severity treatment, icon + word |
| Assessment confidence | Degree of uncertainty | Numeric value / interval + provenance, not traffic-light alone |
| Selection | What operator is inspecting | Outline, emphasis, focus and selected background |
| Scenario time | Live vs historical replay | Persistent, prominent mode label and time controls |

Do not borrow a severity color to encode affiliation, or freshness color to encode entity kind. `UNKNOWN` information is not the same as `LOW` severity and not the same as `STALE` data.

### 7.2 Suggested current-state mapping

Treat this as **UI mapping only**, not permission to add states that do not exist in backend contracts.

| Domain | Real value | Presentation rule |
| --- | --- | --- |
| Track state | `active` | Signal/teal stroke + `ACTIVE` |
| Track state | `lost` | Caution status + `LOST` |
| Track state | `closed` | Neutral muted status + `CLOSED` |
| Asset availability | `available` | Positive + `AVAILABLE` |
| Asset availability | `assigned` | Informational + `ASSIGNED` |
| Asset availability | `maintenance` | Caution + `MAINTENANCE` |
| Asset availability | `unavailable` | Neutral or caution + exact label, according to actual reason |
| Connection | `disconnected` | Explicit disconnect icon + `DISCONNECTED`, escalate independently only when required |
| Alerts | `critical` | Critical + `CRITICAL` |
| Alerts | `high` | High + `HIGH` |
| Alerts | `medium` | Caution + `MEDIUM` |
| Alerts | `low` | Informational + `LOW` |
| Command | `FAILED`/`REJECTED`/`TIMED_OUT` | Critical or high attention as appropriate, exact outcome always visible |
| Incident | `OPEN`/`INVESTIGATING`/`RESPONDING`/`CLOSED` | Exact workflow state + distinct icon/label, not all one urgency color |
| Scenario | Live/replay/paused | Persistent explicit mode state, clock and control context |

Important: A critical command failure isn't equivalent to a high-severity geographic incident. They can share a hue **without sharing a meaning** only if labels and context distinguish them. Preserve current urgency/priority source fields.

### 7.3 Freshness

Retain the existing design concept in `MapCanvas.tsx` where freshness uses an **outer ring rather than recoloring the underlying entity marker**.

- `live`: no caution ring; explicit current timestamp / live label elsewhere.
- `delayed`: caution ring and `DELAYED` state when selected.
- `stale`: high/critical attention ring based on freshness rules and explicit `STALE` label.
- `unavailable`: neutral dashed ring/strike and explicit `UNAVAILABLE` label.
- `unknown`: neutral hatch/question mark and explicit `UNKNOWN`.

Any threshold for these states is **domain logic**, not an aesthetic decision. Do not alter thresholds merely to match new colors. When using translucency to de-emphasize stale entities, retain adequate shape/readability and inspectability. Operators must never confuse a historical position with live position.

### 7.4 Confidence and provenance

- Present the percentage or method where supported; distinguish **classification hypothesis** from raw observation.
- A higher bar/value need not become greener. Prefer neutral ink/teal scales, labels and provenance references.
- Make contradictory evidence inspectable, not visually suppressed.
- Never display an algorithmic classification as fact simply because it has an attractive badge.

### 7.5 Tactical affiliation and standards

Do **not** describe the custom EMAD palette as NATO APP-6 or MIL-STD-2525 compliant. If standardized tactical symbols become a requirement, implement them from the relevant licensed specification and validate independently. Preserve affiliation symbol conventions rather than mapping everything to lime/teal.

---

## 8. Operational map and geospatial layers

### 8.1 Map surface system

EMAD's branded map should feel related to Alloy Pearl but preserve real geographical legibility.

| Map element | Candidate color | Notes |
| --- | --- | --- |
| Land | `#D4DED8` | Mineral-mist land base |
| Water | `#C7DCE0` | Muted blue-grey water, not electric cyan |
| Major roads | `#F2F4ED` | Stronger path shape without glaring pure white |
| Minor roads | `#E7EDE6` | Reduced visual priority |
| Administrative boundary | `#A6BDB0` | Subtle, can be dashed depending on map style |
| Building mass | `#CBD8CE` | Do not obscure annotations |
| Geographic label | `#334A47` | Clear contrast against base |
| Map controls | Ceramic `#F2F4ED` | Control border + visible focus |
| Track path | Signal Teal `#3C7780` | Direction / history, with line style and label |
| Geofence / hazard | Severity-dependent, low-opacity fill + clear outline | Must not erase underlying geography |

The list is a **candidate custom vector style**, not instructions to artificially recolor a third-party raster basemap. If the app uses the existing raster fallback or a user-supplied `MAP_STYLE_URL`, keep geographic data accurate and style only layers under our control.

### 8.2 Shapes, strokes and layering

- Asset, track and source must be distinguishable by *shape/outline/label* and not only color.
- Selection ring is high-contrast and distinct from freshness ring. Never let selected styling cancel stale signaling.
- Keep observation footprints/geofences below assets/tracks where appropriate; selected annotations remain legible above fills.
- Use opacity conservatively. Avoid low-opacity operational symbols over detailed satellite imagery.
- History and trajectory lines must be visually different from roads/geofences.
- MapLibre source/layer IDs, rendering order, hit testing, click selection, measurement mode and geospatial output must remain intact.
- Do not interfere with map attribution, copyright, or provider requirements.

### 8.3 Map accessibility and operational validation

Evaluate at normal and high zoom, with overlapping entities, several geofences, routes, error states, and all supported basemap providers. Also evaluate under common color-vision deficiencies, browser zoom, and real display brightness.

Never confuse `REPLAY` with `LIVE`. The replay scrubber, range, return-to-live action and replay timestamps must remain visible and usable. Always preserve the same current/historical distinction in map overlay panels and inspector metadata.

---

## 9. Data visualization and analytics

- Analytics uses restrained palettes, with signal teal for primary series and graphite-based neutrals for comparison series.
- Lime may highlight **one important brand-related** datum in a marketing-style graphic, but not arbitrarily represent positive performance.
- Critical operational charts use operational colors and labeled legends, not brand lime.
- Do not use hue as the sole method of differentiating multiple lines: add dash, symbol, direct labels or a clear legend.
- Numeric formatting and units are mandatory; do not remove uncertainty and confidence notes.
- For operator charts, chart background should match Ceramic and axes should use readable secondary text.
- Avoid gratuitous 3D, glow, gradients, animated metrics, and charts without decisions they support.

---

## 10. Iconography, motion, and micro-interactions

- Use a coherent family of simple, geometric, mostly 1.5–2px stroke icons.
- Prefer existing project icon components and preserve accessible names.
- Icon color denotes status *only when status semantics explicitly require it*.
- Loading/hover transitions: normally `120–180ms`, ease-out, restrained translation or opacity.
- No pulsing neon effects, sweeping radar decoration, fake signal scans, or attention-grabbing perpetual animation.
- Honor `prefers-reduced-motion: reduce`; preserve instant, understandable feedback without animation.
- Live markers may update location, but animation must not falsely suggest motion from stale telemetry.

---

## 11. Corporate, logo and bilingual identity

### 11.1 Corporate tone

The brand should communicate confidence through restraint: graphite backgrounds, limestone/mineral whitespace, disciplined engineering diagrams and infrequent lime accents. **Not luxury metallics** and not an intentionally rugged or retro military UI.

Suggested exploration-only statements, **not official registered slogans**:

- **Engineering operational confidence.**
- **From evidence to decision.**

Do not make claims of military certifications, external-system deployments, or AI capabilities not implemented.

### 11.2 Logo and wordmark

The EMAD logo is **deferred**. Keep a neutral text-only `EMAD` or existing approved brand treatment until specifically directed otherwise. Do not invent a shield, hexagon, pillar, Arabic letterform, SAMI-like symbol, or corporate seal during the interface migration.

### 11.3 Arabic and English

- Primary English name: `EMAD`.
- Arabic: `عِماد` (respect Arabic typesetting).
- Ensure Arabic labels shape correctly and RTL layouts preserve expected reading order.
- Treat coordinates, timestamps, IDs and code as LTR content as appropriate.
- Do not implement arbitrary uppercase transformations or Latin tracking on Arabic.
- Do not silently change product-facing wording or role names during a pure visual migration.

---

## 12. Accessibility and human-factors quality bar

1. WCAG 2.2 AA contrast goals for text and relevant non-text UI.
2. Visible keyboard focus, no focus traps, tab navigation and screen-reader labels preserved.
3. Color-independent status interpretation: label + icon/shape/pattern as appropriate.
4. Important timestamps, coordinates and confidence remain readable at common operator densities.
5. Tested 200% browser zoom where feasible; avoid text clipping and inaccessible horizontal controls.
6. Hover, selected, focus, disabled, error, stale and alert states remain distinguishable.
7. Motion respects reduced-motion preferences.
8. Tooltips don't carry essential information that cannot be reached by keyboard.
9. Operational map controls and attribution remain functional.
10. Validate the actual composite UI, not only primitive CSS colors.

A single permanent balanced-light identity must work in ordinary office light and dimmer command-room use. Do not claim it works optimally in all light conditions without real-world testing.

---

## 13. Reference screens and design verification

Apply and visually validate the identity against **real application states** on at least these four screens:

### 13.1 Operational Picture (`/map`)

- Mineral workspace, graphite structural navigation.
- Branded map where possible; source map fallback remains correct.
- Track/asset/source visual identity, active selection, freshness ring, timeline scrubber.
- Alert/geofence fills readable over map layers.
- Inspector panel maintains provenance and confidence context.

### 13.2 Entity Board (`/board`)

- Dense rows and tables remain legible, even in compact mode.
- Clear status, availability, source and recency columns.
- Sticky context, overflow, selected row and hover are distinct.
- Values align and retain units.

### 13.3 Alerts (`/alerts`)

- Four severities distinct in label/icon/color.
- Active/acknowledged/resolved visually distinguishable.
- Acknowledgement and filtering workflows work unchanged.
- Badges readable on ceramic and mist surfaces.

### 13.4 Incident detail (`/incidents/...`)

- Incident lifecycle, evidence links, related entities, assessments, mission assignments and command actions remain clear.
- Destructive/high-consequence actions do not become visually similar to decorative lime actions.
- Forms and tabs use coherent focus/validation.
- No change to auth or command-permission enforcement.

Also spot-check `/assets`, `/tracks`, `/timeline`, and `/scenarios`, plus open menus, dialogs, offline/error states and responsive layouts.

---

## 14. Codebase-specific integration notes

Repository baseline observed during design preparation:

- React + TypeScript + Vite + Tailwind v4 + TanStack Router/Query + Zustand + Base UI / local UI primitives + MapLibre.
- `apps/operator-ui/src/index.css` owns Tailwind theme aliases *and* a second legacy `:root` palette.
- `apps/operator-ui/src/components/map/MapCanvas.tsx` resolves colors in JS but also has some hardcoded colors for map and area layers.
- `apps/operator-ui/src/components/shared/badges.tsx` maps state names to shared tones.
- `apps/operator-ui/src/routes/board.tsx` has additional track/asset tone mappings.
- `apps/operator-ui/src/components/board/board.css` and `components/map/picture.css` depend on legacy palette vars.
- `apps/operator-ui/src/components/ui/` holds shared buttons, cards, forms, dialogs, tables and other primitives.
- Existing Playwright operator workflows cover critical paths and must continue passing.

### Migration implications

1. Take a visual baseline of the existing screens **before** changing shared styles.
2. Introduce canonical tokens, then temporary compatibility aliases.
3. Build central semantic state-to-presentation mappings, preserving distinct domain dimensions.
4. Migrate one representative operator workflow, then propagate through shared components and remaining screens.
5. Update MapLibre palette and controlled style layers deliberately. **Do not touch geospatial computation or operational state interpretation.**
6. Remove retired palette values and duplicated aliases after proving no consumers remain.
7. Update the old design implementation plan before giving it to an agent as executable instructions.

This file defines the **target identity**, not a license to perform arbitrary code refactoring. Keep changes within the visual/UI scope unless a local code change is necessary to preserve existing behavior.

---

## 15. Definition of done

The revised Alloy Pearl identity is ready for approval when all of the following are true:

- [ ] There is exactly **one normal app appearance**, with no light/dark toggle.
- [ ] The eight foundation colors are represented through a canonical token source.
- [ ] Lime is clearly a restrained brand signature, not a status color.
- [ ] Graphite gives the UI industrial identity without dominating the main workspace.
- [ ] Light surfaces remain low-glare, differentiated and readable.
- [ ] UI state, entity kind, freshness, selection, confidence, affiliation and severity are not conflated.
- [ ] Component styles use semantic roles rather than scattered hardcoded hex values.
- [ ] Core UI controls, text and icons meet required contrast in actual states.
- [ ] Map overlays and legend remain understandable without relying only on color.
- [ ] Live versus replay is impossible to miss in the operational picture.
- [ ] English/Arabic names and numerical/coordinate rendering remain correct.
- [ ] Desktop/responsive and dense/empty/loading/error screens are reviewed.
- [ ] Existing type checks, build, operational Playwright flows and regression tests still pass.
- [ ] No logo, third-party affiliation, certification or feature claim has been fabricated.
- [ ] Old Basalt & Copper instructions have been retired or updated to this version.

**Design review questions:** Does EMAD look like a modern, confident engineering product? Can an operator identify critical facts immediately? Are the bright areas genuinely comfortable and the graphite anchors proportionate? Does the visual style remain recognizable even when lime is absent? Are map states still trustworthy? If any answer is no, revise the implementation within this token/semantic framework rather than inventing another theme.

---

## 16. Rules for coding agents and future contributors

**Must:**

- Read `PRODUCT.md`, `ARCHITECTURE.md` and relevant implementation code before changing workflow visuals.
- Treat **Alloy Pearl, Balanced** as current design authority.
- Use **one balanced-light mode** and a centralized token system.
- Preserve existing API and domain behavior, permissions, map logic, auditability, scenario replay, and important terminology.
- Keep operational and brand palettes independent.
- Verify actual contrast and UI behavior with screenshots/tests, not just CSS inspection.
- Document deviations and any deferred issues.

**Must not:**

- Restore Basalt & Copper, set dark default, or add a theme switcher.
- Duplicate SAMI or AEC brand assets or imply affiliation.
- Invent an EMAD logo or use generic tactical symbols as corporate identity.
- Recolor statuses by subjective visual taste while changing their meanings.
- Hide stale/replay/confidence information for aesthetic simplicity.
- Replace working functionality, data model or security behavior with mock visuals.
- Treat this identity specification as a request to implement new ISR, AI, sensors or hardware functionality.

**Authority:** This document owns visual design. Product and architecture documents own domain and system behavior. When in doubt, keep the operator informed and the underlying system unchanged.
