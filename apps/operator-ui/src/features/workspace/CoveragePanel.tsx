import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  fixedScopeInput,
  useAPI,
  type AcquisitionInput,
  type CoverageAction,
  type CoverageResult,
  type InventorySource,
  type ScopeSnapshot,
} from '@/lib/api'
import { useWorkspace } from '@/lib/workspaceContext'
import { scopePeriodLabel, useAnalystScope } from './AnalystScopeProvider'
import {
  formatFreshness,
  formatSpatialRelation,
  partitionCoverageSources,
  sourceReadinessChip,
  type CoverageSource,
} from './coverageClassification'
import {
  formatInventoryDateRange,
  inventoryAreaForBounds,
  scopeForInventorySource,
  useCoverageInventory,
} from './useCoverageInventory'
import '@/styles/scope-workflows.css'

export function useCoverage(scope?: ScopeSnapshot, enabled = true) {
  const api = useAPI()
  const { prefix, session } = useWorkspace()
  const historical = scope?.temporal.mode === 'historical'
  return useQuery({
    queryKey: [...prefix, 'scope-coverage', scope?.scopeHash, scope?.authorizationRevision],
    queryFn: ({ signal }) => api.queryCoverage({ scope: fixedScopeInput(scope!) }, signal),
    gcTime: 0,
    enabled: enabled && !!scope && session.permissions.includes('sources.read'),
    retry: false,
    staleTime: historical ? Infinity : 30_000,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    refetchOnMount: !historical,
    refetchInterval: historical ? false : 60_000,
  })
}

export function quietMeaning(result: CoverageResult | undefined, family: string, unavailable = false) {
  if (unavailable) return 'This query is unavailable. The record count is unknown; other layers remain usable.'
  const sources = result?.sources.filter((s) => s.family === family) ?? []
  if (!sources.length) {
    return 'No coverage evaluation is available for this family. Missing records do not establish absence.'
  }
  if (sources.every((s) => s.support === 'not_admitted' || s.support === 'not_configured')) {
    return 'No admitted source is active for this family yet. Review provider prerequisites and activation in Sources & access.'
  }
  if (sources.every((s) => s.support === 'unsupported')) {
    return 'This family cannot reconstruct the selected historical period. Choose compatible dates or inspect exact retained evidence.'
  }
  if (sources.some((s) => s.dataResult === 'unavailable_query')) {
    return 'A source query is unavailable. Its count is unknown; inspect source access and recovery status.'
  }
  if (sources.every((s) => s.dataResult === 'successful_empty_retrieval')) {
    return 'Completed provider queries returned no matching retained records for this area and interval. This does not establish factual absence.'
  }
  if (sources.some((s) => s.retrieval === 'partial_retrieval')) {
    return 'Retrieval is partial: pages, normalization or publication may be incomplete. Inspect receipts and queued work.'
  }
  if (sources.every((s) => s.retrieval === 'no_retrieval')) {
    return 'No completed retrieval is recorded for this area and period. Inspect approved coverage or request acquisition.'
  }
  return 'No matching retained records are displayed. Review retrieval coverage, dates, source freshness and filters.'
}

type SmartItemState = {
  state: 'submitting' | 'saved' | 'failed'
  requestId?: string
  statusText?: string
  message?: string
}

const actionKey = (a: CoverageAction) => `${a.sourceId}/${a.profileKey}/${a.profileRevisionId}`

