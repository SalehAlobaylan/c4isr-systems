import {
  useState,
  type CSSProperties,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from 'react'
import { Link } from '@tanstack/react-router'
import { ExternalLinkIcon, MaximizeIcon, MinimizeIcon, XIcon } from '@/components/icons'
import type { DrawerSize } from './useMapPanelController'
import { MAX_DRAWER_WIDTH, MIN_DRAWER_WIDTH } from './useMapPanelController'

export interface FullPageDestination {
  to: string
  search?: Record<string, unknown>
  label: string
}

interface MapDrawerProps {
  open: boolean
  title: string
  subtitle?: string
  regionLabel: string
  closeLabel?: string
  width: number
  size: DrawerSize
  fullPageLink?: FullPageDestination
  onResizeWidth: (nextWidth: number) => void
  onToggleMaximize: () => void
  onToggleWide: () => void
  onClose: () => void
  drawerRef?: React.RefObject<HTMLElement | null>
  children: ReactNode
}

export function MapDrawer({
  open,
  title,
  subtitle,
  regionLabel,
  closeLabel = 'Close map tool',
  width,
  size,
  fullPageLink,
  onResizeWidth,
  onToggleMaximize,
  onToggleWide,
  onClose,
  drawerRef,
  children,
}: MapDrawerProps) {
  const [dragging, setDragging] = useState(false)

  const onResizeStart = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.preventDefault()
    const startX = event.clientX
    const startWidth = width
    event.currentTarget.setPointerCapture(event.pointerId)
    setDragging(true)

    const onMove = (moveEvent: PointerEvent) => {
      onResizeWidth(startWidth + (moveEvent.clientX - startX))
    }
    const onUp = () => {
      setDragging(false)
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
    }
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
  }

  const onResizeKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
    event.preventDefault()
    const step = event.shiftKey ? 48 : 20
    onResizeWidth(event.key === 'ArrowRight' ? width + step : width - step)
  }

  const sizeButtonLabel = size === 'full' ? 'Return tool to map' : 'Maximize map workbench'

  return (
    <>
      <section
        ref={drawerRef}
        hidden={!open}
        className="map-drawer"
        aria-label={regionLabel}
        data-size={size}
        style={{ '--map-drawer-w': `${width}px` } as CSSProperties}
      >
        <header className="map-drawer-header">
          <div className="map-drawer-title-group">
            <div className="map-drawer-title-row">
              <p className="map-drawer-title">{title}</p>
            </div>
            {subtitle && <p className="map-drawer-subtitle">{subtitle}</p>}
          </div>
          <div className="map-drawer-actions">
            {fullPageLink && (
              <Link
                to={fullPageLink.to}
                search={fullPageLink.search as never}
                className="map-drawer-iconbtn"
                title={fullPageLink.label}
                aria-label={fullPageLink.label}
              >
                <ExternalLinkIcon className="size-3.5" />
              </Link>
            )}
            <button
              type="button"
              className="map-drawer-iconbtn"
              aria-label={sizeButtonLabel}
              title={size === 'full' ? 'Return panel to docked width' : 'Maximize panel'}
              onClick={onToggleMaximize}
            >
              {size === 'full' ? <MinimizeIcon className="size-3.5" /> : <MaximizeIcon className="size-3.5" />}
            </button>
            <button
              type="button"
              className="map-drawer-iconbtn"
              aria-label={closeLabel}
              title="Close panel (Esc)"
              onClick={onClose}
            >
              <XIcon className="size-4" />
            </button>
          </div>
        </header>
        <div className="map-drawer-body">{children}</div>
      </section>

      {open && size !== 'full' && (
        <div
          className="map-drawer-resizer"
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize map support panel"
          aria-valuemin={MIN_DRAWER_WIDTH}
          aria-valuemax={MAX_DRAWER_WIDTH}
          aria-valuenow={width}
          tabIndex={0}
          data-dragging={dragging || undefined}
          onPointerDown={onResizeStart}
          onKeyDown={onResizeKeyDown}
          onDoubleClick={onToggleWide}
          title="Drag, double-click, or use arrow keys to resize panel"
        />
      )}
    </>
  )
}
