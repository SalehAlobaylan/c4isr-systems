import {useEffect, useRef, useState} from 'react'
import {Box3, BufferGeometry, DirectionalLight, HemisphereLight, Mesh, type Object3D, PerspectiveCamera, Points, PointsMaterial, Raycaster, Scene, Sphere, Vector2, Vector3, WebGLRenderer} from 'three'
import {OrbitControls} from 'three/addons/controls/OrbitControls.js'
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js'
import {fetchDisplayBytes, type Display, type DisplaySource} from './displayBytes'
import {useWorkspace} from '@/lib/workspaceContext'
import {geographicSurfacePoint, localGroundCoordinates, makeBuildingVolumes, makeOpticalLayer, makeVectorLayer, pickPolygon, vectorColors, type EnrichmentDisplay, type OpticalDisplay, type RadarDisplay, type SceneFeature, type SceneLayer, type VectorDisplay} from './GeographicSceneLayers'
import {previewLayers} from './LayerCatalog'
import type {ContextMarker} from './GeographicContext'

function releaseMesh(scene: Object3D) {
  scene.traverse(object => {
    if (!(object instanceof Mesh)) return
    object.geometry.dispose()
    const materials = Array.isArray(object.material) ? object.material : [object.material]
    for (const material of materials) material.dispose()
  })
}

type Overlay = {layer: SceneLayer; display: Display}
type Runtime = {world: Scene; terrain: Object3D; render: () => void; hits: Map<string, ReturnType<typeof makeVectorLayer>>}
type PreviewProps = {
  display: Display; overlays?: Overlay[]; selection?: string;
  onSelect?: (value: string) => void; onLocate?: (feature: SceneFeature) => void;
  onLocateTerrain?: (point: [number, number, number]) => void;
  contextMarkers?: ContextMarker[];
  onContextDrawn?: (count: number) => void;
}

export default function TerrainPreview(props: PreviewProps) {
  const {api, signal} = useWorkspace()
  return <GeographicScene {...props} source={api.environmentDisplayContent} signal={signal} />
}

