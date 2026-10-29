# Map workspace usability refinements — 29 October 2026

Implemented against the retained local installation at http://127.0.0.1:5198. This is a focused map and investigation workflow correction, not completion of release qualification or a redesign of every workbench.

## Problems addressed

- Phone layouts imposed a 950px workspace and a 640px map stage inside a smaller viewport, putting essential controls off screen. The workspace now fits the viewport and scrolls detailed content within the workbench.
- Investigations, coverage and imagery were buried in the bottom launcher. Direct task actions now explain the intended sequence without requiring launcher discovery.
- Long rolling-time labels dominated the scope bar. Monitoring now shows a compact period and timezone; full timing remains available in filters and scope details. Historical studies retain their year and visible exact interval.
- Detailed workbenches were squeezed into narrow panels. Desktop panels have more width; phone and short-screen workbenches open in the available workspace with a visible close action.
- Repeated explanatory text preceded actual cases. Focused workbenches remove duplicate system navigation and put evidence-policy details behind a labelled disclosure. Investigation copy, saved-case targets and form spacing are clearer. Starting another investigation warns before discarding an edited draft.
- Failed map queries appeared as unexplained exclamation marks. Layer failures now explain uncertainty and offer retry/source inspection. Read-capacity failures have a specific explanation. Pointer coordinates are explicitly labelled.
- Investigations and coverage triggered unrelated overview catalogs. Overview reads now follow the active section. Catalog caches survive view switches for their bounded lifetime, and the ordinary map avoids a second event read used only by the geography inspector.

## Verification

Production TypeScript/build checks passed. Existing map-task and analyst-scope checks exercised 22 applicable cases: 15 passed initially, and seven date-visibility failures passed after restoring historical interval visibility. The 22 explicit-token duplicates are intentionally skipped by these suites. No new test suite was written. Initial failure artifacts remain retained.

Live browser evaluation covered desktop and phone map entry, investigation navigation, coordinate tools, closing workbenches and viewport overflow. Evidence screenshots and initial/final regression artifacts are under ignored `private/qualification/ux-2026-10-29/`. This evaluator-operated work is not an independent human usability session.

## Remaining limitations

Server read-capacity exhaustion and dense-workload performance are not resolved by clearer presentation or fewer background requests. Membership/evidence workflows still expose technical identifiers. Source classification, real analytical usefulness, independent review and release qualification remain separate unfinished work. UI changes do not establish source coverage or turn missing evidence into a conclusion.
