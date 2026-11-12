import {useRef, useState} from 'react'
import {useInfiniteQuery, useQuery, useQueryClient} from '@tanstack/react-query'
import {fixedScopeInput, useAPI} from '@/lib/api'
import type {components} from '@/lib/openapi.generated'
import {useWorkspace} from '@/lib/workspaceContext'
import {useWorkbenchHost} from './WorkbenchContext'

type Definition = components['schemas']['EnvironmentDefinition']
type Release = components['schemas']['EnvironmentRelease']
type SaveFile = (options: {suggestedName: string}) => Promise<{createWritable(): Promise<WritableStream<Uint8Array>>}>
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
  const [note, setNote] = useState('')
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
  const selection = profiles.data?.find(item => item.id === profile)
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
    setNote('')
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
    const result = await api.reviewEnvironment(selected.id, action, {manifestSha256: selected.manifestSha256, note}, signal)
    cache.setQueryData([...base, 'release', result.id], result)
    setNote('')
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
  const errors = [profiles.error, available.error, definitions.error, preflight.error, job.error, releases.error, detail.error]
  return <section className="ux-panel space-y-4" aria-label="Environment Library">
    <h2>Environment Library</h2>
    {profiles.isPending && <p role="status">Loading preparation profiles…</p>}
    {(message || errors.find(Boolean)) && <p role="alert">{message || errors.find(Boolean)?.message}</p>}
    {profiles.data && <>
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
        <select className="atlas-field" value={release} disabled={busy} onChange={event => {setRelease(event.target.value);setNote('')}}>
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
        <ul>{selected.manifest.members.map(item => <li key={item.name}>{item.layer} · {sizeLabel(item.bytes)}</li>)}</ul>
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
        <details><summary>Unsupported capabilities</summary><p>{selected.manifest.quality.unsupported.join(', ')}</p></details>
        {can('download') && ['RELEASED', 'STAGED', 'VERIFIED'].includes(selected.state) &&
          <button className="ux-button" disabled={busy} onClick={() => void act(() => download(selected, false))}>Download prepared data</button>}
        {can('download') && selected.state === 'RELEASED' &&
          <button className="ux-button" disabled={busy} onClick={() => void act(() => download(selected, true))}>Download rebuild bundle</button>}
        {(can('qualify') && selected.state === 'STAGED' || can('release') && ['STAGED', 'VERIFIED', 'RELEASED'].includes(selected.state)) && <>
          <label className="block">Review evidence<textarea className="atlas-field" disabled={busy} maxLength={2000} value={note} onChange={event => setNote(event.target.value)} /></label>
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
