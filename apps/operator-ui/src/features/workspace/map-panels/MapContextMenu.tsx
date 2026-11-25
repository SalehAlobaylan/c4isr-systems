import { useEffect, useRef } from 'react'
import { CopyIcon, CrosshairIcon, PlusIcon, RulerIcon, ShieldAlertIcon } from '@/components/icons'

export interface MapContextMenuTarget {
  lng: number
  lat: number
  x: number
  y: number
}

interface MapContextMenuProps {
  target: MapContextMenuTarget | null
  canReadSources: boolean
  onClose: () => void
  onCopyCoords: (lng: number, lat: number) => void
  onMeasureFromHere: (lng: number, lat: number) => void
  onDrawAreaFromHere: (lng: number, lat: number) => void
  onBuffer10Km: (lng: number, lat: number) => void
  onCheckCoverage: () => void
  onZoomHere: (lng: number, lat: number) => void
}

function formatWgs84(lng: number, lat: number): string {
  const ns = lat >= 0 ? 'N' : 'S'
  const ew = lng >= 0 ? 'E' : 'W'
  return `${Math.abs(lat).toFixed(4)}° ${ns}, ${Math.abs(lng).toFixed(4)}° ${ew}`
}

export function MapContextMenu({
  target,
  canReadSources,
  onClose,
  onCopyCoords,
  onMeasureFromHere,
  onDrawAreaFromHere,
  onBuffer10Km,
  onCheckCoverage,
  onZoomHere,
}: MapContextMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!target) return
    const onPointerDown = (event: PointerEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        onClose()
      }
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        onClose()
      }
    }
    document.addEventListener('pointerdown', onPointerDown)
    window.addEventListener('keydown', onKeyDown, true)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      window.removeEventListener('keydown', onKeyDown, true)
    }
  }, [target, onClose])

  if (!target) return null

  const { lng, lat, x, y } = target
  const label = formatWgs84(lng, lat)

  return (
    <div
      ref={menuRef}
      className="map-context-menu"
      role="menu"
      aria-label="Map location actions"
      style={{
        left: `min(${Math.round(x)}px, calc(100% - 264px))`,
        top: `min(${Math.round(y)}px, calc(100% - 260px))`,
      }}
    >
      <div className="map-context-menu-coords font-mono">{label}</div>

      <button
        type="button"
        role="menuitem"
        onClick={() => {
          onCopyCoords(lng, lat)
          onClose()
        }}
      >
        <span className="map-context-menu-label">
          <CopyIcon className="size-3.5" />
          Copy WGS84 coordinates
        </span>
      </button>

      <button
        type="button"
        role="menuitem"
        onClick={() => {
          onMeasureFromHere(lng, lat)
          onClose()
        }}
      >
        <span className="map-context-menu-label">
          <RulerIcon className="size-3.5" />
          Measure distance from here
        </span>
      </button>

      <button
        type="button"
        role="menuitem"
        onClick={() => {
          onDrawAreaFromHere(lng, lat)
          onClose()
        }}
      >
        <span className="map-context-menu-label">
          <CrosshairIcon className="size-3.5" />
          Draw study polygon from here
        </span>
      </button>

      <button
        type="button"
        role="menuitem"
        onClick={() => {
          onBuffer10Km(lng, lat)
          onClose()
        }}
      >
        <span className="map-context-menu-label">
          <PlusIcon className="size-3.5" />
          Create 10 km context buffer
        </span>
      </button>

      {canReadSources && (
        <button
          type="button"
          role="menuitem"
          onClick={() => {
            onCheckCoverage()
            onClose()
          }}
        >
          <span className="map-context-menu-label">
            <ShieldAlertIcon className="size-3.5" />
            Check source coverage here
          </span>
        </button>
      )}

      <div className="map-context-menu-sep" aria-hidden="true" />

      <button
        type="button"
        role="menuitem"
        onClick={() => {
          onZoomHere(lng, lat)
          onClose()
        }}
      >
        <span className="map-context-menu-label">
          <PlusIcon className="size-3.5" />
          Zoom in here
        </span>
      </button>
    </div>
  )
}
