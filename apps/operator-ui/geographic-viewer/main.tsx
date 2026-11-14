import '@fontsource/ibm-plex-sans/latin-400.css'
import '@fontsource/ibm-plex-sans-arabic/arabic-400.css'
import 'maplibre-gl/dist/maplibre-gl.css'
import './viewer.css'
import {lazy, Suspense, useEffect, useRef, useState} from 'react'
import {createRoot} from 'react-dom/client'
import maplibregl, {type Map as LibreMap} from 'maplibre-gl'
import type {FeatureCollection} from 'geojson'
import {fetchDisplayBytes, type Display, type DisplaySource} from '@/features/workspace/displayBytes'
import {openStreetArchive, streetAttribution} from '@/features/workspace/streetMap'
import {previewLayers} from '@/features/workspace/LayerCatalog'
import type {SceneLayer} from '@/features/workspace/GeographicSceneLayers'

const Terrain = lazy(async () => ({default: (await import('@/features/workspace/TerrainPreview')).GeographicScene}))
const lifetime = new AbortController()
const source: DisplaySource = async (id, signal) => {
  const response = await fetch(`./displays/${id}`, {signal, cache: 'no-store'})
  if (!response.ok || !response.body) throw new Error('An issued display file is missing. Verify the package again.')
  return {body: response.body, etag: response.headers.get('ETag')}
}

type Issuance = {schema: string; name: string; releaseId: string; viewerVersion: string; expiresAt: string;
  manifestSha256: string; displays: Display[]; terms: {attribution: string; legalBasis: string; termsUrl: string}[]}
type Navigation = {version: string; definition: string; attribution: string; features: FeatureCollection}

function LocalMap({navigation, streets, terrain}: {navigation: Navigation; streets?: Display; terrain: Display}) {
  const root = useRef<HTMLDivElement>(null)
  const map = useRef<LibreMap>(null)
  const [error, setError] = useState('')
  const [loaded, setLoaded] = useState(false)
  const countries = navigation.features.features.filter(f => f.properties?.kind === 'country')
  const subdivisions = navigation.features.features.filter(f => f.properties?.kind === 'region')
  const focus = (bounds: number[]) => map.current?.fitBounds(bounds as [number, number, number, number], {padding: 36, duration: 0, maxZoom: 16})

  useEffect(() => {
    const controller = new AbortController()
    const markers: maplibregl.Marker[] = []
    let closeStreets: (() => void) | undefined
    let m: LibreMap
    try {
      m = new maplibregl.Map({container: root.current!, center: [43, 26], zoom: 3.5, minZoom: 2, maxZoom: 18,
        renderWorldCopies: false, style: {version: 8, sources: {}, layers: [{id: 'background', type: 'background', paint: {'background-color': '#12252e'}}]}})
    } catch {
      setError('The map requires WebGL 2. The retained place list and source information are available below.')
      return
    }
    map.current = m
    m.addControl(new maplibregl.NavigationControl({showCompass: false}))
    m.addControl(new maplibregl.ScaleControl({unit: 'metric'}))
    m.once('load', () => {
      m.addSource('regional', {type: 'geojson', data: navigation.features})
      m.addLayer({id: 'countries', type: 'fill', source: 'regional', filter: ['==', ['get', 'kind'], 'country'], paint: {'fill-color': '#263c42', 'fill-opacity': .88}})
      m.addLayer({id: 'boundaries', type: 'line', source: 'regional', paint: {'line-color': '#61777a', 'line-width': .9}})
      const showLabels = () => {
        markers.splice(0).forEach(marker => marker.remove())
        const extent = m.getBounds()
        const list = m.getZoom() < 6 ? countries : subdivisions
        for (const feature of list) {
          const point = feature.properties?.labelPoint as [number, number]
          if (!extent.contains(point)) continue
          const text = document.createElement('bdi')
          text.dir = 'auto'
          text.className = 'place-label'
          text.textContent = feature.properties?.labelArabic || feature.properties?.name
          markers.push(new maplibregl.Marker({element: text}).setLngLat(point).addTo(m))
        }
      }
      showLabels()
      m.on('moveend', showLabels)
      setLoaded(true)
      if (!streets) return
      void fetchDisplayBytes(source, streets, controller.signal).then(bytes => openStreetArchive(m, bytes, controller.signal)).then(archive => {
        closeStreets = archive.close
        if (controller.signal.aborted) closeStreets()
      }).catch(() => { if (!controller.signal.aborted) setError('Street detail could not open. Regional navigation remains available.') })
    })
    return () => {controller.abort(); closeStreets?.(); markers.forEach(marker => marker.remove()); m.remove(); map.current = null}
  }, [navigation, streets?.id])

  return <section aria-label="Local Atlas">
    <div className="actions">
      <button disabled={!loaded} onClick={() => focus([25, 10, 65, 45])}>Middle East</button>
      <button disabled={!loaded} onClick={() => {const saudi = countries.find(f => f.id === 'country:SAU'); if (saudi) focus(saudi.properties?.bounds)}}>Saudi Arabia</button>
      <button disabled={!loaded} onClick={() => focus(terrain.bounds)}>Environment extent</button>
    </div>
    {error && <p role="status">{error}</p>}
    <div ref={root} aria-label="Retained regional geography" className="map" />
    <p>{navigation.attribution} · {navigation.definition}</p>
    {streets && <p>{streetAttribution} · detail at zoom 12–16 within {streets.bounds.map(value => value.toFixed(3)).join(', ')}.</p>}
    <details><summary>Countries and Saudi regions</summary><ul>{[...countries, ...subdivisions].map(feature => <li key={feature.id}>
      <button disabled={!loaded} onClick={() => focus(feature.properties?.bounds)}><bdi>{feature.properties?.name}</bdi></button>
    </li>)}</ul></details>
  </section>
}

