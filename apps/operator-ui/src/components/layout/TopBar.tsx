import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Dialog as BaseDialog } from '@base-ui-components/react/dialog'

import { ActivityIcon, UserIcon } from '@/components/icons'
import { ScenarioBar } from '@/components/layout/ScenarioBar'
import { Menu } from '@/components/ui/menu'
import { Tooltip } from '@/components/ui/tooltip'
import { API_TOKEN, api, fetchHealth, realtimeUrl } from '@/lib/api'
import { formatRelative } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { cn } from '@/lib/utils'
import { useRealtime, type RealtimeStatus } from '@/realtime/RealtimeProvider'
import { toast } from '@/stores/toasts'

const PRIMARY_VIEWS = [
  { index: 1, to: '/board', label: 'Board' },
  { index: 2, to: '/map', label: 'Picture' },
  { index: 3, to: '/alerts', label: 'Alerts' },
  { index: 4, to: '/incidents', label: 'Incidents' },
  { index: 5, to: '/timeline', label: 'Audit' },
] as const

const SECONDARY_VIEWS = [
  { to: '/assets', label: 'Assets' },
  { to: '/tracks', label: 'Tracks' },
  { to: '/scenarios', label: 'Scenarios' },
] as const

const HELP_KEYS: Array<[string, string]> = [
  ['1', 'Board view'],
  ['2', 'Picture view'],
  ['3', 'Alerts view'],
  ['4', 'Incidents view'],
  ['5', 'Audit view'],
  ['Space', 'Pause or resume the active scenario run'],
  ['R', 'Replay — restart the active run with the same seed'],
  ['[', 'Collapse the picture detail rail'],
  [']', 'Expand the picture detail rail'],
  ['L', 'Toggle the map layer control'],
  ['?', 'Close this panel · Esc also closes'],
]

/**
 * Link state is spelled out, never implied by a colour. `LINK` alone left an
 * operator guessing whether the picture they were looking at was current, so
 * the states DESIGN.md §7.1 names are used verbatim.
 */
const WS_LABEL: Record<RealtimeStatus, string> = {
  connected: 'LIVE',
  connecting: 'CONNECTING',
  reconnecting: 'LINK RETRY',
  disconnected: 'DISCONNECTED',
}

const WS_DOT: Record<RealtimeStatus, string> = {
  connected: 'aegis-dot aegis-dot--live',
  connecting: 'aegis-dot aegis-dot--init',
  reconnecting: 'aegis-dot aegis-dot--init',
  disconnected: 'aegis-dot aegis-dot--down',
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName.toLowerCase()
  return (
    tag === 'input' ||
    tag === 'select' ||
    tag === 'textarea' ||
    target.isContentEditable ||
    target.closest('dialog, [role="dialog"]') !== null
  )
}

