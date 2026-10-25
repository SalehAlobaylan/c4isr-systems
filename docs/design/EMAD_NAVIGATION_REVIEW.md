# Workspace, workbench and preview navigation

25 October 2026. Implemented in the running Windows installation. This extends the [adaptive map dock](EMAD_ADAPTIVE_DOCK_REVIEW.md); its compact allocation remains in place. See the [application screenshots](navigation/index.html).

## What made the previous controls difficult

The interface presented three different decisions as flat native selects: the authorized data context, the workbench to open, and the record family to preview. Workspace entries repeated `default · local`, used short run IDs as their primary names, and interleaved Public Context with many past exercises. Capability names offered little help about their purpose. The result made a large installation look like a long configuration list.

The new presentation gives each decision its own interaction:

1. **Where am I working?** The header context button opens a searchable workspace browser. Public Context, active exercises, history, legacy contexts and the current authorized Studio context have separate groups. The current context is marked. Rows show available exercise names, start dates and state; short references are secondary and full IDs remain searchable. The same workspace/environment is not repeated on every exercise row.
2. **What do I want to do?** The map's **Workbenches** button opens purpose-based cards: Start here, Explore, Analyze, Manage and Simulation. Systems keeps its real grid. Descriptions explain the work each card opens, search recognizes useful terms such as satellite, hazards, exports and AIS, and unavailable capabilities explain the required access.
3. **Which records should the tray browse?** The preview-feed control opens a small anchored list with typed icons and scoped retained counts or unavailable/off states. It does not need the larger workspace browser. Preview choice and layer visibility remain independent.

Both larger browsers are temporary. Closing or choosing restores the underlying map; no new permanent panel occupies its canvas.

## Large catalogs and deliberate transitions

The installed account currently has **42 selectable contexts**. History initially shows six entries per group, with Show all and search across the complete authorized catalog. Pins offer direct return to useful contexts. Only resource references are stored as account-scoped device preferences; exercise names and evidence are not persisted in those preferences. Removed resources disappear even when a stale pin remains in device storage.

Exercise operations and replay now use an explicit exercise choice. The header's exercise fallback also opens the browser. These paths no longer choose a run based on its position in an ID-sorted array. Existing authoring access checks remain separate from ordinary exercise observation access.

The current metadata is honest about its limits. Legacy scenario runs use their stored scenario names. Native ordinary projections currently provide a generic Simulation exercise label, state and start time; their dates and references distinguish runs. Private authoring-scene names, actors and truth are not fetched to manufacture more descriptive labels. Undated or metadata-restricted contexts say that the start time is unavailable.

## Implementation and isolation

`GET /api/v1/auth/me` adds optional, compatible `resourceDetails` alongside the unchanged `resources` contract. Details cover only the already selectable resources. Exercise metadata additionally requires `scenarios.read` in each candidate resource. The projection selects only ordinary scenario/native-exercise name, status and start time; it reads no authoring scene, frozen configuration, actor, sensor or truth document. Native stale-lease state follows the existing ordinary run-view semantics.

The existing five-second authorization revalidation also refreshes changed catalog entries and labels without replacing the active query client, aborting the workspace or remounting draft forms. Changed permissions still take the existing full invalidation path. UI choice never replaces server authorization.

No database migration or content reinstall was introduced. The API was rebuilt and restarted through the supported local lifecycle, preserving the existing database and object storage.

Primary code:

- `components/navigation/ChoiceBrowser.tsx`: grouped search, bounded sections, selection, pins, empty/error/busy states and keyboard movement.
- `components/navigation/WorkspaceChooser.tsx`: authorized contexts, ordinary exercise metadata, sorting and device pins.
- `features/workspace/MapWorkbenchChooser.tsx`: workbench cards and the compact preview-feed list.
- `components/layout/WorkspaceHeader.tsx` and `features/workspace/SimulationGateway.tsx`: explicit navigation actions.
- `components/WorkspaceProvider.tsx`: catalog refresh without remounting the workspace.
- `styles/navigation.css`: shared appearance, 12px minimum navigation labels, bounded dialogs and phone layouts.
- `internal/platform/access/catalog.go`: permission-filtered ordinary navigation metadata.

## Accessibility and verification

Search receives focus on opening. Arrow keys move through choices; native Tab and Enter remain available. Escape returns focus to the trigger and preserves an underlying map control panel. Pinning restores focus after moving a row into Pinned. Dialogs trap focus and remain inside a 320px phone viewport. The smaller feed selector uses the existing Base UI selection primitives and a bounded, scrollable popup.

Verification:

- Type checking and production build passed; the previously documented large renderer bundle warning remains.
- Full frontend suite: **84 passed / 58 intentional profile skips**. Four new checks cover 50 exercises, search beyond collapsed history, pin focus, explicit switching, catalog updates and removed pins, unavailable capabilities, useful search terms, phone bounds and Escape behavior.
- The live map/workspace run passed 32 checks initially. Two checks needed timing/locator updates for the new interaction: wait for the deliberate country-fit animation before comparing camera state, and distinguish the historical-result heading from the same explanation in feed options. Both targeted reruns passed, verifying all **34 live checks**.
- Access/app Go package checks passed. The real PostGIS catalog authorization test passed, including foreign-run omission and removal of run details after scenario-read permission is revoked.
- **16 production real-backend browser scenarios passed**, including context switching, private-data isolation, revocation, imagery review/export and simulation/replay compatibility. Domain APIs and WebSockets are not mocked in this gate.
- The [persistent installation capture](navigation/acceptance.json) verified 42 authorized contexts, eight initially displayed context choices, 11 workbenches, nine preview feeds and the unchanged 44px idle dock. It exercised an explicit operations gateway choice and return to Public Context, with no page exceptions or requests for private scene/runtime/truth documents. The actual phone dialog fit within 390 × 844.

The installed screenshots and capture receipt supplement the automated suites. The mocked 50-exercise catalog is a presentation stress test, separate from the installation's real retained contexts.
