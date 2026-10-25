import type {FeatureCollection} from 'geojson'
import type {GeoJSONSource} from 'maplibre-gl'

// A view-only transition between two already permitted snapshots. This never
// extrapolates a contact, writes query data or advances the simulation clock.
export function interpolateSnapshots(from: FeatureCollection, to: FeatureCollection, fraction: number): FeatureCollection {
  const t=Math.max(0,Math.min(1,Number.isFinite(fraction) ? fraction : 1))
  const previous=new Map(from.features.map(f=>[f.properties?.id,f]))
  return {...to,features:to.features.map(f=>{
    const old=previous.get(f.properties?.id)
    if(f.geometry.type!=='Point'||old?.geometry.type!=='Point') return f
    const [x,y]=old.geometry.coordinates, [nx,ny]=f.geometry.coordinates
    const dx=((nx-x+540)%360)-180
    return {...f,geometry:{...f.geometry,coordinates:[((x+dx*t+540)%360)-180,y+(ny-y)*t]}}
  })}
}

export function paintSnapshot(source: Pick<GeoJSONSource,'setData'>|undefined, target: FeatureCollection, retained: Record<string,FeatureCollection>, key: string, smooth: boolean) {
  if(!source) return
  const from=retained[key]
  const paint=(value: FeatureCollection)=>{retained[key]=value;source.setData(value)}
  if(!smooth||!from||target.features.length>64||matchMedia('(prefers-reduced-motion: reduce)').matches){paint(target);return}
  const start=performance.now();let handle=0,last=-Infinity
  const step=(now: number)=>{
    const t=Math.min(1,(now-start)/250)
    if(t===1||now-last>=50){paint(t===1?target:interpolateSnapshots(from,target,t));last=now}
    if(t<1) handle=requestAnimationFrame(step)
  }
  step(start)
  return ()=>cancelAnimationFrame(handle)
}
