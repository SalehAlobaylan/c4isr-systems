import { useEffect, useMemo, useRef, useState } from 'react'
import type { FeatureCollection } from 'geojson'
import { ExternalLinkIcon, RefreshIcon, SignalIcon, TrashIcon } from '@/components/icons'
import {
  COV_TACTICAL_PRESETS,
  applyCovPreset,
  buildCoverageOverlay,
  cloneCovDraft,
  exportCovNetwork,
  fetchCovEnvironments,
  fetchCovHealth,
  fetchCovRun,
  fetchCovRuns,
  findCovAdversarialRoute,
  isPointInCovBounds,
  restartLocalCovService,
  startCovRun,
  startLocalCovService,
  type CovAdversarialRoute,
  type CovEnvironment,
  type CovHealth,
  type CovPlacementTool,
  type CovPresetId,
  type CovRunJob,
  type CovSensorType,
  type CovSetupDraft,
} from './coverageClient'

interface CoverageSetupPanelProps {
  activeTool: CovPlacementTool
  onSelectTool: (tool: CovPlacementTool) => void
  onOverlayChange: (overlay: FeatureCollection) => void
  onFitBounds: (bounds: [number, number, number, number]) => void
  onNotice: (message: string) => void
  registerMapClick: (handler: ((point: [number, number]) => void) | null) => void
}

function generateCovJobId(): string {
  const hex =
    typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID().replace(/-/g, '').slice(0, 12)
      : Array.from({ length: 12 }, () => Math.floor(Math.random() * 16).toString(16)).join('')
  return `covjob_${hex}`
}

function openUrlInNewTab(url: string): void {
  const link = document.createElement('a')
  link.href = url
  link.target = '_blank'
  link.rel = 'noopener noreferrer'
  link.style.display = 'none'
  document.body.appendChild(link)
  link.click()
  link.remove()
}

const SENSOR_BADGE_COLOR: Record<CovSensorType, string> = {
  radar: '#38bdf8',
  eo_ir: '#fbbf24',
  acoustic: '#c084fc',
}

