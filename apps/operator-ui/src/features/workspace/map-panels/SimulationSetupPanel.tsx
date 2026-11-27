import { useEffect, useMemo, useRef, useState } from 'react'
import type { FeatureCollection } from 'geojson'
import { ExternalLinkIcon, RadarIcon, RefreshIcon } from '@/components/icons'
import {
  SIM_TACTICAL_PRESETS,
  applyTacticalPreset,
  buildSimViewerHref,
  buildSimulationOverlay,
  draftFromEnvironment,
  fetchSimEnvironments,
  fetchSimHealth,
  fetchSimNetworks,
  fetchSimRuns,
  isPointInEnvBounds,
  startLocalSimService,
  startSimRun,
  subscribeSimRun,
  type SharedSensorNetwork,
  type SimEnvironment,
  type SimHealth,
  type SimPlacementTool,
  type SimRunJob,
  type SimRunMode,
  type SimSetupDraft,
  type SimTacticalPresetId,
} from './simClient'

interface SimulationSetupPanelProps {
  activeTool: SimPlacementTool
  onSelectTool: (tool: SimPlacementTool) => void
  onOverlayChange: (overlay: FeatureCollection) => void
  onFitBounds: (bounds: [number, number, number, number]) => void
  onNotice: (message: string) => void
  registerMapClick: (handler: ((point: [number, number]) => void) | null) => void
}

function generateJobId(): string {
  const hex =
    typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID().replace(/-/g, '').slice(0, 12)
      : Array.from({ length: 12 }, () => Math.floor(Math.random() * 16).toString(16)).join('')
  return `simjob_${hex}`
}

function openUrlInNewTab(url: string): boolean {
  const link = document.createElement('a')
  link.href = url
  link.target = '_blank'
  link.rel = 'noopener noreferrer'
  link.style.display = 'none'
  document.body.appendChild(link)
  link.click()
  link.remove()
  return true
}

