import maplibregl, {type GeoJSONSource, type Map as LibreMap} from 'maplibre-gl'
import {useEffect,useImperativeHandle,useRef,useState,type Ref} from 'react'
import type {Feature,FeatureCollection} from 'geojson'
import {LOCAL_BASEMAP_ONLY,useAPI} from '@/lib/api'
import {openStreetArchive} from './streetMap'
import {resolveTokenColor} from '@/lib/tokens'
import 'maplibre-gl/dist/maplibre-gl.css'

export type Camera={lng:number;lat:number;zoom:number;bearing:number;pitch:number}
export type AtlasHandle={fitBounds:(bounds:[number,number,number,number])=>void;focus:(lng:number,lat:number)=>void;previous:()=>void;north:()=>void;restore:(camera:Camera)=>void}
export {layerInfo, type AtlasLayer} from './LayerCatalog'
import {layerInfo, type AtlasLayer} from './LayerCatalog'
const empty:FeatureCollection={type:'FeatureCollection',features:[]}
const color=(token:string)=>resolveTokenColor(token)??'transparent'
export type RasterOverlay={url:string;bounds:[number,number,number,number];opacity:number;label:string;side?:'before'|'after';role?:'quality'}
let rtlReady:Promise<void>|undefined
function loadLocalRTL(){return rtlReady??=maplibregl.setRTLTextPlugin(new URL(`${import.meta.env.BASE_URL}vendor/rtl-text-0.3.0/mapbox-gl-rtl-text.js`,location.origin).href,false)}
export function AtlasCanvas({proposalOverlay=empty,cameraRef,initialCamera,onCamera,onPointer,onBounds,regions,scope,selection,boundaryLevel,features,datasets,referenceOpacity,basemap,streetArchive,onSelect,onRegion,mode,onDraw,sketch,rasters,swipe=.5,onError}:{proposalOverlay?:FeatureCollection;cameraRef:Ref<AtlasHandle>;initialCamera:Camera;onCamera:(c:Camera)=>void;onPointer:(p:[number,number])=>void;onBounds:(b:[number,number,number,number])=>void;regions:FeatureCollection;scope?:Feature;selection?:Feature;boundaryLevel:'auto'|'country'|'region';features:Partial<Record<AtlasLayer,FeatureCollection>>;datasets:string[];referenceOpacity:number;basemap:'streets'|'quiet'|'reference';streetArchive?:ArrayBuffer;onSelect:(f:Feature[])=>void;onRegion:(f:Feature)=>void;mode:'inspect'|'measure'|'area';onDraw:(p:[number,number])=>void;sketch:FeatureCollection;rasters:RasterOverlay[];swipe?:number;onError:(message:string)=>void}){
 const api=useAPI(),root=useRef<HTMLDivElement>(null),map=useRef<LibreMap|null>(null),[ready,setReady]=useState(false),callbacks=useRef({onCamera,onPointer,onBounds,onSelect,onRegion,boundaryLevel,mode,onDraw,onError})
 callbacks.current={onCamera,onPointer,onBounds,onSelect,onRegion,boundaryLevel,mode,onDraw,onError}
 const canonicalRegions=useRef(regions),canonicalFeatures=useRef(features)
 canonicalRegions.current=regions;canonicalFeatures.current=features
 const pendingView=useRef<{bounds:[number,number,number,number]}|{camera:Camera}|undefined>(undefined)
 const previous=useRef<Camera[]>([]),last=useRef(initialCamera),hover=useRef<string|number|undefined>(undefined),labels=useRef<maplibregl.Marker[]>([])
 const fit=(bounds:[number,number,number,number])=>{pendingView.current={bounds};if(!map.current)return;previous.current.push(last.current);map.current.fitBounds(bounds,{padding:70,maxZoom:15,duration:550})}
 useImperativeHandle(cameraRef,()=>({fitBounds:fit,focus:(lng,lat)=>fit([lng-.02,lat-.02,lng+.02,lat+.02]),previous:()=>{const c=previous.current.pop();if(c)map.current?.easeTo({center:[c.lng,c.lat],zoom:c.zoom,bearing:c.bearing,pitch:c.pitch,duration:450})},north:()=>map.current?.easeTo({bearing:0,pitch:0}),restore:c=>{pendingView.current={camera:c};map.current?.easeTo({center:[c.lng,c.lat],zoom:c.zoom,bearing:c.bearing,pitch:c.pitch})}}),[])
 useEffect(()=>{
   if(!root.current)return
   const protocol='emadref'+crypto.randomUUID().replaceAll('-',''),requestController=new AbortController()
   maplibregl.addProtocol(protocol,async(request,controller)=>{const [z,x,y,ids]=request.url.slice(protocol.length+3).split('/');if(requestController.signal.aborted||controller.signal.aborted)throw new DOMException('Aborted','AbortError');const bytes=await api.getReferenceTile(Number(z),Number(x),Number(y),decodeURIComponent(ids).split(','),controller.signal);if(controller.signal.aborted)throw new DOMException('Aborted','AbortError');return {data:bytes}})
   let m:LibreMap
   try{m=new maplibregl.Map({container:root.current,center:[initialCamera.lng,initialCamera.lat],zoom:initialCamera.zoom,bearing:initialCamera.bearing,pitch:initialCamera.pitch,maxZoom:19,minZoom:1,renderWorldCopies:false,attributionControl:{compact:true},style:{version:8,sources:LOCAL_BASEMAP_ONLY?{}:{osm:{type:'raster',tiles:['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],tileSize:256,maxzoom:19,attribution:'© OpenStreetMap contributors'}},layers:[{id:'background',type:'background',paint:{'background-color':color('--emad-surface-canvas')}},...(LOCAL_BASEMAP_ONLY?[]:[{id:'osm',type:'raster' as const,source:'osm',layout:{visibility:'none' as const},paint:{'raster-saturation':-.55,'raster-opacity':.82}}])]}})}catch{requestController.abort();maplibregl.removeProtocol(protocol);root.current?.replaceChildren();callbacks.current.onError('The map renderer is unavailable on this device. Use retained lists and workbenches; your exact selected area is preserved.');return}
   map.current=m
   void loadLocalRTL().then(()=>{if(!requestController.signal.aborted&&root.current)root.current.dataset.rtlStatus=maplibregl.getRTLTextPluginStatus()}).catch(()=>{if(!requestController.signal.aborted)callbacks.current.onError('Local Arabic shaping could not load. Retained names remain available in the accessible list.')})
   m.addControl(new maplibregl.NavigationControl({visualizePitch:true}),'top-right');m.addControl(new maplibregl.FullscreenControl({container:root.current.closest<HTMLElement>('.atlas-workspace')??root.current}),'top-right');m.addControl(new maplibregl.ScaleControl({maxWidth:100,unit:'metric'}),'bottom-left')
   m.once('style.load',()=>{
     if(requestController.signal.aborted)return
     const addGeo=(id:string,data:FeatureCollection=empty)=>m.addSource(id,{type:'geojson',data,promoteId:id==='regions'?'id':undefined})
     addGeo('capture-proposal');addGeo('regions');addGeo('scope');addGeo('sketch');addGeo('selection')
     m.addSource('references',{type:'vector',tiles:[`${protocol}://{z}/{x}/{y}/none`],minzoom:0,maxzoom:16})
     m.addLayer({id:'region-land',source:'regions',type:'fill',filter:['==',['get','kind'],'country'],layout:{visibility:'none'},paint:{'fill-color':color('--emad-map-ground')}})
     m.addLayer({id:'region-fill',source:'regions',type:'fill',paint:{'fill-color':color('--emad-signal-teal'),'fill-opacity':['case',['boolean',['feature-state','hover'],false],.16,.015]}})
     m.addLayer({id:'region-line',source:'regions',type:'line',paint:{'line-color':color('--emad-text-secondary'),'line-width':['case',['==',['get','kind'],'country'],1,.65],'line-opacity':.45}})
     m.addLayer({id:'scope-fill',source:'scope',type:'fill',paint:{'fill-color':color('--emad-signal-teal'),'fill-opacity':.06}})
     m.addLayer({id:'scope-line',source:'scope',type:'line',paint:{'line-color':color('--emad-signal-teal'),'line-width':2.2,'line-dasharray':[4,2]}})
     m.addLayer({id:'ref-fill',source:'references','source-layer':'reference',type:'fill',minzoom:12,filter:['all',['==',['geometry-type'],'Polygon'],['==',['get','layer'],'buildings']],paint:{'fill-color':color('--emad-text-secondary'),'fill-opacity':.14}})
     m.addLayer({id:'ref-line',source:'references','source-layer':'reference',type:'line',filter:['all',['!=',['get','layer'],'countries'],['!=',['get','layer'],'buildings']],paint:{'line-color':['match',['get','layer'],'coastline',color('--emad-map-history'),'major-roads',color('--emad-text-secondary'),'roads',color('--emad-map-history'),color('--emad-text-secondary')],'line-width':['interpolate',['linear'],['zoom'],4,.35,10,.8,15,2.4],'line-opacity':.7}})
     m.addLayer({id:'ref-point',source:'references','source-layer':'reference',type:'circle',filter:['==',['geometry-type'],'Point'],paint:{'circle-radius':3,'circle-color':color('--emad-map-history')}})
     for(const [kind,info]of Object.entries(layerInfo)){
       addGeo(kind+'-areas')
       m.addSource(kind+'-points',{type:'geojson',data:empty,cluster:true,clusterRadius:40,clusterMaxZoom:12,clusterProperties:{records:['+',['coalesce',['get','count'],1]]}})
       m.addLayer({id:kind+'-fill',source:kind+'-areas',type:'fill',paint:{'fill-color':color(info.token),'fill-opacity':kind==='coverage'?.03:.16}})
       m.addLayer({id:kind+'-outline',source:kind+'-areas',type:'line',paint:{'line-color':color(info.token),'line-width':kind==='imagery'?2:1.4,'line-dasharray':kind==='coverage'?[3,3]:[1,0]}})
       m.addLayer({id:kind+'-cluster',source:kind+'-points',type:'circle',filter:['has','point_count'],paint:{'circle-radius':['step',['get','records'],14,20,19,100,24],'circle-color':color(info.token),'circle-stroke-width':2,'circle-stroke-color':color('--emad-surface-panel')}})
       const canvas=document.createElement('canvas');canvas.width=canvas.height=48;const ctx=canvas.getContext('2d')!;ctx.font='bold 30px Segoe UI Symbol, sans-serif';ctx.textAlign='center';ctx.textBaseline='middle';ctx.lineWidth=4;ctx.strokeStyle=color('--emad-surface-panel');ctx.strokeText(info.glyph,24,24);ctx.fillStyle=color(info.token);ctx.fillText(info.glyph,24,24);m.addImage(kind+'-icon',ctx.getImageData(0,0,48,48),{pixelRatio:2})
       m.addLayer({id:kind+'-symbol',source:kind+'-points',type:'symbol',filter:['!', ['has','point_count']],layout:{'icon-image':kind+'-icon','icon-allow-overlap':true,'icon-size':1}})
     }
     m.addLayer({id:'selection-ring',source:'selection',type:'circle',filter:['==',['geometry-type'],'Point'],paint:{'circle-color':'transparent','circle-radius':14,'circle-stroke-width':2,'circle-stroke-color':color('--emad-map-selection')}})
     m.addLayer({id:'selection-outline',source:'selection',type:'line',filter:['!=',['geometry-type'],'Point'],paint:{'line-color':color('--emad-map-selection'),'line-width':3,'line-dasharray':[2,1]}})
     m.addLayer({id:'sketch-line',source:'sketch',type:'line',paint:{'line-color':color('--emad-map-measure'),'line-width':2,'line-dasharray':[2,2]}})
     m.addLayer({id:'sketch-point',source:'sketch',type:'circle',filter:['==',['geometry-type'],'Point'],paint:{'circle-color':color('--emad-map-measure'),'circle-radius':4}})
     m.addLayer({id:'capture-proposal-outline',source:'capture-proposal',type:'line',paint:{'line-color':['match',['get','proposalKind'],'scene','#ba8fff','#ffbf69'],'line-width':3,'line-dasharray':[3,2]}})
     // Strict effect recreation must apply the latest requested geographic
     // view to the new renderer, rather than restore its initial fallback.
     const requested=pendingView.current
     if(requested&&'bounds'in requested)m.fitBounds(requested.bounds,{padding:70,maxZoom:15,duration:0})
     else if(requested){const c=requested.camera;m.jumpTo({center:[c.lng,c.lat],zoom:c.zoom,bearing:c.bearing,pitch:c.pitch})}
     if(root.current)root.current.dataset.mapCenter=JSON.stringify([m.getCenter().lng,m.getCenter().lat])
     setReady(true)
   })
   m.on('moveend',()=>{const c=m.getCenter();if(root.current)root.current.dataset.mapCenter=JSON.stringify([c.lng,c.lat]);last.current={lng:c.lng,lat:c.lat,zoom:m.getZoom(),bearing:m.getBearing(),pitch:m.getPitch()};callbacks.current.onCamera(last.current);const b=m.getBounds();callbacks.current.onBounds([Math.max(-180,b.getWest()),Math.max(-85,b.getSouth()),Math.min(180,b.getEast()),Math.min(85,b.getNorth())])})
   m.on('mousemove',e=>{callbacks.current.onPointer([e.lngLat.lng,e.lngLat.lat]);if(!m.getLayer('region-fill'))return;const fs=m.queryRenderedFeatures(e.point,{layers:['region-fill']});if(hover.current!==undefined)m.setFeatureState({source:'regions',id:hover.current},{hover:false});hover.current=fs[0]?.id;if(hover.current!==undefined)m.setFeatureState({source:'regions',id:hover.current},{hover:true});m.getCanvas().style.cursor=callbacks.current.mode!=='inspect'?'crosshair':fs.length?'pointer':'';m.getCanvas().title=fs[0]?.properties.name??''})
   m.on('click',async e=>{
     if(callbacks.current.mode!=='inspect'){callbacks.current.onDraw([e.lngLat.lng,e.lngLat.lat]);return}
     if(callbacks.current.boundaryLevel!=='auto'){const hit=m.queryRenderedFeatures(e.point,{layers:['region-fill']}).find(f=>callbacks.current.boundaryLevel==='country'?f.properties.kind==='country':f.properties.kind==='region'||f.properties.kind==='governorate'||f.properties.kind==='study');const original=hit&&canonicalRegions.current.features.find(f=>String(f.id)===String(hit.id));if(original)callbacks.current.onRegion(original);return}
     const layers=Object.keys(layerInfo).flatMap(k=>[k+'-symbol',k+'-cluster',k+'-fill',k+'-outline']).filter(id=>m.getLayer(id))
     const hits=m.queryRenderedFeatures([[e.point.x-10,e.point.y-10],[e.point.x+10,e.point.y+10]],{layers}),cluster=hits.find(f=>f.properties.cluster)
     if(cluster){const src=m.getSource(cluster.source) as GeoJSONSource;const zoom=await src.getClusterExpansionZoom(cluster.properties.cluster_id);if(!requestController.signal.aborted&&cluster.geometry.type==='Point')m.easeTo({center:cluster.geometry.coordinates as [number,number],zoom});return}
     if(hits.length){const unique=[...new Map(hits.map(f=>{const original=Object.values(canonicalFeatures.current).flatMap(c=>c?.features??[]).find(v=>v.properties?.recordId===f.properties.recordId&&v.geometry.type===f.geometry.type);return [String(f.properties.recordId),original??f as Feature]})).values()];callbacks.current.onSelect(unique);return}
     const streetLayers=['street-roads','street-buildings','street-water'].filter(id=>m.getLayer(id));const streets=streetLayers.length?m.queryRenderedFeatures(e.point,{layers:streetLayers}):[];if(streets.length){callbacks.current.onSelect(streets.slice(0,8).map(f=>({type:'Feature',id:f.id,geometry:f.geometry,properties:{...f.properties,kind:'retained street',title:f.properties.name||f.properties.source_id,source:'OpenStreetMap contributors',meaning:'Rendered tile geometry from the retained OpenStreetMap edition.'}})) as Feature[]);return}
     const refs=m.queryRenderedFeatures(e.point,{layers:['ref-fill','ref-line','ref-point']});if(refs.length){callbacks.current.onSelect(refs.slice(0,8).map(f=>({type:'Feature',id:f.id,geometry:f.geometry,properties:{...f.properties,kind:'reference',meaning:'Visible vector-tile geometry. Inspect the retained dataset for the full feature.'}})) as Feature[]);return}
     const boundaries=m.queryRenderedFeatures(e.point,{layers:['region-fill']});if(boundaries[0]){const original=canonicalRegions.current.features.find(f=>String(f.id)===String(boundaries[0].id));if(original)callbacks.current.onRegion(original)}
   })
   m.on('error',e=>{if(!requestController.signal.aborted&&e.error?.message&&!e.error.message.includes('Abort'))callbacks.current.onError(e.error.message.includes('tile.openstreetmap')?'Street basemap unavailable; retained evidence and reference layers remain available.':'A map source could not be loaded. Check Layers and source access.')})
   const ro=new ResizeObserver(()=>{if(!requestController.signal.aborted)m.resize()});ro.observe(root.current)
   root.current.dataset.protocol=protocol
   return()=>{requestController.abort();ro.disconnect();labels.current.forEach(l=>l.remove());m.remove();maplibregl.removeProtocol(protocol);map.current=null}
 },[])
 useEffect(()=>{const m=map.current;if(!ready||!m)return;(m.getSource('regions') as GeoJSONSource).setData({type:'FeatureCollection',features:regions.features.map(f=>({...f,properties:{...f.properties,id:f.id}}))});(m.getSource('scope') as GeoJSONSource).setData({type:'FeatureCollection',features:scope?[scope]:[]})},[ready,regions,scope])
 useEffect(()=>{const m=map.current;if(!ready||!m)return;for(const kind of Object.keys(layerInfo) as AtlasLayer[]){const collection=features[kind]??empty;(m.getSource(kind+'-areas') as GeoJSONSource).setData({type:'FeatureCollection',features:collection.features.filter(f=>f.geometry.type!=='Point')});(m.getSource(kind+'-points') as GeoJSONSource).setData({type:'FeatureCollection',features:collection.features.filter(f=>f.geometry.type==='Point')})}},[ready,features])
 useEffect(()=>{const m=map.current;if(!ready||!m)return;const protocol=root.current?.dataset.protocol;(m.getSource('references') as maplibregl.VectorTileSource).setTiles([`${protocol}://{z}/{x}/{y}/${encodeURIComponent(datasets.join(',')||'none')}`]);for(const id of ['ref-fill','ref-line','ref-point'])m.setLayoutProperty(id,'visibility',datasets.length?'visible':'none')},[ready,datasets.join(',')])
 useEffect(()=>{const m=map.current;if(!ready||!m)return;m.setPaintProperty('ref-line','line-opacity',referenceOpacity);m.setPaintProperty('ref-fill','fill-opacity',referenceOpacity*.2);m.setPaintProperty('ref-point','circle-opacity',referenceOpacity);if(m.getLayer('osm')){m.setLayoutProperty('osm','visibility',basemap==='reference'||streetArchive?'none':'visible');m.setPaintProperty('osm','raster-opacity',basemap==='quiet'?.4:.82)}},[ready,referenceOpacity,basemap,streetArchive])
 useEffect(()=>{
   const m=map.current
   if(!ready||!m)return
   const reference=basemap==='reference'
   m.setPaintProperty('background','background-color',color(reference?'--emad-map-water':'--emad-surface-canvas'))
   m.setLayoutProperty('region-land','visibility',reference?'visible':'none')
 },[ready,basemap])
 useEffect(()=>{
   const m=map.current
   if(!ready||!m||!streetArchive)return
   const controller=new AbortController()
   let close:(()=>void)|undefined
   void openStreetArchive(m,streetArchive,controller.signal,'region-fill').then(streets=>{close=streets.close;if(controller.signal.aborted)close();else if(root.current)root.current.dataset.streetMap='loaded'})
     .catch(()=>{if(!controller.signal.aborted)callbacks.current.onError('The retained street map could not open. Reference geography remains available.')})
   return()=>{controller.abort();close?.();if(root.current)delete root.current.dataset.streetMap}
 },[ready,streetArchive])
 useEffect(()=>{if(ready)(map.current?.getSource('selection') as GeoJSONSource)?.setData({type:'FeatureCollection',features:selection?[selection]:[]})},[ready,selection])
 useEffect(()=>{if(ready)(map.current?.getSource('capture-proposal') as GeoJSONSource)?.setData(proposalOverlay)},[ready,proposalOverlay])
 useEffect(()=>{if(ready)(map.current?.getSource('sketch') as GeoJSONSource)?.setData(sketch)},[ready,sketch])
 // Names use the bundled Arabic font in DOM labels. Browsers shape them locally;
 // MapLibre's separately pinned worker plugin also loads without external assets.
 useEffect(()=>{const m=map.current;if(!m||!ready||!scope)return;const el=document.createElement('bdi');el.dir='auto';el.className='atlas-area-name';el.textContent=String(scope.properties?.name??'Selected area');el.title='Selected area name · display label';const marker=new maplibregl.Marker({element:el,anchor:'top-left'}).setLngLat((scope.geometry.type==='Point'?scope.geometry.coordinates:scope.geometry.type==='Polygon'?scope.geometry.coordinates[0][0]:scope.geometry.type==='MultiPolygon'?scope.geometry.coordinates[0][0][0]:[m.getCenter().lng,m.getCenter().lat]) as [number,number]).addTo(m);return()=>{marker.remove()}},[ready,scope])
 const rasterIds=useRef<string[]>([]),latestRasters=useRef(rasters)
 latestRasters.current=rasters
 useEffect(()=>{
   const m=map.current;if(!ready||!m)return
   let cancelled=false
   const clear=()=>{for(const id of rasterIds.current){if(m.getLayer(id))m.removeLayer(id);if(m.getSource(id))m.removeSource(id)}rasterIds.current=[];if(root.current)root.current.dataset.retainedRasterCount='0'}
   if(!rasters.length){clear();return}
   const images=rasters.map(r=>new Promise<HTMLImageElement>((resolve,reject)=>{const img=new Image();img.onload=()=>resolve(img);img.onerror=()=>reject(new Error('Retained raster decode failed. Inspect source artifact integrity.'));img.src=r.url}))
   void Promise.all(images).then(loaded=>{
     if(cancelled||map.current!==m)return
     const next:string[]=[]
     loaded.forEach((img,i)=>{
       const r=rasters[i],canvas=document.createElement('canvas');canvas.width=img.naturalWidth;canvas.height=img.naturalHeight
       const c=canvas.getContext('2d')!;c.drawImage(img,0,0)
       if(r.side==='after')c.clearRect(0,0,canvas.width*swipe,canvas.height)
       if(r.side==='before')c.clearRect(canvas.width*swipe,0,canvas.width*(1-swipe),canvas.height)
       const [w,s,e,n]=r.bounds,id='retained-raster-'+crypto.randomUUID()
       m.addSource(id,{type:'canvas',canvas,animate:false,coordinates:[[w,n],[e,n],[e,s],[w,s]]})
       m.addLayer({id,type:'raster',source:id,paint:{'raster-opacity':latestRasters.current[i]?.opacity??r.opacity,'raster-fade-duration':0}},'region-fill');next.push(id)
     })
     clear();rasterIds.current=next;m.triggerRepaint()
     if(root.current)root.current.dataset.retainedRasterCount=String(next.length)
   }).catch(e=>{if(!cancelled)callbacks.current.onError(e.message)})
   return()=>{cancelled=true}
 },[ready,rasters.map(r=>r.url+JSON.stringify(r.bounds)).join('|'),swipe])
 useEffect(()=>{const m=map.current;if(ready&&m)rasterIds.current.forEach((id,i)=>{if(m.getLayer(id))m.setPaintProperty(id,'raster-opacity',rasters[i]?.opacity??0)})},[ready,rasters.map(r=>r.opacity).join()])
 // Cluster labels use DOM text, avoiding a remote font dependency. Limit labels
 // by the actual viewport; all data remains in the WebGL source.
 useEffect(()=>{const m=map.current;if(!m||!ready)return;let frame=0,timer:ReturnType<typeof setTimeout>|undefined,signature="";const refresh=()=>{frame=requestAnimationFrame(()=>{const next=JSON.stringify(Object.keys(layerInfo).map(k=>m.queryRenderedFeatures(undefined,{layers:[k+'-cluster',k+'-symbol']}).filter(f=>f.properties.cluster||f.properties.aggregate).map(f=>[f.properties.cluster_id,f.properties.recordId,f.properties.records??f.properties.count,f.geometry])));if(signature===next)return;signature=next;labels.current.forEach(l=>l.remove());labels.current=[];const seen=new Set<string>();for(const kind of Object.keys(layerInfo)){const hits=m.queryRenderedFeatures(undefined,{layers:[kind+'-cluster',kind+'-symbol']}).filter(f=>f.properties.cluster||f.properties.aggregate);for(const f of hits){const id=kind+(f.properties.cluster_id??f.properties.recordId);if(seen.has(id)||labels.current.length>=100||f.geometry.type!=='Point')continue;seen.add(id);const el=document.createElement('span');el.className='atlas-cluster-count';el.textContent=String(f.properties.records??f.properties.count);el.setAttribute('aria-hidden','true');labels.current.push(new maplibregl.Marker({element:el}).setLngLat(f.geometry.coordinates as [number,number]).addTo(m))}}})};const update=()=>{if(timer!==undefined)return;timer=setTimeout(()=>{timer=undefined;refresh()},100)};m.on('render',update);m.on('moveend',update);update();return()=>{if(timer!==undefined)clearTimeout(timer);cancelAnimationFrame(frame);m.off('render',update);m.off('moveend',update);labels.current.forEach(l=>l.remove());labels.current=[]}},[ready])
 return <div ref={root} className="atlas-canvas" data-proposal-footprint-count={proposalOverlay.features.length} data-map-ready={ready} data-public-feature-count={Object.values(features).reduce((n,v)=>n+(v?.features.length??0),0)} data-navigation-feature-count={regions.features.length} data-map-layers={Object.keys(features).map(k=>k+'-symbol').join(' ')} aria-label="Interactive Public Context map"/>
}
