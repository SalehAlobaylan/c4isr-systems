import {normalizeNumbers} from '@/lib/internationalInput'
import type { Geometry } from 'geojson'

export type Rectangle = { west: number; south: number; east: number; north: number }
export type OperationArea = { key: string; name: string; rectangle?: Rectangle; geometry?:Geometry; reason?: string }

export function parseRectangle(value?: string): Rectangle | undefined {
  const parts = value===undefined?undefined:normalizeNumbers(value).split(',')
  if (!parts || parts.length !== 4 || parts.some(p => !/^[-+]?(?:\d+\.?\d*|\.\d+)$/.test(p.trim()))) return
  const [west, south, east, north] = parts.map(Number)
  if (![west, south, east, north].every(Number.isFinite) || west < -180 || east > 180 || south < -90 || north > 90 || west >= east || south >= north) return
  return { west, south, east, north }
}
export const rectangleText = (b: Rectangle) => [b.west, b.south, b.east, b.north].join(',')
export const containsRectangle = (parent: Rectangle, child: Rectangle) => child.west >= parent.west && child.east <= parent.east && child.south >= parent.south && child.north <= parent.north
export const localRectangle = (b?: Rectangle) => !!b && b.east - b.west <= .10000000001 && b.north - b.south <= .10000000001

/** A bounding box is not an analytical replacement for an arbitrary polygon.
 * Until polygon operations arrive, only an exact axis-aligned rectangle can
 * prefill these rectangular legacy operations. */
export function operationArea(input: { regionId?: string; geometry?: string; bbox?: string; name?: string; unavailable?:string }): OperationArea {
  const key = JSON.stringify([input.regionId, input.geometry, input.bbox, input.unavailable])
  const name = input.name ?? (input.bbox ? 'Selected rectangle' : 'No selected area')
  const unavailable = (reason: string): OperationArea => ({ key, name, reason })
  if (input.unavailable) return unavailable(input.unavailable)
  if (input.geometry) {
    try {
      let g = JSON.parse(input.geometry) as Geometry
      if(g.type==='MultiPolygon'&&g.coordinates.length===1)g={type:'Polygon',coordinates:g.coordinates[0]}
      if(g.type!=='Polygon'&&g.type!=='MultiPolygon')return unavailable('Choose an explicit polygon before bounded processing.')
      if (g.type !== 'Polygon' || g.coordinates.length !== 1 || g.coordinates[0].length !== 5) return {...unavailable('Enter a deliberate local rectangle inside this exact area, or draw a local subset on the map. Holes and separate components remain excluded.'),geometry:g}
      const ring = g.coordinates[0]
      if (ring.some(p => p.length < 2 || !p.slice(0, 2).every(Number.isFinite)) || ring[0][0] !== ring[4][0] || ring[0][1] !== ring[4][1]) return unavailable('The selected geometry is invalid. Select an area again.')
      const west = Math.min(...ring.map(p => p[0])), east = Math.max(...ring.map(p => p[0]))
      const south = Math.min(...ring.map(p => p[1])), north = Math.max(...ring.map(p => p[1]))
      const rectangle = parseRectangle([west, south, east, north].join(','))
      const corners = new Set(ring.slice(0, 4).map(p => `${p[0]},${p[1]}`))
      if (!rectangle || east - west > 180 || corners.size !== 4 || ring.some((p, i) => (p[0] !== west && p[0] !== east) || (p[1] !== south && p[1] !== north) || i < 4 && p[0] !== ring[i + 1][0] && p[1] !== ring[i + 1][1])) return {...unavailable('Enter a local rectangle inside the exact selected polygon; no bounding-box substitution will be made.'),geometry:g}
      return { key, name, rectangle,geometry:g }
    } catch { return unavailable('The selected geometry is unavailable. Select an area again.') }
  }
  if (input.regionId) return unavailable('The selected region has not resolved to usable geometry. Return to the map and choose a local rectangle.')
  const rectangle = parseRectangle(input.bbox)
  return rectangle ? { key, name, rectangle } : unavailable('Choose an explicit local area on the map before submitting work.')
}

/** Conservative UI check; the server repeats ST_Covers on exact WGS84 geometry. */
export function containsOperationRectangle(area:OperationArea,b:Rectangle):boolean{
 if(area.rectangle)return containsRectangle(area.rectangle,b)
 const g=area.geometry;if(!g||g.type!=='Polygon'&&g.type!=='MultiPolygon')return false
 const polygons=g.type==='Polygon'?[g.coordinates]:g.coordinates
 const corners=[[b.west,b.south],[b.east,b.south],[b.east,b.north],[b.west,b.north]],eps=1e-10
 const cross=(a:number[],b:number[],c:number[])=>(b[0]-a[0])*(c[1]-a[1])-(b[1]-a[1])*(c[0]-a[0])
 const on=(a:number[],b:number[],p:number[])=>Math.abs(cross(a,b,p))<eps&&p[0]>=Math.min(a[0],b[0])-eps&&p[0]<=Math.max(a[0],b[0])+eps&&p[1]>=Math.min(a[1],b[1])-eps&&p[1]<=Math.max(a[1],b[1])+eps
 const inside=(p:number[],ring:number[][])=>{let value=false;for(let i=0,j=ring.length-1;i<ring.length;j=i++){const a=ring[i],b=ring[j];if(on(a,b,p))return true;if((a[1]>p[1])!==(b[1]>p[1])&&p[0]<(b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0])value=!value}return value}
 return polygons.some(rings=>{
  if(!corners.every(p=>inside(p,rings[0])&&!rings.slice(1).some(hole=>inside(p,hole))))return false
  for(let r=0;r<rings.length;r++){const ring=rings[r];for(let i=1;i<ring.length;i++)for(let k=0;k<4;k++){const a=ring[i-1],c=ring[i],d=corners[k],e=corners[(k+1)%4];if(cross(a,c,d)*cross(a,c,e)<-eps&&cross(d,e,a)*cross(d,e,c)<-eps)return false};if(r>0&&ring.some(p=>p[0]>b.west&&p[0]<b.east&&p[1]>b.south&&p[1]<b.north))return false}
  return true
 })
}
