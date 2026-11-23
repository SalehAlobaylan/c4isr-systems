import {Fragment, lazy, Suspense, useEffect, useMemo, useRef, useState} from 'react'
import {useInfiniteQuery, useQuery, useQueryClient} from '@tanstack/react-query'
import {GEOGRAPHIC_PREVIEW_ENABLED, fixedScopeInput, useAPI} from '@/lib/api'
import type {components} from '@/lib/openapi.generated'
import {useWorkspace} from '@/lib/workspaceContext'
import {useWorkbenchHost} from './WorkbenchContext'
import {previewLayers} from './LayerCatalog'
import {WeatherField} from './WeatherField'
import {CoastalContext} from './CoastalContext'
import type {SceneLayer} from './GeographicSceneLayers'
import {GeographicContext, type ContextMarker} from './GeographicContext'
import {FrozenGeographicContext} from './FrozenGeographicContext'
import {useWorkspacePreference} from './preferences'
import {MunicipalWorkloadReview} from './MunicipalWorkloadReview'
import {PortActivityReview} from './PortActivityReview'
import {HistoricalEvidenceReview} from './HistoricalEvidenceReview'
import {CivilianAccessReplay} from './CivilianAccessReplay'
import {CivilianAerialReview} from './CivilianAerialReview'
import {RegionalResponseReview} from './RegionalResponseReview'
import {SupportingSourceReview} from './SupportingSourceReview'

type Definition = components['schemas']['EnvironmentDefinition']
type Release = components['schemas']['EnvironmentRelease']
type DisplayBuildInput = components['schemas']['EnvironmentDisplayBuildInput']
type SaveFile = (options: {suggestedName: string}) => Promise<{createWritable(): Promise<WritableStream<Uint8Array>>}>
const TerrainPreview = lazy(() => import('./TerrainPreview'))
const activeBuild = (state?: string) => state === 'QUEUED' || state === 'RUNNING'
const sizeLabel = (bytes: number) => `${(bytes / (1024 * 1024)).toLocaleString(undefined, {maximumFractionDigits: 1})} MiB`
const bufferedDownloadLimit = 64 * 1024 * 1024
const downloadLimitMessage = 'Use the command-line download for this package, or use a browser with direct file saving.'

function checkedDownload(body: ReadableStream<Uint8Array>, length: number, limit = Infinity) {
  let bytes = 0
  return body.pipeThrough(new TransformStream<Uint8Array, Uint8Array>({
    transform(chunk, controller) {
      bytes += chunk.byteLength
      if (bytes > limit) throw new Error(downloadLimitMessage)
      if (length > 0 && bytes > length) throw new Error('The package size differs from the download size.')
      controller.enqueue(chunk)
    },
    flush() {
      if (!bytes || (length > 0 && bytes !== length)) throw new Error('The package download ended early.')
    },
  }))
}

export function EnvironmentLibrary() {
  const host = useWorkbenchHost()
  const area = host?.scope?.area
  const requested = host?.search.id ?? ''
  const linkedRelease = /^georelease_[a-z0-9]{26}$/.test(requested) ? requested : ''
  return <LibraryView key={`${area?.revisionId ?? host?.selectionKey ?? 'world'}:${linkedRelease}`} linkedRelease={linkedRelease} />
}

