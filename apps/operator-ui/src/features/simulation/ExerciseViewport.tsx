import {useEffect,useRef,useState} from 'react'
import * as T from 'three'
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js'
import {OrbitControls} from 'three/addons/controls/OrbitControls.js'
import type {ExerciseEnvironmentView} from '@/lib/api'
import type {Vector} from './spatial'

export type DisplayContact={id:string;local:Vector;altitudeKnown:boolean;kind:'contact'|'controlled'|'truth';confidence?:number;accuracyM?:number;sampleMs?:number;errorTo?:Vector}
type Props={compact?:boolean;environment:ExerciseEnvironmentView;load:()=>Promise<Blob>;contacts:DisplayContact[];meaning:string}
function dispose(root:T.Object3D){root.traverse(o=>{const m=o as T.Mesh;m.geometry?.dispose();for(const mat of m.material?Array.isArray(m.material)?m.material:[m.material]:[]){for(const value of Object.values(mat))if(value instanceof T.Texture)value.dispose();mat.dispose()}})}
// This renderer accepts only released geography and explicit display contacts.
// It cannot load a scene, discover hidden actors, or execute a simulator.
export default function ExerciseViewport({environment,load,contacts,meaning,compact}:Props){
 const host=useRef<HTMLDivElement>(null),update=useRef<()=>void>(()=>{}),reset=useRef<()=>void>(()=>{}),focus=useRef<(id:string)=>void>(()=>{}),data=useRef(contacts);data.current=contacts
 const [status,setStatus]=useState('Loading released geographic environment…'),[ready,setReady]=useState(false)
 useEffect(()=>update.current(),[contacts])
 useEffect(()=>{
  const node=host.current;if(!node)return;let cancelled=false,renderer:T.WebGLRenderer|undefined,controls:OrbitControls|undefined,observer:ResizeObserver|undefined
  const world=new T.Scene(),markers=new T.Group(),objects=new Map<string,T.Object3D>(),camera=new T.PerspectiveCamera(45,1,.1,100000);let geometry:T.Object3D|undefined
  world.add(markers);setReady(false);setStatus('Loading released geographic environment…')
  const draw=()=>{if(!cancelled&&renderer)renderer.render(world,camera)}
  const fit=()=>{if(!geometry)return;const box=new T.Box3().setFromObject(geometry),center=box.getCenter(new T.Vector3()),size=Math.max(100,box.getSize(new T.Vector3()).length());camera.position.copy(center).add(new T.Vector3(size*.55,size*.65,size*.65));controls?.target.copy(center);controls?.update();draw()};reset.current=fit
  focus.current=id=>{const o=objects.get(id);if(!o)return;camera.position.copy(o.position).add(new T.Vector3(140,180,180));controls?.target.copy(o.position);controls?.update();draw()}
  const resize=()=>{if(!renderer||!node.clientWidth||!node.clientHeight)return;renderer.setSize(node.clientWidth,node.clientHeight,false);camera.aspect=node.clientWidth/node.clientHeight;camera.updateProjectionMatrix();draw()}
  update.current=()=>{dispose(markers);markers.clear();objects.clear();if(!geometry)return;for(const p of data.current.slice(0,256)){
   const color=p.kind==='truth'?0xffb86b:p.kind==='controlled'?0x7df0a2:0x72d9ff,pos=new T.Vector3(p.local.x,p.local.z,-p.local.y)
   if(!p.altitudeKnown){const ray=new T.Raycaster(new T.Vector3(pos.x,10000,pos.z),new T.Vector3(0,-1,0)),hit=ray.intersectObject(geometry,true)[0];pos.y=(hit?.point.y??0)+4}
   const marker=new T.Mesh(new T.SphereGeometry(p.kind==='truth'?10:8,12,8),new T.MeshBasicMaterial({color,wireframe:p.kind==='truth'}));marker.position.copy(pos);markers.add(marker);objects.set(p.id,marker)
   const uncertainty=Math.min(500,Math.max(p.accuracyM??0,12));const ring=new T.Mesh(new T.RingGeometry(uncertainty,uncertainty+2,40),new T.MeshBasicMaterial({color,side:T.DoubleSide,transparent:true,opacity:.65}));ring.position.copy(pos);ring.rotation.x=-Math.PI/2;markers.add(ring)
   if(p.errorTo){const line=new T.Line(new T.BufferGeometry().setFromPoints([pos,new T.Vector3(p.errorTo.x,p.errorTo.z,-p.errorTo.y)]),new T.LineBasicMaterial({color:0xffe09a}));markers.add(line)}
  }draw()}
  void(async()=>{try{
   renderer=new T.WebGLRenderer({antialias:true});renderer.setPixelRatio(Math.min(devicePixelRatio,1.5));renderer.setClearColor(0x14212a);renderer.domElement.setAttribute('aria-label','Released environment and permitted exercise positions');node.appendChild(renderer.domElement)
   renderer.toneMapping=T.ACESFilmicToneMapping;renderer.toneMappingExposure=1;controls=new OrbitControls(camera,renderer.domElement);renderer.domElement.tabIndex=0;controls.listenToKeyEvents(renderer.domElement);controls.addEventListener('change',draw);world.add(new T.HemisphereLight(0xddeeff,0x4d5b46,1.2));const sun=new T.DirectionalLight(0xffffff,1.4);sun.position.set(3000,8000,1000);world.add(sun)
   const blob=await load();if(cancelled)return;const bytes=await blob.arrayBuffer(),hash=[...new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))].map(v=>v.toString(16).padStart(2,'0')).join('');if(hash!==environment.model.sha256||bytes.byteLength!==environment.model.bytes)throw new Error('Released environment integrity check failed.')
   const manager=new T.LoadingManager();manager.setURLModifier(url=>{if(!url.startsWith('data:')&&!url.startsWith('blob:'))throw new Error('External geometry resource blocked');return url})
   const gltf=await new GLTFLoader(manager).parseAsync(bytes,'');if(cancelled){dispose(gltf.scene);return};geometry=gltf.scene;world.add(geometry);observer=new ResizeObserver(resize);observer.observe(node);resize();update.current();fit();setReady(true);setStatus('Retained environment verified · drag to orbit, scroll to zoom')
  }catch(e){if(!cancelled)setStatus(e instanceof Error?e.message:'3D view unavailable. Read the permitted position list below.')}})()
  return()=>{cancelled=true;update.current=()=>{};observer?.disconnect();controls?.dispose();renderer?.dispose();renderer?.forceContextLoss();renderer?.domElement.remove();dispose(world);world.clear()}
 },[environment.model.sha256,load])
 return <section aria-label="Exercise spatial view" className="space-y-3"><div className="flex flex-wrap justify-between gap-2"><h2 className="text-lg font-semibold">{meaning}</h2><button className="min-h-11 rounded border border-edge px-3" onClick={()=>reset.current()}>Fit environment</button></div><p role="status">{status}</p><div ref={host} data-render-ready={ready} className={compact?'h-64 w-full overflow-hidden rounded border border-edge sm:h-80':'h-80 w-full overflow-hidden rounded border border-edge sm:h-[520px]'}/><p className="ux-muted">Contacts with unknown altitude are projected to the surface for orientation; this is not a measured height.</p><div className={compact?'spatial-auxiliary':'space-y-3'}><details><summary className="min-h-9 cursor-pointer text-sm">Legend, uncertainty & navigation</summary><p className="text-sm">Cyan: perceived contacts · green: controlled telemetry · amber wireframe: privileged truth, when explicitly enabled. Rings show reported uncertainty, with a 12 m minimum display locator. Focus the viewport and use arrow keys to pan. Unknown altitude is shown as a ground projection on the surface model; it is never supplied as a measured height.</p></details><details><summary className="min-h-9 cursor-pointer text-sm">Contacts & controlled assets · {contacts.length}</summary><ul className="grid gap-2 sm:grid-cols-2">{contacts.map(p=><li key={p.id} className="min-w-0 rounded border border-edge p-2"><button className="min-h-11 break-all text-left underline" disabled={!ready} onClick={()=>focus.current(p.id)}>Focus {p.id}</button><p>{p.kind} · {p.altitudeKnown?'Resolved altitude':'Altitude unknown · ground projection'}{p.confidence!=null?` · confidence ${(p.confidence*100).toFixed(0)}%`:''}{p.sampleMs!=null?` · sampled ${(p.sampleMs/1000).toFixed(1)} virtual s`:''}</p></li>)}</ul>{!contacts.length&&<p>No permitted positions at this time. A running world does not imply that a sensor has detected every actor.</p>}</details><details><summary className="min-h-11 cursor-pointer">Geographic provenance and limitations</summary><p>{environment.environment.provenance}</p><ul>{environment.environment.prepared?.limitations.map(v=><li key={v}>{v}</li>)}</ul></details></div></section>
}
