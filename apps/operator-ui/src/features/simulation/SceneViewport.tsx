import { useEffect, useRef, useState } from 'react'
import * as T from 'three'
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'
import { localFromPosition, type StudioScene } from './spatial'
import type {components} from '@/lib/openapi.generated'

type Props={compact?:boolean;scene:StudioScene;load:(id:string)=>Promise<Blob>;selected:string;onSelect:(id:string)=>void;placements?:components['schemas']['ScenePlacement'][];snapshot?:components['schemas']['RuntimeSnapshot']|null}
function release(root:T.Object3D){root.traverse(object=>{const mesh=object as T.Mesh;mesh.geometry?.dispose();if(mesh.material)for(const material of Array.isArray(mesh.material)?mesh.material:[mesh.material]){for(const value of Object.values(material))if(value instanceof T.Texture){value.dispose();const data=value.image;if(typeof ImageBitmap!=='undefined'&&data instanceof ImageBitmap)data.close()}material.dispose()}})}
export default function SceneViewport({scene,load,selected,onSelect,snapshot,placements,compact}:Props){
 const host=useRef<HTMLDivElement>(null),reset=useRef<()=>void>(()=>{}),focus=useRef<()=>void>(()=>{}),[state,setState]=useState('Loading verified scene geometry…'),[ready,setReady]=useState(false),[label,setLabel]=useState<{id:string;x:number;y:number}|null>(null)
 const select=useRef(onSelect);select.current=onSelect
 const selectedRef=useRef(selected);selectedRef.current=selected
 const highlights=useRef(new Map<string,T.MeshBasicMaterial>()),redraw=useRef<()=>void>(()=>{})
 const draft=useRef(placements);draft.current=placements
 const latest=useRef(snapshot);latest.current=snapshot
 useEffect(()=>{if(host.current)host.current.dataset.snapshotMs=snapshot?String(snapshot.elapsedMs):''},[snapshot])
 const move=useRef<()=>void>(()=>{})
 useEffect(()=>move.current(),[snapshot,placements])
 useEffect(()=>{for(const [id,material]of highlights.current)material.color.setHex(id===selected?0xffc96b:0x7cd8e5);redraw.current()},[selected])
 useEffect(()=>{
  const container=host.current;if(!container)return
  let cancelled=false,renderer:T.WebGLRenderer|undefined,controls:OrbitControls|undefined,observer:ResizeObserver|undefined
  const world=new T.Scene(),resources:T.Object3D[]=[],objects=new Map<string,T.Object3D>(),markers=new Map<string,T.Mesh>();setReady(false);setState('Loading verified scene geometry…')
  const manager=new T.LoadingManager();manager.setURLModifier(url=>{if(!url.startsWith('data:')&&!url.startsWith('blob:'))throw new Error('External model resource blocked.');return url})
  const draw=()=>{if(!cancelled&&renderer&&camera){const began=performance.now();renderer.render(world,camera);container.dataset.renderFrame=String(renderer.info.render.frame);container.dataset.renderMs=String(performance.now()-began);const object=objects.get(selectedRef.current);if(object){const p=object.getWorldPosition(new T.Vector3()).project(camera);setLabel(p.z<1&&p.z>-1&&Math.abs(p.x)<1&&Math.abs(p.y)<1?{id:selectedRef.current,x:(p.x+1)*50,y:(1-p.y)*50}:null)}else setLabel(null)}}
  redraw.current=draw;highlights.current.clear()
  const camera=new T.PerspectiveCamera(45,1,.1,50000)
  const routeGroup=new T.Group();world.add(routeGroup)
  move.current=()=>{const snap=latest.current;const authored=draft.current??scene.frozen.input.actors
   if(snap){for(const actor of snap.actors){const o=objects.get(actor.id),marker=markers.get(actor.id);if(o){o.position.set(actor.local.x,actor.local.z,-actor.local.y);o.rotation.y=Math.PI-actor.heading*Math.PI/180;o.visible=actor.active}if(marker){marker.position.set(actor.local.x,actor.local.z+.1,-actor.local.y);marker.visible=actor.active}}}
   else {for(const [id,o]of objects){const a=authored.find(v=>v.id===id);o.visible=!!a;const marker=markers.get(id);if(marker)marker.visible=!!a;if(a){const p=localFromPosition(scene.frozen.environment.frame.origin,a.position);o.position.set(p.x,p.z,-p.y);o.rotation.y=Math.PI-a.heading*Math.PI/180;marker?.position.set(p.x,p.z+.1,-p.y)}}
    release(routeGroup);routeGroup.clear();for(const a of authored){const p=[a.position,...a.waypoints].map(v=>localFromPosition(scene.frozen.environment.frame.origin,v));if(p.length>1){const geometry=new T.BufferGeometry().setFromPoints(p.map(v=>new T.Vector3(v.x,v.z+1,-v.y)));routeGroup.add(new T.Line(geometry,new T.LineBasicMaterial({color:a.id===selectedRef.current?0xffc96b:0x7cd8e5,transparent:true,opacity:.65})))}}}
   draw()
  }

  const fit=()=>{const box=new T.Box3().setFromObject(world),center=box.getCenter(new T.Vector3()),size=Math.max(100,box.getSize(new T.Vector3()).length());camera.position.copy(center).add(new T.Vector3(size*.7,size*.8,size*.9));camera.near=Math.max(.1,size/10000);camera.far=Math.max(50000,size*10);camera.updateProjectionMatrix();controls?.target.copy(center);controls?.update();draw()};reset.current=fit
  focus.current=()=>{const object=objects.get(selectedRef.current);if(!object)return;const bounds=new T.Box3().setFromObject(object),target=bounds.getCenter(new T.Vector3()),distance=Math.max(12,bounds.getSize(new T.Vector3()).length()*1.5);camera.position.copy(target).add(new T.Vector3(distance*.7,distance*.6,distance*.8));controls?.target.copy(target);controls?.update();draw()}
  const resize=()=>{if(!renderer)return;const width=container.clientWidth,height=container.clientHeight;if(!width||!height)return;renderer.setSize(width,height,false);camera.aspect=width/height;camera.updateProjectionMatrix();draw()}
  const lost=(e:Event)=>{e.preventDefault();if(!cancelled){setReady(false);setState('3D context unavailable. Use the geographic overview and exact actor properties below.')}}
  const pick=(event:PointerEvent)=>{if(!renderer)return;const rect=renderer.domElement.getBoundingClientRect(),ray=new T.Raycaster();ray.setFromCamera(new T.Vector2((event.clientX-rect.left)/rect.width*2-1,-(event.clientY-rect.top)/rect.height*2+1),camera);for(const hit of ray.intersectObjects(world.children,true)){let o:T.Object3D|null=hit.object;while(o&&!o.userData.actorID)o=o.parent;if(o?.userData.actorID){select.current(o.userData.actorID);break}}}
  void(async()=>{try{
   renderer=new T.WebGLRenderer({antialias:true,alpha:false});renderer.setPixelRatio(Math.min(window.devicePixelRatio,1.5));renderer.setClearColor(0x14212a);renderer.domElement.setAttribute('aria-label','Authored 3D scene');renderer.domElement.addEventListener('webglcontextlost',lost);renderer.domElement.addEventListener('pointerup',pick);container.appendChild(renderer.domElement)
   controls=new OrbitControls(camera,renderer.domElement);renderer.domElement.tabIndex=0;controls.listenToKeyEvents(renderer.domElement);controls.enableDamping=false;controls.addEventListener('change',draw)
   renderer.toneMapping=T.ACESFilmicToneMapping;renderer.toneMappingExposure=1;world.add(new T.HemisphereLight(0xddeeff,0x4d5b46,1.2));const sun=new T.DirectionalLight(0xffffff,1.4);sun.position.set(3000,8000,1000);world.add(sun)
   const bases=new Map<string,T.Object3D>()
   for(const model of scene.frozen.models){const blob=await load(model.upload.id);if(cancelled)return;const bytes=await blob.arrayBuffer(),hash=[...new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))].map(v=>v.toString(16).padStart(2,'0')).join('');if(hash!==model.upload.sha256||bytes.byteLength!==model.upload.bytes)throw new Error('Visual model integrity check failed.');const parsed=await new GLTFLoader(manager).parseAsync(model.upload.mediaType==='model/gltf+json'?new TextDecoder().decode(bytes):bytes,'');resources.push(parsed.scene);if(cancelled){release(parsed.scene);return}bases.set(model.id,parsed.scene)}
   const environment=bases.get(scene.frozen.environment.modelVersionId);if(environment)world.add(environment.clone(true))
   const origin=scene.frozen.environment.frame.origin
   for(const actor of scene.frozen.input.actors){const template=scene.frozen.templates.find(t=>t.id===actor.templateId),base=template&&bases.get(template.modelVersionId);if(!base)throw new Error('Pinned actor model is unavailable.');const object=base.clone(true),p=localFromPosition(origin,actor.position);object.position.set(p.x,p.z,-p.y);object.rotation.y=Math.PI-actor.heading*Math.PI/180;object.userData.actorID=actor.id;objects.set(actor.id,object);world.add(object)
    const material=new T.MeshBasicMaterial({color:actor.id===selectedRef.current?0xffc96b:0x7cd8e5,side:T.DoubleSide});highlights.current.set(actor.id,material);const marker=new T.Mesh(new T.RingGeometry(6,8,24),material);marker.position.set(p.x,p.z+.1,-p.y);marker.rotation.x=-Math.PI/2;marker.userData.actorID=actor.id;markers.set(actor.id,marker);world.add(marker)
   }
   const geo=scene.frozen.geometry
   if(geo.triangles.length&&!scene.frozen.environment.prepared){const geometry=new T.BufferGeometry();geometry.setAttribute('position',new T.Float32BufferAttribute(geo.terrain.flatMap(p=>[p.x,p.z,-p.y]),3));geometry.setIndex(geo.triangles);geometry.computeVertexNormals();world.add(new T.Mesh(geometry,new T.MeshStandardMaterial({color:0x566e5c,roughness:1,side:T.DoubleSide})))}
   const grid=new T.GridHelper(2000,20,0x57778a,0x293f4c);grid.position.y=-1;world.add(grid);const axes=new T.AxesHelper(150);world.add(axes)
   for(const point of geo.points){const marker=new T.Mesh(new T.BoxGeometry(12,12,12),new T.MeshBasicMaterial({color:0xc0c9cc}));marker.position.set(point.local.x,point.local.z+6,-point.local.y);world.add(marker)}
   observer=new ResizeObserver(resize);observer.observe(container);resize();fit();move.current();if(!cancelled){setReady(true);setState(`${scene.frozen.input.actors.length} authored actors · ${renderer.info.render.triangles} rendered triangles · metre-scale geometry`)}
  }catch(e){if(!cancelled){setReady(false);setState(e instanceof Error?e.message:'3D unavailable. Use the geographic overview and properties.')}}})()
  return()=>{cancelled=true;observer?.disconnect();controls?.removeEventListener('change',draw);controls?.dispose();if(renderer){renderer.domElement.removeEventListener('webglcontextlost',lost);renderer.domElement.removeEventListener('pointerup',pick);renderer.dispose();renderer.forceContextLoss();renderer.domElement.remove()}release(world);for(const r of resources)release(r);world.clear()}
 },[scene,load])
 return <section aria-label="Georeferenced viewport" className="scene-viewport-panel space-y-2"><div className="scene-toolbar"><h2 className="font-semibold">{snapshot?'Privileged world · 3D':placements?'Draft preview · 3D':'Authored scene · 3D'}</h2><div><button className="ux-button" disabled={!ready||!selected} onClick={()=>focus.current()}>Focus selected actor</button><button className="ux-button" onClick={()=>reset.current()}>Reset scene view</button></div></div><p role="status" className="scene-render-status">{state}</p><div className="relative"><div ref={host} style={compact?{height:'100%',minHeight:0}:undefined} data-render-ready={ready} className="scene-viewport relative h-80 w-full overflow-hidden rounded border border-edge sm:h-[440px]"/>{ready&&label&&<span className="pointer-events-none absolute max-w-[90%] -translate-x-1/2 -translate-y-9 rounded border border-[#ffc96b] bg-[#14212a] px-2 py-1 text-sm text-[#ffc96b]" style={{left:`${label.x}%`,top:`${label.y}%`}}>{label.id}</span>}</div>{!compact&&<div className="flex flex-wrap gap-2">{scene.frozen.input.actors.map(a=><button key={a.id} className="min-h-11 rounded border border-edge px-3" aria-pressed={selected===a.id} onClick={()=>onSelect(a.id)}>Select {a.id}</button>)}</div>}<details><summary className="min-h-9 cursor-pointer text-xs">Scene navigation & coordinate legend</summary><p className="text-sm">East = red X · up = green Y · south = blue Z. Grid: 100 m spacing. Drag to orbit, right-drag to pan, pinch or scroll to zoom. Focus the viewport and use arrow keys to pan. Routes show authored waypoints. Freeze saves validated edits; runtime uses the frozen scene. New templates appear after freezing. Geometry retains its declared metre scale.</p></details></section>
}
