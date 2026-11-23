import {useInfiniteQuery} from '@tanstack/react-query'
import {Link} from '@tanstack/react-router'
import {frozenInput, useAnalystScope} from '@/features/workspace/AnalystScopeProvider'
import {useWorkbenchHost} from '@/features/workspace/WorkbenchContext'
import {useAPI} from '@/lib/api'
import {formatTimestamp} from '@/lib/format'
import type {components} from '@/lib/openapi.generated'
import {useWorkspace} from '@/lib/workspaceContext'

type Asset = components['schemas']['EnvironmentAsset']

function RetainedAsset({asset}: {asset: Asset}) {
  const data = asset.representation
  const edition = [asset.referenceId, asset.sceneId, data.productEdition].filter(Boolean).join(' · ')
  return <li>
    <strong>{edition}</strong>
    <p>{data.layer} · {data.nativeResolution} · {data.dataTime}</p>
    <details>
      <summary>Asset provenance</summary>
      <dl>
        <div><dt>Producer</dt><dd>{data.producer}</dd></div>
        <div><dt>Format and size</dt><dd>{data.format} · {asset.bytes.toLocaleString()} bytes</dd></div>
        <div><dt>Coordinates and vertical datum</dt><dd>{data.crs} · {data.verticalDatum}</dd></div>
        <div><dt>Meaning</dt><dd>{data.semantics}</dd></div>
        <div><dt>Uncertainty</dt><dd>{data.uncertainty}</dd></div>
        <div><dt>Evidence</dt><dd>{data.evidence}</dd></div>
        <div><dt>Retrieved</dt><dd>{formatTimestamp(data.retrievedAt)}</dd></div>
        <div><dt>Retained until</dt><dd>{formatTimestamp(asset.retainUntil)}</dd></div>
        <div><dt>Asset ID</dt><dd>{asset.id}</dd></div>
        <div><dt>SHA-256</dt><dd>{asset.sha256}</dd></div>
      </dl>
    </details>
  </li>
}

export function RetainedSourceData({sourceId}: {sourceId: string}) {
  const api = useAPI(), {prefix} = useWorkspace()
  const scope = useAnalystScope(), host = useWorkbenchHost()
  const linkScope = scope?.searchFor(host?.scope ? frozenInput(host.scope) : scope?.input)
  const assets = useInfiniteQuery({
    queryKey: [...prefix, 'retained-source-assets', sourceId],
    initialPageParam: undefined as string | undefined,
    queryFn: ({pageParam, signal}) => api.environmentAssets(pageParam, signal, sourceId),
    getNextPageParam: page => page.next,
    retry: false,
    refetchInterval: 30000,
  })
  const releases = useInfiniteQuery({
    queryKey: [...prefix, 'retained-source-releases', sourceId],
    initialPageParam: undefined as string | undefined,
    queryFn: ({pageParam, signal}) => api.environmentReleases(undefined, pageParam, signal, sourceId),
    getNextPageParam: page => page.next,
    retry: false,
    refetchInterval: 30000,
  })
  const assetItems = assets.data?.pages.flatMap(page => page.items) ?? []
  const releaseItems = releases.data?.pages.flatMap(page => page.items) ?? []

  return <div className="source-retained">
    <section aria-label="Retained environment inputs">
      <h4>Retained environment inputs {assets.isSuccess && `(${assetItems.length}${assets.hasNextPage ? '+' : ''})`}</h4>
      {assets.isPending && <p role="status">Loading inputs…</p>}
      {assets.isError && !assets.isFetchNextPageError && <p role="alert">Inputs unavailable. <button className="underline" onClick={() => void assets.refetch()}>Retry inputs</button></p>}
      {assets.isSuccess && !assetItems.length && <p>No retained inputs are currently available.</p>}
      {(assets.isSuccess || assets.isFetchNextPageError) && assetItems.length > 0 && <ul>{assetItems.map(asset => <RetainedAsset key={asset.id} asset={asset}/>)}</ul>}
      {assets.hasNextPage && (assets.isSuccess || assets.isFetchNextPageError) && <button className="ux-button" disabled={assets.isFetching} onClick={() => void assets.fetchNextPage()}>{assets.isFetchingNextPage ? 'Loading inputs…' : assets.isFetchNextPageError ? 'Retry next inputs' : 'Show more inputs'}</button>}
    </section>
    <section aria-label="Environment releases">
      <h4>Environment releases</h4>
      {releases.isPending && <p role="status">Loading releases…</p>}
      {releases.isError && !releases.isFetchNextPageError && <p role="alert">Releases unavailable. <button className="underline" onClick={() => void releases.refetch()}>Retry releases</button></p>}
      {releases.isSuccess && !releaseItems.length && <p>No environment releases are visible in this workspace.</p>}
      {(releases.isSuccess || releases.isFetchNextPageError) && releaseItems.length > 0 && <ul>{releaseItems.map(release => <li key={release.id}>
        <Link className="underline" to="/map" search={old => ({...linkScope, mapState: old.mapState, tool: 'environments', workbench: JSON.stringify({id: release.id, manifest: release.manifestSha256})})}>{release.name}</Link>
        <p>{release.state} · {formatTimestamp(release.createdAt)}</p>
      </li>)}</ul>}
      {releases.hasNextPage && (releases.isSuccess || releases.isFetchNextPageError) && <button className="ux-button" disabled={releases.isFetching} onClick={() => void releases.fetchNextPage()}>{releases.isFetchingNextPage ? 'Loading releases…' : releases.isFetchNextPageError ? 'Retry next releases' : 'Show more releases'}</button>}
    </section>
  </div>
}
