import type {Feature, Geometry} from 'geojson'
import {
  BufferGeometry, ExtrudeGeometry, Float32BufferAttribute, Group, LineBasicMaterial, LineSegments, Mesh, MeshBasicMaterial,
  NearestFilter, Path, Shape, SRGBColorSpace, Texture, Vector2, type Object3D,
} from 'three'

export type VectorLayer = 'roads' | 'buildings' | 'water'
export type SceneLayer = 'imagery' | 'land-cover' | 'ground-delta' | 'radar' | VectorLayer
export type Dimension = {value: number | null; unit: 'm' | 'count'; origin: 'provider-stated' | 'unknown'; evidence: string; uncertainty: 'UNKNOWN'}
export type SceneFeature = Feature<Geometry, {
  sourceId: string; edition: string; product: string; dataTime: string; producer: string;
  tags: Record<string, string>; height: {kind: 'unknown' | 'source-tag'; metres?: number};
  dimensions?: Record<string, Dimension>;
}> & {id: string; paths: number[][][]}
export type VectorDisplay = {
  features: SceneFeature[]; availableFeatures: number; displayedFeatures: number; vertices: number; method: string;
}
export type OpticalDisplay = {
  scene: string; acquiredAt: string; sceneMetadataSha256: string;
  bands: {id: string; scale: number; offset: number}[]; validScl: number[]; grid: string; color: string;
  bounds: [number, number, number, number]; width: number; height: number; validPixels: number; texture: string;
}
export type EnrichmentDisplay = Pick<OpticalDisplay, 'bounds' | 'width' | 'height' | 'validPixels' | 'texture'> & {
  method: string; legend: {code: number | null; label: string; rgb: string; pixels: number}[];
  sources: {layer: string; producer: string; edition: string; dataTime: string}[];
  comparison: {component: number; pairedSamples: number; surfaceValidSamples: number; medianM: number; p05M: number; p95M: number}[];
}
export type RadarDisplay = Pick<OpticalDisplay, 'bounds' | 'width' | 'height' | 'validPixels' | 'texture'> & {
  method: string; displayDbRange: [number, number];
  radar: {product: string; sourceStart: string; sourceEnd: string; absoluteOrbit: number; relativeOrbit: number; direction: string;
    mode: string; polarization: string; processing: string; unit: string; crs: string; spacingM: number; layoverShadow: string}
}

const a = 6378137
const e2 = 6.6943799901413165e-3
const radians = Math.PI / 180

export function geographicCoordinates(origin: number[], east: number, up: number, south: number) {
  return geographicSurfacePoint(origin, east, up, south).slice(0, 2) as [number, number]
}

export function geographicSurfacePoint(origin: number[], east: number, up: number, south: number): [number, number, number] {
  const lng = origin[0] * radians, lat = origin[1] * radians, height = origin[2]
  const sin = Math.sin(lat), cos = Math.cos(lat)
  const meridian = Math.sqrt(1 - e2 * sin * sin)
  const prime = a / meridian
  const along = (prime + height) * cos + up * cos + south * sin
  const x = along * Math.cos(lng) - east * Math.sin(lng)
  const y = along * Math.sin(lng) + east * Math.cos(lng)
  const z = (prime * (1 - e2) + height + up) * sin - south * cos
  const distance = Math.hypot(x, y)
  let angle = Math.atan2(z, distance * (1 - e2))
  for (let iteration = 0; iteration < 5; iteration++) {
    const s = Math.sin(angle)
    angle = Math.atan2(z + e2 * a * s / Math.sqrt(1 - e2 * s * s), distance)
  }
  const localSin = Math.sin(angle), localCos = Math.cos(angle)
  const localHeight = distance * localCos + z * localSin - a * Math.sqrt(1 - e2 * localSin * localSin)
  return [Math.atan2(y, x) / radians, angle / radians, localHeight]
}

export function localGroundCoordinates(origin: number[], longitude: number, latitude: number) {
  const lng = longitude * radians, lat = latitude * radians
  const lat0 = origin[1] * radians, delta = lng - origin[0] * radians
  const sin = Math.sin(lat), cos = Math.cos(lat)
  const prime = a / Math.sqrt(1 - e2 * sin * sin)
  const prime0 = a / Math.sqrt(1 - e2 * Math.sin(lat0) ** 2)
  const east = prime * cos * Math.sin(delta)
  const north = Math.cos(lat0) * (prime * (1 - e2) * sin - prime0 * (1 - e2) * Math.sin(lat0))
    - Math.sin(lat0) * (prime * cos * Math.cos(delta) - prime0 * Math.cos(lat0))
  return [east, -north]
}

