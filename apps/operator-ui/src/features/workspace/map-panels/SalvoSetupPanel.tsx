import { useEffect, useMemo, useRef, useState } from 'react'
import type { FeatureCollection } from 'geojson'
import { ExternalLinkIcon, RefreshIcon, ShieldAlertIcon, TrashIcon } from '@/components/icons'
import {
  SALVO_PRESETS,
  applySalvoPreset,
  buildSalvoOverlay,
  cloneSalvoDraft,
  fetchSalvoEnvironments,
  fetchSalvoHealth,
  fetchSalvoNetworks,
  fetchSalvoRun,
  fetchSalvoRuns,
  isPointInSalvoBounds,
  startLocalSalvoService,
  startSalvoRun,
  type SalvoBatteryTier,
  type SalvoEnvironment,
  type SalvoHealth,
  type SalvoNetworkItem,
  type SalvoPlacementTool,
  type SalvoPresetId,
  type SalvoRunJob,
  type SalvoSetupDraft,
} from './salvoClient'

interface SalvoSetupPanelProps {
  activeTool: SalvoPlacementTool
  onSelectTool: (tool: SalvoPlacementTool) => void
  onOverlayChange: (overlay: FeatureCollection) => void
  onFitBounds: (bounds: [number, number, number, number]) => void
  onNotice: (message: string) => void
  registerMapClick: (handler: ((point: [number, number]) => void) | null) => void
}

function generateSalvoJobId(): string {
  const hex =
    typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID().replace(/-/g, '').slice(0, 12)
      : Array.from({ length: 12 }, () => Math.floor(Math.random() * 16).toString(16)).join('')
  return `salvjob_${hex}`
}

function openUrlInNewTab(url: string): void {
  const a = document.createElement('a')
  a.href = url
  a.target = '_blank'
  a.rel = 'noopener noreferrer'
  a.style.display = 'none'
  document.body.appendChild(a)
  a.click()
  a.remove()
}

const TIER_META: Record<SalvoBatteryTier, { label: string; color: string; mag: number }> = {
  tier1_sam: { label: 'Tier-1 SAM ($1.2M)', color: '#38bdf8', mag: 6 },
  tier2_shorad: { label: 'Tier-2 SHORAD ($180k)', color: '#34d399', mag: 8 },
  tier3_point_defense: { label: 'Tier-3 PD ($18k)', color: '#fbbf24', mag: 16 },
}