export function CoveragePanel({
  scope: provided,
  active = true,
  compact = false,
}: {
  scope?: ScopeSnapshot
  active?: boolean
  compact?: boolean
}) {
  const analyst = useAnalystScope()
  const scope = provided ?? analyst?.snapshot
  const api = useAPI()
  const { prefix, session, signal } = useWorkspace()
  const coverage = useCoverage(scope, active)
  const inventory = useCoverageInventory(active)

  // Smart Fetch checklist state (eligible request_acquisition actions)
  const [checkedFetchKeys, setCheckedFetchKeys] = useState<string[]>([])
  const [checkedReviewKeys, setCheckedReviewKeys] = useState<string[]>([])
  const [smartItemStates, setSmartItemStates] = useState<Record<string, SmartItemState>>({})
  const [smartBusy, setSmartBusy] = useState(false)
  const [smartNotice, setSmartNotice] = useState('')
  const [reviewBusy, setReviewBusy] = useState(false)
  const [reviewNotice, setReviewNotice] = useState('')
  const batchIntentsRef = useRef<Record<string, AcquisitionInput>>({})

  // Single-profile manual intent state (power / audit disclosure)
  const [selected, setSelected] = useState('')
  const [status, setStatus] = useState('')
  const [busy, setBusy] = useState(false)
  const [last, setLast] = useState('')
  const [manualOpen, setManualOpen] = useState(false)
  const intent = useRef<AcquisitionInput | undefined>(undefined)
  const acquisitionFormRef = useRef<HTMLFieldSetElement | null>(null)

  const canRequest = session.permissions.includes('acquisitions.request')
  const canRead = session.permissions.includes('acquisitions.read')

  const requests = useQuery({
    queryKey: [...prefix, 'acquisition-requests'],
    queryFn: ({ signal }) => api.listAcquisitions(undefined, signal),
    enabled: active && canRead,
    refetchInterval: active ? 15_000 : false,
  })

  const request = useQuery({
    queryKey: [...prefix, 'acquisition-request', last],
    queryFn: ({ signal }) => api.getAcquisition(last, signal),
    enabled: active && canRead && !!last,
    refetchInterval: active ? 5_000 : false,
    retry: false,
  })

  // Reset all workspace-scoped intent state when workspace/environment changes
  useEffect(() => {
    intent.current = undefined
    batchIntentsRef.current = {}
    setLast('')
    setSelected('')
    setStatus('')
    setBusy(false)
    setSmartItemStates({})
    setSmartNotice('')
    setReviewNotice('')
    setCheckedFetchKeys([])
    setCheckedReviewKeys([])
  }, [JSON.stringify(prefix)])

  const sources = coverage.data?.sources ?? []
  const sourceById = useMemo(() => new Map(sources.map((s) => [s.id, s])), [sources])
  const inventoryBySource = useMemo(
    () => new Map<string, InventorySource>((inventory.data?.sources ?? []).map((s) => [s.sourceId, s])),
    [inventory.data],
  )

  const actions = useMemo(
    () =>
      sources.flatMap((source) =>
        source.actions.filter((a) => ['request_acquisition', 'request_coverage_review'].includes(a.kind)),
      ),
    [sources],
  )

  const fetchActions = useMemo(
    () => actions.filter((a) => a.kind === 'request_acquisition'),
    [actions],
  )
  const reviewActions = useMemo(
    () => actions.filter((a) => a.kind === 'request_coverage_review'),
    [actions],
  )

  const fetchActionSignature = useMemo(
    () => `${scope?.scopeHash ?? ''}:${fetchActions.map(actionKey).join('|')}`,
    [scope?.scopeHash, fetchActions],
  )

  // Pre-check all eligible fetchable sources whenever the evaluated scope or eligible list changes
  useEffect(() => {
    batchIntentsRef.current = {}
    setSmartItemStates({})
    setSmartNotice('')
    setReviewNotice('')
    setCheckedFetchKeys(fetchActions.map(actionKey))
    setCheckedReviewKeys([])
  }, [fetchActionSignature])

  const toggleFetchKey = (key: string) => {
    setCheckedFetchKeys((prev) => (prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key]))
  }

  const toggleReviewKey = (key: string) => {
    setCheckedReviewKeys((prev) => (prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key]))
  }

  const submitSmartBatch = async (targetKeys: string[]) => {
    if (!scope || targetKeys.length === 0) return
    const targetActions = fetchActions.filter((a) => targetKeys.includes(actionKey(a)))
    if (targetActions.length === 0) return

    const frozenScope = fixedScopeInput(scope)
    setSmartBusy(true)
    setSmartNotice('')

    let savedCount = 0
    let failedCount = 0
    let latestId = ''
    let lastErrorMessage = ''

    for (const action of targetActions) {
      if (!action.sourceId || !action.profileKey || !action.profileRevisionId) continue
      const key = actionKey(action)
      if (!batchIntentsRef.current[key]) {
        batchIntentsRef.current[key] = {
          requestKey: crypto.randomUUID(),
          sourceId: action.sourceId,
          profileKey: action.profileKey,
          profileRevisionId: action.profileRevisionId,
          purpose: 'discovery',
          scope: frozenScope,
        }
      }
      setSmartItemStates((prev) => ({
        ...prev,
        [key]: { state: 'submitting', message: 'Saving durable intent…' },
      }))
      try {
        const saved = await api.createAcquisition(batchIntentsRef.current[key])
        if (signal.aborted) return
        savedCount++
        latestId = saved.id
        delete batchIntentsRef.current[key]
        setSmartItemStates((prev) => ({
          ...prev,
          [key]: {
            state: 'saved',
            requestId: saved.id,
            statusText: saved.status,
            message: saved.reason,
          },
        }))
      } catch (e) {
        if (signal.aborted) return
        failedCount++
        lastErrorMessage = e instanceof Error ? e.message : 'Submission unavailable'
        setSmartItemStates((prev) => ({
          ...prev,
          [key]: {
            state: 'failed',
            message: `${lastErrorMessage}. Retry preserves the original area, interval and request key.`,
          },
        }))
      }
    }

    if (signal.aborted) return
    if (latestId) setLast(latestId)
    if (savedCount > 0 && failedCount === 0) {
      setSmartNotice(
        `Queued ${savedCount} ${savedCount === 1 ? 'acquisition intent' : 'acquisition intents'} for worker execution. Retained records will appear automatically as retrieval completes.`,
      )
    } else if (savedCount > 0 && failedCount > 0) {
      setSmartNotice(
        `Saved ${savedCount}, while ${failedCount} could not be confirmed (${lastErrorMessage}). Retry failed items to reuse their exact frozen intents and request keys.`,
      )
    } else if (failedCount > 0) {
      setSmartNotice(
        `${lastErrorMessage}. Retry preserves each pinned area, interval and request key.`,
      )
    }

    if (savedCount > 0) {
      void requests.refetch()
      void coverage.refetch()
      void inventory.refetch()
    }
    setSmartBusy(false)
  }

  const submitCoverageReviews = async () => {
    if (!scope || checkedReviewKeys.length === 0) return
    const targets = reviewActions.filter((a) => checkedReviewKeys.includes(actionKey(a)))
    if (targets.length === 0) return

    const frozenScope = fixedScopeInput(scope)
    setReviewBusy(true)
    setReviewNotice('')

    let savedCount = 0
    let failedCount = 0
    let latestId = ''
    let lastErr = ''

    for (const action of targets) {
      if (!action.sourceId || !action.profileKey || !action.profileRevisionId) continue
      const key = `review:${actionKey(action)}`
      if (!batchIntentsRef.current[key]) {
        batchIntentsRef.current[key] = {
          requestKey: crypto.randomUUID(),
          sourceId: action.sourceId,
          profileKey: action.profileKey,
          profileRevisionId: action.profileRevisionId,
          purpose: 'discovery',
          scope: frozenScope,
        }
      }
      try {
        const saved = await api.createAcquisition(batchIntentsRef.current[key])
        if (signal.aborted) return
        savedCount++
        latestId = saved.id
        delete batchIntentsRef.current[key]
      } catch (e) {
        if (signal.aborted) return
        failedCount++
        lastErr = e instanceof Error ? e.message : 'Review request unavailable'
      }
    }

    if (signal.aborted) return
    if (latestId) setLast(latestId)
    if (savedCount > 0 && failedCount === 0) {
      setCheckedReviewKeys([])
      setReviewNotice(
        `Saved ${savedCount} coverage expansion ${savedCount === 1 ? 'request' : 'requests'} for source administrator review. No provider retrieval was started.`,
      )
      void requests.refetch()
      void coverage.refetch()
    } else {
      setReviewNotice(
        savedCount > 0
          ? `Saved ${savedCount} coverage review requests; ${failedCount} failed (${lastErr}).`
          : `Coverage review request failed (${lastErr}). Retry preserves the original request key.`,
      )
    }
    setReviewBusy(false)
  }

  const submitManual = async () => {
    if (!scope) return
    const action = actions.find((a) => actionKey(a) === selected)
    if (!intent.current) {
      if (!action?.sourceId || !action.profileKey || !action.profileRevisionId) return
      intent.current = {
        requestKey: crypto.randomUUID(),
        sourceId: action.sourceId,
        profileKey: action.profileKey,
        profileRevisionId: action.profileRevisionId,
        purpose: 'discovery',
        scope: fixedScopeInput(scope),
      }
    }
    setBusy(true)
    try {
      const saved = await api.createAcquisition(intent.current)
      if (signal.aborted) return
      setLast(saved.id)
      setStatus(
        `${saved.kind === 'COVERAGE_REVIEW' ? 'Coverage review saved' : 'Acquisition saved'} · ${saved.status}. ${saved.reason}`,
      )
      void requests.refetch()
      void coverage.refetch()
      void inventory.refetch()
    } catch (e) {
      if (signal.aborted) return
      setStatus(
        `${e instanceof Error ? e.message : 'Submission unavailable'}. Retry preserves this intent, its original area, interval and request key.`,
      )
    } finally {
      if (!signal.aborted) setBusy(false)
    }
  }

  const failedSmartKeys = useMemo(
    () => Object.entries(smartItemStates).filter(([, s]) => s.state === 'failed').map(([k]) => k),
    [smartItemStates],
  )

  const {
    ready: readySources,
    gaps: gapSources,
    outsideArea: outsideSources,
    notAdmitted: unadmittedSources,
    applicableCount,
  } = partitionCoverageSources(sources)
  const readyCount = readySources.length

  const verdictTone: 'positive' | 'caution' | 'critical' | undefined = coverage.isError
    ? 'critical'
    : coverage.data?.storage === 'blocked'
      ? 'critical'
      : gapSources.length > 0 || sources.length === 0
        ? 'caution'
        : applicableCount > 0
          ? 'positive'
          : undefined

  const verdictTitle = coverage.isPending
    ? 'Evaluating selected area and period…'
    : coverage.isError
      ? 'Coverage query unavailable'
      : coverage.data?.storage === 'blocked'
        ? 'New acquisition paused · storage blocked'
        : sources.length === 0
          ? 'No authorized sources configured'
          : applicableCount === 0
            ? outsideSources.length > 0
              ? 'No configured sources overlap this area'
              : 'No admitted sources active yet'
            : gapSources.length === 0
              ? `All ${readyCount} in-scope ${readyCount === 1 ? 'source' : 'sources'} ready`
              : `Partial readiness · ${readyCount} of ${applicableCount} sources ready`

  const footprints = inventory.data?.footprints ?? []
  const storageBlocked = coverage.data?.storage === 'blocked'

  const renderSourceCard = (source: CoverageSource) => {
    const chip = sourceReadinessChip(source)
    const primaryAction = source.actions[0]
    const inv = inventoryBySource.get(source.id)
    const fetchAction = source.actions.find((a) => a.kind === 'request_acquisition')
    const isStaticOrUnadmitted =
      source.support === 'retained_reference' ||
      source.support === 'not_admitted' ||
      source.support === 'not_configured'

    return (
      <article key={source.id} className="coverage-source-card">
        <header className="coverage-source-head">
          <strong>{source.name}</strong>
          <span className="coverage-chip" data-tone={chip.tone}>
            <span aria-hidden="true">{chip.glyph}</span>
            {chip.label}
          </span>
        </header>
        <p className="ux-muted">
          {source.support === 'not_admitted' || source.support === 'not_configured' ? (
            <>
              Family: <strong>{source.family}</strong> · Status: {source.support.replaceAll('_', ' ')}
            </>
          ) : (
            <>
              Retained records: <strong>{source.usableRetainedRecords ?? 'unknown'}</strong> · Freshness:{' '}
              {formatFreshness(source.freshness)}
              {!isStaticOrUnadmitted ? ` · Missing windows: ${source.missingWindows.length}` : ''} · Footprint:{' '}
              {formatSpatialRelation(source.spatialRelation)}
            </>
          )}
        </p>
        {source.supportReason && <p>{source.supportReason}</p>}
        {primaryAction && (
          <p>
            {primaryAction.reason} Recovery owner: <strong>{primaryAction.owner}</strong>.
          </p>
        )}

        <div className="flex flex-wrap gap-2">
          {analyst && inv && inv.retainedCount > 0 && (source.usableRetainedRecords ?? 0) === 0 && (
            <button
              type="button"
              className="ux-button"
              onClick={() => analyst.setScope(scopeForInventorySource(inv, analyst.input))}
            >
              Jump to retained ({inv.retainedCount.toLocaleString()} · {formatInventoryDateRange(inv.earliestAt, inv.latestAt)})
            </button>
          )}
          {analyst && source.spatialRelation === 'none' && inv?.bounds && (
            <button
              type="button"
              className="ux-button"
              onClick={() => analyst.changeArea(inventoryAreaForBounds(inv.bounds!, inv.name))}
            >
              Jump to approved area
            </button>
          )}
          {canRequest && fetchAction && !checkedFetchKeys.includes(actionKey(fetchAction)) && (
            <button
              type="button"
              className="ux-button"
              onClick={() => toggleFetchKey(actionKey(fetchAction))}
            >
              Include in Smart Fetch
            </button>
          )}
        </div>

        <details>
          <summary className="cursor-pointer font-medium">
            Source details, profiles &amp; query receipts ({source.receipts.length})
          </summary>
          <div className="mt-2 space-y-1.5">
            <p>
              {source.support.replaceAll('_', ' ')} · data result: {source.dataResult.replaceAll('_', ' ')} · retrieval:{' '}
              {source.retrieval.replaceAll('_', ' ')} · completeness: {source.observationCompleteness}. Provider reports
              are not independently verified facts.
            </p>
            {source.profiles.map((profile) => (
              <p key={profile.key}>
                {profile.key} · {profile.gate} · {formatSpatialRelation(profile.spatialRelation)} · execution{' '}
                {profile.executionGranted ? 'granted' : 'requires approval'}
              </p>
            ))}
            {source.receipts.map((receipt) => (
              <p key={receipt.runId}>
                Query receipt {receipt.runId} ·{' '}
                {receipt.queryComplete ? 'completed query footprint' : 'partial / unavailable'} · pages {receipt.pages} ·
                rejected {receipt.rejectedRecords} · unpublished {receipt.unpublishedRecords}
                {receipt.providerTruncated ? ' · provider response capped' : ''}
              </p>
            ))}
            {source.actions.map((action, i) => (
              <p key={i}>
                {action.reason} Owner: {action.owner}.
                {action.requestId && (
                  <button type="button" onClick={() => setLast(action.requestId!)}>
                    Inspect request {action.requestId}
                  </button>
                )}
              </p>
            ))}
          </div>
        </details>
      </article>
    )
  }

  const details = (
    <div className="scope-workflow coverage-details space-y-3">
      {/* Task-First Readiness Verdict Card */}
      <div className="coverage-verdict" data-tone={verdictTone}>
        <div className="coverage-verdict-head">
          <strong>{verdictTitle}</strong>
          {coverage.data && (
            <span className="coverage-chip" data-tone={storageBlocked ? 'critical' : 'positive'}>
              Storage: {coverage.data.storage}
            </span>
          )}
        </div>
        <p className="ux-muted">{scopePeriodLabel(coverage.data?.scope ?? scope)}</p>
        {coverage.data && sources.length > 0 && (
          <div className="coverage-verdict-pills" aria-label="Coverage breakdown">
            {readyCount > 0 && (
              <span className="coverage-chip" data-tone="positive">
                <span aria-hidden="true">●</span>
                {readyCount} ready
              </span>
            )}
            {fetchActions.length > 0 && (
              <span className="coverage-chip" data-tone="caution">
                <span aria-hidden="true">↓</span>
                {fetchActions.length} fetchable now
              </span>
            )}
            {gapSources.length > fetchActions.length && (
              <span className="coverage-chip" data-tone="caution">
                <span aria-hidden="true">◐</span>
                {gapSources.length} {gapSources.length === 1 ? 'gap' : 'gaps'}
              </span>
            )}
            {outsideSources.length > 0 && (
              <span className="coverage-chip">
                <span aria-hidden="true">◇</span>
                {outsideSources.length} outside area
              </span>
            )}
            {unadmittedSources.length > 0 && (
              <span className="coverage-chip">
                <span aria-hidden="true">◌</span>
                {unadmittedSources.length} not admitted
              </span>
            )}
          </div>
        )}
        {coverage.data && (
          <p role="status" className="ux-muted">
            Storage: {coverage.data.storage} · evaluated {new Date(coverage.data.evaluatedAt).toLocaleString()}.
          </p>
        )}
        {coverage.isError && (
          <p role="alert">
            Coverage query unavailable. Counts and readiness are unknown.{' '}
            <button type="button" onClick={() => void coverage.refetch()}>
              Retry coverage
            </button>
          </p>
        )}
        {storageBlocked && (
          <p role="alert">
            New acquisition is paused. A source administrator must reconcile storage or the approved retention policy.
            Retained history stays available.
          </p>
        )}
        {gapSources.length > 0 && !storageBlocked && (
          <p className="ux-muted">
            {gapSources.length} in-scope {gapSources.length === 1 ? 'source has' : 'sources have'} a coverage or
            retrieval gap for this area and period.
            {fetchActions.length > 0
              ? ' Select eligible sources below to fetch now within approved policy.'
              : ' Review each source card below for its recovery owner.'}
          </p>
        )}
        <div>
          <button type="button" disabled={!active || coverage.isFetching} onClick={() => void coverage.refetch()}>
            Refresh coverage evaluation
          </button>
        </div>
      </div>

      {/* TIER 1: 1-Click Smart Fetch Checklist (eligible within approved policy) */}
      {canRequest && fetchActions.length > 0 && (
        <section className="smart-fetch-card" aria-label="Smart fetch eligible sources">
          <header className="smart-fetch-head">
            <div>
              <h3>Available to fetch now ({fetchActions.length})</h3>
              <p className="ux-muted">
                Pre-approved for this area and period. Each checked source queues one durable, auditable acquisition
                intent.
              </p>
            </div>
            {fetchActions.length > 1 && (
              <button
                type="button"
                className="map-preset-btn"
                disabled={smartBusy}
                onClick={() =>
                  setCheckedFetchKeys(
                    checkedFetchKeys.length === fetchActions.length ? [] : fetchActions.map(actionKey),
                  )
                }
              >
                {checkedFetchKeys.length === fetchActions.length ? 'Clear all' : 'Select all'}
              </button>
            )}
          </header>

          <div className="smart-fetch-list" role="group" aria-label="Eligible sources to fetch">
            {fetchActions.map((action) => {
              const key = actionKey(action)
              const src = action.sourceId ? sourceById.get(action.sourceId) : undefined
              const inv = action.sourceId ? inventoryBySource.get(action.sourceId) : undefined
              const itemState = smartItemStates[key]
              const checked = checkedFetchKeys.includes(key)
              const hasQuota = inv?.dailyBudget != null && inv.dailyBudget > 0
              const attemptsToday = inv?.attemptsToday ?? 0
              const quotaExhausted = hasQuota && attemptsToday >= inv!.dailyBudget!

              return (
                <div key={key} className="smart-fetch-row" data-state={itemState?.state}>
                  <label className="smart-fetch-label">
                    <input
                      type="checkbox"
                      checked={checked}
                      disabled={smartBusy || storageBlocked || itemState?.state === 'saved'}
                      onChange={() => toggleFetchKey(key)}
                    />
                    <span className="smart-fetch-copy">
                      <span className="smart-fetch-title">
                        <strong>{src?.name ?? action.sourceId}</strong>
                        <span className="smart-fetch-profile">{action.profileKey}</span>
                      </span>
                      <span className="smart-fetch-meta">
                        <span>{formatSpatialRelation(src?.spatialRelation ?? 'full')}</span>
                        {src && src.missingWindows.length > 0 && (
                          <>
                            <span aria-hidden="true">·</span>
                            <span>
                              {src.missingWindows.length}{' '}
                              {src.missingWindows.length === 1 ? 'missing window' : 'missing windows'}
                            </span>
                          </>
                        )}
                        {hasQuota && (
                          <>
                            <span aria-hidden="true">·</span>
                            <span className="smart-fetch-quota" data-exhausted={quotaExhausted || undefined}>
                              Today: {attemptsToday}/{inv!.dailyBudget}
                            </span>
                          </>
                        )}
                      </span>
                    </span>
                  </label>

                  {itemState && (
                    <div className="smart-fetch-row-status">
                      <span
                        className="coverage-chip"
                        data-tone={
                          itemState.state === 'saved'
                            ? 'positive'
                            : itemState.state === 'failed'
                              ? 'critical'
                              : undefined
                        }
                      >
                        <span aria-hidden="true">
                          {itemState.state === 'saved' ? '●' : itemState.state === 'failed' ? '!' : '◌'}
                        </span>
                        {itemState.state === 'saved'
                          ? itemState.statusText ?? 'QUEUED'
                          : itemState.state === 'failed'
                            ? 'Retry needed'
                            : 'Saving…'}
                      </span>
                      {itemState.state === 'failed' && (
                        <button
                          type="button"
                          className="map-preset-btn"
                          disabled={smartBusy}
                          onClick={() => void submitSmartBatch([key])}
                        >
                          Retry same intent
                        </button>
                      )}
                    </div>
                  )}
                </div>
              )
            })}
          </div>

          <div className="smart-fetch-actions">
            <button
              type="button"
              className="ux-button"
              data-primary="true"
              disabled={smartBusy || !scope || !active || storageBlocked || checkedFetchKeys.length === 0}
              onClick={() => void submitSmartBatch(checkedFetchKeys)}
            >
              {smartBusy
                ? 'Queuing intents…'
                : `Fetch ${checkedFetchKeys.length} selected ${checkedFetchKeys.length === 1 ? 'source' : 'sources'}`}
            </button>
            {failedSmartKeys.length > 0 && (
              <button
                type="button"
                className="ux-button"
                disabled={smartBusy || storageBlocked}
                onClick={() => void submitSmartBatch(failedSmartKeys)}
              >
                Retry {failedSmartKeys.length} failed with same {failedSmartKeys.length === 1 ? 'intent' : 'intents'}
              </button>
            )}
          </div>

          {smartNotice && (
            <p role="status" className="ux-notice">
              {smartNotice}
            </p>
          )}
        </section>
      )}

      {/* TIER 2: Outside Approved Area — Jump to Covered Footprints OR Request Admin Expansion */}
      {(outsideSources.length > 0 || reviewActions.length > 0) && (
        <section className="smart-outside-card" aria-label="Outside approved area and coverage expansion">
          <div className="smart-fetch-head">
            <div>
              <h3>Outside approved area ({outsideSources.length || reviewActions.length})</h3>
              <p className="ux-muted">
                These sources have approved profiles for specific regions that do not cover your entire selected area.
              </p>
            </div>
          </div>

          {analyst && footprints.length > 0 && (
            <div className="space-y-1.5">
              <p className="text-xs font-semibold">Jump to a covered footprint instead</p>
              <div className="map-radar-chips" role="group" aria-label="Jump to covered area">
                {footprints.map((fp) => (
                  <button
                    key={fp.key}
                    type="button"
                    className="map-radar-chip"
                    title={`Move scope to ${fp.name} (${fp.sourceNames.join(', ')})`}
                    onClick={() => analyst.changeArea(inventoryAreaForBounds(fp.bounds, fp.name))}
                  >
                    <span className="map-radar-chip-label">◇ Jump to {fp.name}</span>
                    <span className="map-radar-chip-count">
                      {fp.sourceIds.length} {fp.sourceIds.length === 1 ? 'src' : 'srcs'}
                    </span>
                  </button>
                ))}
              </div>
            </div>
          )}

          {canRequest && reviewActions.length > 0 && (
            <div className="smart-review-box">
              <p className="text-xs font-semibold">Or request source administrator coverage expansion</p>
              <p className="ux-muted">
                A coverage expansion request asks an administrator to widen a profile&rsquo;s approved area. It does not
                authorize or start provider retrieval.
              </p>
              <div className="smart-fetch-list" role="group" aria-label="Sources eligible for coverage expansion review">
                {reviewActions.map((action) => {
                  const key = actionKey(action)
                  const src = action.sourceId ? sourceById.get(action.sourceId) : undefined
                  return (
                    <label key={key} className="smart-fetch-label">
                      <input
                        type="checkbox"
                        checked={checkedReviewKeys.includes(key)}
                        disabled={reviewBusy || storageBlocked}
                        onChange={() => toggleReviewKey(key)}
                      />
                      <span className="smart-fetch-copy">
                        <span className="smart-fetch-title">
                          <strong>{src?.name ?? action.sourceId}</strong>
                          <span className="smart-fetch-profile">{action.profileKey}</span>
                        </span>
                        <span className="smart-fetch-meta">{action.reason}</span>
                      </span>
                    </label>
                  )
                })}
              </div>
              <div>
                <button
                  type="button"
                  className="ux-button"
                  disabled={reviewBusy || !scope || !active || storageBlocked || checkedReviewKeys.length === 0}
                  onClick={() => void submitCoverageReviews()}
                >
                  {reviewBusy
                    ? 'Sending review request…'
                    : checkedReviewKeys.length > 0
                      ? `Request admin coverage expansion (${checkedReviewKeys.length})`
                      : 'Request admin coverage expansion'}
                </button>
              </div>
              {reviewNotice && (
                <p role="status" className="ux-notice">
                  {reviewNotice}
                </p>
              )}
            </div>
          )}
        </section>
      )}

      <p className="ux-muted">
        Provider coverage, retained records, rights and reference detail are separate. Worker health remains in Sources
        &amp; access.
      </p>

      {/* Structured Source Readiness Groups */}
      {coverage.data && (
        <>
          {sources.length === 0 && (
            <p>No authorized sources are configured for evaluation. Ask a source administrator about coverage.</p>
          )}
          {gapSources.length > 0 && (
            <div className="coverage-source-group">
              <h3 className="coverage-group-heading">Needs attention in this scope ({gapSources.length})</h3>
              <div className="coverage-source-list">{gapSources.map(renderSourceCard)}</div>
            </div>
          )}
          {readySources.length > 0 && (
            <div className="coverage-source-group">
              <h3 className="coverage-group-heading">Ready in this scope ({readySources.length})</h3>
              <div className="coverage-source-list">{readySources.map(renderSourceCard)}</div>
            </div>
          )}
          {outsideSources.length > 0 && (
            <details className="coverage-source-disclosure" open={applicableCount === 0}>
              <summary className="coverage-group-summary">
                Outside selected area ({outsideSources.length})
              </summary>
              <div className="coverage-source-list">{outsideSources.map(renderSourceCard)}</div>
            </details>
          )}
          {unadmittedSources.length > 0 && (
            <details className="coverage-source-disclosure" open={applicableCount === 0 && outsideSources.length === 0}>
              <summary className="coverage-group-summary">
                Not admitted · awaiting activation ({unadmittedSources.length})
              </summary>
              <div className="coverage-source-list">{unadmittedSources.map(renderSourceCard)}</div>
            </details>
          )}
          <p className="ux-muted">{coverage.data.meaning}</p>
        </>
      )}

      {/* TIER 3: Manual Single-Profile Intent (Advanced / Audit Disclosure) */}
      {canRequest && (
        <details
          className="coverage-manual-disclosure map-card"
          open={manualOpen || !!intent.current}
          onToggle={(e) => setManualOpen(e.currentTarget.open)}
        >
          <summary className="cursor-pointer font-semibold">
            Manual single-profile intent (advanced)
          </summary>
          <fieldset
            ref={acquisitionFormRef}
            className="mt-2"
            disabled={busy || !scope || !active || storageBlocked}
          >
            <legend className="sr-only">Deliberate acquisition or coverage review</legend>
            <label>
              Approved source profile
              <select value={selected} disabled={!!intent.current} onChange={(e) => setSelected(e.target.value)}>
                <option value="">Choose a source and profile</option>
                {actions.map((action) => (
                  <option key={actionKey(action)} value={actionKey(action)}>
                    {coverage.data?.sources.find((s) => s.id === action.sourceId)?.name} · {action.profileKey} ·{' '}
                    {action.kind === 'request_coverage_review' ? 'review needed' : 'within approved coverage'}
                  </option>
                ))}
              </select>
            </label>
            <p className="ux-muted">
              The server saves the exact area, interval, purpose and revision before worker execution. A coverage review
              does not authorize retrieval.
            </p>
            <button
              type="button"
              className="ux-button"
              data-primary="true"
              disabled={!selected && !intent.current}
              onClick={() => void submitManual()}
            >
              {busy
                ? 'Saving…'
                : intent.current
                  ? 'Retry or reopen same acquisition intent'
                  : 'Submit acquisition intent'}
            </button>
            {intent.current && (
              <>
                <p className="ux-muted">
                  Pinned intent:{' '}
                  {new Date(intent.current.scope.temporal.interval!.from).toLocaleString(undefined, {
                    timeZone: intent.current.scope.temporal.timezone,
                    timeZoneName: 'shortOffset',
                  })}{' '}
                  →{' '}
                  {new Date(intent.current.scope.temporal.interval!.to).toLocaleString(undefined, {
                    timeZone: intent.current.scope.temporal.timezone,
                    timeZoneName: 'shortOffset',
                  })}
                  .
                </p>
                <button
                  type="button"
                  onClick={() => {
                    intent.current = undefined
                    setSelected('')
                    setStatus('New intent: choose the current map scope and approved profile.')
                  }}
                >
                  Start a new acquisition intent
                </button>
              </>
            )}
          </fieldset>
        </details>
      )}

      {status && (
        <p role="status" className="ux-notice">
          {status}
        </p>
      )}

      {canRead && (
        <details open={!!last} className="map-card">
          <summary className="cursor-pointer font-semibold">
            Acquisition and coverage requests ({requests.data?.items.length ?? 0})
          </summary>
          <div className="mt-2 space-y-2">
            {requests.data?.items.map((item) => (
              <button key={item.id} type="button" onClick={() => setLast(item.id)}>
                {item.scope.area.name} · {item.status} · {new Date(item.createdAt).toLocaleString()}
              </button>
            ))}
            {requests.isError && <p role="alert">Request list unavailable.</p>}
            {request.data && (
              <section aria-label="Acquisition request detail" className="map-inset-card">
                <p>
                  <strong>{request.data.id}</strong> · {request.data.status} · owner: {request.data.recoveryOwner}
                </p>
                <p className="ux-muted">{scopePeriodLabel(request.data.scope)}</p>
                <p>{request.data.reason}</p>
                {request.data.parts?.map((part) => (
                  <p key={part.id} className="ux-muted">
                    {part.status} · attempt {part.attempts} · {part.reason ?? ''}{' '}
                    {part.nextAttemptAt ? `Next attempt ${new Date(part.nextAttemptAt).toLocaleString()}` : ''} ·
                    recovery: {part.recoveryOwner}
                  </p>
                ))}
                {canRequest &&
                  request.data.status !== 'SUCCEEDED' &&
                  request.data.status !== 'CANCELLED' &&
                  (request.data.requesterId === session.operator.id ||
                    session.permissions.includes('sources.manage')) && (
                    <button
                      type="button"
                      onClick={() => {
                        setBusy(true)
                        void api
                          .cancelAcquisition(last)
                          .then(() => {
                            if (signal.aborted) return
                            setStatus('Cancellation saved. Already retained evidence remains traceable.')
                            void request.refetch()
                            void requests.refetch()
                          })
                          .catch((e) => {
                            if (!signal.aborted) setStatus(e.message)
                          })
                          .finally(() => {
                            if (!signal.aborted) setBusy(false)
                          })
                      }}
                    >
                      Cancel unstarted and active request work
                    </button>
                  )}
              </section>
            )}
            {request.isError && <p role="alert">This request is unavailable under current source access.</p>}
          </div>
        </details>
      )}
    </div>
  )

  return compact ? (
    <details className="coverage-summary">
      <summary>
        Coverage &amp; acquisition ·{' '}
        {coverage.isError
          ? 'unavailable'
          : coverage.data
            ? applicableCount > 0
              ? `${readyCount}/${applicableCount} in-scope sources ready${fetchActions.length > 0 ? ` · ${fetchActions.length} fetchable` : ''}`
              : `${sources.length} configured · none overlap this area`
            : 'evaluating'}
      </summary>
      {details}
    </details>
  ) : (
    <section aria-label="Coverage and acquisition">
      <h2>Coverage for this area and period</h2>
      {details}
    </section>
  )
}