export function GeographicScene({display, overlays = [], selection, onSelect, onLocate, onLocateTerrain, contextMarkers, onContextDrawn, source, signal}: PreviewProps & {source: DisplaySource; signal: AbortSignal}) {
  const host = useRef<HTMLDivElement>(null)
  const reset = useRef<() => void>(undefined)
  const selectionCallback = useRef(onSelect)
  selectionCallback.current = onSelect
  const [retry, setRetry] = useState(0)
  const [status, setStatus] = useState('Verifying terrain…')
  const [error, setError] = useState('')
  const [runtime, setRuntime] = useState<Runtime>()
  const [point, setPoint] = useState<[number, number, number]>()
  const [pointMissing, setPointMissing] = useState(false)

  useEffect(() => {
    const container = host.current!
    const local = new AbortController()
    const loadSignal = AbortSignal.any([signal, local.signal])
    const world = new Scene()
    let renderer: WebGLRenderer | undefined
    let controls: OrbitControls | undefined
    let observer: ResizeObserver | undefined
    let terrain: Object3D | undefined
    let frame = 0
    let stopped = false
    const hits: Runtime['hits'] = new Map()

    function dispose() {
      stopped = true
      observer?.disconnect()
      if (frame) cancelAnimationFrame(frame)
      controls?.dispose()
      reset.current = undefined
      if (renderer) {
        renderer.domElement.removeEventListener('webglcontextlost', lost)
        renderer.dispose()
        renderer.forceContextLoss()
        renderer.domElement.remove()
        renderer = undefined
      }
      if (terrain) releaseMesh(terrain)
      world.clear()
    }
    function lost(event: Event) {
      event.preventDefault()
      local.abort()
      dispose()
      setError('The 3D display lost its graphics context.')
      setRuntime(undefined)
    }
    function scopeEnded() {
      dispose()
      setRuntime(undefined)
      setStatus('Terrain preview closed.')
    }

    async function load() {
      const bytes = await fetchDisplayBytes(source, display, loadSignal)
      try {
        renderer = new WebGLRenderer({antialias: false, powerPreference: 'high-performance'})
      } catch {
        throw new Error('3D terrain requires WebGL 2. Release details are available above.')
      }
      const {scene} = await new GLTFLoader().parseAsync(bytes, '')
      if (stopped || loadSignal.aborted || !renderer) {
        releaseMesh(scene)
        return
      }
      const box = new Box3().setFromObject(scene, true)
      const extent = box.getBoundingSphere(new Sphere())
      if (!Number.isFinite(extent.radius) || extent.radius <= 0) {
        releaseMesh(scene)
        throw new Error('The terrain has no usable extent.')
      }
      const camera = new PerspectiveCamera(45, 1, extent.radius / 1000, extent.radius * 30)
      const canvas = renderer.domElement
      terrain = scene
      // Geographic inspection keeps a predictable framebuffer on compact and
      // high-DPI devices. Text and controls retain their native browser scale.
      renderer.setPixelRatio(1)
      renderer.setClearColor(0x14212a)
      canvas.tabIndex = 0
        canvas.setAttribute('aria-label', display.recipe === 'ground-glb-v1' ? 'Interactive modeled bare earth' : 'Interactive reference terrain')
      canvas.addEventListener('webglcontextlost', lost)
      container.appendChild(canvas)
      world.add(scene, new HemisphereLight(0xf2f7ff, 0x6a5c43, 2))
      const sun = new DirectionalLight(0xfff5de, 2)
      sun.position.copy(extent.center).addScaledVector(new Vector3(-.7, 1, -.5), extent.radius)
      sun.target.position.copy(extent.center)
      world.add(sun, sun.target)
      const orbit = new OrbitControls(camera, canvas)
      controls = orbit
      orbit.minDistance = extent.radius / 200
      orbit.maxDistance = extent.radius * 15
      orbit.listenToKeyEvents(canvas)
      const draw = () => {
        frame = 0
        if (!renderer) return
        renderer.render(world, camera)
        canvas.dataset.drawCalls = String(renderer.info.render.calls)
        canvas.dataset.geometryCount = String(renderer.info.memory.geometries)
        canvas.dataset.textureCount = String(renderer.info.memory.textures)
      }
      const render = () => { if (!frame && renderer) frame = requestAnimationFrame(draw) }
      const ray = new Raycaster()
      const inspect = (cursor: Vector2) => {
        ray.setFromCamera(cursor, camera)
        const origin = display.frame?.origin
        const hit = ray.intersectObject(scene, true)[0]?.point
        const surface = hit && Array.isArray(origin) && origin.length === 3 && origin.every(Number.isFinite)
          ? geographicSurfacePoint(origin, hit.x, hit.y, hit.z) : undefined
        setPoint(surface)
        setPointMissing(!surface)
        return surface
      }
      canvas.addEventListener('keydown', event => {
        if (event.key !== 'Enter') return
        event.preventDefault()
        inspect(new Vector2())
      }, {signal: loadSignal})
      let down: [number, number] | undefined
      canvas.addEventListener('pointerdown', event => { down = [event.clientX, event.clientY] }, {signal: loadSignal})
      canvas.addEventListener('pointercancel', () => {down = undefined}, {signal: loadSignal})
      canvas.addEventListener('pointerup', event => {
        const start = down
        down = undefined
        if (event.button !== 0 || !start || Math.hypot(event.clientX - start[0], event.clientY - start[1]) > 5) return
        const bounds = canvas.getBoundingClientRect()
        if (!bounds.width || !bounds.height) return
        ray.params.Line!.threshold = Math.max(1, Math.min(18, camera.position.distanceTo(orbit.target) / bounds.height * 3))
        const surface = inspect(new Vector2((event.clientX - bounds.left) / bounds.width * 2 - 1,
          1 - (event.clientY - bounds.top) / bounds.height * 2))
        let distance = Infinity, chosen = ''
        for (const [id, overlay] of hits) {
          const hit = ray.intersectObject(overlay.object, false)[0]
          if (hit?.index === undefined || hit.distance >= distance) continue
          const feature = overlay.bySegment[Math.floor(hit.index / 2)]
          if (!feature) continue
          chosen = `${id}:${feature.id}`
          distance = hit.distance
        }
        if (!chosen && surface) {
          for (const [id, overlay] of hits) {
            const feature = pickPolygon(overlay.polygons, surface[0], surface[1])
            if (feature) { chosen = `${id}:${feature.id}`; break }
          }
        }
        if (chosen) selectionCallback.current?.(chosen)
      }, {signal: loadSignal})
      const fit = () => {
        const halfAngle = Math.atan(Math.tan(Math.PI / 8) * Math.min(1, camera.aspect))
        const distance = extent.radius / Math.sin(halfAngle) * 1.05
        orbit.target.copy(extent.center)
        camera.position.copy(extent.center).addScaledVector(new Vector3(.6, .85, 1).normalize(), distance)
        orbit.update()
        render()
      }
      const resize = () => {
        const {clientWidth: width, clientHeight: height} = container
        if (!renderer || !width || !height) return
        renderer.setSize(width, height)
        camera.aspect = width / height
        camera.updateProjectionMatrix()
        render()
      }
      reset.current = fit
      orbit.addEventListener('change', render)
      observer = new ResizeObserver(resize)
      observer.observe(container)
      resize()
      fit()
      if (frame) cancelAnimationFrame(frame)
      draw()
      setRuntime({world, terrain: scene, render, hits})
      setStatus(display.recipe === 'ground-glb-v1' ? 'Verified modeled bare earth' : 'Verified reference terrain')
    }

    setRuntime(undefined)
    setPoint(undefined)
    setPointMissing(false)
    setError('')
    setStatus('Verifying terrain…')
    signal.addEventListener('abort', scopeEnded, {once: true})
    void load().catch(error => {
      if (loadSignal.aborted) return
      dispose()
      setError(error instanceof Error ? error.message : 'Terrain could not be displayed.')
    })
    return () => {
      signal.removeEventListener('abort', scopeEnded)
      local.abort()
      dispose()
    }
  }, [source, display.id, display.sha256, display.bytes, display.recipe, retry, signal])

  useEffect(() => {
    const origin = display.frame?.origin
    if (!runtime || !contextMarkers?.length || !Array.isArray(origin) || origin.length !== 3 || !origin.every(Number.isFinite)) {
      onContextDrawn?.(0)
      return
    }
    const points: Vector3[] = []
    const ray = new Raycaster(new Vector3(), new Vector3(0, -1, 0))
    for (const marker of contextMarkers) {
      const [east, south] = localGroundCoordinates(origin, ...marker.position)
      ray.ray.origin.set(east, 25000, south)
      const hit = ray.intersectObject(runtime.terrain, true)[0]
      if (hit) points.push(hit.point.add(new Vector3(0, 4, 0)))
    }
    const geometry = new BufferGeometry().setFromPoints(points)
    const material = new PointsMaterial({color: 0xff9c66, size: 6, sizeAttenuation: false})
    const markers = new Points(geometry, material)
    markers.renderOrder = 4
    runtime.world.add(markers)
    onContextDrawn?.(points.length)
    runtime.render()
    return () => {runtime.world.remove(markers); geometry.dispose(); material.dispose(); runtime.render()}
  }, [contextMarkers, runtime, display.frame, onContextDrawn])

  return <section aria-label="Reference terrain" className="space-y-2">
    {error ? <p role="alert">{error}</p> : <p role="status">{status}</p>}
    {error && <button className="ux-button" onClick={() => setRetry(value => value + 1)}>Retry terrain preview</button>}
    <div ref={host} hidden={!!error} className="h-64 w-full overflow-hidden rounded border border-edge sm:h-80" />
    {runtime && <>
      <button className="ux-button" onClick={() => reset.current?.()}>Reset terrain view</button>
      <p>Drag to orbit, use the arrow keys to pan, and pinch or scroll to zoom. Click the surface or press Enter on the canvas to inspect a point.</p>
      {point && <div className="space-y-1">
        <p role="status">Terrain point {point[0].toFixed(6)}°, {point[1].toFixed(6)}° · {point[2].toFixed(2)} m above the WGS84 ellipsoid</p>
        {onLocateTerrain && <button className="ux-button" onClick={() => onLocateTerrain(point)}>Locate terrain point in Atlas</button>}
      </div>}
      {pointMissing && <p role="status">No retained terrain at this location.</p>}
      {overlays.map(overlay => <SceneOverlay key={overlay.display.id} overlay={overlay} source={source} signal={signal}
        runtime={runtime} selection={selection} onSelect={onSelect} onLocate={onLocate} />)}
    </>}
    <p>Metres · east, up, south · {display.verticalDatum} · Physical accuracy {display.physicalAccuracy.toLowerCase()}</p>
  </section>
}