function LibraryView({linkedRelease}: {linkedRelease: string}) {
  const host = useWorkbenchHost()
  const {session, prefix, signal} = useWorkspace()
  const api = useAPI()
  const cache = useQueryClient()
  const active = host?.visible !== false
  const can = (permission: string) => session.permissions.includes(`environments.${permission}`)
  const canManage = can('manage')
  const canBuild = can('build')
  const canPrepare = canManage || canBuild
  const scope = host?.scope
  const [name, setName] = useState(scope?.area.name ?? '')
  const [profile, setProfile] = useState('reference-context-v1')
  const [assets, setAssets] = useState<string[]>([])
  const [definition, setDefinition] = useState<Definition>()
  const [build, setBuild] = useState('')
  const [release, setRelease] = useState(linkedRelease)
  const [roadFocus, setRoadFocus] = useState<{releaseId: string; featureId: string}>()
  const [lineageRelease, setLineageRelease] = useState('')
  const [qualityRelease, setQualityRelease] = useState('')
  const [impactTarget, setImpactTarget] = useState('')
  const [sourceID, setSourceID] = useState('')
  const [storageOpen, setStorageOpen] = useState(false)
  const [decisionRelease, setDecisionRelease] = useState('')
  const [draft, setDraft] = useState<{releaseId: string; digest: string; note: string}>()
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const intent = useRef<{revision: number; id: string; key: string} | undefined>(undefined)
  const base = [...prefix, 'environment-library']
  const profiles = useQuery({queryKey: [...base, 'profiles'], enabled: active, retry: false, queryFn: ({signal}) => api.environmentProfiles(signal)})
  const enabled = active && !!profiles.data
  const available = useInfiniteQuery({
    queryKey: [...base, 'assets'], enabled: enabled && canManage, initialPageParam: undefined as string | undefined,
    queryFn: ({pageParam, signal}) => api.environmentAssets(pageParam, signal),
    getNextPageParam: page => page.next,
  })
  const definitions = useInfiniteQuery({
    queryKey: [...base, 'definitions'], enabled: enabled && canPrepare, initialPageParam: undefined as string | undefined,
    queryFn: ({pageParam, signal}) => api.environmentDefinitions(pageParam, signal),
    getNextPageParam: page => page.next,
  })
  const preflight = useQuery({
    queryKey: [...base, 'preflight', definition?.id, definition?.revision], enabled: enabled && !!definition,
    queryFn: ({signal}) => api.environmentPreflight(definition!.id, signal),
  })
  const job = useQuery({
    queryKey: [...base, 'build', build], enabled: enabled && !!build,
    queryFn: ({signal}) => api.environmentBuild(build, signal),
    refetchInterval: query => active && activeBuild(query.state.data?.state) ? 3000 : false,
  })
  const releases = useInfiniteQuery({
    queryKey: [...base, 'releases', build], enabled: enabled && (!build || job.data?.state === 'SUCCEEDED'),
    initialPageParam: undefined as string | undefined,
    queryFn: ({pageParam, signal}) => api.environmentReleases(build || undefined, pageParam, signal),
    getNextPageParam: page => page.next,
  })
  const detail = useQuery({
    queryKey: [...base, 'release', release], enabled: enabled && !!release,
    queryFn: ({signal}) => api.environmentRelease(release, signal),
  })
  const lineage = useQuery({
    queryKey: [...base, 'lineage', release], enabled: enabled && !!release && lineageRelease === release,
    queryFn: ({signal}) => api.environmentLineage(release, signal), retry: false,
  })
  const quality = useQuery({queryKey: [...base, 'quality-report', release], enabled: enabled && !!release && qualityRelease === release,
    queryFn: ({signal}) => api.environmentQualityReport(release, signal), retry: false})
  const impact = useInfiniteQuery({queryKey: [...base, 'source-impact', impactTarget],
    enabled: enabled && canManage && session.permissions.includes('sources.manage') && !!impactTarget,
    initialPageParam: undefined as string | undefined, queryFn: ({pageParam, signal}) => api.environmentSourceImpact(impactTarget, pageParam, signal),
    getNextPageParam: page => page.releases.next, retry: false})
  const backlog = useInfiniteQuery({queryKey: [...base, 'storage-backlog'], enabled: enabled && canManage && storageOpen,
    initialPageParam: undefined as string | undefined, queryFn: ({pageParam, signal}) => api.environmentStorageBacklog(pageParam, signal),
    getNextPageParam: page => page.next, retry: false})
  const decisions = useQuery({queryKey: [...base, 'decisions', release],
    enabled: enabled && !!release && decisionRelease === release && (can('qualify') || can('release')),
    queryFn: ({signal}) => api.environmentDecisions(release, signal), retry: false})
  const selection = profiles.data?.find(item => item.id === profile)
  const sourcePolicy = impact.data?.pages[0].currentPolicy
  const bindings = available.data?.pages.flatMap(page => page.items) ?? []
  const visibleBindings = bindings.filter(item =>
    selection?.requiredLayers.includes(item.representation.layer) || selection?.optionalLayers?.includes(item.representation.layer))
  const allDefinitions = definitions.data?.pages.flatMap(page => page.items) ?? []
  const allReleases = releases.data?.pages.flatMap(page => page.items) ?? []

  async function act(fn: () => Promise<void>) {
    setBusy(true)
    setMessage('')
    try {
      await fn()
    } catch (error) {
      if (!signal.aborted) setMessage(error instanceof Error ? error.message : 'The environment operation failed.')
    } finally {
      if (!signal.aborted) setBusy(false)
    }
  }

  function chooseDefinition(value?: Definition) {
    setDefinition(value)
    setName(value?.name ?? scope?.area.name ?? '')
    setProfile(value?.profileId ?? 'reference-context-v1')
    setAssets(value?.assetIds ?? [])
    setBuild('')
    setRelease('')
    setDraft(undefined)
    intent.current = undefined
  }

  async function save() {
    let areaRevisionId = definition?.areaRevisionId
    if (!areaRevisionId) {
      if (!scope?.area.geometry) throw new Error('Select a bounded area on the map.')
      const area = scope.area.persistent ? scope.area : await api.persistArea(fixedScopeInput(scope).area, signal)
      areaRevisionId = area.revisionId
    }
    const saved = await api.saveEnvironment({
      areaRevisionId, name, profileId: profile,
      assetIds: assets, expectedRevision: definition?.revision ?? 0,
    }, definition?.id, signal)
    chooseDefinition(saved)
    await cache.invalidateQueries({queryKey: [...base, 'definitions']})
  }

  async function submit() {
    if (!definition) return
    if (intent.current?.id !== definition.id || intent.current?.revision !== definition.revision) {
      intent.current = {id: definition.id, revision: definition.revision, key: crypto.randomUUID()}
    }
    const result = await api.submitEnvironment(definition.id, {revision: definition.revision, requestKey: intent.current.key}, signal)
    cache.setQueryData([...base, 'build', result.id], result)
    setBuild(result.id)
    setRelease('')
  }

  async function review(action: 'verify' | 'release' | 'withdraw', selected: Release) {
    if (draft?.releaseId !== selected.id || draft.digest !== selected.manifestSha256 || !draft.note.trim()) return
    const result = await api.reviewEnvironment(selected.id, action, {manifestSha256: selected.manifestSha256, note: draft.note}, signal)
    cache.setQueryData([...base, 'release', result.id], result)
    setDraft(undefined)
    await cache.invalidateQueries({queryKey: [...base, 'decisions', result.id]})
    await cache.invalidateQueries({queryKey: [...base, 'releases']})
  }

  async function download(selected: Release, rebuild: boolean) {
    const filename = `environment-${selected.manifestSha256.slice(0, 12)}${rebuild ? '-rebuild' : ''}.zip`
    const picker = (window as Window & {showSaveFilePicker?: SaveFile}).showSaveFilePicker
    if (picker) {
      const file = await picker.call(window, {suggestedName: filename})
      const {body, length} = await api.downloadEnvironment(selected.id, rebuild, signal)
      await checkedDownload(body, length).pipeTo(await file.createWritable(), {signal})
      return
    }
    const {body, length} = await api.downloadEnvironment(selected.id, rebuild, signal)
    if (length > bufferedDownloadLimit) {
      await body.cancel()
      throw new Error(downloadLimitMessage)
    }
    const blob = await new Response(checkedDownload(body, length, bufferedDownloadLimit)).blob()
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = filename
    anchor.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }

  if (!active) return null
  const edited = !!definition && (definition.profileId !== profile || definition.name !== name ||
    [...assets].sort().join() !== [...definition.assetIds].sort().join())
  const selected = detail.isError ? undefined : detail.data
  const note = draft && selected && draft.releaseId === selected.id && draft.digest === selected.manifestSha256 ? draft.note : ''
  const errors = [profiles.error, available.error, definitions.error, preflight.error, job.error, releases.error, detail.error]
  return <section className="ux-panel space-y-4" aria-label="Environment Library">
    <h2>Environment Library</h2>
    {profiles.isPending && <p role="status">Loading preparation profiles…</p>}
    {(message || errors.find(Boolean)) && <p role="alert">{message || errors.find(Boolean)?.message}</p>}
    {profiles.data && <>
      {canManage && <details onToggle={event => setStorageOpen(event.currentTarget.open)}>
        <summary>Storage and cleanup</summary>
        {backlog.isError ? <p role="alert">Storage backlog could not be read.</p> : backlog.data ? <>
          {backlog.data.pages.every(page => !page.items.length) && <p>No pending uploads or unreclaimed objects.</p>}
          <ul>{backlog.data.pages.flatMap(page => page.items).map(item => <li className="break-all" key={item.assetId}>
            {item.displayId ?? item.releaseId ?? item.assetId} · {item.state.toLowerCase()} · {sizeLabel(item.bytes)}
            {item.failureCode && ` · ${item.failureCode}`}{item.purgeRetryAt && ` · cleanup retry ${new Date(item.purgeRetryAt).toLocaleString()}`}
          </li>)}</ul>
          {backlog.hasNextPage && <button type="button" className="ux-button" onClick={() => void backlog.fetchNextPage()}>More storage entries</button>}
        </> : <p role="status">Loading storage backlog…</p>}
      </details>}
      {canManage && session.permissions.includes('sources.manage') && <section aria-label="Source policy review" className="space-y-2">
        <form className="space-y-2" onSubmit={event => {event.preventDefault();setImpactTarget(sourceID.trim())}}>
          <label className="block">Retained source ID<input className="atlas-field" value={sourceID} maxLength={512} onChange={event => setSourceID(event.target.value)} /></label>
          <button type="submit" className="ux-button" disabled={!sourceID.trim()}>Review source policy impact</button>
        </form>
        {!!impactTarget && <div aria-label="Source policy impact" className="space-y-2">
          <h4>Policy impact for {impactTarget}</h4>
          <button type="button" className="ux-button" onClick={() => setImpactTarget('')}>Close source policy impact</button>
          {impact.isError ? <p role="alert">Source inventory could not be read.</p> : impact.data && sourcePolicy ? <>
            <p>Policy edition {sourcePolicy.productVersion} · {sourcePolicy.enabled ? 'enabled' : 'disabled'}</p>
            {sourcePolicy.analystQuestion && <p>Intended use · {sourcePolicy.analystQuestion}</p>}
            {typeof sourcePolicy.rights?.attribution === 'string' && <p>{sourcePolicy.rights.attribution}</p>}
            {typeof sourcePolicy.rights?.termsUrl === 'string' && sourcePolicy.rights.termsUrl.startsWith('https://') &&
              <a href={sourcePolicy.rights.termsUrl} target="_blank" rel="noreferrer">Reviewed source terms</a>}
            <p className="break-all">Policy revision <code>{sourcePolicy.profileRevisionId}</code></p>
            <p>{impact.data.pages[0].retainedInputs} retained inputs · {sizeLabel(impact.data.pages[0].retainedBytes)} · {impact.data.pages[0].pendingInputs} pending uploads · {impact.data.pages[0].tombstones} tombstones</p>
            {impact.data.pages[0].nextExpiry && <p>Earliest retained input expiry {new Date(impact.data.pages[0].nextExpiry).toLocaleString()}.</p>}
            <ul>{impact.data.pages.flatMap(page => page.releases.items).map(item => <li key={item.id}>{item.id} · {item.state.toLowerCase()} · {item.displays} displays</li>)}</ul>
            {impact.hasNextPage && <button type="button" className="ux-button" onClick={() => void impact.fetchNextPage()}>More dependent releases</button>}
          </> : <p role="status">Loading source inventory…</p>}
        </div>}
      </section>}
      {canPrepare && <>
        <label className="block">Saved environment
          <select className="atlas-field" value={definition?.id ?? ''} disabled={busy} onChange={event => chooseDefinition(allDefinitions.find(item => item.id === event.target.value))}>
            <option value="">{canManage ? 'New environment for the selected map area' : 'Select a saved environment'}</option>
            {allDefinitions.map(item => <option key={item.id} value={item.id}>{item.name} · revision {item.revision}</option>)}
          </select>
        </label>
        {definitions.hasNextPage && <button className="ux-button" onClick={() => void definitions.fetchNextPage()}>More saved environments</button>}
        {canManage && <fieldset aria-label="Environment definition" className="space-y-2" disabled={busy}>
          <label className="block">Name<input className="atlas-field" value={name} maxLength={160} onChange={event => setName(event.target.value)} /></label>
          <label className="block">Preparation profile
            <select className="atlas-field" value={profile} onChange={event => {setProfile(event.target.value);setAssets([])}}>
              {profiles.data.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}
            </select>
          </label>
          {selection?.contract && <details>
            <summary>Profile formats and limits</summary>
            <p>Up to {selection.contract.maxComponents} area components and {selection.contract.maxInputs} source inputs.</p>
            <ul>{selection.contract.layers.map(layer => <li key={layer.layer}>{layer.layer} · {layer.formats.join(', ')}{layer.verticalDatums?.length ? ` · ${layer.verticalDatums.join(', ')}` : ''}</li>)}</ul>
          </details>}
          <fieldset className="space-y-2">
            <legend>Retained layers</legend>
            <p className="ux-muted">Required layers: {selection?.requiredLayers.join(', ')}.</p>
            {visibleBindings.map(item => <label className="block" key={item.id}>
              <input type="checkbox" checked={assets.includes(item.id)} onChange={event => setAssets(current =>
                event.target.checked ? [...current, item.id] : current.filter(id => id !== item.id))} />
              {' '}{item.representation.layer} · {item.representation.producer} · {item.representation.productEdition}
              <small className="block ux-muted">{item.representation.dataTime} · {item.representation.nativeResolution} · {item.representation.uncertainty}</small>
            </label>)}
            {available.hasNextPage && <button className="ux-button" onClick={() => void available.fetchNextPage()}>More retained layers</button>}
            {available.isSuccess && !visibleBindings.length && <p role="status">No retained layers are visible for this profile.</p>}
          </fieldset>
          <button className="ux-button" disabled={!name.trim() || (!definition && !scope?.area.geometry) || (!!definition && !edited)}
            onClick={() => void act(save)}>{definition ? 'Save revision' : 'Save selected area and layers'}</button>
          {!definition && !scope?.area.geometry && <p>Choose or draw a bounded area on the map to prepare data.</p>}
        </fieldset>}
        {preflight.data && <section aria-label="Preparation eligibility" className="space-y-2">
          <h3>{preflight.data.eligible ? 'Eligible to prepare' : 'Preparation blocked'}</h3>
          <p>{sizeLabel(preflight.data.inputBytes)} retained input · {preflight.data.rebuildAllowed ? 'Originals available for a rebuild bundle' : 'Originals have redistribution restrictions'}</p>
          {!!preflight.data.coverage?.length && <details>
            <summary>Declared source coverage</summary>
            <ul>{preflight.data.coverage.map(({layer, required, status}) => <li key={layer}>
              {layer} · {required ? 'required' : 'optional'} · {status === 'DECLARED_COVERED' ? 'footprints cover area' : status === 'DECLARED_INCOMPLETE' ? 'footprints leave gaps' : 'no eligible input'}
            </li>)}</ul>
          </details>}
          {preflight.data.diagnostics.length > 0 && <ul>{preflight.data.diagnostics.map((item, index) => <li key={index}>{item}</li>)}</ul>}
          {canBuild && <button className="ux-button" disabled={busy || edited || !preflight.data.eligible || !!build}
            onClick={() => void act(submit)}>Prepare saved revision</button>}
        </section>}
        {job.data && <section aria-label="Preparation progress">
          <p role="status">Preparation {job.data.state.toLowerCase()} · attempt {job.data.attempt}{job.data.failureCode ? ` · ${job.data.failureCode.replaceAll('_', ' ')}` : ''}</p>
          {canBuild && activeBuild(job.data.state) && <button className="ux-button" disabled={busy} onClick={() => void act(async () => {
            const result = await api.cancelEnvironment(build, signal)
            cache.setQueryData([...base, 'build', build], result)
          })}>Cancel preparation</button>}
          {!activeBuild(job.data.state) && <button className="ux-button" disabled={busy} onClick={() => {setBuild('');intent.current = undefined}}>Return to library</button>}
        </section>}
      </>}
      <label className="block">Available releases
        <select className="atlas-field" value={release} disabled={busy} onChange={event => {setRelease(event.target.value);setDraft(undefined)}}>
          <option value="">Select a release</option>
          {allReleases.map(item => <option key={item.id} value={item.id}>{item.name || 'Prepared reference data'} · {item.state.toLowerCase()} · {item.manifestSha256.slice(0, 12)} · {new Date(item.createdAt).toLocaleString()}</option>)}
        </select>
      </label>
      {releases.hasNextPage && <button className="ux-button" onClick={() => void releases.fetchNextPage()}>More releases</button>}
      {selected?.manifest && <section aria-label="Release quality" className="space-y-2">
        <h3>{selected.name || profiles.data.find(item => item.id === selected.manifest?.profileId)?.name || 'Prepared reference data'}</h3>
        <p role="status">Release status: {selected.state.toLowerCase()}</p>
        <p className="break-all">Manifest SHA-256 <code>{selected.manifestSha256}</code></p>
        <p>{selected.manifest.quality.meaning}</p>
        <p>{selected.manifest.quality.heightMethod} Physical accuracy: {selected.manifest.quality.physicalAccuracy.toLowerCase()}.</p>
        <details onToggle={event => setQualityRelease(event.currentTarget.open ? selected.id : '')}>
          <summary>Coordinate and sample checks</summary>
          {quality.isError ? <p role="alert">Structured quality evidence could not be read.</p> : quality.data?.manifestSha256 === selected.manifestSha256 ? <>
            <p>Maximum control disagreement: {quality.data.coordinateCheck.maximumErrorM.toPrecision(3)} m. Limit: {quality.data.coordinateCheck.limitM} m.</p>
            <p>{quality.data.profileContractSha256 ? <span className="break-all">Profile SHA-256 <code>{quality.data.profileContractSha256}</code></span> : 'This release predates profile digests.'}</p>
            <ul>{quality.data.coverage?.map(item => <li key={item.member}>
              {item.layer} · {item.member} · {item.state === 'SAMPLE_MASK_RECORDED'
                ? `${item.validSamples?.toLocaleString()} of ${item.selectedSamples?.toLocaleString()} selected samples valid${item.registration ? ` · ${item.registration}` : ''}`
                : item.state === 'FEATURE_COUNT_RECORDED' ? `${item.featureCount?.toLocaleString()} retained features` : 'coverage unknown'}
            </li>)}</ul>
          </> : <p role="status">Loading quality evidence…</p>}
        </details>
        <ul>{selected.manifest.members.map(item => <li key={item.name}>{item.layer} · {sizeLabel(item.bytes)}</li>)}</ul>
        {selected.manifest.quality.observations?.weather && <WeatherField key={selected.manifestSha256} field={selected.manifest.quality.observations.weather} />}
        {selected.manifest.quality.coastal && <CoastalContext key={selected.manifestSha256} value={selected.manifest.quality.coastal} />}
        {!!selected.manifest.quality.enrichment?.modeledBuildingContext?.length && <details>
          <summary>Modeled building height context</summary>
          {selected.manifest.quality.enrichment.modeledBuildingContext.map(report => <div key={report.member}>
            <p>{report.edition} · {report.modeledHeights.toLocaleString()} modeled heights from {report.centroids.toLocaleString()} centroids · {report.unknown.toLocaleString()} unknown.</p>
            <p>{report.medianM !== null && `Median modeled height: ${report.medianM.toLocaleString()} m. `}{report.meaning}</p>
          </div>)}
        </details>}
        {!!selected.manifest.inputs?.length && <details>
          <summary>Source dates, resolution and attribution</summary>
          <ul className="space-y-2">
            {selected.manifest.inputs.map(({id, representation, rights}) => <li key={id}>
              <strong>{representation.layer} · {representation.producer} · {representation.productEdition}</strong>
              <p>{representation.dataTime} · {representation.nativeResolution} · {representation.crs}</p>
              <p>{representation.uncertainty}</p>
              {typeof rights.attribution === 'string' && <small>{rights.attribution}</small>}
            </li>)}
          </ul>
        </details>}
        <details onToggle={event => setLineageRelease(event.currentTarget.open ? selected.id : '')} key={selected.id}>
          <summary>Source lineage and download rights</summary>
          {lineage.isError ? <p role="alert">Current source rights could not be read.</p> : lineage.data && lineage.data.manifestSha256 === selected.manifestSha256 ? <ul className="space-y-2">
            {lineage.data.inputs.map(input => <li key={input.assetId}>
              <strong>{input.representation.producer} · {input.representation.productEdition}</strong>
              <p>{input.attribution}</p>
              <p>Retained until {new Date(input.retainUntil).toLocaleDateString()}{input.policyExpiresAt ? ` · source policy expires ${new Date(input.policyExpiresAt).toLocaleDateString()}` : ''}</p>
              {input.admissionId && <details>
                <summary>Retained source identity</summary>
                <p className="break-all">Original admission <code>{input.admissionId}</code></p>
                <p className="break-all">Current admission <code>{input.currentAdmissionId}</code></p>
                <p className="break-all">Original SHA-256 <code>{input.sha256}</code></p>
                {input.representation.evidence && <p>{input.representation.evidence}</p>}
              </details>}
              <p>{input.originalDownload ? 'Source permits distribution of originals.' : 'Source restricts distribution of originals.'}</p>
              {input.parentState === 'UNAVAILABLE' && <p>Linked catalogue parent unavailable.</p>}
              {input.viewerExport !== undefined && <p>Viewer export {input.viewerExport ? 'allowed' : 'restricted'} · context export {input.contextExport ? 'allowed' : 'restricted'}.</p>}
              <p>Prepared members: {lineage.data!.members.filter(member => member.inputId === input.assetId).map(member => member.name).join(', ') || 'No member link recorded.'}</p>
              {canManage && session.permissions.includes('sources.manage') && <button type="button" className="ux-button" onClick={() => {setSourceID(input.sourceId);setImpactTarget(input.sourceId)}}>Inspect this source policy</button>}
            </li>)}
          </ul> : <p role="status">Loading source lineage…</p>}
        </details>
        {selected.state !== 'WITHDRAWN' && <Fragment key={`reviews:${selected.id}:${selected.manifestSha256}`}>
          <MunicipalWorkloadReview release={selected} />
          <CivilianAccessReplay release={selected}
            onLocate={GEOGRAPHIC_PREVIEW_ENABLED && can('download') && selected.manifest.members.some(member => member.layer === 'surface')
              ? featureId => setRoadFocus({releaseId: selected.id, featureId}) : undefined} />
          <CivilianAerialReview release={selected} />
          <RegionalResponseReview release={selected} />
          <SupportingSourceReview release={selected} />
          <PortActivityReview release={selected} />
          <HistoricalEvidenceReview release={selected} source="flood" />
          <HistoricalEvidenceReview release={selected} source="outages" />
        </Fragment>}
        <details><summary>Unsupported capabilities</summary><p>{selected.manifest.quality.unsupported.join(', ')}</p></details>
        {(can('qualify') || can('release')) && <details key={`decisions:${selected.id}`}
          onToggle={event => setDecisionRelease(event.currentTarget.open ? selected.id : '')}>
          <summary>Release decision history</summary>
          {decisions.isError ? <p role="alert">Release decision history is unavailable.</p> : decisions.data ? <ol>
            {decisions.data.map(item => <li key={`${item.occurredAt}:${item.state}`}>
              <strong>{item.state.toLowerCase()}</strong> · <time dateTime={item.occurredAt}>{new Date(item.occurredAt).toLocaleString()}</time>
              {item.humanId && ` · ${item.humanId}`}<p>{item.note}</p>
            </li>)}
          </ol> : <p role="status">Loading release decisions…</p>}
        </details>}
        {GEOGRAPHIC_PREVIEW_ENABLED && can('download') && (selected.state === 'RELEASED' || can('qualify') && ['STAGED', 'VERIFIED'].includes(selected.state)) &&
          selected.manifest.members.some(member => member.layer === 'surface') &&
          <ReleaseTerrain key={`terrain:${selected.id}:${selected.manifestSha256}`} release={selected}
            roadFocus={roadFocus?.releaseId === selected.id ? roadFocus.featureId : undefined} onClearRoadFocus={() => setRoadFocus(undefined)} />}
        {can('download') && ['RELEASED', 'STAGED', 'VERIFIED'].includes(selected.state) &&
          <button className="ux-button" disabled={busy} onClick={() => void act(() => download(selected, false))}>Download prepared data</button>}
        {can('download') && selected.state === 'RELEASED' &&
          <button className="ux-button" disabled={busy} onClick={() => void act(() => download(selected, true))}>Download rebuild bundle</button>}
        {(can('qualify') && selected.state === 'STAGED' || can('release') && ['STAGED', 'VERIFIED', 'RELEASED'].includes(selected.state)) && <>
          <label className="block">Review evidence<textarea className="atlas-field" disabled={busy} maxLength={2000} value={note}
            placeholder="Record the checks performed and any evidence reference"
            onChange={event => setDraft({releaseId: selected.id, digest: selected.manifestSha256, note: event.target.value})} /></label>
          {can('qualify') && selected.state === 'STAGED' &&
            <button className="ux-button" disabled={busy || !note.trim()} onClick={() => void act(() => review('verify', selected))}>Record verification</button>}
          {can('release') && selected.state === 'VERIFIED' &&
            <button className="ux-button" disabled={busy || !note.trim()} onClick={() => void act(() => review('release', selected))}>Release package</button>}
          {can('release') && <button className="ux-button" disabled={busy || !note.trim()} onClick={() => void act(() => review('withdraw', selected))}>Withdraw package</button>}
        </>}
      </section>}
    </>}
  </section>
}

