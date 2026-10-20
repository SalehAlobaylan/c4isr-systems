import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Dialog as BaseDialog } from '@base-ui-components/react/dialog'

import { UserIcon } from '@/components/icons'
import { Menu } from '@/components/ui/menu'
import { Tooltip } from '@/components/ui/tooltip'
import { API_TOKEN, api, fetchHealth, realtimeUrl } from '@/lib/api'
import { formatRelative } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { cn } from '@/lib/utils'
import { useRealtime, type RealtimeStatus } from '@/realtime/RealtimeProvider'
import { toast } from '@/stores/toasts'

/**
 * The two surfaces the console is for.
 *
 * Everything else in the nav is a side feature: real, reachable, and built on
 * the same API, but not what an operator lives in. They are rendered a tier
 * below, because a nav that gives five views equal weight is a nav that
 * gives an operator five equal things to hold in their head before they have
 * looked at anything.
 *
 * The numbers run 1..9 across both tiers in the order they appear, so a
 * number always means the same thing: the nth tab in the nav.
 */
const PRIMARY_VIEWS = [
  { index: 1, to: '/map', label: 'Map' },
  { index: 2, to: '/systems', label: 'Systems' },
] as const

const SECONDARY_VIEWS = [
  { index: 3, to: '/board', label: 'Board' },
  { index: 4, to: '/alerts', label: 'Alerts' },
  { index: 5, to: '/incidents', label: 'Incidents' },
  { index: 6, to: '/timeline', label: 'Audit' },
  { index: 7, to: '/assets', label: 'Assets' },
  { index: 8, to: '/tracks', label: 'Tracks' },
  { index: 9, to: '/scenarios', label: 'Scenarios' },
] as const

const ALL_VIEWS = [...PRIMARY_VIEWS, ...SECONDARY_VIEWS] as const

