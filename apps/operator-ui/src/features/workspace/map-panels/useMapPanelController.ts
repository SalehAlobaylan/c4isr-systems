import { useCallback, useEffect, useRef, useState } from 'react'
import { useWorkspacePreference } from '../preferences'
import type { MapTool } from '../WorkbenchContext'

export type SupportPanel = '' | 'layers' | 'records' | 'tools' | 'views'
export type DrawerSize = 'normal' | 'wide' | 'full'

export const MIN_DRAWER_WIDTH = 310
export const MAX_DRAWER_WIDTH = 680
export const DEFAULT_DRAWER_WIDTH = 380
export const WIDE_DRAWER_WIDTH = 540

const validWidth = (value: unknown): value is number =>
  typeof value === 'number' &&
  Number.isFinite(value) &&
  value >= MIN_DRAWER_WIDTH &&
  value <= MAX_DRAWER_WIDTH

const isCompactViewport = () => window.innerWidth <= 760 || window.innerHeight <= 500

/**
 * Deterministic layout controller for the Public Context map support panels.
 *
 * Replaces competing dock orientation/size booleans with four independent,
 * non-overlapping visual roles:
 * 1. Top Scope & Readiness Bar (always visible unless Clean Map is active)
 * 2. Fixed Left Activity Rail + Single Resizable Drawer (one active panel or workbench)
 * 3. Floating Selection Peek Card on the map canvas (never hijacks the drawer)
 * 4. Anchored popovers (Time & Filters) and on-map HUD (Contextual Legend, Live Transport)
 */
export function useMapPanelController(tool: MapTool | undefined, autoCollapseTool: boolean) {
  const [drawerWidth, setDrawerWidth] = useWorkspacePreference<number>(
    'map-drawer-width-v1',
    DEFAULT_DRAWER_WIDTH,
    validWidth,
  )
  const [panel, setPanelRaw] = useState<SupportPanel>('')
  const [drawerSize, setDrawerSize] = useState<DrawerSize>(() =>
    tool && isCompactViewport() ? 'full' : 'normal',
  )
  const [drawerHiddenForTool, setDrawerHiddenForTool] = useState(() => Boolean(tool && autoCollapseTool))
  const [cleanMap, setCleanMap] = useState(false)
  const [timePopoverOpen, setTimePopoverOpen] = useState(false)
  const [legendOpen, setLegendOpen] = useState(false)

  const panelTriggerRef = useRef<string | undefined>(undefined)
  const prevToolRef = useRef<MapTool | undefined>(tool)
  const prevAutoCollapseRef = useRef<boolean>(autoCollapseTool)
  const keepPanelOnToolClearRef = useRef<boolean>(false)
  const currentPanelRef = useRef<SupportPanel>(panel)
  currentPanelRef.current = panel

  useEffect(() => {
    if (tool !== prevToolRef.current) {
      setTimePopoverOpen(false)
      setCleanMap(false)
      if (tool) {
        keepPanelOnToolClearRef.current = false
        setPanelRaw('')
        setDrawerHiddenForTool(autoCollapseTool)
        setDrawerSize(isCompactViewport() ? 'full' : 'normal')
      } else {
        setDrawerHiddenForTool(false)
        if (keepPanelOnToolClearRef.current) {
          keepPanelOnToolClearRef.current = false
          if (window.innerHeight <= 500) setDrawerSize('full')
        } else {
          setPanelRaw('')
          setDrawerSize('normal')
        }
      }
      prevToolRef.current = tool
      prevAutoCollapseRef.current = autoCollapseTool
    } else if (tool && autoCollapseTool !== prevAutoCollapseRef.current) {
      setDrawerHiddenForTool(autoCollapseTool)
      prevAutoCollapseRef.current = autoCollapseTool
    }
  }, [tool, autoCollapseTool])

  const openPanel = useCallback(
    (next: SupportPanel, triggerLabel?: string, options?: { keepOnToolClear?: boolean }) => {
      setTimePopoverOpen(false)
      setCleanMap(false)
      if (options?.keepOnToolClear) {
        keepPanelOnToolClearRef.current = true
      }
      if (next) {
        if (!currentPanelRef.current) {
          const active = document.activeElement as HTMLElement | null
          panelTriggerRef.current = triggerLabel ?? active?.getAttribute('aria-label') ?? undefined
        }
        setDrawerHiddenForTool(false)
        if (window.innerHeight <= 500) {
          setDrawerSize('full')
        }
      }
      setPanelRaw(next)
    },
    [],
  )

  const togglePanel = useCallback(
    (target: Exclude<SupportPanel, ''>, triggerLabel?: string) => {
      if (panel === target && !cleanMap) {
        if (timePopoverOpen) {
          setTimePopoverOpen(false)
        } else {
          openPanel('')
        }
      } else {
        openPanel(target, triggerLabel)
      }
    },
    [panel, cleanMap, timePopoverOpen, openPanel],
  )

  const toggleCleanMap = useCallback(() => {
    setTimePopoverOpen(false)
    setCleanMap((prev) => !prev)
  }, [])

  const toggleMaximize = useCallback(() => {
    setDrawerSize((prev) =>
      prev === 'full' ? (drawerWidth >= WIDE_DRAWER_WIDTH - 20 ? 'wide' : 'normal') : 'full',
    )
  }, [drawerWidth])

  const toggleWideWidth = useCallback(() => {
    const nextWidth = drawerWidth >= WIDE_DRAWER_WIDTH - 20 ? DEFAULT_DRAWER_WIDTH : WIDE_DRAWER_WIDTH
    setDrawerWidth(nextWidth)
    setDrawerSize(nextWidth >= WIDE_DRAWER_WIDTH - 20 ? 'wide' : 'normal')
  }, [drawerWidth, setDrawerWidth])

  const setBoundedWidth = useCallback(
    (next: number, viewportWidth: number) => {
      const maxAllowed = Math.min(MAX_DRAWER_WIDTH, Math.max(MIN_DRAWER_WIDTH, Math.round(viewportWidth * 0.62)))
      const clamped = Math.max(MIN_DRAWER_WIDTH, Math.min(maxAllowed, Math.round(next)))
      setDrawerWidth(clamped)
      setDrawerSize(clamped >= WIDE_DRAWER_WIDTH - 20 ? 'wide' : 'normal')
    },
    [setDrawerWidth],
  )

  const hasDrawerOpen = !cleanMap && (Boolean(panel) || (Boolean(tool) && !drawerHiddenForTool))
  const effectiveWidth =
    drawerSize === 'full' ? undefined : Math.max(MIN_DRAWER_WIDTH, Math.min(MAX_DRAWER_WIDTH, drawerWidth))

  return {
    panel,
    openPanel,
    togglePanel,
    drawerWidth: effectiveWidth ?? DEFAULT_DRAWER_WIDTH,
    setBoundedWidth,
    drawerSize,
    setDrawerSize,
    toggleMaximize,
    toggleWideWidth,
    drawerHiddenForTool,
    setDrawerHiddenForTool,
    hasDrawerOpen,
    cleanMap,
    setCleanMap,
    toggleCleanMap,
    timePopoverOpen,
    setTimePopoverOpen,
    legendOpen,
    setLegendOpen,
    panelTriggerRef,
  }
}