function ReleaseTerrain({release, roadFocus, onClearRoadFocus}: {release: Release; roadFocus?: string; onClearRoadFocus: () => void}) {
  const host = useWorkbenchHost()
  const {prefix, api, session} = useWorkspace()
  const [open, setOpen] = useState(false)
  const [contextOpen, setContextOpen] = useState(false)
  const [handoffOpen, setHandoffOpen] = useState(false)
  const [markers, setMarkers] = useState<ContextMarker[]>([])
  const [drawn, setDrawn] = useState(0)
  const [placeInput, setPlaceInput] = useState('')
  const [placeID, setPlaceID] = useState('')
  const [placeDrawn, setPlaceDrawn] = useState(false)
  const [showNearbyPlaces, setShowNearbyPlaces] = useState(false)
  const [workOpen, setWorkOpen] = useState(false)
  const [local, setLocal] = useState<{display?: string; layers?: string; feature?: string}>({})
  const panel = useRef<HTMLDetailsElement>(null)
  useEffect(() => {
    if (roadFocus) panel.current?.scrollIntoView({block: 'nearest'})
  }, [roadFocus])
  const path = host?.search
  const matchingPath = path?.id === release.id && path.manifest === release.manifestSha256 &&
    (!path.atlasArea || path.atlasArea === host?.scope?.area.revisionId)
  const activeDisplay = matchingPath && path?.view === 'environment-3d' ? path.display : host ? undefined : local.display
  const layerTokens = (matchingPath ? path?.layers : local.layers)?.split(',').filter(Boolean) ?? []
  const enabledLayers = new Map(layerTokens.map(token => token.split(':', 2) as [string, string | undefined]))
  const feature = matchingPath ? path?.feature : local.feature
  const allowed = session.permissions.includes('environments.download')
  const show = open || !!activeDisplay || !!roadFocus
  function update(next: {display?: string; layers?: string; feature?: string; basemap?: string; terrainPoint?: string}, view = 'environment-3d', replace = false) {
    if (host) host.update({...host.search, ...next, id: release.id, manifest: release.manifestSha256,
      atlasArea: host.scope?.area.revisionId, view, bbox: undefined, terrainPoint: next.terrainPoint}, replace)
    else setLocal(next)
  }
  const displays = useInfiniteQuery({
    queryKey: [...prefix, 'environment-displays', release.id, release.manifestSha256],
    initialPageParam: undefined as string | undefined, enabled: show, retry: false, gcTime: 0,
    refetchInterval: show ? 30_000 : false,
    queryFn: ({pageParam, signal}) => api.environmentDisplays(release.id, pageParam, signal),
    getNextPageParam: page => page.next,
  })
  const available = displays.data?.pages.flatMap(page => page.items).filter(item =>
    item.releaseId === release.id && item.manifestSha256 === release.manifestSha256 &&
    item.state === 'READY' && item.schemaVersion === 1 && new Date(item.retainUntil).getTime() > Date.now(),
  ) ?? []
  const terrain = available.filter(item => item.recipe === 'terrain-glb-v1' || item.recipe === 'terrain-glb-v2' || item.recipe === 'ground-glb-v1')
  const street = available.find(item => item.recipe === 'street-pmtiles-v1')
  const selected = roadFocus
    ? terrain.find(item => item.id === activeDisplay && item.recipe !== 'ground-glb-v1') ?? terrain.find(item => item.recipe !== 'ground-glb-v1')
    : terrain.find(item => item.id === activeDisplay)
  const layerOptions = (Object.entries(previewLayers) as [SceneLayer, (typeof previewLayers)[SceneLayer]][]).map(([layer, definition]) => {
    const candidates = available.filter(item => item.recipe === definition.recipe && item.bytes <= definition.maxBytes)
    const pinned = enabledLayers.get(layer)
    const display = pinned ? candidates.find(item => item.id === pinned) : candidates.find(item => feature?.startsWith(`${item.id}:`)) ?? candidates[0]
    return {layer, definition, display}
  })
  const roadDisplay = layerOptions.find(item => item.layer === 'roads')?.display
  const locatingRoad = !!roadFocus && !!roadDisplay
  if (locatingRoad) enabledLayers.set('roads', roadDisplay.id)
  const currentTokens = locatingRoad ? [...layerTokens.filter(token => !token.startsWith('roads:')), `roads:${roadDisplay.id}`] : layerTokens
  const rasterLayers = new Set(['imagery', 'land-cover', 'ground-delta', 'radar'])
  const isGround = selected?.recipe === 'ground-glb-v1'
  const canReadPlaces = allowed && session.permissions.includes('sources.read')
  const referencePlace = useQuery({
    queryKey: [...prefix, 'terrain-reference-place', placeID],
    queryFn: ({signal}) => api.listGeographicRegions({id: placeID}, signal),
    enabled: !!selected && canReadPlaces && !!placeID, gcTime: 0, retry: false,
    refetchInterval: selected && canReadPlaces && placeID ? 30_000 : false,
  })
  const place = referencePlace.isSuccess && canReadPlaces ? referencePlace.data.features[0] : undefined
  const placeMarker = useMemo<ContextMarker | undefined>(() => {
    if (place?.properties?.allowTerrainDisplay !== true || place.geometry?.type !== 'Point' ||
      !Array.isArray(place.geometry.coordinates)) return undefined
    const [longitude, latitude] = place.geometry.coordinates
    if (typeof longitude !== 'number' || typeof latitude !== 'number') return undefined
    return {id: placeID, title: String(place.properties.name), date: '', position: [longitude, latitude]}
  }, [place, placeID])
  const placeSource = place?.properties?.sourceId
  const placeRelease = place?.properties?.releaseId
  const areaRevision = selected?.areaRevisionId
  const nearby = useQuery({
    queryKey: [...prefix, 'terrain-place-group', placeSource, placeRelease, areaRevision, host?.scope?.scopeHash],
    enabled: showNearbyPlaces && !!areaRevision && !!host?.scope && typeof placeSource === 'string' &&
      typeof placeRelease === 'string' && !!placeRelease && canReadPlaces,
    retry: false, gcTime: 0, refetchInterval: 30_000,
    queryFn: async ({signal}) => {
      const page = await api.readCatalog({
        kind: 'geography', limit: 100,
        scope: {area: {revisionId: areaRevision!}, temporal: fixedScopeInput(host!.scope!).temporal},
        filters: {kind: 'place', sourceId: String(placeSource), releaseId: String(placeRelease)},
      }, signal)
      if (page.snapshotId) await api.releaseCatalog(page.snapshotId).catch(() => {})
      return page
    },
  })
  const nearbyPlaces = useMemo(() => {
    if (!showNearbyPlaces || !canReadPlaces || !nearby.isSuccess) return undefined
    const rows = nearby.data.items as unknown as Array<{
      id?: string; geometry?: {type: string; coordinates?: unknown};
      properties?: {allowTerrainDisplay?: boolean; name?: string};
    }>
    return rows.flatMap(({id, geometry, properties}) => {
      if (!id || properties?.allowTerrainDisplay !== true || geometry?.type !== 'Point' ||
        !Array.isArray(geometry.coordinates)) return []
      const [lon, lat] = geometry.coordinates
      if (!Number.isFinite(lon) || !Number.isFinite(lat)) return []
      return [{id, title: String(properties.name ?? id), date: '', position: [lon, lat] as [number, number]}]
    }).slice(0, 24)
  }, [nearby.data, nearby.isSuccess, showNearbyPlaces, canReadPlaces])
  const overlays = layerOptions.flatMap(({layer, display}) =>
    display && enabledLayers.has(layer) && (!isGround || rasterLayers.has(layer)) ? [{layer, display}] : [])
  return <details ref={panel} open={!!activeDisplay || !!roadFocus || undefined} onToggle={event => {
    setOpen(event.currentTarget.open)
    if (!event.currentTarget.open && activeDisplay) update({}, 'environment-atlas')
    if (!event.currentTarget.open && roadFocus) onClearRoadFocus()
  }}>
    <summary>3D terrain preview</summary>
    {show && <div className="space-y-2">
      {displays.isPending && <p role="status">Checking available terrain…</p>}
      {displays.isError ? <>
        <p role="alert">Terrain availability could not be checked.</p>
        <button className="ux-button" onClick={() => void displays.refetch()}>Retry availability check</button>
      </> : <>
        {displays.isSuccess && !terrain.length && <p role="status">No terrain preview is available for this release.</p>}
        {roadFocus && displays.isSuccess && !displays.hasNextPage && !locatingRoad &&
          <p role="status">The retained road layer is unavailable for this release.</p>}
        {!allowed && <p role="status">Rendering requires the environment download permission.</p>}
        {activeDisplay && displays.isSuccess && !selected && !displays.hasNextPage && <p role="status">The linked terrain is unavailable in this release.</p>}
        {terrain.map(item => <button key={item.id} className="ux-button" disabled={!allowed} aria-pressed={selected?.id === item.id}
          onClick={() => {onClearRoadFocus(); update({display: selected?.id === item.id ? undefined : item.id, layers: currentTokens.join(','), feature})}}>
          {selected?.id === item.id ? 'Close' : 'Open'} {item.recipe === 'ground-glb-v1' ? 'modeled bare earth' : 'terrain'} · {sizeLabel(item.bytes)} · {item.sha256.slice(0, 8)}
        </button>)}
        {displays.hasNextPage && <button className="ux-button" disabled={displays.isFetchingNextPage}
          onClick={() => void displays.fetchNextPage()}>More terrain artifacts</button>}
        {session.permissions.includes('environments.qualify') && ['reference-context-v1', 'reference-surface-v1'].includes(release.manifest?.profileId ?? '') &&
          <details key={`${release.id}:${release.manifestSha256}`} onToggle={event => setWorkOpen(event.currentTarget.open)}>
            <summary>Build a display</summary>
            <DisplayBuildForm release={release} active={workOpen} onPublished={() => void displays.refetch()} />
          </details>}
        {allowed && street && host?.showAtlas && <button className="ux-button" onClick={() => {
          update({basemap: street.id}, 'environment-atlas')
          host.showAtlas?.()
        }}>Open retained street map · zoom 12–16</button>}
        {allowed && selected && <>
          <fieldset className="ux-actions"><legend>Display layers</legend>
            {layerOptions.map(({layer, definition, display}) => <label key={layer} title={definition.explanation}>
              <input type="checkbox" checked={enabledLayers.has(layer) && (!isGround || rasterLayers.has(layer))}
                disabled={isGround && !rasterLayers.has(layer) || !display && !enabledLayers.has(layer)}
                onChange={event => {
                  onClearRoadFocus()
                  const tokens = layerTokens.filter(token => token.split(':', 1)[0] !== layer &&
                    !(event.target.checked && rasterLayers.has(layer) && rasterLayers.has(token.split(':', 1)[0])))
                  if (event.target.checked && display) tokens.push(`${layer}:${display.id}`)
                  update({display: selected.id, layers: tokens.join(','), feature: !event.target.checked && feature?.startsWith(`${display?.id}:`) ? undefined : feature}, undefined, true)
                }} />
              {definition.name}{!display && ' · unavailable'}{display && isGround && !rasterLayers.has(layer) && ' · reference surface only'}
            </label>)}
          </fieldset>
          {host?.showAtlas && <button className="ux-button" onClick={() => {
            update({display: selected.id, layers: layerTokens.join(','), feature}, 'environment-atlas')
            host.showAtlas?.()
          }}>Return to Atlas</button>}
          <Suspense fallback={<p role="status">Loading the 3D viewer…</p>}>
            <TerrainPreview key={selected.id} display={selected} overlays={overlays} selection={feature} roadFocus={roadFocus}
              contextMarkers={contextOpen ? markers : undefined}
              onContextDrawn={setDrawn}
              placeMarker={placeMarker} nearbyPlaces={nearbyPlaces} onPlaceDrawn={setPlaceDrawn}
              onSelect={value => {onClearRoadFocus(); update({display: selected.id, layers: currentTokens.join(','), feature: value || undefined}, undefined, true)}}
              onLocate={host?.showAtlas ? () => {update({display: selected.id, layers: layerTokens.join(','), feature}, 'environment-atlas'); host.showAtlas?.()} : undefined}
              onLocateTerrain={host?.showAtlas ? ([longitude, latitude]) => {
                update({display: selected.id, layers: layerTokens.join(','), feature: undefined,
                  terrainPoint: `${longitude.toFixed(6)},${latitude.toFixed(6)}`}, 'environment-atlas')
                host.showAtlas?.()
              } : undefined} />
          </Suspense>
          {canReadPlaces && <details>
            <summary>Reference place</summary>
            <label className="block">Place reference from the Atlas
              <input className="atlas-field" value={placeInput} maxLength={256} onChange={event => setPlaceInput(event.target.value)} />
            </label>
            <button className="ux-button" disabled={!placeInput.trim()} onClick={() => setPlaceID(placeInput.trim())}>Locate place on terrain</button>
            {placeID && <button className="ux-button" onClick={() => {setPlaceID(''); setPlaceInput('')}}>Clear place</button>}
            {placeID && <p role="status">{referencePlace.isPending ? 'Checking the retained place…' :
              placeMarker ? <><bdi dir="auto">{placeMarker.title}</bdi> · <bdi dir="auto">{String(place?.properties?.attribution ?? '')}</bdi> · {placeDrawn ? 'Placed on the preview surface' : 'No terrain surface at the reference point'}</> :
              'This place is unavailable for terrain display.'}</p>}
            {placeMarker && host?.scope && typeof placeRelease === 'string' && !!placeRelease && <label className="block">
              <input type="checkbox" checked={showNearbyPlaces} onChange={event => setShowNearbyPlaces(event.target.checked)} />
              {' '}Show nearby places from this edition
            </label>}
            {showNearbyPlaces && placeMarker && <p role="status">{nearby.isError ? 'Nearby references are unavailable.' :
              nearby.isPending ? 'Checking nearby references…' : `${nearbyPlaces?.length ?? 0} nearby labels selected for the terrain preview.`}</p>}
            {!!nearbyPlaces?.length && <ul aria-label="Nearby reference places">{nearbyPlaces.map(item =>
              <li key={item.id}><bdi dir="auto">{item.title}</bdi> · <code>{item.id}</code></li>)}</ul>}
          </details>}
          <details onToggle={event => setContextOpen(event.currentTarget.open)}>
            <summary>Retained context and coverage</summary>
            {contextOpen && <GeographicContext releaseArea={selected.areaRevisionId} atlasScope={host?.scope} onMarkers={setMarkers} drawn={drawn} />}
          </details>
          {session.permissions.includes('context.export') && session.permissions.includes('observations.read') && host?.scope &&
            <details onToggle={event => setHandoffOpen(event.currentTarget.open)}>
              <summary>Frozen geographic handoff</summary>
              {release.state !== 'RELEASED' ? <p>This release requires an independent release decision before a handoff can be frozen.</p> :
                handoffOpen && <FrozenGeographicContext key={`${selected.id}:${overlays.map(item => item.display.id).join(':')}`}
                  terrain={isGround ? terrain.find(item => item.recipe !== 'ground-glb-v1') ?? selected : selected}
                  displays={[...(isGround ? [selected] : []), ...overlays.map(item => item.display)]} scope={host.scope} />}
            </details>}
        </>}
      </>}
    </div>}
  </details>
}