export function TopBar() {
  const navigate = useNavigate()
  const { status: realtimeStatus, lastEventAt } = useRealtime()
  const [helpOpen, setHelpOpen] = useState(false)
  const shellRef = useRef<HTMLDivElement | null>(null)
  const barRef = useRef<HTMLElement | null>(null)
  const brandRef = useRef<HTMLDivElement | null>(null)
  const navRef = useRef<HTMLElement | null>(null)
  const topRightRef = useRef<HTMLDivElement | null>(null)
  const [navOwnRow, setNavOwnRow] = useState(false)

  // Whether the bar's single row fits depends on content as much as on width:
  // the unauthenticated operator chip is wider than the signed-in one, so the
  // same viewport fits or clips depending on session state. A container query
  // can see the bar's width but never its content, so the fit is measured.
  // The items are `flex: none`, so their widths are identical before and after
  // the row moves and the measurement cannot oscillate.
  useEffect(() => {
    const shell = shellRef.current
    const bar = barRef.current
    const nav = navRef.current
    const brand = brandRef.current
    const topRight = topRightRef.current
    if (!shell || !bar || !nav || !brand || !topRight) return

    let disposed = false
    const measure = () => {
      if (disposed) return
      const navGap = Number.parseFloat(getComputedStyle(nav).columnGap) || 0
      const items = Array.from(nav.children) as HTMLElement[]
      const content =
        items.reduce((sum, item) => {
          const style = getComputedStyle(item)
          return (
            sum +
            item.offsetWidth +
            (Number.parseFloat(style.marginLeft) || 0) +
            (Number.parseFloat(style.marginRight) || 0)
          )
        }, 0) + navGap * Math.max(0, items.length - 1)
      const barStyle = getComputedStyle(bar)
      const chrome =
        (Number.parseFloat(barStyle.paddingLeft) || 0) +
        (Number.parseFloat(barStyle.paddingRight) || 0) +
        (Number.parseFloat(barStyle.columnGap) || 0) * 2
      const available = shell.clientWidth - brand.offsetWidth - topRight.offsetWidth - chrome
      setNavOwnRow(content > available + 1)
    }

    measure()
    const observer = new ResizeObserver(measure)
    for (const element of [shell, brand, topRight, nav]) observer.observe(element)
    // Weights arrive with the webfont; a measurement taken before that is
    // taken on fallback metrics.
    void document.fonts.ready.then(measure)
    return () => {
      disposed = true
      observer.disconnect()
    }
  }, [])

  const healthQuery = useQuery({
    queryKey: queryKeys.health(),
    queryFn: ({ signal }) => fetchHealth(signal),
    refetchInterval: 30_000,
    retry: 1,
  })

  const sessionQuery = useQuery({
    queryKey: queryKeys.currentOperator(),
    queryFn: () => api.getCurrentOperator(),
    retry: false,
    staleTime: 5 * 60_000,
  })

  const authenticated = Boolean(sessionQuery.data?.operator?.id)
  const operatorID = sessionQuery.data?.operator?.id ?? 'not authenticated'
  const operatorRole = sessionQuery.data?.operator?.role ?? (API_TOKEN ? 'checking' : 'token required')
  const operatorLabel = operatorID.toUpperCase()

  const healthOk = healthQuery.isSuccess && healthQuery.data.status === 'ok'
  const healthDown = healthQuery.isError
  const healthDegraded = healthQuery.isSuccess && healthQuery.data.status !== 'ok'
  const healthLabel = healthOk
    ? 'SYS OK'
    : healthDown
      ? 'SYS DOWN'
      : healthDegraded
        ? 'SYS DEGRADED'
        : 'SYS …'
  // A degraded backend needs its time context visible, not only in a tooltip:
  // "since when has this been degraded?" is the question an operator asks
  // first, and DESIGN.md §7.4 forbids essential information living only on
  // hover (§7.7 asks for a caution label *and* time context).
  const healthSince = healthQuery.data ? formatRelative(healthQuery.data.time) : null
  const healthTone = healthOk ? 'aegis-ok' : healthDown ? 'aegis-crit' : healthDegraded ? 'aegis-warn' : 'aegis-faint'
  const healthDetail = healthDegraded || healthDown ? ` · ${healthSince ?? 'unknown'}` : ''

  const toggleHelp = useCallback(() => setHelpOpen((open) => !open), [])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey) return
      if (isEditableTarget(event.target)) return
      if (event.key === '?') {
        event.preventDefault()
        toggleHelp()
        return
      }
      if (helpOpen) return
      if (/^[1-5]$/.test(event.key)) {
        event.preventDefault()
        const view = PRIMARY_VIEWS[Number(event.key) - 1]
        if (view) void navigate({ to: view.to as never })
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [helpOpen, navigate, toggleHelp])

  return (
    <>
      {/* The wrapper carries the measured fit state. A container cannot query
          itself, and the fit depends on content the browser cannot compare —
          see the measurement effect above. */}
      <div ref={shellRef} className={cn('topbar-shell', navOwnRow && 'topbar-shell--navrows')}>
        <header ref={barRef} className="aegis-topbar">
        <div ref={brandRef} className="aegis-brand">
          <span className="aegis-glyph" aria-hidden="true">
            ع
          </span>
          <div>
            <h1 dir="ltr">
              EMAD | <bdi lang="ar" dir="rtl">عِماد</bdi>
            </h1>
            {/* One of the two sublines DESIGN.md §2.3 approves, chosen because
                this screen is the operational picture. The alternative,
                "C4ISR-aware", was an unapproved phrase that also edges toward
                a certification claim the product does not make (§1). */}
            <p className="sub">C4ISR Operational Awareness</p>
          </div>
        </div>
        <nav ref={navRef} className="aegis-nav" aria-label="Console views">
          {PRIMARY_VIEWS.map((view) => (
            <Link
              key={view.to}
              to={view.to as never}
              activeProps={{ 'aria-selected': 'true' }}
              inactiveProps={{ 'aria-selected': 'false' }}
              title={`${view.label} (${view.index})`}
            >
              {view.label}
            </Link>
          ))}
          <span className="aegis-nav-sec" aria-label="Secondary views">
            {SECONDARY_VIEWS.map((view) => (
              <Link key={view.to} to={view.to} title={view.label}>
                {view.label}
              </Link>
            ))}
          </span>
        </nav>
        <div ref={topRightRef} className="aegis-topright">
          <span
            className={cn('aegis-live', realtimeStatus !== 'connected' && 'aegis-live--down')}
            title={
              lastEventAt
                ? `Last realtime event ${formatRelative(lastEventAt)}`
                : 'Waiting for realtime events'
            }
          >
            <span className={WS_DOT[realtimeStatus]} aria-hidden="true" />
            {WS_LABEL[realtimeStatus]}
          </span>
          <Tooltip
            content={
              healthQuery.data
                ? `database: ${healthQuery.data.database} · checked ${formatRelative(healthQuery.data.time)}`
                : healthQuery.isError
                  ? 'Backend health check failed'
                  : 'Checking backend health…'
            }
          >
            <button
              type="button"
              className="aegis-sys aegis-sys-health"
              onClick={() => void healthQuery.refetch()}
              title="Backend health — click to re-check"
            >
              <ActivityIcon className={cn('size-3.5', healthTone)} />
              <span className={healthTone}>
                {healthLabel}
                {healthDetail}
              </span>
            </button>
          </Tooltip>
          <Menu
            align="end"
            trigger={
              <button
                type="button"
                className="aegis-iconbtn"
                title={authenticated ? `Signed in as ${operatorID}` : 'Authentication status'}
                aria-label={authenticated ? `Signed in as ${operatorID}` : 'Authentication status'}
              >
                <UserIcon className="size-3.5" />
                <span className="aegis-operator-id">{operatorLabel}</span>
              </button>
            }
            items={[
              {
                label: 'Copy operator ID',
                disabled: !authenticated,
                onSelect: () => {
                  if (!authenticated) return
                  void navigator.clipboard.writeText(operatorID)
                  toast({ title: 'Operator ID copied', variant: 'success' })
                },
              },
              {
                label: 'Copy realtime endpoint',
                onSelect: () => {
                  void navigator.clipboard.writeText(realtimeUrl())
                  toast({ title: 'Realtime endpoint copied', variant: 'success' })
                },
              },
            ]}
          />
          <button
            type="button"
            className="aegis-iconbtn"
            aria-haspopup="dialog"
            aria-expanded={helpOpen}
            onClick={toggleHelp}
            title="Keyboard shortcuts and controls (?)"
          >
            Shortcuts{' '}
            <span className="aegis-kbd" aria-hidden="true">
              ?
            </span>
          </button>
        </div>
      </header>
      </div>

      <ScenarioBar />

      <BaseDialog.Root open={helpOpen} onOpenChange={setHelpOpen}>
        <BaseDialog.Portal>
          <BaseDialog.Backdrop className="aegis-sheet-backdrop" />
          <BaseDialog.Popup className="aegis-sheet">
            <div className="aegis-sheet-h">
              <BaseDialog.Title>Controls &amp; shortcuts</BaseDialog.Title>
              <span className="spacer" />
              <BaseDialog.Close className="aegis-iconbtn">Close</BaseDialog.Close>
            </div>
            <div className="aegis-sheet-b">
              <div className="aegis-legend-group">
                <h3>Keyboard</h3>
                {HELP_KEYS.map(([key, label]) => (
                  <div key={key} className="aegis-kvrow">
                    <span>
                      <span className="aegis-kbd">{key}</span>
                    </span>
                    <span className="txt">{label}</span>
                  </div>
                ))}
              </div>
              <div className="aegis-legend-group">
                <h3>Panels</h3>
                <p className="aegis-note">
                  The run bar, board chips, map overlay, tallies strip and detail rail each
                  collapse from the chevron in their header. Your arrangement is remembered on
                  this device; Reset layout in the scenario bar restores the defaults. The
                  picture rail also drags: grab its left edge, or focus the handle and use the
                  arrow keys.
                </p>
              </div>
            </div>
            <div className="aegis-sheet-f">
              <span className="cap faint">{operatorRole.toUpperCase()} · {operatorLabel}</span>
              <span className="spacer" />
              <a
                className="aegis-sheet-link"
                href={realtimeUrl()}
                onClick={(event) => event.preventDefault()}
              >
                Realtime endpoint
              </a>
            </div>
          </BaseDialog.Popup>
        </BaseDialog.Portal>
      </BaseDialog.Root>
    </>
  )
}
