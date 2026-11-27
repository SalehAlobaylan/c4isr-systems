import { useEffect, useMemo, useRef, useState } from 'react'
import type { FeatureCollection } from 'geojson'
import { LayoutGridIcon, RefreshIcon } from '@/components/icons'
import {
  buildBenchOverlay,
  fetchBenchEnvironments,
  fetchBenchHealth,
  fetchBenchLeaderboard,
  fetchBenchModels,
  runBenchmarkApi,
  runMonteCarloApi,
  startLocalBenchService,
  type BenchEnvironment,
  type BenchHealth,
  type BenchModelsStatus,
  type BenchmarkReport,
  type MonteCarloResult,
} from './benchClient'

interface BenchmarksPanelProps {
  pickingPoint: boolean
  onSetPickingPoint: (active: boolean) => void
  onOverlayChange: (overlay: FeatureCollection) => void
  onFitBounds: (bounds: [number, number, number, number]) => void
  onNotice: (message: string) => void
  registerMapClick: (handler: ((point: [number, number]) => void) | null) => void
}

export function BenchmarksPanel({
  pickingPoint,
  onSetPickingPoint,
  onOverlayChange,
  onFitBounds,
  onNotice,
  registerMapClick,
}: BenchmarksPanelProps) {
  const [health, setHealth] = useState<BenchHealth | null>(null)
  const [envs, setEnvs] = useState<BenchEnvironment[]>([])
  const [models, setModels] = useState<BenchModelsStatus | null>(null)
  const [leaderboard, setLeaderboard] = useState<BenchmarkReport | null>(null)
  const [envId, setEnvId] = useState('')
  const [point, setPoint] = useState<[number, number] | null>(null)
  const [interceptAglM, setInterceptAglM] = useState(550)
  const [threatSpeedMps, setThreatSpeedMps] = useState(125)
  const [trials, setTrials] = useState(120)
  const [windSigma, setWindSigma] = useState(4.0)
  const [windAddE, setWindAddE] = useState(10)
  const [mcResult, setMcResult] = useState<MonteCarloResult | null>(null)
  const [mcBusy, setMcBusy] = useState(false)
  const [benchBusy, setBenchBusy] = useState(false)
  const [loading, setLoading] = useState(true)
  const [starting, setStarting] = useState(false)
  const [error, setError] = useState('')
  const hasFitted = useRef(false)

  const selectedEnv = useMemo(() => envs.find((e) => e.id === envId) ?? envs[0], [envs, envId])

  const refreshService = async (fitOnFirst = false, allowAutoStart = false) => {
    setLoading(true)
    setError('')
    try {
      const [h, envList, mod, lb] = await Promise.all([
        fetchBenchHealth(),
        fetchBenchEnvironments(),
        fetchBenchModels(),
        fetchBenchLeaderboard().catch(() => null),
      ])
      setHealth(h)
      setEnvs(envList)
      setModels(mod)
      setLeaderboard(lb)
      if (envList.length > 0) {
        const chosen = envList.find((e) => e.id === envId) ?? envList[0]
        setEnvId(chosen.id)
        setPoint((prev) => prev ?? chosen.centerLonLat)
        if (fitOnFirst && !hasFitted.current) {
          hasFitted.current = true
          onFitBounds(chosen.boundsWgs84)
        }
      }
    } catch (e) {
      if (allowAutoStart) {
        setStarting(true)
        const boot = await startLocalBenchService()
        setStarting(false)
        if (boot.ok) {
          await refreshService(fitOnFirst, false)
          return
        }
        setHealth(null)
        setError(boot.error || 'Could not start Benchmark service automatically.')
      } else {
        setHealth(null)
        setError(e instanceof Error ? e.message : 'Local Benchmark service offline (:8768).')
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refreshService(true, true)
    return () => {
      registerMapClick(null)
      onSetPickingPoint(false)
      onOverlayChange({ type: 'FeatureCollection', features: [] })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    onOverlayChange(buildBenchOverlay(selectedEnv, point, mcResult))
  }, [selectedEnv, point, mcResult, onOverlayChange])

  useEffect(() => {
    if (!pickingPoint || !selectedEnv) {
      registerMapClick(null)
      return
    }
    registerMapClick(([lng, lat]) => {
      setPoint([Number(lng.toFixed(6)), Number(lat.toFixed(6))])
      onSetPickingPoint(false)
      onNotice(`Monte Carlo intercept point set to ${lat.toFixed(4)}°, ${lng.toFixed(4)}°`)
    })
    return () => registerMapClick(null)
  }, [pickingPoint, selectedEnv, registerMapClick, onSetPickingPoint, onNotice])

  const handleRunMC = async () => {
    if (!selectedEnv || !point) return
    setMcBusy(true)
    setError('')
    try {
      const res = await runMonteCarloApi({
        environmentId: selectedEnv.id,
        lon: point[0],
        lat: point[1],
        interceptAglM,
        threatSpeedMps,
        threatHeadingDeg: 45,
        trials,
        windSigmaMps: windSigma,
        windAddEnuMps: [windAddE, -3, 0],
      })
      setMcResult(res)
      onNotice(
        `Monte Carlo (${res.totalTrajectories.toLocaleString()} trajs in ${res.elapsedS}s): P95 ${res.buildingHits.p95} bldg hits`,
      )
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Monte Carlo ensemble failed.')
    } finally {
      setMcBusy(false)
    }
  }

  const handleRunBenchmark = async () => {
    setBenchBusy(true)
    setError('')
    try {
      const rep = await runBenchmarkApi({
        biomes: envs.slice(0, 3).map((e) => e.id),
        seeds: [1001, 2002],
      })
      setLeaderboard(rep)
      onNotice(`Multi-biome AI benchmark completed in ${rep.elapsedS}s.`)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Benchmark failed.')
    } finally {
      setBenchBusy(false)
    }
  }

  return (
    <div className="map-drawer-pane">
      {/* 1. Service & ML Models Status */}
      <div className="map-card">
        <div className="map-panel-section-head">
          <h3>1. Monte Carlo &amp; AI Models</h3>
          <button type="button" className="ux-button" onClick={() => void refreshService(false)} disabled={loading}>
            <RefreshIcon className="size-3.5" />
          </button>
        </div>

        {health ? (
          <div className="space-y-1.5 text-xs">
            <p className="ux-muted">
              <strong>Bench Ready</strong> · <code>{health.engineVersion.replace('emad-bench-py-', 'v')}</code>
            </p>
            <div className="grid grid-cols-2 gap-1.5">
              <div className="rounded border border-slate-300/60 p-1.5">
                <div className="text-[10px] uppercase ux-muted">ML Debris Surrogate</div>
                <div className="font-semibold">
                  {models?.debrisSurrogate.available ? 'Active (<1 ms)' : 'Not trained'}
                </div>
                {models?.debrisSurrogate.metadata && (
                  <div className="text-[10px] font-mono ux-muted">
                    R² {models.debrisSurrogate.metadata.valR2DriftE?.toFixed(2)} · {models.debrisSurrogate.metadata.inferenceMs}ms
                  </div>
                )}
              </div>
              <div className="rounded border border-slate-300/60 p-1.5">
                <div className="text-[10px] uppercase ux-muted">RL AI WTA Policy</div>
                <div className="font-semibold">
                  {models?.aiWtaPolicy.available ? 'Trained (Live)' : 'Not trained'}
                </div>
                {models?.aiWtaPolicy.metadata?.validation?.ai_policy && (
                  <div className="text-[10px] font-mono ux-muted">
                    Val {models.aiWtaPolicy.metadata.validation.ai_policy.meanScore} pts
                  </div>
                )}
              </div>
            </div>
          </div>
        ) : starting || loading ? (
          <p className="ux-muted text-xs">Starting unified Simulation Hub on <code>127.0.0.1:8765</code>…</p>
        ) : (
          <div className="ux-notice space-y-2 text-xs" role="alert">
            <p><strong>Benchmark service offline.</strong></p>
            {error && <p>{error}</p>}
            <button
              type="button"
              className="ux-button"
              data-primary="true"
              onClick={async () => {
                setStarting(true)
                const b = await startLocalBenchService()
                setStarting(false)
                if (b.ok) await refreshService(true, false)
                else setError(b.error || 'Start failed')
              }}
            >
              Start Benchmark Service
            </button>
          </div>
        )}
      </div>

      {/* 2. Monte Carlo Debris Probability Contours */}
      {selectedEnv && (
        <div className="map-card">
          <div className="map-panel-section-head">
            <h3>2. Monte Carlo Debris Risk Map</h3>
          </div>
          <label className="block">
            <span>Verified biome</span>
            <select
              className="atlas-field"
              value={selectedEnv.id}
              onChange={(e) => {
                const next = envs.find((x) => x.id === e.target.value)
                if (!next) return
                setEnvId(next.id)
                setPoint(next.centerLonLat)
                setMcResult(null)
                onFitBounds(next.boundsWgs84)
              }}
            >
              {envs.map((env) => (
                <option key={env.id} value={env.id}>{env.title}</option>
              ))}
            </select>
          </label>
          <div className="flex flex-wrap gap-1.5">
            <button
              type="button"
              className="ux-button"
              data-primary={pickingPoint ? 'true' : undefined}
              onClick={() => onSetPickingPoint(!pickingPoint)}
            >
              {pickingPoint ? 'Click Map for Intercept…' : 'Pick Intercept Point'}
            </button>
            <button type="button" className="ux-button" onClick={() => onFitBounds(selectedEnv.boundsWgs84)}>
              Zoom Biome
            </button>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <label className="block">
              <span>Intercept AGL (m)</span>
              <input type="number" className="atlas-field font-mono" value={interceptAglM} onChange={(e) => setInterceptAglM(Number(e.target.value) || 500)} />
            </label>
            <label className="block">
              <span>Threat Speed (m/s)</span>
              <input type="number" className="atlas-field font-mono" min={30} max={900} step={15} value={threatSpeedMps} onChange={(e) => setThreatSpeedMps(Number(e.target.value) || 125)} />
            </label>
            <label className="block">
              <span>Ensemble Trials (N)</span>
              <input type="number" className="atlas-field font-mono" min={20} max={300} step={20} value={trials} onChange={(e) => setTrials(Number(e.target.value) || 120)} />
            </label>
            <label className="block">
              <span>Wind Uncertainty σ (m/s)</span>
              <input type="number" className="atlas-field font-mono" min={0.5} max={15} step={0.5} value={windSigma} onChange={(e) => setWindSigma(Number(e.target.value) || 3.5)} />
            </label>
            <label className="block col-span-2">
              <span>Crosswind +East (m/s)</span>
              <input type="number" className="atlas-field font-mono" value={windAddE} onChange={(e) => setWindAddE(Number(e.target.value) || 0)} />
            </label>
          </div>
          <button
            type="button"
            className="ux-button w-full justify-center"
            data-primary="true"
            disabled={!health || mcBusy}
            onClick={() => void handleRunMC()}
          >
            <LayoutGridIcon className="size-4" />
            {mcBusy ? 'Integrating Monte Carlo Ensemble…' : 'Compute 2D Probability Risk Contours'}
          </button>

          {mcResult && (
            <div className="space-y-1.5 pt-2 border-t border-slate-300/40 text-xs">
              <div className="font-mono text-[11px]">
                <strong>{mcResult.totalTrajectories.toLocaleString()}</strong> trajectories in {mcResult.elapsedS}s · Drift{' '}
                <strong>{mcResult.meanDriftM}m</strong>
              </div>
              <div className="grid grid-cols-2 gap-1.5 font-mono text-[11px]">
                <div className="rounded border border-slate-300/60 p-1.5">
                  <div className="ux-muted text-[10px]">Bldg Hits (Mean / P95)</div>
                  <strong>{mcResult.buildingHits.mean} / {mcResult.buildingHits.p95}</strong>
                </div>
                <div className="rounded border border-slate-300/60 p-1.5">
                  <div className="ux-muted text-[10px]">WorldPop (Mean / P95)</div>
                  <strong>~{mcResult.populationExposed.mean} / ~{mcResult.populationExposed.p95}</strong>
                </div>
              </div>
              <div className="text-[11px] ux-muted">
                Risk footprint: ≥5% <strong>{mcResult.riskArea5PctKm2} km²</strong> · ≥20%{' '}
                <strong>{mcResult.riskArea20PctKm2} km²</strong> · Peak P(hit){' '}
                <strong>{Math.round(mcResult.maxCellHitProbability * 100)}%</strong>
              </div>
              {mcResult.surrogatePreview && (
                <div className="rounded border border-sky-400/40 bg-sky-500/10 p-1.5 text-[11px]">
                  <strong>ML Surrogate ({mcResult.surrogatePreview.inferenceMs} ms):</strong> Drift{' '}
                  {mcResult.surrogatePreview.driftDistanceM}m · Risk{' '}
                  {Math.round(mcResult.surrogatePreview.riskScore * 100)}% · Urban{' '}
                  {Math.round(mcResult.surrogatePreview.urbanExposureFraction * 100)}%
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* 3. Multi-Biome AI WTA Leaderboard */}
      <div className="map-card">
        <div className="map-panel-section-head">
          <h3>3. Multi-Biome AI WTA Leaderboard</h3>
          <button
            type="button"
            className="ux-button"
            disabled={!health || benchBusy}
            onClick={() => void handleRunBenchmark()}
          >
            {benchBusy ? 'Running…' : 'Re-Run Suite'}
          </button>
        </div>
        {error && <p className="ux-notice text-xs" role="alert">{error}</p>}
        {leaderboard ? (
          <div className="space-y-1.5 text-xs">
            <p className="ux-muted text-[11px]">
              Biomes: {leaderboard.biomes.join(', ')} · {leaderboard.seeds.length} seeds · {leaderboard.elapsedS}s
            </p>
            <div className="space-y-1 font-mono text-[11px]">
              {leaderboard.leaderboard.map((row, idx) => (
                <div
                  key={row.policy}
                  className="rounded border border-slate-300/60 p-2 space-y-0.5"
                  style={idx === 0 ? { borderColor: '#10b981', background: 'rgba(16,185,129,0.08)' } : undefined}
                >
                  <div className="flex items-center justify-between">
                    <strong>#{idx + 1} {row.policy.replace('_', ' ')}</strong>
                    <strong className={idx === 0 ? 'text-emerald-700' : ''}>{row.meanCompositeScore} pts</strong>
                  </div>
                  <div className="ux-muted text-[10px]">
                    Kill {row.meanDefenseSuccessPct}% · {row.meanLeakers} leak · ${row.meanCostKUsd}k · {row.meanDecoyWaste} decoy · {row.totalAirspaceViolations} air viol
                  </div>
                </div>
              ))}
            </div>
          </div>
        ) : (
          <p className="ux-muted text-xs">No saved benchmark scorecard yet. Click "Re-Run Suite" to benchmark Greedy vs WTA vs AI Policy.</p>
        )}
      </div>
    </div>
  )
}
