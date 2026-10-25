# EMAD adaptive map dock

25 October 2026. This increment supersedes the dock sizing and placement in the [previous cleanup](EMAD_MAP_CLEANUP_REVIEW.md). See the [captured application views](map-dock/index.html).

## Problem and implemented behavior

Consolidating tools into one dock removed overlapping panels, but the dock still claimed too much height. Its permanent summary row, preview sizing and automatic expansion made modest tasks displace much of the map. Shrinking every detailed form into that bottom strip would also make those tasks harder to use.

- **Idle:** one 44px toolbar. Situation counts, pause, refresh and previous view use a temporary disclosure instead of another permanent row.
- **Previews:** 124px including the toolbar on a wide desktop; 152px on narrower screens. Records retain thumbnails, source and acquisition dates.
- **Detailed work:** automatic side placement at desktop widths of at least 1100px. Short landscape screens at least 700px wide also use the side. The map and panel receive separate layout space.
- **Phones:** a bounded bottom panel. At 390 × 844 it is 236px; ordinary opening never exceeds 248px. Contents scroll inside it. Compact toolbar menus keep controls within the screen.
- **User control:** Layout offers automatic, side and bottom placement, expansion and maximization. Ordinary tool opening uses the compact state. A maximized workbench does not return automatically after reload.
- **Existing workflows:** Systems keeps its six domains and source/processing sections. Other workbenches reuse existing business components. Container queries adapt multi-column forms to the allocated panel width. Temporary controls preserve workbenches and drafts.
- **Keyboard:** Escape closes an open menu first; otherwise it returns from a control panel and restores focus to an available toolbar control. Switching panels avoids focus being stolen by the departing panel. Short-screen popovers stay below the scope bar.

Layout preferences remain account/resource scoped. A new preference version prevents the previous large remembered dock from being applied to the new default. No migration or content reinstall is required.

## Measured geometry

The [persistent-installation receipt](map-dock/layout-acceptance.json) records a 1440 × 1000 viewport:

- Workspace height: 870px; idle map height: 765px, or 87.9% of the workspace.
- Toolbar: 44px; desktop preview tray: 124px.
- Detailed side panel: 374px; map width: 1066px, or 74.0% of the workspace width.
- Map height with the side panel: 809px. The panel does not overlay the map.

These measure available layout space, not frame-rate performance. Allocation changes with viewport height and width.

## Verification

The persistent installation is running at `http://127.0.0.1:5173/map`, using its existing database, object storage and content. Screenshots and measured geometry were refreshed on 25 October. No database or volume was reset.

- Type checking and production build passed. The existing large renderer chunk warning remains.
- Full frontend suite: **80 passed, 54 intentionally skipped** for separately selected profiles.
- Six focused public-context browser checks passed after the loading-state presentation fix.
- **34 live map/workspace checks passed** on 25 October, including the new result-width regression. The final four adaptive checks also passed after ensuring Expand cannot shrink a panel at constrained sizes. Screenshots and geometry were refreshed from actual retained data.
- **16 production real-backend browser scenarios passed**, including exact evidence downloads, retained imagery comparison, independent review/export, public contacts, official reports, Studio, replay, evaluator isolation and revocation. Domain APIs and WebSockets are not mocked in this gate.
- `git diff --check` passed.

Container adaptations share the CSS layer used by existing desktop workspaces. This prevents desktop columns from collapsing results to zero text width inside a narrow side panel. Single-column forms reset wider column spans; selected analysis results precede inputs and history. The regression verifies readable result width in side, maximized and phone layouts.

Reproduce the browser checks with the supported `task dev` installation running:

```powershell
$env:EMAD_RECOVERY_ACCEPTANCE='1'
pnpm --dir apps/operator-ui exec playwright test --config playwright.recovery.config.ts adaptive-dock.spec.ts map-cleanup.spec.ts atlas.spec.ts workspace-redesign.spec.ts workspace-polish.spec.ts --reporter=line --timeout=40000
$env:C4ISR_TEST_INTEGRATION='1'
$env:C4ISR_TEST_BROWSER='1'
go test -count=1 -timeout 20m -run TestWorkspaceBrowserCompatibilityGate -v ./internal/integration
```

The adaptive capture waits for decoded Systems imagery. Keyboard checks include Escape, focus restoration and phone/short-landscape menu bounds; workflow checks preserve real geography, imagery, Systems and unsaved drafts.

## Implementation

- `apps/operator-ui/src/features/workspace/atlasDockLayout.ts`: responsive allocation and compact controls.
- `apps/operator-ui/src/features/workspace/PublicAtlasWorkspace.tsx`: toolbar, menus, compact opening, preferences and focus.
- `apps/operator-ui/src/features/workspace/AtlasToolbarMenu.tsx`: outside-click dismissal, menu exclusivity and Escape.
- `apps/operator-ui/src/styles/atlas-dock.css`: bounded dock, responsive workbench containers and image-state typography.
- `apps/operator-ui/src/features/workspace/ArtifactImage.tsx`: identifiable image-state presentation.
- `apps/operator-ui/tests/recovery/adaptive-dock.spec.ts`: measured space, explicit expansion, reload and viewport acceptance.

Provider admission, historical meaning, provenance, geographic predicates and Public/Exercise authorization continue through the existing components and API contracts.
