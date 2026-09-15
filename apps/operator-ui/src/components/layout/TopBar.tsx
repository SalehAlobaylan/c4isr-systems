import { useQuery } from '@tanstack/react-query'

import { ActivityIcon, SignalIcon, UserIcon } from '@/components/icons'
import { Button } from '@/components/ui/button'
import { Menu } from '@/components/ui/menu'
import { Tooltip } from '@/components/ui/tooltip'
import { OPERATOR_ID, fetchHealth, realtimeUrl } from '@/lib/api'
import { formatRelative } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { cn } from '@/lib/utils'
import { useRealtime, type RealtimeStatus } from '@/realtime/RealtimeProvider'
import { toast } from '@/stores/toasts'

const WS_LABEL: Record<RealtimeStatus, string> = {
  connected: 'LINK LIVE',
  connecting: 'LINK INIT',
  reconnecting: 'LINK RETRY',
  disconnected: 'LINK DOWN',
}

const WS_DOT: Record<RealtimeStatus, string> = {
  connected: 'bg-emerald-400 shadow-[0_0_6px_rgba(52,211,153,0.9)]',
  connecting: 'bg-amber-400 animate-pulse',
  reconnecting: 'bg-amber-400 animate-pulse',
  disconnected: 'bg-red-500',
}

export function TopBar() {
  const { status: realtimeStatus, lastEventAt } = useRealtime()

  const healthQuery = useQuery({
    queryKey: queryKeys.health(),
    queryFn: ({ signal }) => fetchHealth(signal),
    refetchInterval: 30_000,
    retry: 1,
  })

  const healthOk = healthQuery.isSuccess && healthQuery.data.status === 'ok'
  const healthDegraded = healthQuery.isSuccess && healthQuery.data.status !== 'ok'
  const healthDown = healthQuery.isError
  const healthLabel = healthOk
    ? 'SYS OK'
    : healthDegraded
      ? 'SYS DEGRADED'
      : healthDown
        ? 'SYS DOWN'
        : 'SYS …'

  return (
    <header className="flex h-14 shrink-0 items-center justify-between gap-4 border-b border-edge bg-panel px-4">
      <div className="flex min-w-0 items-center gap-3">
        <h1 className="truncate text-sm font-semibold tracking-widest text-ink uppercase">
          Common Operational Picture
        </h1>
        <span className="hidden font-mono text-[10px] tracking-wider text-ink-faint lg:inline">
          C4ISR / C2
        </span>
      </div>

      <div className="flex shrink-0 items-center gap-2">
        <Menu
          align="end"
          trigger={
            <button
              type="button"
              className="hidden cursor-pointer items-center gap-1.5 rounded-md border border-edge px-2.5 py-1.5 transition-colors hover:border-edge-strong hover:bg-panel-raised md:flex"
            >
              <UserIcon className="size-3.5 text-ink-faint" />
              <span className="font-mono text-[11px] text-ink-muted">{OPERATOR_ID}</span>
            </button>
          }
          items={[
            {
              label: 'Copy operator ID',
              onSelect: () => {
                void navigator.clipboard.writeText(OPERATOR_ID)
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

        <Tooltip
          content={
            healthQuery.data
              ? `database: ${healthQuery.data.database} · checked ${formatRelative(healthQuery.data.time)}`
              : healthQuery.isError
                ? 'Backend health check failed'
                : 'Checking backend health…'
          }
        >
          <div className="flex cursor-default items-center gap-1.5 rounded-md border border-edge px-2.5 py-1.5">
            <ActivityIcon
              className={cn(
                'size-3.5',
                healthOk
                  ? 'text-emerald-400'
                  : healthDown
                    ? 'text-red-400'
                    : healthDegraded
                      ? 'text-amber-400'
                      : 'text-ink-faint animate-pulse',
              )}
            />
            <span
              className={cn(
                'font-mono text-[10px] font-semibold tracking-wider',
                healthOk
                  ? 'text-emerald-300'
                  : healthDown
                    ? 'text-red-300'
                    : healthDegraded
                      ? 'text-amber-300'
                      : 'text-ink-faint',
              )}
            >
              {healthLabel}
            </span>
          </div>
        </Tooltip>

        <Tooltip
          content={
            lastEventAt
              ? `Last event ${formatRelative(lastEventAt)}`
              : 'Waiting for realtime events'
          }
        >
          <div className="flex cursor-default items-center gap-1.5 rounded-md border border-edge px-2.5 py-1.5">
            <span className={cn('size-2 rounded-full', WS_DOT[realtimeStatus])} />
            <SignalIcon
              className={cn(
                'size-3.5',
                realtimeStatus === 'connected' ? 'text-emerald-400' : 'text-ink-faint',
              )}
            />
            <span
              className={cn(
                'font-mono text-[10px] font-semibold tracking-wider',
                realtimeStatus === 'connected' ? 'text-emerald-300' : 'text-ink-muted',
              )}
            >
              {WS_LABEL[realtimeStatus]}
            </span>
          </div>
        </Tooltip>

        <Button
          variant="ghost"
          size="sm"
          onClick={() => void healthQuery.refetch()}
          title="Refresh health"
        >
          REFRESH
        </Button>
      </div>
    </header>
  )
}