export async function makeOpticalLayer(data: Pick<OpticalDisplay, 'bounds' | 'width' | 'height' | 'texture'>, terrain: Object3D, frame: Record<string, unknown>, signal: AbortSignal) {
  const origin = frame.origin
  if (!Array.isArray(origin) || origin.length !== 3 || !origin.every(Number.isFinite)) throw new Error('The imagery frame is unavailable.')
  const packed = Uint8Array.from(atob(data.texture), value => value.charCodeAt(0))
  const bitmap = await createImageBitmap(new Blob([packed], {type: 'image/png'}), {imageOrientation: 'flipY'})
  if (signal.aborted || bitmap.width !== data.width || bitmap.height !== data.height) {
    bitmap.close()
    signal.throwIfAborted()
    throw new Error('The imagery pixels differ from the declared grid.')
  }
  const texture = new Texture(bitmap)
  texture.colorSpace = SRGBColorSpace
  texture.flipY = false
  texture.minFilter = NearestFilter
  texture.magFilter = NearestFilter
  texture.generateMipmaps = false
  texture.needsUpdate = true
  const material = new MeshBasicMaterial({map: texture, alphaTest: .5, polygonOffset: true, polygonOffsetFactor: -1, polygonOffsetUnits: -1})
  const group = new Group()
  const [west, south, east, north] = data.bounds
  terrain.traverse(object => {
    if (!(object instanceof Mesh)) return
    const geometry = object.geometry.clone() as BufferGeometry
    geometry.deleteAttribute('normal')
    const positions = geometry.getAttribute('position')
    const uv = new Float32Array(2 * positions.count)
    for (let i = 0; i < positions.count; i++) {
      const [lon, lat] = geographicCoordinates(origin, positions.getX(i), positions.getY(i), positions.getZ(i))
      uv[i * 2] = (lon - west) / (east - west)
      uv[i * 2 + 1] = (lat - south) / (north - south)
    }
    geometry.setAttribute('uv', new Float32BufferAttribute(uv, 2))
    const mesh = new Mesh(geometry, material)
    mesh.renderOrder = 1
    group.add(mesh)
  })
  return {object: group, dispose() {
    for (const mesh of group.children) (mesh as Mesh).geometry.dispose()
    material.dispose()
    texture.dispose()
    bitmap.close()
  }}
}

export const vectorColors: Record<VectorLayer, number> = {roads: 0xffda8c, buildings: 0xfaf3de, water: 0x69d4ff}

function insideRing(lon: number, lat: number, ring: number[][]) {
  let inside = false
  for (let i = 1; i < ring.length; i++) {
    const [x0, y0] = ring[i - 1], [x1, y1] = ring[i]
    if ((y0 > lat) !== (y1 > lat) && lon < x0 + (x1 - x0) * (lat - y0) / (y1 - y0)) inside = !inside
  }
  return inside
}

export function pickPolygon(features: SceneFeature[], lon: number, lat: number) {
  return features.find(({geometry}) => {
    const polygons = geometry.type === 'Polygon' ? [geometry.coordinates] : geometry.type === 'MultiPolygon' ? geometry.coordinates : []
    return polygons.some(([outer, ...holes]) => insideRing(lon, lat, outer) && !holes.some(hole => insideRing(lon, lat, hole)))
  })
}

export function makeBuildingVolumes(features: SceneFeature[]) {
  const group = new Group()
  const material = new MeshBasicMaterial({color: 0xcaa872, transparent: true, opacity: .62})
  for (const feature of features) {
    const {geometry, paths, properties} = feature
    const metres = properties.height.metres
    if (properties.height.kind !== 'source-tag' || !metres || geometry.type !== 'Polygon' ||
      paths.length !== geometry.coordinates.length || paths.some(path => path[0][0] !== path.at(-1)![0] || path[0][2] !== path.at(-1)![2])) continue
    const [outer, ...holes] = paths.map(path => path.map(([east, , south]) => new Vector2(east, -south)))
    const shape = new Shape(outer)
    shape.holes = holes.map(ring => new Path(ring))
    const solid = new ExtrudeGeometry(shape, {depth: metres, bevelEnabled: false})
    solid.rotateX(-Math.PI / 2)
    const mesh = new Mesh(solid, material)
    const ring = paths[0]
    mesh.position.y = ring.reduce((sum, point) => sum + point[1], 0) / ring.length
    group.add(mesh)
  }
  return {object: group, count: group.children.length, dispose() {
    for (const child of group.children) (child as Mesh).geometry.dispose()
    material.dispose()
  }}
}

export function makeVectorLayer(features: SceneFeature[], color: number) {
  const positions: number[] = []
  const bySegment: SceneFeature[] = []
  for (const feature of features) {
    for (const path of feature.paths) {
      for (let i = 1; i < path.length; i++) {
        positions.push(...path[i - 1], ...path[i])
        bySegment.push(feature)
      }
    }
  }
  const geometry = new BufferGeometry().setAttribute('position', new Float32BufferAttribute(positions, 3))
  const material = new LineBasicMaterial({color})
  const object = new LineSegments(geometry, material)
  object.renderOrder = 2
  return {object, bySegment, polygons: features.filter(f => f.geometry.type === 'Polygon' || f.geometry.type === 'MultiPolygon'),
    dispose() { geometry.dispose(); material.dispose() }}
}
