import { useCallback, useEffect, useRef, useState } from 'react'
import { useWorkspacePreference } from '../preferences'
import type { MapTool } from '../WorkbenchContext'

export type SupportPanel = '' | 'layers' | 'records' | 'tools' | 'views' | 'simulate'
export type DrawerSize = 'normal' | 'wide' | 'full'

export const MIN_DRAWER_WIDTH = 310
export const MAX_DRAWER_WIDTH = 760
export const DEFAULT_DRAWER_WIDTH = 360
export const ANALYTICAL_DRAWER_WIDTH = 460
export const WIDE_DRAWER_WIDTH = 560

const WIDE_TOOLS = new Set<MapTool>([
  'imagery',
  'review',
  'context',
  'systems',
  'weather',
  'tracks',
  'environments',
  'simulation',
])

const ANALYTICAL_TOOLS = new Set<MapTool>([
  'coverage',
  'investigations',
  'geography',
  'sources',
  'jobs',
])

export function recommendedDrawerWidth(
  panel: SupportPanel,
  tool: MapTool | undefined,
  viewportWidth: number,
): number {
  const cap = Math.min(MAX_DRAWER_WIDTH, Math.max(MIN_DRAWER_WIDTH, Math.round(viewportWidth * 0.58)))
  if (panel === 'simulate') return Math.min(420, cap)
  if (panel) return Math.min(DEFAULT_DRAWER_WIDTH, cap)
  if (tool && WIDE_TOOLS.has(tool)) return Math.min(WIDE_DRAWER_WIDTH, cap)
  if (tool && ANALYTICAL_TOOLS.has(tool)) return Math.min(ANALYTICAL_DRAWER_WIDTH, cap)
  return Math.min(DEFAULT_DRAWER_WIDTH, cap)
}

const validStoredWidth = (value: unknown): value is number | null =>
  value === null ||
  (typeof value === 'number' &&
    Number.isFinite(value) &&
    value >= MIN_DRAWER_WIDTH &&
    value <= MAX_DRAWER_WIDTH)

const isCompactViewport = () => window.innerWidth <= 760 || window.innerHeight <= 500

/**
 * Deterministic layout controller for the Public Context map support panels.
 *
 * Uses content-aware default drawer widths (360px compact tools, 460px
 * analytical workbenches, 560px wide visual workbenches) while honoring manual
 * drag or keyboard resize overrides.
 */
export function useMapPanelController(tool: MapTool | undefined, autoCollapseTool: boolean) {
  const [customWidth, setCustomWidth] = useWorkspacePreference<number | null>(
    'map-drawer-width-v2',
    null,
    validStoredWidth,
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
  const lastPanelRef = useRef<Exclude<SupportPanel, ''>>('layers')
  const prevToolRef = useRef<MapTool | undefined>(tool)
  const prevAutoCollapseRef = useRef<boolean>(autoCollapseTool)
  const keepPanelOnToolClearRef = useRef<boolean>(false)
  const currentPanelRef = useRef<SupportPanel>(panel)
  currentPanelRef.current = panel

  useEffect(() => {
    if (panel) lastPanelRef.current = panel
  }, [panel])

  useEffect(() => {
    if (tool !== prevToolRef.current) {
      setTimePopoverOpen(false)
      setCleanMap(false)
      if (tool) {
        keepPanelOnToolClearRef.current = false
        setPanelRaw('')
        setDrawerHiddenForTool(autoCollapseTool)
        const wide = WIDE_TOOLS.has(tool)
        setDrawerSize(isCompactViewport() ? 'full' : wide ? 'wide' : 'normal')
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
        setDrawerSize(window.innerHeight <= 500 ? 'full' : 'normal')
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

  const recommended = recommendedDrawerWidth(panel, tool, typeof window === 'undefined' ? 1440 : window.innerWidth)
  const resolvedWidth =
    customWidth !== null
      ? Math.max(MIN_DRAWER_WIDTH, Math.min(MAX_DRAWER_WIDTH, customWidth))
      : recommended

  const toggleMaximize = useCallback(() => {
    setDrawerSize((prev) =>
      prev === 'full' ? (resolvedWidth >= WIDE_DRAWER_WIDTH - 20 ? 'wide' : 'normal') : 'full',
    )
  }, [resolvedWidth])

  const toggleWideWidth = useCallback(() => {
    const isCurrentlyWide = resolvedWidth >= WIDE_DRAWER_WIDTH - 20
    const nextWidth = isCurrentlyWide ? DEFAULT_DRAWER_WIDTH : WIDE_DRAWER_WIDTH
    setCustomWidth(nextWidth)
    setDrawerSize(nextWidth >= WIDE_DRAWER_WIDTH - 20 ? 'wide' : 'normal')
  }, [resolvedWidth, setCustomWidth])

  const setBoundedWidth = useCallback(
    (next: number, viewportWidth: number) => {
      const maxAllowed = Math.min(MAX_DRAWER_WIDTH, Math.max(MIN_DRAWER_WIDTH, Math.round(viewportWidth * 0.65)))
      const clamped = Math.max(MIN_DRAWER_WIDTH, Math.min(maxAllowed, Math.round(next)))
      setCustomWidth(clamped)
      setDrawerSize(clamped >= WIDE_DRAWER_WIDTH - 20 ? 'wide' : 'normal')
    },
    [setCustomWidth],
  )

  const hasDrawerOpen = !cleanMap && (Boolean(panel) || (Boolean(tool) && !drawerHiddenForTool))
  const effectiveWidth = drawerSize === 'full' ? undefined : resolvedWidth

  return {
    panel,
    openPanel,
    togglePanel,
    drawerWidth: effectiveWidth ?? DEFAULT_DRAWER_WIDTH,
    isWideWidth: resolvedWidth >= WIDE_DRAWER_WIDTH - 20,
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
    lastPanelRef,
  }
}