export function SalvoSetupPanel({
  activeTool,
  onSelectTool,
  onOverlayChange,
  onFitBounds,
  onNotice,
  registerMapClick,
}: SalvoSetupPanelProps) {
  const [health, setHealth] = useState<SalvoHealth | null>(null)
  const [envs, setEnvs] = useState<SalvoEnvironment[]>([])
  const [networks, setNetworks] = useState<SalvoNetworkItem[]>([])
  const [draft, setDraft] = useState<SalvoSetupDraft | null>(null)
  const [runs, setRuns] = useState<SalvoRunJob[]>([])
  const [activeJob, setActiveJob] = useState<SalvoRunJob | null>(null)
  const [autoOpen3D, setAutoOpen3D] = useState(true)
  const [loading, setLoading] = useState(true)
  const [starting, setStarting] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const hasFitted = useRef(false)

  const selectedEnv = useMemo(
    () => envs.find((e) => e.id === draft?.environmentId) ?? envs[0],
    [envs, draft?.environmentId],
  )

  const refreshService = async (fitOnFirst = false, allowAutoStart = false) => {
    setLoading(true)
    setError('')
    try {
      const [h, envList, runList, netList] = await Promise.all([
        fetchSalvoHealth(),
        fetchSalvoEnvironments(),
        fetchSalvoRuns(),
        fetchSalvoNetworks().catch(() => [] as SalvoNetworkItem[]),
      ])
      setHealth(h)
      setEnvs(envList)
      setRuns(runList)
      setNetworks(netList)
      if (envList.length > 0) {
        if (fitOnFirst && !hasFitted.current) {
          hasFitted.current = true
          onFitBounds(envList[0].boundsWgs84)
        }
        setDraft((prev) =>
          prev && envList.some((e) => e.id === prev.environmentId) ? prev : cloneSalvoDraft(envList[0].defaults),
        )
      }
      if (runList.length > 0 && !activeJob) setActiveJob(runList[0])
    } catch (e) {
      if (allowAutoStart) {
        setStarting(true)
        const boot = await startLocalSalvoService()
        setStarting(false)
        if (boot.ok) {
          await refreshService(fitOnFirst, false)
          return
        }
        setHealth(null)
        setError(boot.error || 'Could not start Salvo Defense service automatically.')
      } else {
        setHealth(null)
        setError(e instanceof Error ? e.message : 'Local Salvo Defense service offline (expected :8767).')
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refreshService(true, true)
    return () => {
      registerMapClick(null)
      onSelectTool(null)
      onOverlayChange({ type: 'FeatureCollection', features: [] })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    const resOverlay = activeJob?.status === 'completed' ? activeJob.resultOverlay : null
    onOverlayChange(buildSalvoOverlay(selectedEnv, draft ?? undefined, resOverlay))
  }, [selectedEnv, draft, activeJob, onOverlayChange])

  useEffect(() => {
    if (!activeTool || !selectedEnv || !draft) {
      registerMapClick(null)
      return
    }
    registerMapClick(([lng, lat]) => {
      if (!isPointInSalvoBounds(lng, lat, selectedEnv.boundsWgs84)) {
        onNotice('Click inside the dashed amber environment boundary.')
        return
      }
      const rLng = Number(lng.toFixed(6))
      const rLat = Number(lat.toFixed(6))
      setDraft((prev) => {
        if (!prev) return prev
        if (activeTool === 'move-asset') {
          const next = prev.assets.map((a, idx) => (idx === 0 ? { ...a, lon: rLng, lat: rLat } : a))
          onNotice(`Moved defended hub to ${rLat.toFixed(4)}°, ${rLng.toFixed(4)}°`)
          return { ...prev, assets: next }
        }
        const tier: SalvoBatteryTier =
          activeTool === 'add-sam' ? 'tier1_sam' : activeTool === 'add-shorad' ? 'tier2_shorad' : 'tier3_point_defense'
        const meta = TIER_META[tier]
        const idx = prev.batteries.filter((b) => b.tier === tier).length + 1
        onNotice(`Placed ${meta.label} #${idx}`)
        return {
          ...prev,
          batteries: [
            ...prev.batteries,
            {
              id: `${tier.slice(0, 5)}-${Date.now().toString(36).slice(-3)}`,
              tier,
              label: `${meta.label.split(' (')[0]} ${idx}`,
              lon: rLng,
              lat: rLat,
              magazine: meta.mag,
            },
          ],
        }
      })
      if (activeTool === 'move-asset') onSelectTool(null)
    })
    return () => registerMapClick(null)
  }, [activeTool, selectedEnv, draft, registerMapClick, onSelectTool, onNotice])

  useEffect(() => {
    if (!activeJob || activeJob.status !== 'running') return
    let cancelled = false
    const timer = window.setInterval(() => {
      void fetchSalvoRun(activeJob.jobId)
        .then((u) => {
          if (cancelled) return
          setActiveJob(u)
          setRuns((prev) => prev.map((x) => (x.jobId === u.jobId ? u : x)))
          if (u.status !== 'running') {
            window.clearInterval(timer)
            void fetchSalvoRuns().then((l) => { if (!cancelled) setRuns(l) })
          }
        })
        .catch(() => {})
    }, 350)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [activeJob?.jobId, activeJob?.status])

  const buildViewerHref = (job: SalvoRunJob) => {
    const url = new URL(job.viewerUrl)
    url.searchParams.set('mapOrigin', window.location.origin)
    return url.toString()
  }

  const handleRun = async () => {
    if (!draft || !health) return
    setError('')
    setSubmitting(true)
    onSelectTool(null)
    const jobId = generateSalvoJobId()
    const href = `${health.viewerOrigin}/?run=${encodeURIComponent(jobId)}&mapOrigin=${encodeURIComponent(window.location.origin)}`
    if (autoOpen3D) openUrlInNewTab(href)
    try {
      const job = await startSalvoRun(draft, jobId)
      setActiveJob(job)
      setRuns((prev) => [job, ...prev.filter((x) => x.jobId !== job.jobId)])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to start salvo simulation.')
    } finally {
      setSubmitting(false)
    }
  }

  const envNetworks = useMemo(
    () => networks.filter((n) => !draft || n.environmentId === draft.environmentId),
    [networks, draft],
  )

  return (
    <div className="map-drawer-pane">
      {/* 1. Service & Verified Biome */}
      <div className="map-card">
        <div className="map-panel-section-head">
          <h3>1. Salvo Defense &amp; Biome</h3>
          <button type="button" className="ux-button" onClick={() => void refreshService(false)} disabled={loading}>
            <RefreshIcon className="size-3.5" />
          </button>
        </div>
        {health ? (
          <div className="flex items-center justify-between gap-2 text-xs">
            <span className="ux-muted">
              <strong>Ready</strong> · <code>{health.engineVersion.replace('emad-salvo-py-', 'v')}</code>
              {health.aiPolicyAvailable ? ' · AI Policy ON' : ''}
            </span>
            <a
              className="ux-button"
              href={`${health.viewerOrigin}/?mapOrigin=${encodeURIComponent(window.location.origin)}`}
              target="_blank"
              rel="noreferrer"
            >
              Open 3D ↗
            </a>
          </div>
        ) : starting || loading ? (
          <p className="ux-muted text-xs">Starting local Salvo Defense service on <code>127.0.0.1:8767</code>…</p>
        ) : (
          <div className="ux-notice space-y-2 text-xs" role="alert">
            <p><strong>Local Salvo Defense service offline.</strong></p>
            {error && <p>{error}</p>}
            <button
              type="button"
              className="ux-button"
              data-primary="true"
              onClick={async () => {
                setStarting(true)
                const b = await startLocalSalvoService()
                setStarting(false)
                if (b.ok) await refreshService(true, false)
                else setError(b.error || 'Start failed')
              }}
            >
              Start Salvo Service
            </button>
            <p className="ux-muted">Manual fallback: <code>task dev:salvo</code></p>
          </div>
        )}

        {envs.length > 0 && draft && selectedEnv && (
          <>
            <label className="block">
              <span>Verified geographic biome</span>
              <select
                className="atlas-field"
                value={draft.environmentId}
                onChange={(e) => {
                  const next = envs.find((x) => x.id === e.target.value)
                  if (!next) return
                  setDraft(cloneSalvoDraft(next.defaults))
                  onFitBounds(next.boundsWgs84)
                }}
              >
                {envs.map((env) => (
                  <option key={env.id} value={env.id}>{env.title}</option>
                ))}
              </select>
            </label>
            <div className="flex flex-wrap gap-1.5">
              <button type="button" className="ux-button" onClick={() => onFitBounds(selectedEnv.boundsWgs84)}>
                Zoom to biome
              </button>
              <button
                type="button"
                className="ux-button"
                onClick={() => setDraft(cloneSalvoDraft(selectedEnv.defaults))}
              >
                Reset
              </button>
            </div>
          </>
        )}
      </div>

      {draft && selectedEnv && (
        <>
          {/* 2. Quick Salvo Presets & Network Import */}
          <div className="map-card">
            <div className="map-panel-section-head"><h3>2. Tactical Raid Presets</h3></div>
            <div className="grid grid-cols-2 gap-1.5">
              {SALVO_PRESETS.map((p) => (
                <button
                  key={p.id}
                  type="button"
                  className="ux-button text-left"
                  data-primary={draft.presetId === p.id ? 'true' : undefined}
                  title={p.summary}
                  onClick={() => {
                    setDraft(applySalvoPreset(selectedEnv, p.id as SalvoPresetId))
                    onNotice(`Loaded Salvo preset: ${p.label}`)
                  }}
                >
                  {p.label}
                </button>
              ))}
            </div>

            {envNetworks.length > 0 && (
              <label className="block pt-1">
                <span>Import Sensor Network (from Coverage)</span>
                <select
                  className="atlas-field"
                  value={draft.sensorNetworkId ?? ''}
                  onChange={(e) => {
                    const id = e.target.value || null
                    const net = envNetworks.find((x) => x.layoutId === id)
                    setDraft({
                      ...draft,
                      sensorNetworkId: id,
                      sensors: net ? net.sensors : selectedEnv.defaults.sensors,
                    })
                    if (net) onNotice(`Imported ${net.sensorCount} sensors from "${net.name}"`)
                  }}
                >
                  <option value="">— Default Radar + EO/IR —</option>
                  {envNetworks.map((n) => (
                    <option key={n.layoutId} value={n.layoutId}>{n.name} ({n.sensorCount} sensors)</option>
                  ))}
                </select>
              </label>
            )}
          </div>

          {/* 3. Layered Defense Batteries */}
          <div className="map-card">
            <div className="map-panel-section-head">
              <h3>3. Layered Batteries ({draft.batteries.length})</h3>
              {activeTool && <button type="button" className="ux-button" onClick={() => onSelectTool(null)}>Done</button>}
            </div>
            <div className="flex flex-wrap gap-1.5">
              {(
                [
                  ['add-sam', '+ Tier-1 SAM'],
                  ['add-shorad', '+ Tier-2 SHORAD'],
                  ['add-pd', '+ Tier-3 PD'],
                  ['move-asset', 'Move Hub'],
                ] as [SalvoPlacementTool, string][]
              ).map(([tid, label]) => (
                <button
                  key={tid}
                  type="button"
                  className="ux-button"
                  data-primary={activeTool === tid ? 'true' : undefined}
                  onClick={() => onSelectTool(activeTool === tid ? null : tid)}
                >
                  {label}
                </button>
              ))}
            </div>
            <div className="space-y-1 pt-1">
              {draft.batteries.map((b) => (
                <div key={b.id} className="flex items-center justify-between gap-2 rounded border border-slate-300/60 px-2 py-1 text-xs">
                  <span className="inline-flex items-center gap-1.5 truncate font-medium">
                    <span className="size-2.5 rounded-full shrink-0" style={{ background: TIER_META[b.tier].color }} />
                    {b.label}
                  </span>
                  <div className="flex items-center gap-1">
                    <input
                      type="number"
                      className="atlas-field w-14 py-0.5 text-right font-mono"
                      min={1}
                      max={40}
                      value={b.magazine}
                      title="Magazine rounds"
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          batteries: draft.batteries.map((x) =>
                            x.id === b.id ? { ...x, magazine: Math.max(1, Number(e.target.value) || 4) } : x,
                          ),
                        })
                      }
                    />
                    <button
                      type="button"
                      className="ux-button"
                      disabled={draft.batteries.length <= 1}
                      onClick={() => setDraft({ ...draft, batteries: draft.batteries.filter((x) => x.id !== b.id) })}
                    >
                      <TrashIcon className="size-3" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* 4. Raid Wave Composition */}
          <div className="map-card">
            <div className="map-panel-section-head"><h3>4. Mixed Threat Raid Composition</h3></div>
            <div className="grid grid-cols-2 gap-2">
              <label className="block">
                <span>RF Decoys</span>
                <input
                  type="number"
                  className="atlas-field font-mono"
                  min={0}
                  max={12}
                  value={draft.raid.decoys}
                  onChange={(e) => setDraft({ ...draft, raid: { ...draft.raid, decoys: Math.max(0, Number(e.target.value) || 0) } })}
                />
              </label>
              <label className="block">
                <span>One-Way UAVs</span>
                <input
                  type="number"
                  className="atlas-field font-mono"
                  min={0}
                  max={16}
                  value={draft.raid.uavs}
                  onChange={(e) => setDraft({ ...draft, raid: { ...draft.raid, uavs: Math.max(0, Number(e.target.value) || 0) } })}
                />
              </label>
              <label className="block">
                <span>Cruise Missiles</span>
                <input
                  type="number"
                  className="atlas-field font-mono"
                  min={0}
                  max={10}
                  value={draft.raid.cruiseMissiles}
                  onChange={(e) => setDraft({ ...draft, raid: { ...draft.raid, cruiseMissiles: Math.max(0, Number(e.target.value) || 0) } })}
                />
              </label>
              <label className="block">
                <span>Ballistic Missiles</span>
                <input
                  type="number"
                  className="atlas-field font-mono"
                  min={0}
                  max={6}
                  value={draft.raid.ballisticMissiles}
                  onChange={(e) => setDraft({ ...draft, raid: { ...draft.raid, ballisticMissiles: Math.max(0, Number(e.target.value) || 0) } })}
                />
              </label>
              <label className="block">
                <span>EO/IR Vis (km)</span>
                <input
                  type="number"
                  className="atlas-field font-mono"
                  min={0.5}
                  max={30}
                  step={0.5}
                  value={draft.visibilityKm}
                  onChange={(e) => setDraft({ ...draft, visibilityKm: Number(e.target.value) || 10 })}
                />
              </label>
              <label className="block">
                <span>Seed</span>
                <input
                  type="number"
                  className="atlas-field font-mono"
                  value={draft.seed}
                  onChange={(e) => setDraft({ ...draft, seed: Number(e.target.value) || 1 })}
                />
              </label>
            </div>
            <label className="flex items-center gap-2 text-xs pt-1">
              <input
                type="checkbox"
                checked={draft.enableCivilianTraffic}
                onChange={(e) => setDraft({ ...draft, enableCivilianTraffic: e.target.checked })}
              />
              <span>Freeze civilian airliners (1.5 km HOLD_FIRE airspace)</span>
            </label>
          </div>

          {/* 5. Run & Compare Policies */}
          <div className="map-card">
            <div className="map-panel-section-head"><h3>5. Run Layered Defense Salvo</h3></div>
            {error && <p className="ux-notice text-xs" role="alert">{error}</p>}
            <label className="flex items-center gap-2 text-xs">
              <input type="checkbox" checked={autoOpen3D} onChange={(e) => setAutoOpen3D(e.target.checked)} />
              <span>Open 3D Salvo Workbench in a new tab</span>
            </label>
            <button
              type="button"
              className="ux-button w-full justify-center"
              data-primary="true"
              disabled={!health || submitting}
              onClick={() => void handleRun()}
            >
              <ShieldAlertIcon className="size-4" />
              {submitting ? 'Simulating Multi-Threat Salvo…' : 'Run Salvo & Open 3D Workbench'}
              <ExternalLinkIcon className="size-3.5" />
            </button>

            {runs.length > 1 && (
              <label className="block pt-1">
                <span>Recent salvo runs ({runs.length})</span>
                <select
                  className="atlas-field font-mono text-xs"
                  value={activeJob?.jobId ?? ''}
                  onChange={(e) => {
                    const found = runs.find((r) => r.jobId === e.target.value)
                    if (found) setActiveJob(found)
                  }}
                >
                  {runs.slice(0, 8).map((r) => (
                    <option key={r.jobId} value={r.jobId}>
                      {r.jobId} · {r.environmentId} ({r.status})
                    </option>
                  ))}
                </select>
              </label>
            )}

            {activeJob && (
              <div className="space-y-2 pt-2 border-t border-slate-300/40 text-xs">
                <div className="flex items-center justify-between gap-2">
                  <strong>{activeJob.status === 'completed' ? `Done (${activeJob.elapsedS ?? '—'}s)` : activeJob.stage}</strong>
                  {activeJob.status === 'completed' && (
                    <a className="ux-button" href={buildViewerHref(activeJob)} target="_blank" rel="noreferrer">
                      Open 3D ↗
                    </a>
                  )}
                </div>
                {activeJob.comparison?.wtaVsGreedy && (
                  <div className="rounded border border-emerald-500/40 bg-emerald-500/10 p-2 space-y-1">
                    <div>
                      <strong>WTA vs Greedy:</strong> Score{' '}
                      <strong className="text-emerald-700">
                        {activeJob.comparison.wtaVsGreedy.scoreDelta >= 0 ? '+' : ''}
                        {activeJob.comparison.wtaVsGreedy.scoreDelta}
                      </strong>{' '}
                      · Saved <strong>${activeJob.comparison.wtaVsGreedy.costSavedKUsd.toLocaleString()}k</strong> (
                      {activeJob.comparison.wtaVsGreedy.costSavedPct}%)
                    </div>
                  </div>
                )}
                {activeJob.comparison?.policies && (
                  <div className="space-y-1 font-mono text-[11px]">
                    {Object.entries(activeJob.comparison.policies).map(([pol, row]) => (
                      <div key={pol} className="flex items-center justify-between gap-2 rounded border border-slate-300/50 px-2 py-1">
                        <span className="font-semibold">{pol.replace('_', ' ')}</span>
                        <span>
                          {row.compositeScore}pt · {row.leakers}L · ${row.totalCostKUsd}k · {row.decoyShotsWasted}dec
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  )
}