const buildRecipes = {
  'terrain-glb-v2': 'Native terrain', 'imagery-display-v1': 'Dated optical imagery',
  'roads-display-v1': 'Roads', 'buildings-display-v1': 'Buildings', 'water-display-v1': 'Water',
} as const

function DisplayBuildForm({release, active, onPublished}: {release: Release; active: boolean; onPublished: () => void}) {
  const {api, prefix, signal} = useWorkspace()
  const [recipe, setRecipe] = useState<keyof typeof buildRecipes>('terrain-glb-v2')
  const [scene, setScene] = useState<File>()
  const [jobID, setJobID] = useWorkspacePreference(`environment-display-build.${release.id}.${release.manifestSha256}`, '',
    (value): value is string => typeof value === 'string' && /^(?:geobuild_[a-z0-9]{26})?$/.test(value))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const intent = useRef<DisplayBuildInput | undefined>(undefined)
  const cache = useQueryClient()
  const job = useQuery({queryKey: [...prefix, 'environment-display-build', jobID], enabled: active && !!jobID,
    queryFn: async ({signal}) => {
      const result = await api.environmentBuild(jobID, signal)
      if (result.state === 'SUCCEEDED') onPublished()
      return result
    }, refetchInterval: query => !query.state.data || activeBuild(query.state.data.state) ? 2000 : false})
  const available = new Set(release.manifest?.members.map(member => member.layer))
  const submit = async () => {
    setBusy(true)
    setError('')
    try {
      let sceneMetadata: string | undefined
      if (recipe === 'imagery-display-v1') {
        if (!scene || scene.size > 48 * 1024) throw new Error('Choose the retained scene JSON within 48 KiB.')
        sceneMetadata = await scene.text()
      }
      if (intent.current?.recipe !== recipe || intent.current.sceneMetadata !== sceneMetadata) {
        intent.current = {requestKey: crypto.randomUUID(), manifestSha256: release.manifestSha256, recipe, sceneMetadata}
      }
      const result = await api.submitEnvironmentDisplay(release.id, intent.current, signal)
      cache.setQueryData([...prefix, 'environment-display-build', result.id], result)
      setJobID(result.id)
    } catch (error) {
      if (!signal.aborted) setError(error instanceof Error ? error.message : 'The display build could not be queued.')
    } finally {
      setBusy(false)
    }
  }
  const cancel = async () => {
    setBusy(true)
    setError('')
    try {
      const result = await api.cancelEnvironmentDisplay(jobID, signal)
      cache.setQueryData([...prefix, 'environment-display-build', jobID], result)
    } catch (error) {
      if (!signal.aborted) setError(error instanceof Error ? error.message : 'The display build could not be canceled.')
    } finally {
      setBusy(false)
    }
  }
  return <div className="space-y-2">
    <label className="block">Display recipe<select className="atlas-field" value={recipe} disabled={busy || !!jobID}
      onChange={event => setRecipe(event.target.value as keyof typeof buildRecipes)}>
      {Object.entries(buildRecipes).map(([id, label]) => <option key={id} value={id} disabled={!available.has(id === 'terrain-glb-v2' ? 'surface' : id.split('-')[0])}>{label}</option>)}
    </select></label>
    {recipe === 'imagery-display-v1' && <label className="block">Retained Sentinel-2 scene JSON
      <input className="atlas-field" type="file" accept=".json,application/json" disabled={busy || !!jobID} onChange={event => setScene(event.target.files?.[0])} />
    </label>}
    {!jobID && <button className="ux-button" disabled={busy} onClick={() => void submit()}>Queue display</button>}
    {job.data && <p role="status">Display {job.data.state.toLowerCase()} · attempt {job.data.attempt}{job.data.failureCode && ` · ${job.data.failureCode}`}</p>}
    {job.isError && <>
      <p role="alert">The display build status is unavailable.</p>
      <button className="ux-button" onClick={() => void job.refetch()}>Retry status check</button>
      <button className="ux-button" onClick={() => {intent.current = undefined;setJobID('')}}>Clear saved build</button>
    </>}
    {activeBuild(job.data?.state) && <button className="ux-button" disabled={busy} onClick={() => void cancel()}>Cancel display build</button>}
    {job.data && !activeBuild(job.data.state) && <button className="ux-button" onClick={() => {intent.current = undefined;setJobID('');onPublished()}}>Return to display recipes</button>}
    {error && <p role="alert">{error}</p>}
  </div>
}