export function CoverageSetupPanel({
  activeTool,
  onSelectTool,
  onOverlayChange,
  onFitBounds,
  onNotice,
  registerMapClick,
}: CoverageSetupPanelProps) {
  const [health, setHealth] = useState<CovHealth | null>(null)
  const [envs, setEnvs] = useState<CovEnvironment[]>([])
  const [draft, setDraft] = useState<CovSetupDraft | null>(null)
  const [runs, setRuns] = useState<CovRunJob[]>([])
  const [activeJob, setActiveJob] = useState<CovRunJob | null>(null)
  const [overlayMode, setOverlayMode] = useState<'optimized' | 'baseline' | 'both'>('optimized')
  const [showResultOverlay, setShowResultOverlay] = useState(true)
  const [autoOpen3D, setAutoOpen3D] = useState(true)
  const [loading, setLoading] = useState(true)
  const [startingService, setStartingService] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [activePreset, setActivePreset] = useState<CovPresetId>('baseline_multimodal')
  const [selectedSensorId, setSelectedSensorId] = useState<string | null>(null)
  const [routeAglM, setRouteAglM] = useState(40)
  const [stealthRoute, setStealthRoute] = useState<CovAdversarialRoute | null>(null)
  const [stealthBusy, setStealthBusy] = useState(false)
  const [exportStatus, setExportStatus] = useState('')
  const hasFittedInitial = useRef(false)

  const selectedEnv = useMemo(
    () => envs.find((e) => e.id === draft?.environmentId) ?? envs[0],
    [envs, draft?.environmentId],
  )

  const refreshService = async (fitOnFirstLoad = false, allowAutoStart = false) => {
    setLoading(true)
    setError('')
    try {
      const [h, envList, runList] = await Promise.all([
        fetchCovHealth(),
        fetchCovEnvironments(),
        fetchCovRuns(),
      ])
      setHealth(h)
      setEnvs(envList)
      setRuns(runList)
      if (envList.length > 0) {
        if (fitOnFirstLoad && !hasFittedInitial.current) {
          hasFittedInitial.current = true
          onFitBounds(envList[0].boundsWgs84)
        }
        setDraft((prev) =>
          prev && envList.some((e) => e.id === prev.environmentId) ? prev : cloneCovDraft(envList[0].defaults),
        )
      }
      if (runList.length > 0 && !activeJob) {
        setActiveJob(runList[0])
      }
    } catch (e) {
      if (allowAutoStart) {
        setStartingService(true)
        const boot = await startLocalCovService()
        setStartingService(false)
        if (boot.ok) {
          await refreshService(fitOnFirstLoad, false)
          return
        }
        setHealth(null)
        setError(boot.error || 'Could not start local Sensor Coverage service automatically.')
      } else {
        setHealth(null)
        setError(
          e instanceof Error
            ? e.message
            : 'Local Sensor Coverage service is offline (expected at http://127.0.0.1:8766).',
        )
      }
    } finally {
      setLoading(false)
    }
  }

  const handleStartService = async () => {
    setStartingService(true)
    setError('')
    const boot = await startLocalCovService()
    setStartingService(false)
    if (boot.ok) {
      await refreshService(true, false)
    } else {
      setError(boot.error || 'Could not start local Sensor Coverage service.')
    }
  }

  const handleRestartService = async () => {
    setStartingService(true)
    setError('')
    const boot = await restartLocalCovService()
    setStartingService(false)
    if (boot.ok) {
      await refreshService(false, false)
      onNotice('Sensor Coverage engine restarted.')
    } else {
      setError(boot.error || 'Could not restart Sensor Coverage service automatically.')
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

  // Keep 2D MapLibre overlay synchronized with draft + completed blind-spot polygons + stealth path
  useEffect(() => {
    const result = showResultOverlay && activeJob?.status === 'completed' ? activeJob.resultOverlay : null
    onOverlayChange(buildCoverageOverlay(selectedEnv, draft ?? undefined, result, overlayMode, stealthRoute))
  }, [selectedEnv, draft, activeJob, showResultOverlay, overlayMode, stealthRoute, onOverlayChange])

  // Interactive map placement clicks
  useEffect(() => {
    if (!activeTool || !selectedEnv || !draft) {
      registerMapClick(null)
      return
    }
    registerMapClick(([lng, lat]) => {
      if (!isPointInCovBounds(lng, lat, selectedEnv.boundsWgs84)) {
        onNotice('Click inside the dashed amber environment boundary on the map.')
        return
      }
      const rLng = Number(lng.toFixed(6))
      const rLat = Number(lat.toFixed(6))
      setDraft((prev) => {
        if (!prev) return prev
        if (activeTool === 'add-radar' || activeTool === 'add-eo_ir' || activeTool === 'add-acoustic') {
          if (prev.sensors.length >= 16) {
            onNotice('Maximum 16 sensors reached.')
            return prev
          }
          const stype: CovSensorType =
            activeTool === 'add-radar' ? 'radar' : activeTool === 'add-eo_ir' ? 'eo_ir' : 'acoustic'
          const defaults = selectedEnv.sensorDefaults[stype]
          const count = prev.sensors.filter((s) => s.type === stype).length + 1
          const id = `${stype === 'eo_ir' ? 'eo' : stype}-${Date.now().toString(36).slice(-4)}`
          const label = `${stype === 'eo_ir' ? 'EO/IR' : stype === 'radar' ? 'Radar' : 'Acoustic'} ${count}`
          onNotice(`Added ${label} at ${rLat.toFixed(5)}°, ${rLng.toFixed(5)}°`)
          return {
            ...prev,
            sensors: [
              ...prev.sensors,
              {
                id,
                type: stype,
                label,
                lon: rLng,
                lat: rLat,
                mastAglM: defaults.mastAglM,
                maxRangeM: defaults.maxRangeM,
                minElevationDeg: defaults.minElevationDeg,
                clutterFloorAglM: defaults.clutterFloorAglM,
                sectorCenterDeg: 0,
                sectorWidthDeg: 360,
                locked: false,
              },
            ],
          }
        }
        if (activeTool === 'add-jammer') {
          const jammers = [...(prev.jammers ?? [])]
          jammers.push({
            id: `jammer-${jammers.length + 1}`,
            label: `EW Jammer ${jammers.length + 1}`,
            lon: rLng,
            lat: rLat,
            aglM: 180,
            erpRatio: 16,
            beamWidthDeg: 36,
          })
          onNotice(`Placed Stand-Off EW Jammer at ${rLat.toFixed(5)}°, ${rLng.toFixed(5)}°`)
          return { ...prev, jammers }
        }
        if (activeTool === 'asset') {
          const nextAssets = prev.assets.length
            ? prev.assets.map((a, idx) => (idx === 0 ? { ...a, lon: rLng, lat: rLat } : a))
            : [{ id: 'asset-1', label: 'Defended site', lon: rLng, lat: rLat, warningRadiusM: 2500, priority: 4 }]
          onNotice(`Moved defended asset to ${rLat.toFixed(5)}°, ${rLng.toFixed(5)}°`)
          return { ...prev, assets: nextAssets }
        }
        if (activeTool === 'route-pt') {
          const routes = [...prev.routes]
          if (!routes.length) {
            routes.push({
              id: 'route-custom',
              label: 'Custom ingress route',
              speedMps: 65,
              points: [[rLng, rLat, routeAglM]],
            })
          } else {
            const last = routes[routes.length - 1]
            routes[routes.length - 1] = {
              ...last,
              points: [...last.points, [rLng, rLat, routeAglM]],
            }
          }
          onNotice(`Added ingress point (${routeAglM} m AGL)`)
          return { ...prev, routes }
        }
        return prev
      })
      if (activeTool === 'asset') onSelectTool(null)
    })
    return () => registerMapClick(null)
  }, [activeTool, selectedEnv, draft, routeAglM, registerMapClick, onSelectTool, onNotice])

  // Poll running job until complete
  useEffect(() => {
    if (!activeJob || activeJob.status !== 'running') return
    let cancelled = false
    const timer = window.setInterval(() => {
      void fetchCovRun(activeJob.jobId)
        .then((updated) => {
          if (cancelled) return
          setActiveJob(updated)
          setRuns((prev) => prev.map((r) => (r.jobId === updated.jobId ? updated : r)))
          if (updated.status !== 'running') {
            window.clearInterval(timer)
            void fetchCovRuns().then((list) => {
              if (!cancelled) setRuns(list)
            })
          }
        })
        .catch(() => {
          /* retry next tick */
        })
    }, 350)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [activeJob?.jobId, activeJob?.status])

  const buildViewerHref = (job: CovRunJob) => {
    const url = new URL(job.viewerUrl)
    url.searchParams.set('mapOrigin', window.location.origin)
    return url.toString()
  }

  const handleRun = async () => {
    if (!draft || !health) return
    if (draft.sensors.length === 0) {
      setError('Place at least one sensor inside the environment before running.')
      return
    }
    setError('')
    setSubmitting(true)
    onSelectTool(null)

    const jobId = generateCovJobId()
    const targetUrl = `${health.viewerOrigin}/?run=${encodeURIComponent(jobId)}&mapOrigin=${encodeURIComponent(window.location.origin)}`
    if (autoOpen3D) {
      openUrlInNewTab(targetUrl)
    }

    try {
      const job = await startCovRun(draft, jobId)
      setActiveJob(job)
      setRuns((prev) => [job, ...prev.filter((x) => x.jobId !== job.jobId)])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Coverage study failed to start.')
    } finally {
      setSubmitting(false)
    }
  }

  const handleSelectEnvironment = (envId: string) => {
    const next = envs.find((e) => e.id === envId)
    if (!next) return
    onSelectTool(null)
    setActivePreset('baseline_multimodal')
    setDraft(cloneCovDraft(next.defaults))
    onFitBounds(next.boundsWgs84)
  }

  const handleApplyPreset = (presetId: CovPresetId) => {
    if (!selectedEnv) return
    onSelectTool(null)
    setActivePreset(presetId)
    setDraft(applyCovPreset(selectedEnv, presetId))
    const meta = COV_TACTICAL_PRESETS.find((p) => p.id === presetId)
    if (meta) onNotice(`Loaded coverage preset: ${meta.label}`)
  }

  const handleAdoptOptimizedSites = () => {
    if (!draft || !selectedEnv || !activeJob?.summary?.optimizedSensors?.length) return
    const optById = new Map(activeJob.summary.optimizedSensors.map((s) => [s.id, s]))
    const updated = draft.sensors.map((s) => {
      const match = optById.get(s.id)
      return match ? { ...s, lon: match.lon, lat: match.lat } : s
    })
    // Also add any extra gap-filler sensors created by the optimizer
    for (const os of activeJob.summary.optimizedSensors) {
      if (!updated.some((s) => s.id === os.id)) {
        const defaults = selectedEnv.sensorDefaults[os.type]
        updated.push({
          id: os.id,
          type: os.type,
          label: os.label,
          lon: os.lon,
          lat: os.lat,
          mastAglM: defaults.mastAglM,
          maxRangeM: defaults.maxRangeM,
          minElevationDeg: defaults.minElevationDeg,
          clutterFloorAglM: defaults.clutterFloorAglM,
          sectorCenterDeg: 0,
          sectorWidthDeg: 360,
          locked: false,
        })
      }
    }
    setDraft({
      ...draft,
      sensors: updated,
      optimizer: { ...draft.optimizer, extraSensors: 0 },
    })
    onNotice('Adopted optimized sensor sites onto the 2D map draft.')
  }

  const handleFindStealthPath = async () => {
    if (!draft || !health) return
    setStealthBusy(true)
    setError('')
    try {
      const asset = draft.assets[0]
      const res = await findCovAdversarialRoute({
        environmentId: draft.environmentId,
        aglM: draft.mapLayerAglM,
        speedMps: 65,
        includeBuildings: draft.includeBuildings,
        rcsM2: draft.threat.rcsM2,
        visibilityKm: draft.weather.visibilityKm,
        surfaceWindMps: draft.weather.surfaceWindMps,
        assetLon: asset?.lon,
        assetLat: asset?.lat,
        jammers: draft.jammers ?? [],
        sensors: draft.sensors,
      })
      setStealthRoute(res)
      onNotice(
        `Stealth path @ ${res.aglM}m AGL: ${(res.stealthFraction * 100).toFixed(0)}% blind, ${res.warningTimeS}s warning`,
      )
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Adversarial route solver failed.')
    } finally {
      setStealthBusy(false)
    }
  }

  const handleExportNetwork = async () => {
    if (!draft || !health) return
    setExportStatus('Exporting network…')
    try {
      const optSensors = activeJob?.status === 'completed' ? activeJob.summary?.optimizedSensors : undefined
      const saved = await exportCovNetwork({
        environmentId: draft.environmentId,
        jobId: activeJob?.status === 'completed' ? activeJob.jobId : undefined,
        name: `${draft.title}${optSensors?.length ? ' (Optimized)' : ''}`,
        sensors: optSensors?.length ? undefined : draft.sensors,
      })
      setExportStatus(`Saved ${saved.layoutId} (${saved.sensorCount} sensors) — ready in Simulate & Salvo!`)
      onNotice(`Exported sensor network ${saved.layoutId} to Intercept & Salvo simulators.`)
    } catch (e) {
      setExportStatus(e instanceof Error ? e.message : 'Export failed.')
    }
  }

  const handleExportDraft = () => {
    if (!draft) return
    const blob = new Blob([JSON.stringify(draft, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${draft.environmentId}-coverage-setup.json`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    onNotice('Exported sensor coverage study JSON.')
  }

  const activeSensor = draft?.sensors.find((s) => s.id === selectedSensorId) ?? null

  return (
    <div className="map-drawer-pane">
      {/* 1. Service Status & Verified Environment */}
      <div className="map-card">
        <div className="map-panel-section-head">
          <h3>1. Coverage Engine &amp; Environment</h3>
          <button
            type="button"
            className="ux-button"
            onClick={() => void refreshService(false)}
            disabled={loading}
            title="Reconnect to local Sensor Coverage service"
          >
            <RefreshIcon className="size-3.5" />
          </button>
        </div>

        {health ? (
          <div className="space-y-1.5">
            <div className="flex items-center justify-between gap-2">
              <p className="ux-muted" role="status">
                <strong>Coverage ready</strong> · <code>{health.engineVersion.replace('emad-cov-py-', 'v')}</code> ·{' '}
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
            {(!health.capabilities || !health.capabilities.includes('los-profile')) && (
              <div className="ux-notice flex items-center justify-between gap-2 text-xs">
                <span>
                  Coverage engine needs restart to activate <code>v0.2.0</code> LOS cross-sections &amp; 3D test-sensor sandbox.
                </span>
                <button
                  type="button"
                  className="ux-button shrink-0"
                  disabled={startingService}
                  onClick={() => void handleRestartService()}
                >
                  Restart
                </button>
              </div>
            )}
          </div>
        ) : startingService || loading ? (
          <p className="ux-muted" role="status">
            Starting local Sensor Coverage engine on <code>127.0.0.1:8766</code>…
          </p>
        ) : (
          <div className="ux-notice space-y-2" role="alert">
            <p>
              <strong>Local Sensor Coverage service is offline.</strong>
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
                Start Coverage Service
              </button>
              <button type="button" className="ux-button" onClick={() => void refreshService(true, false)}>
                Retry
              </button>
            </div>
            <p className="text-xs ux-muted">
              Manual fallback: <code>task dev:coverage</code>
            </p>
          </div>
        )}

        {envs.length > 0 && draft && selectedEnv && (
          <>
            <label className="block">
              <span>Verified geographic environment</span>
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
              Manifest <code>{selectedEnv.manifestSha256.slice(0, 12)}…</code> ·{' '}
              {selectedEnv.buildingCount.toLocaleString()} buildings · {selectedEnv.obstructionGrid.cellM} m LOS raster
            </p>
            <div className="flex flex-wrap gap-2">
              <button type="button" className="ux-button" onClick={() => onFitBounds(selectedEnv.boundsWgs84)}>
                Zoom to environment
              </button>
              <button
                type="button"
                className="ux-button"
                onClick={() => {
                  onSelectTool(null)
                  setDraft(cloneCovDraft(selectedEnv.defaults))
                  setActivePreset('baseline_multimodal')
                }}
              >
                Reset defaults
              </button>
            </div>
          </>
        )}
      </div>

      {draft && selectedEnv && (
        <>
          {/* 2. Quick Coverage Presets */}
          <div className="map-card">
            <div className="map-panel-section-head">
              <h3>2. Quick Coverage Presets</h3>
            </div>
            <div className="grid grid-cols-2 gap-1.5">
              {COV_TACTICAL_PRESETS.map((preset) => (
                <button
                  key={preset.id}
                  type="button"
                  className="ux-button text-left"
                  data-primary={activePreset === preset.id ? 'true' : undefined}
                  title={preset.summary}
                  onClick={() => handleApplyPreset(preset.id)}
                >
                  {preset.label}
                </button>
              ))}
            </div>
          </div>

          {/* 3. Map Placement & Sensor Roster */}
          <div className="map-card">
            <div className="map-panel-section-head">
              <h3>3. Sensors &amp; Map Placement ({draft.sensors.length})</h3>
              {activeTool && (
                <button type="button" className="ux-button" onClick={() => onSelectTool(null)}>
                  Done placing
                </button>
              )}
            </div>
            <p className="ux-muted">
              Click a tool below, then click inside the amber environment box on the map to place sensors or move the
              defended site.
            </p>
            <div className="flex flex-wrap gap-1.5">
              {(
                [
                  ['add-radar', '+ Radar'],
                  ['add-eo_ir', '+ EO/IR'],
                  ['add-acoustic', '+ Acoustic'],
                  ['add-jammer', '+ EW Jammer'],
                  ['asset', 'Move Asset'],
                  ['route-pt', '+ Ingress Pt'],
                ] as [CovPlacementTool, string][]
              ).map(([toolId, label]) => (
                <button
                  key={toolId}
                  type="button"
                  className="ux-button"
                  data-primary={activeTool === toolId ? 'true' : undefined}
                  onClick={() => onSelectTool(activeTool === toolId ? null : toolId)}
                >
                  {label}
                </button>
              ))}
            </div>
            {activeTool === 'route-pt' && (
              <div className="flex items-center gap-2 pt-1">
                <label className="text-xs ux-muted">New route pt AGL (m):</label>
                <input
                  type="number"
                  className="atlas-field w-20"
                  min={10}
                  max={1000}
                  step={5}
                  value={routeAglM}
                  onChange={(e) => setRouteAglM(Number(e.target.value) || 40)}
                />
                <button
                  type="button"
                  className="ux-button"
                  onClick={() =>
                    setDraft({
                      ...draft,
                      routes: [
                        ...draft.routes,
                        { id: `route-${draft.routes.length + 1}`, label: `Ingress ${draft.routes.length + 1}`, speedMps: 65, points: [] },
                      ],
                    })
                  }
                >
                  New route
                </button>
              </div>
            )}

            {/* Sensor roster list */}
            <div className="space-y-1.5 pt-1">
              {draft.sensors.map((s) => {
                const isSelected = selectedSensorId === s.id
                return (
                  <div
                    key={s.id}
                    className="flex items-center justify-between gap-2 rounded-md border border-slate-300/60 px-2 py-1.5 text-xs"
                    style={{ background: isSelected ? 'rgba(14,165,233,0.10)' : undefined }}
                  >
                    <button
                      type="button"
                      className="flex items-center gap-1.5 text-left flex-1 min-w-0"
                      onClick={() => setSelectedSensorId(isSelected ? null : s.id)}
                      title="Click to tune mast, range, and sector"
                    >
                      <span
                        className="inline-block size-2.5 rounded-full shrink-0"
                        style={{ background: SENSOR_BADGE_COLOR[s.type] }}
                      />
                      <span className="font-medium truncate">{s.label}</span>
                      <span className="ux-muted">
                        {(s.maxRangeM / 1000).toFixed(1)}km · {s.mastAglM}m
                      </span>
                    </button>
                    <label className="flex items-center gap-1 text-[11px] cursor-pointer" title="Lock site so optimizer keeps it fixed">
                      <input
                        type="checkbox"
                        checked={s.locked}
                        onChange={(e) =>
                          setDraft({
                            ...draft,
                            sensors: draft.sensors.map((x) => (x.id === s.id ? { ...x, locked: e.target.checked } : x)),
                          })
                        }
                      />
                      Lock
                    </label>
                    <button
                      type="button"
                      className="ux-button"
                      disabled={draft.sensors.length <= 1}
                      title="Remove sensor"
                      onClick={() => {
                        setDraft({ ...draft, sensors: draft.sensors.filter((x) => x.id !== s.id) })
                        if (selectedSensorId === s.id) setSelectedSensorId(null)
                      }}
                    >
                      <TrashIcon className="size-3" />
                    </button>
                  </div>
                )
              })}
            </div>

            {activeSensor && (
              <div className="rounded-md border border-sky-400/40 p-2 space-y-2 mt-1 text-xs">
                <div className="font-semibold">Tune: {activeSensor.label}</div>
                <div className="grid grid-cols-2 gap-2">
                  <label className="block">
                    <span>Mast AGL (m)</span>
                    <input
                      type="number"
                      className="atlas-field"
                      min={2}
                      max={100}
                      value={activeSensor.mastAglM}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          sensors: draft.sensors.map((x) =>
                            x.id === activeSensor.id ? { ...x, mastAglM: Number(e.target.value) || 10 } : x,
                          ),
                        })
                      }
                    />
                  </label>
                  <label className="block">
                    <span>Nominal Range (m)</span>
                    <input
                      type="number"
                      className="atlas-field"
                      min={500}
                      max={25000}
                      step={500}
                      value={activeSensor.maxRangeM}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          sensors: draft.sensors.map((x) =>
                            x.id === activeSensor.id ? { ...x, maxRangeM: Number(e.target.value) || 3000 } : x,
                          ),
                        })
                      }
                    />
                  </label>
                  <label className="block">
                    <span>Sector Width (°)</span>
                    <input
                      type="number"
                      className="atlas-field"
                      min={15}
                      max={360}
                      step={15}
                      value={activeSensor.sectorWidthDeg}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          sensors: draft.sensors.map((x) =>
                            x.id === activeSensor.id ? { ...x, sectorWidthDeg: Number(e.target.value) || 360 } : x,
                          ),
                        })
                      }
                    />
                  </label>
                  <label className="block">
                    <span>Sector Azimuth (°)</span>
                    <input
                      type="number"
                      className="atlas-field"
                      min={0}
                      max={359}
                      step={15}
                      value={activeSensor.sectorCenterDeg}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          sensors: draft.sensors.map((x) =>
                            x.id === activeSensor.id ? { ...x, sectorCenterDeg: Number(e.target.value) || 0 } : x,
                          ),
                        })
                      }
                    />
                  </label>
                </div>
              </div>
            )}
          </div>

          {/* 4. Threat, Atmosphere & Optimizer */}
          <div className="map-card">
            <div className="map-panel-section-head">
              <h3>4. Threat, Weather &amp; Optimizer</h3>
            </div>
            <div className="grid grid-cols-2 gap-2">
              <label className="block">
                <span>Threat RCS (m²)</span>
                <input
                  type="number"
                  className="atlas-field"
                  min={0.005}
                  max={20}
                  step={0.05}
                  value={draft.threat.rcsM2}
                  onChange={(e) =>
                    setDraft({ ...draft, threat: { ...draft.threat, rcsM2: Number(e.target.value) || 0.1 } })
                  }
                />
              </label>
              <label className="block">
                <span>Visibility (km · EO/IR)</span>
                <input
                  type="number"
                  className="atlas-field"
                  min={0.5}
                  max={30}
                  step={0.5}
                  value={draft.weather.visibilityKm}
                  onChange={(e) =>
                    setDraft({ ...draft, weather: { ...draft.weather, visibilityKm: Number(e.target.value) || 10 } })
                  }
                />
              </label>
              <label className="block">
                <span>Map Blind-Spot AGL (m)</span>
                <select
                  className="atlas-field"
                  value={draft.mapLayerAglM}
                  onChange={(e) => setDraft({ ...draft, mapLayerAglM: Number(e.target.value) })}
                >
                  {draft.layers.map((l) => (
                    <option key={l.aglM} value={l.aglM}>
                      {l.aglM} m AGL (w={l.weight})
                    </option>
                  ))}
                </select>
              </label>
              <label className="block">
                <span>Extra Gap-Fillers</span>
                <input
                  type="number"
                  className="atlas-field"
                  min={0}
                  max={6}
                  step={1}
                  value={draft.optimizer.extraSensors}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      optimizer: { ...draft.optimizer, extraSensors: Math.max(0, Math.min(6, Number(e.target.value) || 0)) },
                    })
                  }
                />
              </label>
            </div>

            {draft.optimizer.extraSensors > 0 && (
              <label className="block">
                <span>Extra Sensor Type</span>
                <select
                  className="atlas-field"
                  value={draft.optimizer.extraSensorType}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      optimizer: { ...draft.optimizer, extraSensorType: e.target.value as CovSensorType },
                    })
                  }
                >
                  <option value="radar">Radar (9 km nominal)</option>
                  <option value="eo_ir">EO/IR Optical (5 km clear)</option>
                  <option value="acoustic">Acoustic Array (1.5 km NLOS)</option>
                </select>
              </label>
            )}

            <div className="grid grid-cols-2 gap-2 pt-1">
              <label className="flex items-center gap-1.5 text-xs cursor-pointer">
                <input
                  type="checkbox"
                  checked={draft.optimizer.enabled}
                  onChange={(e) => setDraft({ ...draft, optimizer: { ...draft.optimizer, enabled: e.target.checked } })}
                />
                Run Placement Optimizer
              </label>
              <label className="flex items-center gap-1.5 text-xs cursor-pointer">
                <input
                  type="checkbox"
                  checked={draft.includeBuildings}
                  onChange={(e) => setDraft({ ...draft, includeBuildings: e.target.checked })}
                />
                Buildings block LOS
              </label>
              <label className="flex items-center gap-1.5 text-xs cursor-pointer">
                <input
                  type="checkbox"
                  checked={draft.optimizer.allowRooftops}
                  onChange={(e) =>
                    setDraft({ ...draft, optimizer: { ...draft.optimizer, allowRooftops: e.target.checked } })
                  }
                />
                Allow rooftop sites
              </label>
              <label className="flex items-center gap-1.5 text-xs cursor-pointer">
                <input
                  type="checkbox"
                  checked={draft.weather.useEra5Wind}
                  onChange={(e) =>
                    setDraft({ ...draft, weather: { ...draft.weather, useEra5Wind: e.target.checked } })
                  }
                />
                ERA5 Wind ({selectedEnv.weather.surfaceWindMps} m/s)
              </label>
              <label className="flex items-center gap-1.5 text-xs cursor-pointer">
                <input
                  type="checkbox"
                  checked={draft.optimizer.optimizeOrientation ?? true}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      optimizer: { ...draft.optimizer, optimizeOrientation: e.target.checked },
                    })
                  }
                />
                Optimize sector azimuth
              </label>
            </div>

            {(draft.jammers?.length ?? 0) > 0 && (
              <div className="flex items-center justify-between gap-2 rounded border border-orange-400/50 bg-orange-500/10 px-2 py-1 text-xs">
                <span>
                  ⚡ <strong>{draft.jammers?.length}</strong> Stand-Off EW Jammer active
                </span>
                <button
                  type="button"
                  className="ux-button"
                  onClick={() => setDraft({ ...draft, jammers: [] })}
                >
                  Clear EW
                </button>
              </div>
            )}

            {!draft.weather.useEra5Wind && (
              <label className="block">
                <span>Manual Surface Wind (m/s · Acoustic noise)</span>
                <input
                  type="number"
                  className="atlas-field"
                  min={0}
                  max={35}
                  step={1}
                  value={draft.weather.surfaceWindMps}
                  onChange={(e) =>
                    setDraft({ ...draft, weather: { ...draft.weather, surfaceWindMps: Number(e.target.value) || 0 } })
                  }
                />
              </label>
            )}

            <details className="pt-1">
              <summary className="text-xs font-medium cursor-pointer ux-muted">Advanced optimizer tuning</summary>
              <div className="grid grid-cols-2 gap-2 pt-2">
                <label className="block">
                  <span>Min Separation (m)</span>
                  <input
                    type="number"
                    className="atlas-field"
                    min={0}
                    max={5000}
                    step={100}
                    value={draft.optimizer.minSeparationM}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        optimizer: { ...draft.optimizer, minSeparationM: Number(e.target.value) || 0 },
                      })
                    }
                  />
                </label>
                <label className="block">
                  <span>Redundancy Bonus</span>
                  <input
                    type="number"
                    className="atlas-field"
                    min={0}
                    max={1}
                    step={0.05}
                    value={draft.optimizer.redundancyWeight}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        optimizer: { ...draft.optimizer, redundancyWeight: Number(e.target.value) || 0 },
                      })
                    }
                  />
                </label>
                <label className="block">
                  <span>Ingress Corridor Weight</span>
                  <input
                    type="number"
                    className="atlas-field"
                    min={1}
                    max={10}
                    step={0.5}
                    value={draft.optimizer.routeWeight ?? 2.5}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        optimizer: { ...draft.optimizer, routeWeight: Number(e.target.value) || 1 },
                      })
                    }
                  />
                </label>
              </div>
            </details>

            {draft.routes.length > 0 && (
              <details className="pt-1">
                <summary className="text-xs font-medium cursor-pointer ux-muted">
                  Ingress corridors ({draft.routes.length})
                </summary>
                <div className="space-y-1 pt-2">
                  {draft.routes.map((r) => (
                    <div
                      key={r.id}
                      className="flex items-center justify-between gap-2 rounded border border-slate-300/50 px-2 py-1 text-xs"
                    >
                      <span className="truncate font-medium">{r.label}</span>
                      <span className="ux-muted shrink-0">
                        {r.speedMps} m/s · {r.points.length} pts
                      </span>
                      <button
                        type="button"
                        className="ux-button"
                        title="Remove ingress corridor"
                        onClick={() =>
                          setDraft({ ...draft, routes: draft.routes.filter((x) => x.id !== r.id) })
                        }
                      >
                        <TrashIcon className="size-3" />
                      </button>
                    </div>
                  ))}
                </div>
              </details>
            )}

            {/* Compact 2D Map Overlay Legend */}
            <div className="flex flex-wrap gap-x-3 gap-y-1 pt-2 border-t border-slate-300/40 text-[11px] ux-muted">
              <span className="inline-flex items-center gap-1">
                <span className="inline-block size-2 rounded-full" style={{ background: '#0ea5e9' }} /> Radar
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="inline-block size-2 rounded-full" style={{ background: '#f59e0b' }} /> EO/IR
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="inline-block size-2 rounded-full" style={{ background: '#a855f7' }} /> Acoustic
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="inline-block size-2 rounded-full" style={{ background: '#ec4899' }} /> Asset
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="inline-block size-2 rounded-full" style={{ background: '#ef4444' }} /> Blind Spot
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="inline-block size-2 rounded-full" style={{ background: '#10b981' }} /> Optimized
              </span>
            </div>
          </div>

          {/* 5. Run & Launch 3D Coverage Workbench */}
          <div className="map-card">
            <div className="map-panel-section-head">
              <h3>5. Launch 3D Coverage Workbench</h3>
              <button type="button" className="ux-button" onClick={handleExportDraft}>
                Export JSON
              </button>
            </div>
            {error && (
              <p className="ux-notice text-xs" role="alert">
                {error}
              </p>
            )}
            <label className="flex items-center gap-2 text-xs cursor-pointer">
              <input
                type="checkbox"
                checked={autoOpen3D}
                onChange={(e) => setAutoOpen3D(e.target.checked)}
              />
              Open 3D Coverage Workbench in a new tab automatically
            </label>
            <button
              type="button"
              className="ux-button w-full justify-center"
              data-primary="true"
              disabled={!health || submitting}
              onClick={() => void handleRun()}
            >
              <SignalIcon className="size-4" />
              {submitting ? 'Starting coverage optimization…' : 'Run Coverage & Open 3D Workbench'}
            </button>

            {activeJob && (
              <div className="space-y-2 pt-2 border-t border-slate-300/40">
                <div className="flex items-center justify-between gap-2 text-xs">
                  <span className="font-semibold">
                    {activeJob.status === 'running'
                      ? 'Computing…'
                      : activeJob.status === 'completed'
                        ? `Completed (${activeJob.summary?.elapsedS ?? '—'}s)`
                        : activeJob.status}
                  </span>
                  {activeJob.status === 'completed' && (
                    <a
                      className="ux-button"
                      href={buildViewerHref(activeJob)}
                      target="_blank"
                      rel="noreferrer"
                    >
                      <ExternalLinkIcon className="size-3.5" />
                      Open 3D
                    </a>
                  )}
                </div>
                {activeJob.status === 'running' && <p className="ux-muted text-xs">{activeJob.stage}</p>}
                {activeJob.error && <p className="ux-notice text-xs">{activeJob.error}</p>}

                {activeJob.status === 'completed' && activeJob.summary?.baseline && (
                  <div className="space-y-2 text-xs">
                    <div className="grid grid-cols-2 gap-1.5">
                      <div className="rounded border border-slate-300/60 p-2">
                        <div className="ux-muted text-[10px] uppercase">Baseline @ {activeJob.summary.baseline.aglM}m</div>
                        <div className="font-bold text-sm">
                          {(activeJob.summary.baseline.coverageFraction * 100).toFixed(1)}%
                        </div>
                        <div className="ux-muted text-[11px]">
                          Blind {activeJob.summary.baseline.blindAreaKm2.toFixed(2)} km² · Warn{' '}
                          {activeJob.summary.baseline.minWarningTimeS ?? '—'}s
                        </div>
                      </div>
                      {activeJob.summary.optimized && (
                        <div className="rounded border border-emerald-500/50 bg-emerald-500/10 p-2">
                          <div className="ux-muted text-[10px] uppercase">
                            Optimized{' '}
                            {activeJob.summary.comparison?.objectiveGainPct != null
                              ? `(${activeJob.summary.comparison.objectiveGainPct >= 0 ? '+' : ''}${activeJob.summary.comparison.objectiveGainPct.toFixed(1)}%)`
                              : ''}
                          </div>
                          <div className="font-bold text-sm text-emerald-700">
                            {(activeJob.summary.optimized.coverageFraction * 100).toFixed(1)}%
                          </div>
                          <div className="ux-muted text-[11px]">
                            Blind {activeJob.summary.optimized.blindAreaKm2.toFixed(2)} km² · Warn{' '}
                            {activeJob.summary.optimized.minWarningTimeS ?? '—'}s
                          </div>
                        </div>
                      )}
                    </div>

                    {/* Blind-Spot Root Cause Breakdown */}
                    {(activeJob.summary.optimized?.blindCauses ?? activeJob.summary.baseline.blindCauses) && (() => {
                      const bc = (activeJob.summary.optimized?.blindCauses ?? activeJob.summary.baseline.blindCauses)!
                      return (
                        <div className="rounded border border-slate-300/50 px-2 py-1.5 text-[11px] ux-muted">
                          <span className="font-semibold text-slate-700">Blind cause @ {bc.aglM}m:</span>{' '}
                          Buildings <strong>{bc.buildingBlockedKm2.toFixed(2)}</strong> km² · Terrain{' '}
                          <strong>{bc.terrainBlockedKm2.toFixed(2)}</strong> km² · Range{' '}
                          <strong>{bc.outOfRangeOrSectorKm2.toFixed(2)}</strong> km²
                        </div>
                      )
                    })()}

                    {/* Per-Route Warning Time Comparison */}
                    {activeJob.summary.baseline.routes.length > 0 && (
                      <div className="rounded border border-slate-300/50 px-2 py-1.5 space-y-1 text-[11px]">
                        <div className="font-semibold text-slate-700">Ingress Warning Time (s)</div>
                        {activeJob.summary.baseline.routes.map((br) => {
                          const orr = activeJob.summary?.optimized?.routes.find((r) => r.routeId === br.routeId)
                          const delta = orr ? orr.warningTimeS - br.warningTimeS : null
                          return (
                            <div key={br.routeId} className="flex items-center justify-between gap-2">
                              <span className="truncate ux-muted">{br.label}</span>
                              <span className="font-mono shrink-0">
                                {br.warningTimeS.toFixed(1)}s
                                {orr ? ` → ${orr.warningTimeS.toFixed(1)}s` : ''}
                                {delta !== null && Math.abs(delta) >= 0.1 ? (
                                  <span className={delta > 0 ? ' text-emerald-700 font-semibold' : ' text-rose-600'}>
                                    {' '}
                                    ({delta > 0 ? '+' : ''}
                                    {delta.toFixed(1)}s)
                                  </span>
                                ) : null}
                              </span>
                            </div>
                          )
                        })}
                      </div>
                    )}

                    <div className="flex flex-wrap items-center gap-2">
                      <label className="flex items-center gap-1.5 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={showResultOverlay}
                          onChange={(e) => setShowResultOverlay(e.target.checked)}
                        />
                        2D Blind-Spot Overlay
                      </label>
                      {showResultOverlay && activeJob.summary.optimized && (
                        <select
                          className="atlas-field w-auto py-0.5 text-xs"
                          value={overlayMode}
                          onChange={(e) => setOverlayMode(e.target.value as 'optimized' | 'baseline' | 'both')}
                        >
                          <option value="optimized">Optimized gaps</option>
                          <option value="baseline">Baseline gaps</option>
                          <option value="both">Both + move arcs</option>
                        </select>
                      )}
                    </div>

                    {activeJob.summary.optimizedSensors.length > 0 && (
                      <button type="button" className="ux-button w-full justify-center" onClick={handleAdoptOptimizedSites}>
                        Apply Optimized Sites to Map Draft
                      </button>
                    )}

                    <div className="grid grid-cols-2 gap-1.5 pt-1">
                      <button
                        type="button"
                        className="ux-button justify-center"
                        disabled={stealthBusy}
                        onClick={() => void handleFindStealthPath()}
                      >
                        {stealthBusy ? 'Solving A*…' : '🕵 Stealth Path'}
                      </button>
                      <button
                        type="button"
                        className="ux-button justify-center"
                        onClick={() => void handleExportNetwork()}
                      >
                        📤 Export Network
                      </button>
                    </div>
                    {stealthRoute && (
                      <div className="rounded border border-purple-400/50 bg-purple-500/10 px-2 py-1 text-[11px]">
                        <strong>Stealth Corridor ({stealthRoute.aglM}m):</strong>{' '}
                        {(stealthRoute.stealthFraction * 100).toFixed(0)}% unseen ·{' '}
                        <strong>{stealthRoute.warningTimeS}s</strong> warning
                      </div>
                    )}
                    {exportStatus && <p className="text-[11px] text-emerald-700 font-medium">{exportStatus}</p>}
                  </div>
                )}
              </div>
            )}
          </div>

          {/* 6. Recent Coverage Studies */}
          {runs.length > 0 && (
            <div className="map-card">
              <div className="map-panel-section-head">
                <h3>Recent Coverage Studies ({runs.length})</h3>
              </div>
              <div className="space-y-1.5">
                {runs.slice(0, 6).map((r) => (
                  <div
                    key={r.jobId}
                    className="flex items-center justify-between gap-2 rounded border border-slate-300/50 px-2 py-1.5 text-xs"
                  >
                    <button
                      type="button"
                      className="text-left flex-1 min-w-0"
                      onClick={() => setActiveJob(r)}
                    >
                      <div className="font-medium truncate">{r.title}</div>
                      <div className="ux-muted text-[10px]">
                        {r.status} · {r.sensorCount} sensors · {r.jobId.slice(-6)}
                      </div>
                    </button>
                    {r.status === 'completed' && (
                      <a className="ux-button" href={buildViewerHref(r)} target="_blank" rel="noreferrer">
                        3D ↗
                      </a>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
