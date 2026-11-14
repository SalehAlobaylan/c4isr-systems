import maplibregl, {type Map as LibreMap} from 'maplibre-gl'
import {FileSource, PMTiles, TileType} from 'pmtiles'

export const streetAttribution = '© OpenStreetMap contributors · ODbL 1.0'

export async function openStreetArchive(map: LibreMap, bytes: ArrayBuffer, signal: AbortSignal, before?: string) {
  const archive = new PMTiles(new FileSource(new File([bytes], 'retained-street-map.pmtiles')))
  const header = await archive.getHeader()
  signal.throwIfAborted()
  if (header.specVersion !== 3 || header.tileType !== TileType.Mvt || header.minZoom !== 12 || header.maxZoom !== 16) {
    throw new Error('The retained street archive is unsupported.')
  }
  const bounds: [number, number, number, number] = [header.minLon, header.minLat, header.maxLon, header.maxLat]
  const scheme = `emadstreet${crypto.randomUUID().replaceAll('-', '')}`
  const source = 'retained-streets'
  const layers = ['street-water', 'street-buildings', 'street-roads']
  maplibregl.addProtocol(scheme, async ({url}, abort) => {
    const [z, x, y] = url.slice(scheme.length + 3).split('/').map(Number)
    const tileSignal = AbortSignal.any([signal, abort.signal])
    const tile = await archive.getZxy(z, x, y, tileSignal)
    tileSignal.throwIfAborted()
    return {data: tile?.data ?? new ArrayBuffer(0)}
  })
  function close() {
    maplibregl.removeProtocol(scheme)
    if (!map.getStyle()) return
    for (const layer of layers) if (map.getLayer(layer)) map.removeLayer(layer)
    if (map.getSource(source)) map.removeSource(source)
  }
  try {
    map.addSource(source, {type: 'vector', tiles: [`${scheme}://{z}/{x}/{y}`], bounds, minzoom: 12, maxzoom: 16,
      attribution: streetAttribution, promoteId: 'source_id'})
    map.addLayer({id: layers[0], type: 'fill', source, 'source-layer': 'water', maxzoom: 17, paint: {'fill-color': '#386b85', 'fill-opacity': .72}}, before)
    map.addLayer({id: layers[1], type: 'fill', source, 'source-layer': 'buildings', minzoom: 14, maxzoom: 17, paint: {'fill-color': '#74838b', 'fill-opacity': .38}}, before)
    map.addLayer({id: layers[2], type: 'line', source, 'source-layer': 'roads', maxzoom: 17, paint: {
      'line-color': ['match', ['get', 'highway'], ['motorway', 'trunk', 'primary'], '#c1aa80', '#899ca5'],
      'line-width': ['interpolate', ['linear'], ['zoom'], 12, .7, 16, 2.4],
    }}, before)
  } catch (error) {
    close()
    throw error
  }
  return {bounds, close}
}
