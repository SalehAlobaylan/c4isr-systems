import type {Feature,Geometry} from 'geojson'
import {layerInfo,type AtlasLayer,type Camera} from './AtlasCanvas'

export function readStudyArea(raw?:string):Feature|undefined{
 if(!raw||raw.length>8192)return
 try{const g=JSON.parse(raw) as {type?:string;coordinates?:unknown;crs?:unknown};if(g.type!=='Polygon'||g.crs||!Array.isArray(g.coordinates)||g.coordinates.length!==1)return
 const ring=g.coordinates[0] as unknown;if(!Array.isArray(ring)||ring.length<4||ring.length>65||!ring.every(p=>Array.isArray(p)&&p.length===2&&p.every(Number.isFinite)&&Math.abs(p[0])<=180&&Math.abs(p[1])<=85))return
 if(ring[0][0]!==ring.at(-1)[0]||ring[0][1]!==ring.at(-1)[1])return
 return {type:'Feature',geometry:g as Geometry,properties:{name:'Drawn study area',meaning:'Analyst study polygon, not a jurisdiction'}}
 }catch{return}
}
export function readSharedView(raw?:string):{camera:Camera;layers:AtlasLayer[]}|undefined{
 try{if(!raw||raw.length>4096)return;const v=JSON.parse(raw);if(!v||!Array.isArray(v.layers)||v.layers.length>9||!v.layers.every((k:string)=>k in layerInfo)||!v.camera||!['lng','lat','zoom','bearing','pitch'].every(k=>Number.isFinite(v.camera[k]))||Math.abs(v.camera.lng)>180||Math.abs(v.camera.lat)>85||v.camera.zoom<1||v.camera.zoom>19||Math.abs(v.camera.bearing)>360||v.camera.pitch<0||v.camera.pitch>85)return;return {camera:v.camera,layers:v.layers}}catch{return}
}