function Viewer({issuance, navigation}: {issuance: Issuance; navigation: Navigation}) {
  const [mode, setMode] = useState<'atlas' | 'terrain'>('atlas')
  const [layers, setLayers] = useState<SceneLayer[]>([])
  const [selected, setSelected] = useState('')
  const [expired, setExpired] = useState(false)
  const [ground, setGround] = useState(false)
  const terrain = issuance.displays.find(display => display.recipe === 'terrain-glb-v1' || display.recipe === 'terrain-glb-v2')!
  const groundTerrain = issuance.displays.find(display => display.recipe === 'ground-glb-v1')
  const rasterLayers = new Set<SceneLayer>(['imagery', 'land-cover', 'ground-delta', 'radar'])
  const streets = issuance.displays.find(display => display.recipe === 'street-pmtiles-v1')
  const overlays = (Object.keys(previewLayers) as SceneLayer[]).flatMap(layer => {
    const display = issuance.displays.find(display => display.recipe === previewLayers[layer].recipe)
    return display ? [{layer, display}] : []
  })
  useEffect(() => {
    const timer = setInterval(() => { if (Date.now() >= Date.parse(issuance.expiresAt)) {lifetime.abort(); setExpired(true)} }, 1000)
    return () => clearInterval(timer)
  }, [issuance.expiresAt])

  return <main>
    <header><h1>{issuance.name}</h1><p>Portable geographic viewer · Expires {new Date(issuance.expiresAt).toLocaleString()}</p></header>
    {expired ? <p role="alert">The viewing authorization has expired. Close the viewer and follow the package retention terms.</p> : <>
      <nav aria-label="Viewer mode" className="actions">
        <button aria-pressed={mode === 'atlas'} onClick={() => setMode('atlas')}>Atlas</button>
        <button aria-pressed={mode === 'terrain'} onClick={() => setMode('terrain')}>3D preview</button>
      </nav>
      {mode === 'atlas' ? <LocalMap navigation={navigation} streets={streets} terrain={terrain} /> : <>
        {groundTerrain && <label><input type="checkbox" checked={ground} onChange={event => {setGround(event.target.checked); setSelected('')}} /> Show modeled bare earth</label>}
        {!!overlays.length && <fieldset className="actions"><legend>Display layers</legend>{overlays.map(({layer}) => <label key={layer}>
          <input type="checkbox" disabled={ground && !rasterLayers.has(layer)}
            checked={layers.includes(layer) && (!ground || rasterLayers.has(layer))}
            onChange={event => setLayers(event.target.checked ? [...layers.filter(item => !rasterLayers.has(layer) || !rasterLayers.has(item)), layer] : layers.filter(item => item !== layer))} />
          {previewLayers[layer].name}
        </label>)}</fieldset>}
        <Suspense fallback={<p role="status">Loading 3D preview…</p>}>
          <Terrain display={ground && groundTerrain ? groundTerrain : terrain}
            overlays={overlays.filter(({layer}) => layers.includes(layer) && (!ground || rasterLayers.has(layer)))} selection={selected} onSelect={setSelected}
            signal={lifetime.signal} source={source} />
        </Suspense>
      </>}
    </>}
    <details><summary>Release and use terms</summary>
      <p>{issuance.releaseId} · <code>{issuance.manifestSha256}</code></p>
      <ul>{[...new Map(issuance.terms.map(terms => [terms.legalBasis + terms.termsUrl, terms])).values()].map(terms => <li key={terms.legalBasis + terms.termsUrl}>
        <p>{terms.attribution} {terms.legalBasis}</p><p>{terms.termsUrl}</p>
      </li>)}</ul>
    </details>
  </main>
}

const root = createRoot(document.getElementById('root')!)
async function init() {
  const responses = await Promise.all(['./issuance.json', './navigation.json'].map(url => fetch(url, {cache: 'no-store'})))
  if (responses.some(response => !response.ok)) throw new Error('The issuance or navigation file is missing. Verify the package again.')
  const [issuance, navigation] = await Promise.all(responses.map(response => response.json())) as [Issuance, Navigation]
  if (issuance.schema !== 'emad.viewing-issuance.v1' || issuance.viewerVersion !== 'emad-geographic-viewer-v1' ||
    !(Date.parse(issuance.expiresAt) > Date.now()) || !issuance.displays.some(display => display.recipe === 'terrain-glb-v1' || display.recipe === 'terrain-glb-v2')) {
    throw new Error('This viewing authorization is invalid or has expired.')
  }
  root.render(<Viewer issuance={issuance} navigation={navigation} />)
}
void init().catch(error => root.render(<main><h1>Geographic viewer</h1><p role="alert">{String(error.message || error)}</p></main>))