const HELP_KEYS: Array<[string, string]> = [
  ['1', 'Map view'],
  ['2', 'Systems view'],
  ['3', 'Board view'],
  ['4', 'Alerts view'],
  ['5', 'Incidents view'],
  ['6', 'Audit view'],
  ['7', 'Assets view'],
  ['8', 'Tracks view'],
  ['9', 'Scenarios view'],
  ['Space', 'Pause or resume the scenario run (picture view)'],
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
  const location = useLocation()
  const { status: realtimeStatus, lastEventAt } = useRealtime()
  const [helpOpen, setHelpOpen] = useState(false)
  const shellRef = useRef<HTMLDivElement | null>(null)
  const barRef = useRef<HTMLElement | null>(null)
  const brandRef = useRef<HTMLDivElement | null>(null)
  const navRef = useRef<HTMLElement | null>(null)
  const topRightRef = useRef<HTMLDivElement | null>(null)

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

  // The side feature the operator is currently inside, if any. The header
  // says so in the disclosure itself, so the collapse of the side features
  // into one control never costs the operator their bearings.
  const activeSideFeature =
    SECONDARY_VIEWS.find((view) => location.pathname.startsWith(view.to)) ?? null

  const healthOk = healthQuery.isSuccess && healthQuery.data.status === 'ok'
  const healthDown = healthQuery.isError
  const healthDegraded = healthQuery.isSuccess && healthQuery.data.status !== 'ok'
  // A degraded backend needs its time context visible, not only in a tooltip:
  // "since when has this been degraded?" is the question an operator asks
  // first, and DESIGN.md §7.4 forbids essential information living only on
  // hover (§7.7 asks for a caution label *and* time context).
  const healthSince = healthQuery.data ? formatRelative(healthQuery.data.time) : null

  /* One status for the whole console, not two.
     The realtime link and the backend health were separate blocks, and the
     header answered the question "is this thing working?" with two answers
     side by side, which is how a reader ends up reading the one they liked.
     They are now one chip that names the worst state: a console that cannot
     be trusted says so, and a console that can says one word. Everything
     behind the word is one hover away, and the word is what the operator
     reads. */
  const consoleState = !authenticated
    ? { label: 'NOT AUTHENTICATED', tone: 'aegis-crit' as const }
    : healthDown
      ? { label: 'SYSTEM DOWN', tone: 'aegis-crit' as const }
      : realtimeStatus === 'disconnected'
        ? { label: 'LINK LOST', tone: 'aegis-crit' as const }
        : realtimeStatus === 'reconnecting' || realtimeStatus === 'connecting'
          ? { label: 'LINK RETRY', tone: 'aegis-warn' as const }
          : healthDegraded
            ? { label: 'DEGRADED', tone: 'aegis-warn' as const }
            : realtimeStatus === 'connected' && healthOk
              ? { label: 'LIVE', tone: 'aegis-ok' as const }
              : { label: 'CHECKING', tone: 'aegis-faint' as const }
  // A degraded backend needs its time context visible, not only in a tooltip:
  // "since when has this been degraded?" is the question an operator asks
  // first, and DESIGN.md §7.4 forbids essential information living only on
  // hover (§7.7 asks for a caution label *and* time context). So the chip
  // carries it whenever the state is not a good one.
  const consoleStateLabel = `${consoleState.label}${
    healthDegraded || healthDown ? ` · ${healthSince ?? 'unknown'}` : ''
  }`
  const consoleStateDetail = [
    `realtime: ${WS_LABEL[realtimeStatus].toLowerCase()}`,
    healthQuery.data
      ? `service: ${healthQuery.data.status} · database: ${healthQuery.data.database}`
      : healthQuery.isError
        ? 'backend health check failed'
        : 'checking backend health…',
    lastEventAt ? `last event ${formatRelative(lastEventAt)}` : null,
  ]
    .filter(Boolean)
    .join(' · ')

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
      if (/^[1-9]$/.test(event.key)) {
        event.preventDefault()
        const view = ALL_VIEWS[Number(event.key) - 1]
        if (view) void navigate({ to: view.to as never })
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [helpOpen, navigate, toggleHelp])

  return (
    <>
      {/* One row, in the order DESIGN.md §1.3 ranks meaning: where you are,
          then whether you can trust it, then who you are. The brand
          descriptor moved to the footer, because a brand expression is the
          last thing the header should spend its height on. */}
      <div ref={shellRef} className="topbar-shell">
        <header ref={barRef} className="aegis-topbar">
        <div ref={brandRef} className="aegis-brand">
          <span className="aegis-glyph" aria-hidden="true">
            ع
          </span>
          <h1 dir="ltr">
            EMAD | <bdi lang="ar" dir="rtl">عِماد</bdi>
          </h1>
        </div>
        <span className="aegis-vsep" aria-hidden="true" />
        <nav ref={navRef} className="aegis-nav" aria-label="Console views">
          <span className="aegis-nav-primary" role="group" aria-label="Primary views">
            {PRIMARY_VIEWS.map((view) => (
              <Link
                key={view.to}
                to={view.to as never}
                activeProps={{ 'aria-current': 'page' }}
                title={`${view.label} (${view.index})`}
              >
                {view.label}
              </Link>
            ))}
          </span>
          {/* The side features live behind one disclosure. They are real
              views and every one of them is a keystroke away, but they are
              not what the console is for, and eight tabs in the header said
              otherwise. The trigger names the side feature you are actually
              on, so the disclosure never hides where you are. */}
          <Menu
            align="start"
            trigger={
              <button
                type="button"
                className="aegis-nav-more"
                aria-current={activeSideFeature ? 'page' : undefined}
                title={
                  activeSideFeature
                    ? `${activeSideFeature.label} — side feature (${activeSideFeature.index})`
                    : 'Side features'
                }
              >
                <span>{activeSideFeature ? activeSideFeature.label : 'More'}</span>
                <span className="caret" aria-hidden="true">
                  ▾
                </span>
              </button>
            }
            items={SECONDARY_VIEWS.map((view) => ({
              label: view.label,
              hint: String(view.index),
              onSelect: () => void navigate({ to: view.to as never }),
            }))}
          />
        </nav>
        <span className="aegis-vsep" aria-hidden="true" />
        <div ref={topRightRef} className="aegis-topright">
          {/* One chip for the whole console's state. */}
          <Tooltip content={consoleStateDetail}>
            <button
              type="button"
              className="aegis-console-state"
              onClick={() => void healthQuery.refetch()}
              title={`${consoleStateDetail} — click to re-check`}
            >
              <span
                className={cn(
                  'aegis-dot',
                  consoleState.tone === 'aegis-ok'
                    ? 'aegis-dot--live'
                    : consoleState.tone === 'aegis-warn'
                      ? 'aegis-dot--paused'
                      : 'aegis-dot--down',
                )}
                aria-hidden="true"
              />
              <span className={consoleState.tone}>{consoleStateLabel}</span>
            </button>
          </Tooltip>
          {/* The operator is an identity, and it is only shown when there is
              one. The console's state chip already says "NO TOKEN" when the
              session cannot be established, so the operator slot does not
              also have to report a condition wearing an identity's clothes. */}
          {authenticated ? (
            <Menu
              align="end"
              trigger={
                <button
                  type="button"
                  className="aegis-iconbtn aegis-operator"
                  title={`${operatorID} · ${operatorRole}`}
                  aria-label={`Signed in as ${operatorID}`}
                >
                  <UserIcon className="size-3.5" />
                  <span className="aegis-operator-role">{operatorRole.toUpperCase()}</span>
                </button>
              }
              items={[
                {
                  label: 'Copy operator ID',
                  onSelect: () => {
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
          ) : null}
          {/* The shortcut sheet is a help panel, not a feature. A button that
              says "Shortcuts" in the same weight as the operator and the
              status chip promoted it to the same rank. A glyph says the same
              thing in a quarter of the space. */}
          <button
            type="button"
            className="aegis-iconbtn aegis-helpbtn"
            aria-haspopup="dialog"
            aria-expanded={helpOpen}
            aria-label="Keyboard shortcuts and controls"
            onClick={toggleHelp}
            title="Keyboard shortcuts and controls (?)"
          >
            <span aria-hidden="true">?</span>
          </button>
        </div>
      </header>
      </div>

      {/* The scenario transport is not part of the shell. Scenarios are a side
          feature with their own console, and every page's chrome was
          mounting it — which made the system's own header depend on a
          subsystem that has nothing to do with reading the picture, and
          polled it every five seconds to do it. The picture carries it in
          its drawer, where the run that produced the picture is relevant. */}

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
                  this device; Reset layout in the picture drawer restores the defaults. The
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
