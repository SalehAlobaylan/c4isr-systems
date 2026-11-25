import { useRef, useReducer, type ReactNode } from 'react'
import { IntelligencePage } from '@/routes/intelligence'
import { WeatherReportsPage } from '@/routes/weather-reports'
import { ExternalTracksPage } from '@/routes/external-tracks'
import {
  WorkbenchContext,
  toolPermissions,
  type WorkbenchHost,
  type MapTool,
  type WorkbenchSearch,
} from './WorkbenchContext'
import { scopePeriodLabel } from './AnalystScopeProvider'
import { useWorkspace } from '@/lib/workspaceContext'
import { PublicSystemsWorkspace } from './SystemsWorkspace'
import { SimulationGateway } from './SimulationGateway'
import { EnvironmentLibrary } from './EnvironmentLibrary'

const groupFor = (tool?: MapTool) => (tool === 'review' || tool === 'context' ? 'imagery' : tool)
type Entry = { host: WorkbenchHost; routeKey?: string; tool: MapTool; pinned: boolean }
const cleared = (search: WorkbenchSearch, bbox?: string) => ({
  ...search,
  bbox,
  before: undefined,
  after: undefined,
  job: undefined,
  assessment: undefined,
  id: undefined,
  display: undefined,
  manifest: undefined,
  feature: undefined,
  layers: undefined,
  basemap: undefined,
  atlasArea: undefined,
  terrainPoint: undefined,
})

export function MapWorkbenches({
  tool,
  host,
  routeKey,
  geography,
}: {
  tool?: MapTool
  host: WorkbenchHost
  routeKey?: string
  geography: ReactNode
}) {
  const { session } = useWorkspace()
  const entries = useRef(new Map<string, Entry>())
  const active = groupFor(tool)
  const current = useRef(active)
  const [, refresh] = useReducer((v) => v + 1, 0)
  const entering = current.current !== active
  current.current = active
  const adopt = (entry: Entry) => ({ ...host, search: cleared(entry.host.search, host.search.bbox) })

  if (active && tool) {
    const old = entries.current.get(active)
    const pinned = old?.pinned ?? active === 'imagery'
    let value = old?.host ?? host
    if (old && !pinned && value.selectionKey !== host.selectionKey) value = adopt(old)
    if (value.selectionKey === host.selectionKey && !value.scope && host.scope) {
      value = {
        ...value,
        scope: host.scope,
        scopeInput: host.scopeInput,
        regionName: host.regionName,
        operationGeometry: host.operationGeometry,
        areaError: host.areaError,
      }
    }
    if (!pinned && host.scope?.scopeHash !== value.scope?.scopeHash) value = { ...host, search: value.search }
    if (!old || old.routeKey !== routeKey || old.tool !== tool) {
      const nextSearch =
        old && !entering && old.tool === tool && old.routeKey !== routeKey
          ? host.search
          : routeKey
            ? { ...value.search, ...host.search, bbox: old ? value.search.bbox : host.search.bbox }
            : {
                ...value.search,
                view:
                  tool === 'review'
                    ? 'review'
                    : tool === 'context'
                      ? 'context'
                      : tool === 'imagery'
                        ? 'compare'
                        : value.search.view,
              }
      value = { ...value, search: nextSearch }
    }
    entries.current.set(active, {
      tool,
      routeKey,
      pinned,
      host: {
        ...value,
        close: host.close,
        update: (search, replace) => {
          const entry = entries.current.get(active)!
          entry.host = { ...entry.host, search }
          if (current.current === active) host.update(search, replace)
          else refresh()
        },
        open: (next, search) => {
          const target = groupFor(next)!
          const origin = entries.current.get(active)!
          if (target !== active && !entries.current.has(target)) {
            entries.current.set(target, {
              tool: next,
              pinned: true,
              host: { ...origin.host, search: search ?? {}, visible: true },
            })
          }
          host.open(next, search)
        },
      },
    })
  } else {
    // Prune closed unpinned workbenches so hidden DOM does not grow unboundedly,
    // while keeping pinned analytical drafts (such as imagery captures) alive.
    for (const [key, entry] of entries.current) {
      if (!entry.pinned) entries.current.delete(key)
    }
  }

  return (
    <>
      {[...entries.current].map(([key, entry]) => {
        const differs =
          entry.host.selectionKey !== host.selectionKey ||
          entry.host.scope?.scopeHash !== host.scope?.scopeHash
        const visible = key === active
        return (
          <div key={key} hidden={!visible} className="atlas-workbench-instance">
            <WorkbenchContext.Provider value={{ ...entry.host, visible }}>
              <div className="atlas-workbench-scope">
                <div className="atlas-workbench-scope-bar">
                  <span className="atlas-workbench-scope-summary">
                    <strong>{entry.pinned ? 'Pinned draft' : 'Following map'}</strong> · {entry.host.regionName}
                    <details open={entry.host.scope?.temporal.mode === 'historical'}>
                      <summary>Area and time details</summary>
                      {scopePeriodLabel(entry.host.scope)}
                    </details>
                  </span>
                  {visible && (
                    <button
                      type="button"
                      className="atlas-workbench-pin-btn"
                      onClick={() => {
                        entries.current.set(key, {
                          ...entry,
                          pinned: !entry.pinned,
                          host: entry.pinned ? adopt(entry) : entry.host,
                        })
                        refresh()
                      }}
                    >
                      {entry.pinned ? 'Follow current map scope' : 'Pin this scope'}
                    </button>
                  )}
                </div>
                {visible && entry.pinned && differs && (
                  <div className="atlas-workbench-scope-drift">
                    <p>
                      Adopting the map scope replaces area and period, clears scene/pair choices and export
                      previews, and clears the selected result. Completed records retain their original inputs.
                    </p>
                    <button
                      type="button"
                      className="ux-button"
                      onClick={() => {
                        const next = adopt(entry)
                        entries.current.set(key, { ...entry, host: next })
                        host.update(next.search)
                        refresh()
                      }}
                    >
                      Use current map scope
                    </button>
                  </div>
                )}
              </div>
              {entry.host.areaError && <p role="alert">{entry.host.areaError}</p>}
              {toolPermissions[entry.tool] && !session.permissions.includes(toolPermissions[entry.tool]!) ? (
                <p role="status" className="ux-notice">
                  This capability requires {toolPermissions[entry.tool]}. Other permitted map tools remain available.
                </p>
              ) : key === 'systems' ? (
                <PublicSystemsWorkspace />
              ) : key === 'investigations' ? (
                <PublicSystemsWorkspace initialTab="investigations" />
              ) : key === 'coverage' ? (
                <PublicSystemsWorkspace initialTab="coverage" />
              ) : key === 'sources' ? (
                <PublicSystemsWorkspace initialTab="sources" />
              ) : key === 'jobs' ? (
                <PublicSystemsWorkspace initialTab="jobs" />
              ) : key === 'imagery' ? (
                <IntelligencePage />
              ) : key === 'weather' ? (
                <WeatherReportsPage />
              ) : key === 'tracks' ? (
                <ExternalTracksPage />
              ) : key === 'simulation' ? (
                <SimulationGateway />
              ) : key === 'environments' ? (
                visible ? (
                  <EnvironmentLibrary />
                ) : null
              ) : (
                geography
              )}
            </WorkbenchContext.Provider>
          </div>
        )
      })}
    </>
  )
}
