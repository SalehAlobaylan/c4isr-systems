import {useEffect,useRef,useState} from 'react'
import maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import type {FeatureCollection,Geometry,Polygon} from 'geojson'
import type {ImageryResult} from '@/lib/api'

export function ChangeMap({result}:{result:ImageryResult}){
 const host=useRef<HTMLDivElement>(null),[error,setError]=useState(''),[ready,setReady]=useState(false)
 useEffect(()=>{const node=host.current;if(!node)return;let map:maplibregl.Map|undefined,disposed=false;setError('');setReady(false);delete node.dataset.mapReady;try{
  const g=result.grid,count=g.width*g.height;if(count>65536||result.delta.length!==count)throw Error('Invalid mapped grid')
  // Exact angular cells stay inspectable at this bounded grid size and avoid
  // browser-specific image texture decoding. No interpolation or impact area
  // is inferred from the analysis cells.
  const cells:FeatureCollection<Polygon>={type:'FeatureCollection',features:result.delta.map((value,i)=>{
   const magnitude=value===null?0:Math.min(1,Math.abs(value)/2),rgb=value===null?[130,130,130]:value<0?[245-magnitude*190,242-magnitude*140,235-magnitude*35]:[245-magnitude*15,242-magnitude*135,235-magnitude*200]
   const west=g.west+(i%g.width)*g.stepLng,north=g.north-Math.floor(i/g.width)*g.stepLat,east=west+g.stepLng,south=north-g.stepLat
   return {type:'Feature',properties:{color:`rgb(${rgb.map(Math.round).join(',')})`,delta:value},geometry:{type:'Polygon',coordinates:[[[west,north],[east,north],[east,south],[west,south],[west,north]]]}}
  })}
  const east=g.west+g.width*g.stepLng,south=g.north-g.height*g.stepLat
  map=new maplibregl.Map({container:node,style:{version:8,sources:{},layers:[{id:'background',type:'background',paint:{'background-color':'#cdd9dc'}}]},center:[(g.west+east)/2,(g.north+south)/2],zoom:12,attributionControl:{compact:true}})
  map.addControl(new maplibregl.NavigationControl(),'top-right');map.addControl(new maplibregl.ScaleControl({unit:'metric'}),'bottom-left')
  map.on('load',()=>{if(!map||disposed)return;map.addSource('change',{type:'geojson',data:cells});map.addLayer({id:'change',type:'fill',source:'change',paint:{'fill-color':['get','color'],'fill-antialias':false}});if(result.findingGeometry){map.addSource('finding',{type:'geojson',data:{type:'Feature',properties:{},geometry:result.findingGeometry as unknown as Geometry}});map.addLayer({id:'finding',type:'line',source:'finding',paint:{'line-color':'#96225c','line-width':2,'line-dasharray':[3,2]}})}map.fitBounds([[g.west,south],[east,g.north]],{padding:35,duration:0});map.once('idle',()=>{if(!disposed){node.dataset.mapReady='true';setReady(true)}})})
  map.on('error',()=>{if(!disposed)setError('Mapped comparison could not render. The aligned pixel panels and geographic coordinates remain available.')})
 }catch{setError('Mapped comparison unavailable; use the aligned pixel panels and coordinates below.')}return()=>{disposed=true;map?.remove()}},[result])
 return <figure className="space-y-2"><figcaption className="font-semibold">Georeferenced change · WGS84 · north up</figcaption>{error&&<p role="alert">{error}</p>}{!ready&&!error&&<p role="status" className="ux-muted">Rendering retained analysis cells…</p>}<div ref={host} className="h-80 w-full overflow-hidden rounded border border-edge sm:h-96" aria-label="Mapped NDVI change footprint"/><p className="text-sm">Blue: NDVI decrease · pale: unchanged · orange: increase · grey: masked. Purple dashed outlines are thresholded cells. The map uses retained analysis coordinates and reports no verified impact boundary.</p></figure>
}