function SceneOverlay({overlay, source, signal, runtime, selection, onSelect, onLocate}: {
  overlay: Overlay; source: DisplaySource; signal: AbortSignal; runtime: Runtime;
  selection?: string; onSelect?: PreviewProps['onSelect']; onLocate?: PreviewProps['onLocate'];
}) {
  const {layer, display} = overlay
  const [value, setValue] = useState<OpticalDisplay | EnrichmentDisplay | RadarDisplay | VectorDisplay>()
  const [failure, setFailure] = useState(false)
  const [filter, setFilter] = useState('')
  const [retry, setRetry] = useState(0)
  const [extrude, setExtrude] = useState(false)
  const [volumes, setVolumes] = useState(0)
  const picked = selection?.startsWith(`${display.id}:`) ? selection.slice(display.id.length + 1) : undefined
  const features = value && 'features' in value ? value.features : undefined
  const selected = features?.find(feature => feature.id === picked)

  useEffect(() => {
    setValue(undefined)
    setFailure(false)
    const local = new AbortController()
    const loadSignal = AbortSignal.any([local.signal, signal])
    let detach: (() => void) | undefined
    async function mount() {
      const bytes = await fetchDisplayBytes(source, display, loadSignal)
      const layerFile = JSON.parse(new TextDecoder('utf-8', {fatal: true}).decode(bytes))
      if (layerFile.schema !== 'emad.display-layer.v1' || layerFile.manifestSha256 !== display.manifestSha256 || layerFile.layer !== layer) {
        throw new Error('The layer belongs to different source inputs.')
      }
      const data = layerFile.data as OpticalDisplay | EnrichmentDisplay | RadarDisplay | VectorDisplay
      const visual = layer === 'imagery' || layer === 'land-cover' || layer === 'ground-delta' || layer === 'radar'
        ? await makeOpticalLayer(data as OpticalDisplay | EnrichmentDisplay | RadarDisplay, runtime.terrain, display.frame, loadSignal)
        : makeVectorLayer((data as VectorDisplay).features, vectorColors[layer])
      if (loadSignal.aborted) { visual.dispose(); return }
      runtime.world.add(visual.object)
      if ('bySegment' in visual) runtime.hits.set(display.id, visual)
      detach = () => {
        runtime.hits.delete(display.id)
        runtime.world.remove(visual.object)
        visual.dispose()
        runtime.render()
      }
      setValue(data)
      runtime.render()
    }
    void mount().catch(() => { if (!loadSignal.aborted) setFailure(true) })
    return () => { local.abort(); detach?.() }
  }, [source, display.id, display.sha256, display.bytes, display.recipe, display.manifestSha256, layer, runtime, signal, retry])

  useEffect(() => {
    if (!selected?.paths.length) return
    const highlight = makeVectorLayer([selected], 0xff744f)
    highlight.object.renderOrder = 3
    highlight.object.material.depthTest = false
    runtime.world.add(highlight.object)
    runtime.render()
    return () => { runtime.world.remove(highlight.object); highlight.dispose(); runtime.render() }
  }, [selected, runtime])

  useEffect(() => {
    if (layer !== 'buildings' || !features || !extrude) return
    const buildings = makeBuildingVolumes(features)
    runtime.world.add(buildings.object)
    setVolumes(buildings.count)
    runtime.render()
    return () => { runtime.world.remove(buildings.object); buildings.dispose(); runtime.render() }
  }, [layer, features, extrude, runtime])

  if (failure) return <p role="alert">{previewLayers[layer].name} could not be verified. <button className="ux-button" onClick={() => setRetry(value => value + 1)}>Retry layer</button></p>
  if (!value) return <p role="status">Verifying {previewLayers[layer].name.toLowerCase()}…</p>
  if ('radar' in value) return <details open>
    <summary>Radar backscatter · {value.radar.sourceStart.slice(0, 10)}</summary>
    <p>{value.radar.product} · {value.radar.sourceStart} to {value.radar.sourceEnd}</p>
    <p>{value.radar.processing} · {value.radar.mode} · {value.radar.polarization} · {value.radar.direction} orbit {value.radar.absoluteOrbit} (relative {value.radar.relativeOrbit}).</p>
    <p>{value.radar.spacingM} m native sample spacing · {value.radar.crs} · {value.radar.unit}.</p>
    <p>{value.method}. Display range {value.displayDbRange.join(' to ')} dB. {value.validPixels.toLocaleString()} valid display pixels.</p>
    <p>{value.radar.layoverShadow} Contains modified Copernicus Sentinel data processed by Catalyst for Microsoft Planetary Computer.</p>
  </details>
  if ('scene' in value) return <details>
    <summary>{previewLayers[layer].name} · {value.acquiredAt.slice(0, 10)}</summary>
    <p>{value.scene} · {value.acquiredAt}</p>
    <p>{value.bands.map(b => `${b.id} (scale ${b.scale}, offset ${b.offset})`).join(', ')}</p>
    <p>{value.validPixels.toLocaleString()} of {(value.width * value.height).toLocaleString()} valid pixels. Sentinel-2 SCL classes {value.validScl.join(', ')}.</p>
    <p>{value.grid}. {value.color}.</p>
    <p>Scene metadata SHA-256 <code className="break-all">{value.sceneMetadataSha256}</code></p>
  </details>

  if ('legend' in value) return <details open>
    <summary>{previewLayers[layer].name} · {value.validPixels.toLocaleString()} valid pixels</summary>
    <p>{previewLayers[layer].explanation} {value.method}.</p>
    <ul>{value.sources.map(item => <li key={`${item.layer}:${item.edition}`}>{item.producer} · {item.edition} · {item.dataTime}</li>)}</ul>
    {layer === 'land-cover' && <p>© ESA WorldCover project 2021. Contains modified Copernicus Sentinel data (2021) processed by the ESA WorldCover consortium. Class accuracy in this area has not been established.</p>}
    {value.comparison.map(item => <p key={item.component}>{item.pairedSamples.toLocaleString()} of {item.surfaceValidSamples.toLocaleString()} valid surface samples paired.
      {' '}Median {item.medianM.toFixed(1)} m. 5th to 95th percentile: {item.p05M.toFixed(1)} to {item.p95M.toFixed(1)} m.
      {' '}Sources differ in time, resolution and what they represent. Physical accuracy is unknown.</p>)}
    <ul aria-label="Raster legend">{value.legend.map(item => <li key={item.label}>
      <span aria-hidden="true" style={{display: 'inline-block', width: '1em', height: '1em', backgroundColor: `#${item.rgb}`}} />
      {' '}{layer === 'land-cover' && `${item.code} · `}{item.label} · {item.pixels.toLocaleString()}
    </li>)}</ul>
  </details>

  const options = value.features.filter(feature => `${feature.properties.sourceId} ${feature.properties.tags.name ?? ''}`.toLowerCase().includes(filter.trim().toLowerCase())).slice(0, 50)
  if (selected && !options.includes(selected)) options.unshift(selected)
  return <details open={!!picked || undefined}>
    <summary>{previewLayers[layer].name} · {value.displayedFeatures.toLocaleString()} of {value.availableFeatures.toLocaleString()} drawn</summary>
    <p>{value.method}. {previewLayers[layer].explanation}</p>
    {layer === 'buildings' && <>
      <label><input type="checkbox" checked={extrude} onChange={event => setExtrude(event.target.checked)} /> Extrude footprints with source-tagged heights</label>
      {extrude && <p>{volumes} buildings. Each base uses the mean retained surface height along its perimeter.</p>}
    </>}
    <label>Find {previewLayers[layer].name.toLowerCase()} by name or source ID <input className="atlas-field" value={filter} onChange={event => setFilter(event.target.value)} /></label>
    <label>Inspect feature <select className="atlas-field" value={selected?.id ?? ''} onChange={event => onSelect?.(event.target.value ? `${display.id}:${event.target.value}` : '')}>
      <option value="">Select a feature</option>
      {options.map(feature => <option key={feature.id} value={feature.id}>{feature.properties.tags.name || feature.properties.sourceId}</option>)}
    </select></label>
    {picked && !selected && <p role="status">This feature is unavailable in the selected source edition.</p>}
    {selected && <div>
      <p><strong>{selected.properties.sourceId}</strong> · {selected.properties.producer} · {selected.properties.product} · {selected.properties.dataTime}</p>
      <p>Source SHA-256 <code className="break-all">{selected.properties.edition}</code></p>
      {selected.properties.dimensions ? <dl aria-label="Feature dimensions">{Object.entries(selected.properties.dimensions).map(([name, item]) =>
        <div key={name}><dt>{{minHeight: 'Minimum height', roofHeight: 'Roof height', clearance: 'Vertical clearance'}[name] ?? name}</dt>
          <dd>{item.value === null ? 'Unknown' : `${item.value}${item.unit === 'm' ? ' m' : ''} (source tag)`} · {item.evidence}
            {item.value !== null && ' · uncertainty unknown'}</dd></div>)}</dl>
        : layer === 'buildings' && <p>Building height {selected.properties.height.kind === 'source-tag' ? `${selected.properties.height.metres} m (source tag)` : 'unknown'}.</p>}
      {!selected.paths.length && <p>This feature is retained in the source geometry but could not be drawn on the surface.</p>}
      <p>Retained geometry: {selected.geometry.type}. Scene lines are draped on the reference surface.</p>
      {onLocate && <button className="ux-button" onClick={() => onLocate(selected)}>Locate feature in Atlas</button>}
      <details><summary>Retained source tags</summary><dl>{Object.entries(selected.properties.tags).map(([key, value]) => <div key={key}><dt>{key}</dt><dd className="break-words">{value}</dd></div>)}</dl></details>
    </div>}
  </details>
}
