import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api, type ScenarioRun } from '@/lib/api'
import { formatVirtualTime } from '@/lib/format'
import { queryKeys } from '@/lib/queryKeys'
import { cn } from '@/lib/utils'
import { useLayoutStore } from '@/stores/layout'
import { toast } from '@/stores/toasts'

const SPEED_OPTIONS = [
  { value: '1', label: '1×' },
  { value: '2', label: '2×' },
  { value: '5', label: '5×' },
  { value: '10', label: '10×' },
  { value: '50', label: '50×' },
]

const RUNS_FILTER = { limit: 20 } as const

const ACTIVE_STATES = new Set(['RUNNING', 'PAUSED'])

export function ScenarioBar() {
  const queryClient = useQueryClient()
  const runOpen = useLayoutStore((state) => state.runOpen)
  const toggle = useLayoutStore((state) => state.toggle)

  const runsQuery = useQuery({
    queryKey: queryKeys.scenarios.runs(RUNS_FILTER),
    queryFn: () => api.listScenarioRuns(RUNS_FILTER),
    refetchInterval: 5_000,
  })

  const scenariosQuery = useQuery({
    queryKey: queryKeys.scenarios.list(),
    queryFn: () => api.listScenarios(),
  })

  const [startName, setStartName] = useState('')

  const scenarios = scenariosQuery.data?.items ?? []

  const startMutation = useMutation({
    mutationFn: ({ name, speed, seed }: { name: string; speed: number; seed?: number }) =>
      api.startScenario(name, { speed, seed }),
    onSuccess: (run) => {
      toast({
        title: 'Scenario started',
        description: `${run.scenarioName} @ ${run.playbackSpeed}× · seed ${run.seed}`,
        variant: 'success',
      })
      invalidate()
    },
    onError: (error) => {
      toast({
        title: 'Scenario start failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  const activeRun = useMemo(
    () => (runsQuery.data?.items ?? []).find((run) => ACTIVE_STATES.has(run.status)) ?? null,
    [runsQuery.data],
  )

  const invalidate = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.scenarios.all })
  }, [queryClient])

  const runAction = useMutation({
    mutationFn: ({ id, action }: { id: string; action: 'pause' | 'resume' }) =>
      action === 'pause' ? api.pauseScenarioRun(id) : api.resumeScenarioRun(id),
    onSuccess: (run) => {
      toast({
        title: run.status === 'PAUSED' ? 'Scenario paused' : 'Scenario resumed',
        description: run.scenarioName,
        variant: 'success',
      })
      invalidate()
    },
    onError: (error) => {
      toast({
        title: 'Run control failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
      invalidate()
    },
  })

  const replayMutation = useMutation({
    mutationFn: async (run: ScenarioRun) => {
      await api.stopScenarioRun(run.id)
      return api.startScenario(run.scenarioName, {
        speed: run.playbackSpeed || 1,
        seed: run.seed,
      })
    },
    onSuccess: (run) => {
      toast({
        title: 'Replay started',
        description: `${run.scenarioName} @ ${run.playbackSpeed}× · seed ${run.seed}`,
        variant: 'success',
      })
      invalidate()
    },
    onError: (error) => {
      toast({
        title: 'Replay failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
      invalidate()
    },
  })

  const speedMutation = useMutation({
    mutationFn: ({ id, speed }: { id: string; speed: number }) =>
      api.setScenarioRunSpeed(id, speed),
    onSuccess: (run) => {
      toast({ title: `Playback speed ${run.playbackSpeed}×`, description: run.scenarioName })
      invalidate()
    },
    onError: (error) => {
      toast({
        title: 'Speed change failed',
        description: error instanceof Error ? error.message : undefined,
        variant: 'error',
      })
    },
  })

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      const tag = target?.tagName.toLowerCase() ?? ''
      if (
        tag === 'input' ||
        tag === 'select' ||
        tag === 'textarea' ||
        target?.isContentEditable ||
        target?.closest('dialog') ||
        document.querySelector('[role="dialog"][aria-modal="true"]')
      ) {
        return
      }
      if (event.metaKey || event.ctrlKey || event.altKey) return
      if (event.key === ' ' && !target?.closest('button, a, summary, [role="tab"]')) {
        event.preventDefault()
        startPauseResume()
      } else if (event.key.toLowerCase() === 'r' && activeRun) {
        event.preventDefault()
        replayMutation.mutate(activeRun)
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  })

  const startPauseResume = () => {
    if (!activeRun) return
    const action = activeRun.status === 'RUNNING' ? 'pause' : 'resume'
    runAction.mutate({ id: activeRun.id, action })
  }

  const statusLabel = activeRun ? (activeRun.status === 'PAUSED' ? 'PAUSED' : 'LIVE') : 'IDLE'

  return (
    <div className="aegis-band" data-od-id="run-bar">
      <div className="aegis-brow">
        <button
          type="button"
          className="aegis-dsc"
          aria-expanded={runOpen}
          aria-controls="pane-run"
          onClick={() => toggle('runOpen')}
          title="Show playback controls"
        >
          Scenario
        </button>
        <span className="aegis-scenario">
          {activeRun ? `${activeRun.scenarioName} · seed ${activeRun.seed}` : 'No active run'}
        </span>
        <span className="aegis-clock" id="aegis-run-clock">
          {activeRun
            ? `T+${formatVirtualTime(activeRun.virtualTimeMs)} · ${activeRun.playbackSpeed}×`
            : 'T+0:00 · —'}
        </span>
        <span className="aegis-live aegis-run-state">
          <span className={cn('aegis-dot', activeRun?.status === 'PAUSED' ? 'aegis-dot--paused' : 'aegis-dot--live')} aria-hidden="true" />
          {statusLabel}
        </span>
        <span className="aegis-spacer" />
        <span className="aegis-grp">
          <button
            type="button"
            id="aegis-btnPlay"
            aria-pressed={activeRun?.status === 'PAUSED'}
            disabled={!activeRun || runAction.isPending}
            onClick={startPauseResume}
            title="Pause or resume the scenario clock (Space)"
          >
            {activeRun?.status === 'PAUSED' ? 'Resume' : 'Pause'}
          </button>
          <button
            type="button"
            id="aegis-btnReplay"
            disabled={!activeRun || replayMutation.isPending}
            onClick={() => activeRun && replayMutation.mutate(activeRun)}
            title="Replay — restart the active run with the same seed (R)"
          >
            Replay
          </button>
          {scenarios.length > 0 ? (
            <select
              aria-label="Start a scenario"
              value={startName}
              disabled={startMutation.isPending}
              onChange={(event) => {
                const name = event.target.value
                setStartName('')
                if (name) startMutation.mutate({ name, speed: 1 })
              }}
              title="Start a scenario run at 1×"
            >
              <option value="">Start scenario…</option>
              {scenarios.map((scenario) => (
                <option key={scenario.name} value={scenario.name}>
                  {scenario.name}
                </option>
              ))}
            </select>
          ) : null}
          <Link to="/scenarios" className="aegis-band-link" title="Open the scenario console">
            Scenario console
          </Link>
        </span>
      </div>
      {runOpen ? (
        <div className="aegis-brow sub" id="pane-run">
          <span className="aegis-grp">
            <span className="aegis-lbl">Playback</span>
            <select
              aria-label="Playback speed"
              value={activeRun ? String(activeRun.playbackSpeed) : ''}
              disabled={!activeRun || speedMutation.isPending}
              onChange={(event) => {
                if (!activeRun) return
                speedMutation.mutate({ id: activeRun.id, speed: Number(event.target.value) })
              }}
            >
              {!activeRun ? <option value="">—</option> : null}
              {SPEED_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </span>
          <span className="aegis-grp">
            <span className="aegis-lbl">Inject</span>
            <select
              aria-label="Inject edge case"
              value=""
              disabled
              title="Scenario edge-case injection is configured at run start — see the scenario console"
              onChange={() => undefined}
            >
              <option value="">Edge-case injection at start</option>
            </select>
          </span>
          <span className="aegis-spacer" />
          <span className="aegis-lbl">Layout is remembered on this device</span>
          <button type="button" className="aegis-iconbtn" onClick={() => useLayoutStore.getState().reset()}>
            Reset layout
          </button>
        </div>
      ) : null}
    </div>
  )
}
