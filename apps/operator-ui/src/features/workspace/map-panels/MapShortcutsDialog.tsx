import { useEffect, useRef } from 'react'
import { XIcon } from '@/components/icons'

interface MapShortcutsDialogProps {
  open: boolean
  onClose: () => void
}

const SHORTCUT_GROUPS: Array<{ title: string; items: Array<[string, string]> }> = [
  {
    title: 'Navigation & Search',
    items: [
      ['/ or Ctrl+K', 'Focus Omnibox (place, coords, callsign, >command)'],
      ['F', 'Toggle Focus Map (hide/restore panels & rail)'],
      ['[', 'Collapse or reopen the active left drawer'],
      ['Esc', 'Close active menu, popover, selection, or drawer'],
    ],
  },
  {
    title: 'Map Panels & Workbenches',
    items: [
      ['L', 'Toggle Map Layers & Basemap'],
      ['R or B', 'Toggle Scoped Records & Data Radar'],
      ['C', 'Toggle Coverage & Smart Fetch'],
      ['I', 'Toggle Investigations & Citations'],
      ['T', 'Toggle Spatial Tools (Measure, Draw, Buffer)'],
      ['V', 'Toggle Saved Views & Shareable Links'],
      ['?', 'Show or hide this shortcuts guide'],
    ],
  },
  {
    title: 'Canvas Interactions',
    items: [
      ['Hover feature', 'Inspect telemetry tooltip without selecting'],
      ['Right-click map', 'Copy coords, measure, draw, 10 km buffer, check coverage'],
    ],
  },
]

export function MapShortcutsDialog({ open, onClose }: MapShortcutsDialogProps) {
  const closeRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!open) return
    closeRef.current?.focus()
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        onClose()
      }
    }
    window.addEventListener('keydown', onKeyDown, true)
    return () => window.removeEventListener('keydown', onKeyDown, true)
  }, [open, onClose])

  if (!open) return null

  return (
    <div className="map-shortcuts-backdrop" onClick={onClose}>
      <div
        className="map-shortcuts-dialog"
        role="dialog"
        aria-modal="true"
        aria-label="Map keyboard shortcuts"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="map-shortcuts-head">
          <div>
            <h2>Map keyboard shortcuts</h2>
            <p>Shortcuts pause automatically while typing in inputs or dialogs.</p>
          </div>
          <button
            ref={closeRef}
            type="button"
            className="map-drawer-iconbtn"
            aria-label="Close keyboard shortcuts"
            onClick={onClose}
          >
            <XIcon className="size-4" />
          </button>
        </header>

        <div className="map-shortcuts-body">
          {SHORTCUT_GROUPS.map((group) => (
            <section key={group.title} className="map-shortcuts-group">
              <h3>{group.title}</h3>
              <dl>
                {group.items.map(([keyCombo, desc]) => (
                  <div key={keyCombo} className="map-shortcuts-row">
                    <dt>
                      <kbd>{keyCombo}</kbd>
                    </dt>
                    <dd>{desc}</dd>
                  </div>
                ))}
              </dl>
            </section>
          ))}
        </div>
      </div>
    </div>
  )
}