export function SimulationSetupPanel({
  activeTool,
  onSelectTool,
  onOverlayChange,
  onFitBounds,
  onNotice,
  registerMapClick,
}: SimulationSetupPanelProps) {
  const [health, setHealth] = useState<SimHealth | null>(null)
  const [envs, setEnvs] = useState<SimEnvironment[]>([])
  const [draft, setDraft] = useState<SimSetupDraft | null>(null)
  const [runs, setRuns] = useState<SimRunJob[]>([])
  const [activeJob, setActiveJob] = useState<SimRunJob | null>(null)
  const [showResultOverlay, setShowResultOverlay] = useState(true)
  const [autoOpen3D, setAutoOpen3D] = useState(true)
  const [popupBlockedUrl, setPopupBlockedUrl] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [startingService, setStartingService] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [defaultWaypointAgl, setDefaultWaypointAgl] = useState(140)
  const [activePreset, setActivePreset] = useState<SimTacticalPresetId>('default_uav')
  const [networks, setNetworks] = useState<SharedSensorNetwork[]>([])
  const hasFittedInitial = useRef(false)

  const selectedEnv = useMemo(
    () => envs.find((e) => e.id === draft?.environmentId) ?? envs[0],
    [envs, draft?.environmentId],
  )

  const refreshService = async (fitOnFirstLoad = false, allowAutoStart = false) => {
    setLoading(true)
    setError('')
    try {
      const [h, envList, runList, netList] = await Promise.all([
        fetchSimHealth(),
        fetchSimEnvironments(),
        fetchSimRuns(),
        fetchSimNetworks().catch(() => [] as SharedSensorNetwork[]),
      ])
      setHealth(h)
      setEnvs(envList)
      setRuns(runList)
      setNetworks(netList)
      if (envList.length > 0) {
        if (fitOnFirstLoad && !hasFittedInitial.current) {
          hasFittedInitial.current = true
          onFitBounds(envList[0].boundsWgs84)
        }
        setDraft((prev) =>
          prev && envList.some((e) => e.id === prev.environmentId) ? prev : draftFromEnvironment(envList[0]),
        )
      }
      if (runList.length > 0 && !activeJob) {
        setActiveJob(runList[0])
      }
    } catch (e) {
      if (allowAutoStart) {
        setStartingService(true)
        const boot = await startLocalSimService()
        setStartingService(false)
        if (boot.ok) {
          await refreshService(fitOnFirstLoad, false)
          return
        }
        setHealth(null)
        setError(boot.error || 'Could not start local 3D simulation service automatically.')
      } else {
        setHealth(null)
        setError(
          e instanceof Error
            ? e.message
            : 'Local 3D simulation service is offline (expected at http://127.0.0.1:8765).',
        )
      }
    } finally {
      setLoading(false)
    }
  }

  const handleStartService = async () => {
    setStartingService(true)
    setError('')
    const boot = await startLocalSimService()
    setStartingService(false)
    if (boot.ok) {
      await refreshService(true, false)
    } else {
      setError(boot.error || 'Could not start local 3D simulation service.')
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

  // Keep 2D MapLibre overlay synchronized with draft setup + optional completed result footprint
  useEffect(() => {
    const result = showResultOverlay && activeJob?.status === 'completed' ? activeJob.resultOverlay : null
    onOverlayChange(buildSimulationOverlay(selectedEnv, draft ?? undefined, result))
  }, [selectedEnv, draft, activeJob, showResultOverlay, onOverlayChange])

  // Register interactive map click handler whenever a placement tool is active
  useEffect(() => {
    if (!activeTool || !selectedEnv || !draft) {
      registerMapClick(null)
      return
    }
    registerMapClick(([lng, lat]) => {
      if (!isPointInEnvBounds(lng, lat, selectedEnv.boundsWgs84)) {
        onNotice('Click inside the dashed amber environment boundary on the map.')
        return
      }
      const rLng = Number(lng.toFixed(6))
      const rLat = Number(lat.toFixed(6))
      setDraft((prev) => {
        if (!prev) return prev
        if (activeTool === 'radar') {
          onNotice(`Radar placed at ${rLat.toFixed(5)}°, ${rLng.toFixed(5)}°`)
          return { ...prev, radar: { ...prev.radar, lon: rLng, lat: rLat } }
        }
        if (activeTool === 'launcher') {
          onNotice(`Launcher placed at ${rLat.toFixed(5)}°, ${rLng.toFixed(5)}°`)
          return { ...prev, launcher: { ...prev.launcher, lon: rLng, lat: rLat } }
        }
        if (activeTool === 'protected') {
          onNotice(`Protected zone centered at ${rLat.toFixed(5)}°, ${rLng.toFixed(5)}°`)
          return { ...prev, protectedZone: { ...prev.protectedZone, lon: rLng, lat: rLat } }
        }
        if (activeTool === 'threat') {
          const next = [...prev.threat.waypoints, [rLng, rLat, defaultWaypointAgl] as [number, number, number]]
          onNotice(`Added threat waypoint T${next.length} (${defaultWaypointAgl} m AGL)`)
          return { ...prev, threat: { ...prev.threat, waypoints: next } }
        }
        return prev
      })
      if (activeTool !== 'threat') {
        onSelectTool(null)
      }
    })
    return () => registerMapClick(null)
  }, [activeTool, selectedEnv, draft, defaultWaypointAgl, registerMapClick, onSelectTool, onNotice])

  // Stream active job updates via Go SSE (with automatic polling fallback in zero-Docker mode)
  useEffect(() => {
    if (!activeJob || (activeJob.status !== 'running' && activeJob.status !== 'queued')) return
    let cancelled = false
    const unsub = subscribeSimRun(
      activeJob.jobId,
      (updated) => {
        if (cancelled) return
        setActiveJob(updated)
        setRuns((prev) => prev.map((r) => (r.jobId === updated.jobId ? updated : r)))
      },
      () => {
        void fetchSimRuns().then((list) => {
          if (!cancelled) setRuns(list)
        })
      },
    )
    return () => {
      cancelled = true
      unsub()
    }
  }, [activeJob?.jobId, activeJob?.status])

  const buildViewerHref = (job: SimRunJob) => buildSimViewerHref(job.viewerUrl)

  const handleRun = async () => {
    if (!draft || !health) return
    if (draft.threat.waypoints.length < 2) {
      setError('Add at least 2 threat waypoints on the map before running.')
      return
    }
    setError('')
    setPopupBlockedUrl(null)
    setSubmitting(true)
    onSelectTool(null)

    const jobId = generateJobId()
    const targetUrl = `${health.viewerOrigin}/?run=${encodeURIComponent(jobId)}&mapOrigin=${encodeURIComponent(window.location.origin)}`

    // Trigger a native <a target="_blank"> click synchronously inside the user gesture
    // so the browser immediately opens the real 3D simulation tab without popup blocking!
    if (autoOpen3D) {
      openUrlInNewTab(targetUrl)
    }

    try {
      const job = await startSimRun(draft, jobId)
      setActiveJob(job)
      setRuns((prev) => [job, ...prev.filter((x) => x.jobId !== job.jobId)])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Simulation failed to start.')
    } finally {
      setSubmitting(false)
    }
  }

  const handleSelectEnvironment = (envId: string) => {
    const next = envs.find((e) => e.id === envId)
    if (!next) return
    onSelectTool(null)
    setActivePreset('default_uav')
    setDraft(draftFromEnvironment(next))
    onFitBounds(next.boundsWgs84)
  }

  const handleApplyPreset = (presetId: SimTacticalPresetId) => {
    if (!selectedEnv) return
    onSelectTool(null)
    setActivePreset(presetId)
    setDraft(applyTacticalPreset(selectedEnv, presetId))
    const meta = SIM_TACTICAL_PRESETS.find((p) => p.id === presetId)
    if (meta) onNotice(`Loaded preset: ${meta.label} — ${meta.summary}`)
  }

  const handleThreatKindChange = (kind: string) => {
    if (!draft) return
    const speedMps = kind === 'ballistic_missile' ? 340 : kind === 'cruise_missile' ? 230 : 68
    const interceptorSpeedMps = kind === 'ballistic_missile' ? 500 : kind === 'cruise_missile' ? 380 : 295
    setDraft({
      ...draft,
      threat: {
        ...draft.threat,
        kind,
        speedMps,
        ballisticApogeeM: kind === 'ballistic_missile' ? (draft.threat.ballisticApogeeM ?? 1500) : draft.threat.ballisticApogeeM,
      },
      launcher: {
        ...draft.launcher,
        speedMps: interceptorSpeedMps,
      },
    })
  }

  const handleExportDraft = () => {
    if (!draft) return
    const blob = new Blob([JSON.stringify(draft, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${draft.environmentId}-${draft.threat.kind}-setup.json`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    onNotice('Exported scenario draft JSON.')
  }

  return (
    <div className="map-drawer-pane">
      {/* Service Status & Environment */}
      <div className="map-card">
        <div className="map-panel-section-head">
          <h3>1. Verified Environment</h3>
          <button
            type="button"
            className="ux-button"
            onClick={() => void refreshService(false)}
            disabled={loading}
            title="Reconnect to local simulation service"
          >
            <RefreshIcon className="size-3.5" />
          </button>
        </div>

        {health ? (
          <div className="space-y-1">
            <div className="flex items-center justify-between gap-2">
              <p className="ux-muted" role="status">
                <strong>Engine ready</strong> · <code>{health.engineVersion.replace('emad-sim-py-', 'v')}</code> ·{' '}
                {health.viewerOrigin}
              </p>
              <a
                className="ux-button"
                href={`${health.viewerOrigin}/?mapOrigin=${encodeURIComponent(window.location.origin)}`}
                target="_blank"
                rel="noreferrer"
              >
                Open 3D ↗
              </a>
            </div>
            {health.engineVersion === 'emad-sim-py-0.1.0' && (
              <p className="text-xs ux-notice">
                Older engine <code>v0.1.0</code> is still running — restart <code>task dev</code> (or{' '}
                <code>task dev:sim</code>) to activate <code>v0.2.0</code> ballistic arcs, kinetic hazard energy, and
                optimizer windows.
              </p>
            )}
          </div>
        ) : startingService || loading ? (
          <p className="ux-muted" role="status">
            Starting local 3D simulation engine on <code>127.0.0.1:8765</code>…
          </p>
        ) : (
          <div className="ux-notice space-y-2" role="alert">
            <p>
              <strong>Local 3D simulation service is offline.</strong>
            </p>
            {error && <p className="text-xs">{error}</p>}
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                className="ux-button"
                data-primary="true"
                disabled={startingService}
                onClick={() => void handleStartService()}
              >
                Start 3D Simulation Service
              </button>
              <button
                type="button"
                className="ux-button"
                onClick={() => void refreshService(true, false)}
              >
                Retry connection
              </button>
            </div>
            <p className="text-xs ux-muted">
              Manual fallback: <code>task dev:sim-hub</code>
            </p>
          </div>
        )}

        {envs.length > 0 && draft && selectedEnv && (
          <>
            <label className="block">
              <span>Prepared geographic environment</span>
              <select
                className="atlas-field"
                value={draft.environmentId}
                onChange={(e) => handleSelectEnvironment(e.target.value)}
              >
                {envs.map((env) => (
                  <option key={env.id} value={env.id}>
                    {env.title}
                  </option>
                ))}
              </select>
            </label>
            <p className="ux-muted">
              Manifest <code>{selectedEnv.manifestSha256.slice(0, 12)}…</code> · {selectedEnv.buildingCount.toLocaleString()}{' '}
              buildings · {selectedEnv.roadCount.toLocaleString()} roads
              {selectedEnv.hasGlb ? ' · 3D GLB' : ''}
              {selectedEnv.hasImagery ? ' · Sentinel-2' : ''}
            </p>
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                className="ux-button"
                onClick={() => onFitBounds(selectedEnv.boundsWgs84)}
              >
                Zoom to environment
              </button>
              <button
                type="button"
                className="ux-button"
                onClick={() => {
                  onSelectTool(null)
                  setActivePreset('default_uav')
                  setDraft(draftFromEnvironment(selectedEnv))
                }}
              >
                Reset defaults
              </button>
              <button
                type="button"
                className="ux-button"
                onClick={handleExportDraft}
                title="Download current scenario setup as JSON"
              >
                Export JSON
              </button>
            </div>

            <div className="pt-2 border-t border-edge space-y-1.5">
              <span className="text-xs font-medium">Quick Tactical Presets</span>
              <div className="grid grid-cols-2 gap-1.5">
                {SIM_TACTICAL_PRESETS.map((preset) => (
                  <button
                    key={preset.id}
                    type="button"
                    className="ux-button text-left"
                    data-primary={activePreset === preset.id || undefined}
                    title={preset.summary}
                    onClick={() => handleApplyPreset(preset.id)}
                  >
                    {preset.label}
                  </button>
                ))}
              </div>
            </div>

            {networks.filter((n) => n.environmentId === draft.environmentId).length > 0 && (
              <div className="pt-2 border-t border-edge space-y-1">
                <span className="text-xs font-medium">Import Sensor Network (from Coverage)</span>
                <select
                  className="atlas-field"
                  value={draft.sensorNetworkId ?? ''}
                  onChange={(e) => {
                    const id = e.target.value
                    const net = networks.find((x) => x.layoutId === id)
                    if (!net) {
                      setDraft({ ...draft, sensorNetworkId: undefined })
                      return
                    }
                    const rad = net.sensors.find((s) => s.type === 'radar') ?? net.sensors[0]
                    if (rad) {
                      setDraft({
                        ...draft,
                        sensorNetworkId: net.layoutId,
                        radar: {
                          ...draft.radar,
                          lon: rad.lon,
                          lat: rad.lat,
                          heightAglM: rad.mastAglM ?? draft.radar.heightAglM,
                          maxRangeM: rad.maxRangeM ?? draft.radar.maxRangeM,
                        },
                      })
                      onNotice(`Imported radar site from "${net.name}" (${net.sensorCount} sensors).`)
                    }
                  }}
                >
                  <option value="">— Use manual / default radar site —</option>
                  {networks
                    .filter((n) => n.environmentId === draft.environmentId)
                    .map((n) => (
                      <option key={n.layoutId} value={n.layoutId}>
                        {n.name} ({n.sensorCount} sensors)
                      </option>
                    ))}
                </select>
              </div>
            )}
          </>
        )}
      </div>

      {/* Interactive Map Placement */}
      {draft && selectedEnv && (
        <>
          <div className="map-card">
            <div className="map-panel-section-head">
              <h3>2. Place on Map</h3>
              {activeTool && (
                <button type="button" className="ux-button" onClick={() => onSelectTool(null)}>
                  Done placing
                </button>
              )}
            </div>
            <p className="ux-muted">
              Select a tool below, then click directly inside the dashed amber boundary on the map.
            </p>
            <div className="grid grid-cols-2 gap-2">
              {(
                [
                  ['radar', 'Place Radar'],
                  ['launcher', 'Place Launcher'],
                  ['protected', 'Protected Zone'],
                  ['threat', '+ Threat Waypoint'],
                ] as const
              ).map(([id, label]) => (
                <button
                  key={id}
                  type="button"
                  className="ux-button"
                  data-primary={activeTool === id || undefined}
                  aria-pressed={activeTool === id}
                  onClick={() => onSelectTool(activeTool === id ? null : id)}
                >
                  {label}
                </button>
              ))}
            </div>

            {activeTool && (
              <p role="status" className="ux-notice">
                {activeTool === 'threat'
                  ? `Click on the map to append threat waypoints (${defaultWaypointAgl} m AGL). Click "Done placing" when finished.`
                  : `Click on the map inside the Riyadh boundary to move the ${activeTool} site.`}
              </p>
            )}

            {/* Threat Waypoints Summary & AGL Control */}
            <div className="pt-2 border-t border-edge space-y-2">
              <div className="flex items-center justify-between gap-2">
                <strong className="text-xs">Threat Path ({draft.threat.waypoints.length} waypoints)</strong>
                <div className="flex gap-1">
                  <button
                    type="button"
                    className="ux-button"
                    disabled={draft.threat.waypoints.length === 0}
                    onClick={() =>
                      setDraft({
                        ...draft,
                        threat: { ...draft.threat, waypoints: draft.threat.waypoints.slice(0, -1) },
                      })
                    }
                  >
                    Undo
                  </button>
                  <button
                    type="button"
                    className="ux-button"
                    disabled={draft.threat.waypoints.length === 0}
                    onClick={() => {
                      setDraft({ ...draft, threat: { ...draft.threat, waypoints: [] } })
                      onSelectTool('threat')
                    }}
                  >
                    Redraw
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-2">
                <label className="block col-span-2">
                  <span>Threat profile</span>
                  <select
                    className="atlas-field"
                    value={draft.threat.kind}
                    onChange={(e) => handleThreatKindChange(e.target.value)}
                  >
                    <option value="one_way_uav">One-Way Attack UAV (low-altitude cruise)</option>
                    <option value="cruise_missile">Cruise Missile (fast terrain-skimming)</option>
                    <option value="ballistic_missile">Ballistic Missile (high-arc terminal descent)</option>
                  </select>
                </label>
                <label className="block">
                  <span>Threat speed (m/s)</span>
                  <input
                    className="atlas-field font-mono"
                    type="number"
                    min={20}
                    max={650}
                    step={5}
                    value={draft.threat.speedMps}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        threat: { ...draft.threat, speedMps: Number(e.target.value) || 68 },
                      })
                    }
                  />
                </label>
                {draft.threat.kind === 'ballistic_missile' ? (
                  <label className="block">
                    <span>Ballistic apogee (m)</span>
                    <input
                      className="atlas-field font-mono"
                      type="number"
                      min={300}
                      max={5000}
                      step={100}
                      value={draft.threat.ballisticApogeeM ?? 1500}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          threat: { ...draft.threat, ballisticApogeeM: Number(e.target.value) || 1400 },
                        })
                      }
                    />
                  </label>
                ) : (
                  <label className="block">
                    <span>New WP alt (m AGL)</span>
                    <input
                      className="atlas-field font-mono"
                      type="number"
                      min={15}
                      max={3000}
                      step={10}
                      value={defaultWaypointAgl}
                      onChange={(e) => setDefaultWaypointAgl(Number(e.target.value) || 120)}
                    />
                  </label>
                )}
              </div>

              {draft.threat.waypoints.length > 0 && (
                <div className="max-h-36 overflow-y-auto space-y-1 pr-1">
                  {draft.threat.waypoints.map((wp, idx) => (
                    <div key={idx} className="flex items-center justify-between gap-2 text-xs font-mono">
                      <span>
                        T{idx + 1}: {wp[1].toFixed(4)}°, {wp[0].toFixed(4)}°
                      </span>
                      <input
                        aria-label={`Waypoint ${idx + 1} altitude AGL`}
                        className="atlas-field w-20 py-0.5 text-right"
                        type="number"
                        min={15}
                        max={3000}
                        value={Math.round(wp[2])}
                        onChange={(e) => {
                          const next = draft.threat.waypoints.map((w, i) =>
                            i === idx ? ([w[0], w[1], Number(e.target.value) || 50] as [number, number, number]) : w,
                          )
                          setDraft({ ...draft, threat: { ...draft.threat, waypoints: next } })
                        }}
                      />
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="grid grid-cols-2 gap-2 pt-2 border-t border-edge">
              <label className="block">
                <span>Radar mast (m AGL)</span>
                <input
                  className="atlas-field font-mono"
                  type="number"
                  min={2}
                  max={200}
                  value={draft.radar.heightAglM}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      radar: { ...draft.radar, heightAglM: Number(e.target.value) || 14 },
                    })
                  }
                />
              </label>
              <label className="block">
                <span>Protected radius (m)</span>
                <input
                  className="atlas-field font-mono"
                  type="number"
                  min={100}
                  max={2500}
                  step={20}
                  value={draft.protectedZone.radiusM}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      protectedZone: { ...draft.protectedZone, radiusM: Number(e.target.value) || 450 },
                    })
                  }
                />
              </label>
            </div>

            <details className="pt-2 border-t border-edge">
              <summary className="cursor-pointer text-xs font-medium select-none">
                Advanced Radar &amp; Interceptor Tuning
              </summary>
              <div className="mt-2 space-y-2">
                <div className="grid grid-cols-2 gap-2">
                  <label className="block">
                    <span>Radar range (m)</span>
                    <input
                      className="atlas-field font-mono"
                      type="number"
                      min={500}
                      max={15000}
                      step={250}
                      value={draft.radar.maxRangeM}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          radar: { ...draft.radar, maxRangeM: Number(e.target.value) || 6500 },
                        })
                      }
                    />
                  </label>
                  <label className="block">
                    <span>Threat RCS (m²)</span>
                    <input
                      className="atlas-field font-mono"
                      type="number"
                      min={0.01}
                      max={20}
                      step={0.05}
                      value={draft.threat.rcsM2}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          threat: { ...draft.threat, rcsM2: Number(e.target.value) || 0.25 },
                        })
                      }
                    />
                  </label>
                  <label className="block">
                    <span>Interceptor spd (m/s)</span>
                    <input
                      className="atlas-field font-mono"
                      type="number"
                      min={80}
                      max={900}
                      step={10}
                      value={draft.launcher.speedMps}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          launcher: { ...draft.launcher, speedMps: Number(e.target.value) || 295 },
                        })
                      }
                    />
                  </label>
                  <label className="block">
                    <span>Interceptor range (m)</span>
                    <input
                      className="atlas-field font-mono"
                      type="number"
                      min={600}
                      max={15000}
                      step={250}
                      value={draft.launcher.maxRangeM}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          launcher: { ...draft.launcher, maxRangeM: Number(e.target.value) || 6000 },
                        })
                      }
                    />
                  </label>
                </div>
                <label className="flex items-center gap-2 text-xs">
                  <input
                    type="checkbox"
                    checked={draft.radar.includeBuildingsInLos}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        radar: { ...draft.radar, includeBuildingsInLos: e.target.checked },
                      })
                    }
                  />
                  <span>Include OSM buildings in radar LOS masking</span>
                </label>
              </div>
            </details>

            {/* Compact 2D Map Overlay Legend */}
            <div className="pt-2 border-t border-edge flex flex-wrap gap-x-3 gap-y-1 text-[11px] ux-muted">
              <span className="inline-flex items-center gap-1">
                <i className="inline-block size-2 rounded-full" style={{ background: '#f59e0b' }} /> Bounds
              </span>
              <span className="inline-flex items-center gap-1">
                <i className="inline-block size-2 rounded-full" style={{ background: '#38bdf8' }} /> Radar
              </span>
              <span className="inline-flex items-center gap-1">
                <i className="inline-block size-2 rounded-full" style={{ background: '#22c55e' }} /> Launcher / Safe
              </span>
              <span className="inline-flex items-center gap-1">
                <i className="inline-block size-2 rounded-full" style={{ background: '#f065d8' }} /> Protected
              </span>
              <span className="inline-flex items-center gap-1">
                <i className="inline-block size-2 rounded-full" style={{ background: '#ef4444' }} /> Threat / Earliest
              </span>
            </div>
          </div>

          {/* Wind, Shrapnel & Engagement Policy */}
          <div className="map-card">
            <div className="map-panel-section-head">
              <h3>3. Wind, Shrapnel &amp; Policy</h3>
            </div>

            <div className="map-subtabs" role="group" aria-label="Simulation policy mode">
              {(
                [
                  ['compare', 'Compare Both'],
                  ['earliest', 'Earliest'],
                  ['safe_window', 'Safe-Window'],
                ] as [SimRunMode, string][]
              ).map(([modeId, label]) => (
                <button
                  key={modeId}
                  type="button"
                  aria-pressed={draft.mode === modeId}
                  onClick={() => setDraft({ ...draft, mode: modeId })}
                >
                  {label}
                </button>
              ))}
            </div>

            <div className="flex flex-wrap gap-1.5">
              <button
                type="button"
                className="ux-button"
                onClick={() =>
                  setDraft({ ...draft, atmosphere: { windScale: 1.0, windAddEnuMps: [0, 0, 0] } })
                }
              >
                Exact ERA5
              </button>
              <button
                type="button"
                className="ux-button"
                onClick={() =>
                  setDraft({ ...draft, atmosphere: { windScale: 1.0, windAddEnuMps: [14, -5, 0] } })
                }
              >
                Shamal (+14E, -5N)
              </button>
              <button
                type="button"
                className="ux-button"
                onClick={() =>
                  setDraft({ ...draft, atmosphere: { windScale: 2.5, windAddEnuMps: [0, 0, 0] } })
                }
              >
                Strong 2.5×
              </button>
            </div>

            <div className="grid grid-cols-3 gap-2">
              <label className="block">
                <span>Wind ×</span>
                <input
                  className="atlas-field font-mono"
                  type="number"
                  min={0}
                  max={8}
                  step={0.25}
                  value={draft.atmosphere.windScale}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      atmosphere: { ...draft.atmosphere, windScale: Number(e.target.value) || 0 },
                    })
                  }
                />
              </label>
              <label className="block">
                <span>+East m/s</span>
                <input
                  className="atlas-field font-mono"
                  type="number"
                  min={-40}
                  max={40}
                  step={1}
                  value={draft.atmosphere.windAddEnuMps[0]}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      atmosphere: {
                        ...draft.atmosphere,
                        windAddEnuMps: [
                          Number(e.target.value) || 0,
                          draft.atmosphere.windAddEnuMps[1],
                          draft.atmosphere.windAddEnuMps[2],
                        ],
                      },
                    })
                  }
                />
              </label>
              <label className="block">
                <span>+North m/s</span>
                <input
                  className="atlas-field font-mono"
                  type="number"
                  min={-40}
                  max={40}
                  step={1}
                  value={draft.atmosphere.windAddEnuMps[1]}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      atmosphere: {
                        ...draft.atmosphere,
                        windAddEnuMps: [
                          draft.atmosphere.windAddEnuMps[0],
                          Number(e.target.value) || 0,
                          draft.atmosphere.windAddEnuMps[2],
                        ],
                      },
                    })
                  }
                />
              </label>
            </div>

            <div className="grid grid-cols-2 gap-2">
              <label className="block">
                <span>Shrapnel fragments</span>
                <input
                  className="atlas-field font-mono"
                  type="number"
                  min={24}
                  max={500}
                  step={20}
                  value={draft.shrapnel.fragmentCount}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      shrapnel: { fragmentCount: Number(e.target.value) || 180 },
                    })
                  }
                />
              </label>
              <label className="block">
                <span>Seed</span>
                <div className="flex gap-1">
                  <input
                    className="atlas-field font-mono"
                    type="number"
                    value={draft.seed}
                    onChange={(e) => setDraft({ ...draft, seed: Number(e.target.value) || 1 })}
                  />
                  <button
                    type="button"
                    className="ux-button"
                    title="Randomize seed"
                    onClick={() => setDraft({ ...draft, seed: Math.floor(Math.random() * 90000000) + 10000000 })}
                  >
                    ↻
                  </button>
                </div>
              </label>
            </div>
          </div>

          {/* Launch Bar */}
          <div className="map-inset-card space-y-2">
            <div className="map-panel-section-head">
              <h3>4. Execute &amp; Inspect in 3D</h3>
            </div>
            <label className="flex items-center gap-2 text-xs">
              <input
                type="checkbox"
                checked={draft.enableCivilianTraffic ?? true}
                onChange={(e) => setDraft({ ...draft, enableCivilianTraffic: e.target.checked })}
              />
              <span>Freeze civilian airliner traffic (1.5 km airspace deconfliction)</span>
            </label>
            <label className="flex items-center gap-2 text-xs">
              <input
                type="checkbox"
                checked={autoOpen3D}
                onChange={(e) => setAutoOpen3D(e.target.checked)}
              />
              <span>Open 3D replay automatically in a new tab ({health?.viewerOrigin ?? '127.0.0.1:8765'})</span>
            </label>
            <label className="flex items-center gap-2 text-xs">
              <input
                type="checkbox"
                checked={showResultOverlay}
                onChange={(e) => setShowResultOverlay(e.target.checked)}
              />
              <span>Show 90% shrapnel ellipses on this 2D map</span>
            </label>

            <button
              type="button"
              className="ux-button w-full justify-center"
              data-primary="true"
              disabled={!health || submitting || draft.threat.waypoints.length < 2}
              onClick={() => void handleRun()}
            >
              <RadarIcon className="size-4" />
              <span>{submitting ? 'Starting simulation…' : 'Run Simulation & Open 3D View'}</span>
              <ExternalLinkIcon className="size-3.5" />
            </button>

            {popupBlockedUrl && (
              <div role="status" className="ux-notice flex items-center justify-between gap-2">
                <span>Browser blocked the popup tab.</span>
                <a
                  className="ux-button"
                  data-primary="true"
                  href={popupBlockedUrl}
                  target="_blank"
                  rel="noreferrer"
                  onClick={() => setPopupBlockedUrl(null)}
                >
                  Open 3D Tab ↗
                </a>
              </div>
            )}

            {error && (
              <p role="alert" className="ux-notice">
                {error}
              </p>
            )}

            {activeJob && (
              <div className="pt-2 border-t border-edge space-y-1.5 text-xs">
                <div className="flex items-center justify-between gap-2">
                  <strong>
                    Latest: <code>{activeJob.jobId.slice(-6)}</code> ({activeJob.status})
                  </strong>
                  <a
                    className="ux-button"
                    href={buildViewerHref(activeJob)}
                    target="_blank"
                    rel="noreferrer"
                  >
                    Open 3D ↗
                  </a>
                </div>
                <p className="ux-muted">{activeJob.stage}</p>
                {activeJob.comparison?.safeWindowVsEarliest && (
                  <p role="status" className="ux-notice">
                    Safe-Window vs Earliest: Buildings{' '}
                    <strong>
                      {activeJob.comparison.safeWindowVsEarliest.buildingHitsDelta >= 0 ? '+' : ''}
                      {activeJob.comparison.safeWindowVsEarliest.buildingHitsDelta}
                    </strong>{' '}
                    · Roads{' '}
                    <strong>
                      {activeJob.comparison.safeWindowVsEarliest.roadHitsDelta >= 0 ? '+' : ''}
                      {activeJob.comparison.safeWindowVsEarliest.roadHitsDelta}
                    </strong>{' '}
                    · Exposure{' '}
                    <strong>
                      {activeJob.comparison.safeWindowVsEarliest.urbanExposureReductionPct >= 0 ? '-' : '+'}
                      {Math.abs(activeJob.comparison.safeWindowVsEarliest.urbanExposureReductionPct).toFixed(1)}%
                    </strong>
                  </p>
                )}
                {activeJob.summary?.exposure?.hazardousHits != null && (
                  <p className="ux-muted font-mono text-[11px]">
                    Hazardous debris (≥80 J): <strong>{activeJob.summary.exposure.hazardousHits}</strong>
                    /{activeJob.summary.exposure.total}
                    {activeJob.summary.exposure.maxImpactEnergyJ != null
                      ? ` · Peak ${Math.round(activeJob.summary.exposure.maxImpactEnergyJ).toLocaleString()} J`
                      : ''}
                  </p>
                )}
                {activeJob.summary?.exposure?.estimatedPopulationExposed != null && (
                  <p className="ux-muted font-mono text-[11px]">
                    WorldPop exposed: <strong>~{activeJob.summary.exposure.estimatedPopulationExposed.toFixed(1)}</strong> residents
                    {activeJob.summary.roadImpact
                      ? ` · Blocked roads ${activeJob.summary.roadImpact.blockedRoadKm.toFixed(2)} km (+${activeJob.summary.roadImpact.emergencyDetourDelayS.toFixed(0)}s detour)`
                      : ''}
                  </p>
                )}
                {activeJob.summary?.airspaceDeconfliction && (
                  <p className="ux-muted font-mono text-[11px]">
                    Civil Airspace: <strong>{activeJob.summary.airspaceDeconfliction.warningLevel}</strong>
                    {activeJob.summary.airspaceDeconfliction.closestCallsign
                      ? ` (closest ${activeJob.summary.airspaceDeconfliction.closestCallsign} @ ${Math.round(activeJob.summary.airspaceDeconfliction.minHorizontalSeparationM ?? 0)}m)`
                      : ''}
                  </p>
                )}
              </div>
            )}
          </div>
        </>
      )}

      {/* Recent Runs History */}
      {runs.length > 0 && (
        <div className="map-card">
          <div className="map-panel-section-head">
            <h3>Recent Runs ({runs.length})</h3>
          </div>
          <div className="space-y-2">
            {runs.slice(0, 6).map((job) => {
              const delta = job.comparison?.safeWindowVsEarliest
              const selected = activeJob?.jobId === job.jobId
              return (
                <div
                  key={job.jobId}
                  className="rounded border border-edge p-2 text-xs space-y-1"
                  style={selected ? { borderColor: 'var(--emad-link)' } : undefined}
                >
                  <div className="flex items-center justify-between gap-2">
                    <button
                      type="button"
                      className="text-left font-medium underline"
                      onClick={() => setActiveJob(job)}
                    >
                      {job.jobId.slice(-6)} · {job.mode} · seed {job.seed}
                    </button>
                    <a
                      href={buildViewerHref(job)}
                      target="_blank"
                      rel="noreferrer"
                      className="underline"
                    >
                      3D ↗
                    </a>
                  </div>
                  <p className="ux-muted">
                    {job.status === 'completed'
                      ? delta
                        ? `Buildings ${delta.buildingHitsDelta >= 0 ? '+' : ''}${delta.buildingHitsDelta} · Exposure ${delta.urbanExposureReductionPct >= 0 ? '-' : '+'}${Math.abs(delta.urbanExposureReductionPct).toFixed(1)}%`
                        : job.summary?.intercepted
                          ? `Intercepted at t=${job.summary.interceptTimeS}s · ${job.summary.exposure?.buildingHits ?? 0} building hits`
                          : 'Missed intercept'
                      : job.stage}
                  </p>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
