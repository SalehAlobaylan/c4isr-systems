import type { components } from '@/lib/openapi.generated'
export type Position = components['schemas']['SimulationPosition']
export type Vector = components['schemas']['SceneVector']
export type StudioScene = components['schemas']['StudioScene']
export function localFromPosition(origin: Position, position: Position): Vector {
 const e2=(1/298.257223563)*(2-1/298.257223563)
 const ecef=(p:Position)=>{if(typeof p.height!=='number'||p.verticalDatum!=='WGS84_ELLIPSOID')throw new Error('Resolved ellipsoid heights are required.');const lat=p.latitude*Math.PI/180,lon=p.longitude*Math.PI/180,n=6378137/Math.sqrt(1-e2*Math.sin(lat)**2);return [(n+p.height)*Math.cos(lat)*Math.cos(lon),(n+p.height)*Math.cos(lat)*Math.sin(lon),(n*(1-e2)+p.height)*Math.sin(lat)]}
 const a=ecef(position),o=ecef(origin),[x,y,z]=a.map((v,i)=>v-o[i]),lat=origin.latitude*Math.PI/180,lon=origin.longitude*Math.PI/180
 return {x:-Math.sin(lon)*x+Math.cos(lon)*y,y:-Math.sin(lat)*Math.cos(lon)*x-Math.sin(lat)*Math.sin(lon)*y+Math.cos(lat)*z,z:Math.cos(lat)*Math.cos(lon)*x+Math.cos(lat)*Math.sin(lon)*y+Math.sin(lat)*z}
}
