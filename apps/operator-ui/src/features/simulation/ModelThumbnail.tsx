import {useEffect,useState} from 'react'
import * as T from 'three'
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js'
import type {components} from '@/lib/openapi.generated'
// Thumbnails are rendered from the same validated bytes as the viewport.
// Queue creation so six library cards do not hold six WebGL contexts.
let queue=Promise.resolve()
export function ModelThumbnail({model,load}:{model:components['schemas']['ModelAssetVersion'];load:(id:string)=>Promise<Blob>}){
 const [src,setSrc]=useState(''),[error,setError]=useState('')
 useEffect(()=>{let cancelled=false;const render=async()=>{if(cancelled)return;let renderer:T.WebGLRenderer|undefined,scene:T.Scene|undefined;try{
  const blob=await load(model.upload.id),bytes=await blob.arrayBuffer();if(cancelled)return;const hash=[...new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))].map(v=>v.toString(16).padStart(2,'0')).join('');if(hash!==model.upload.sha256||bytes.byteLength!==model.upload.bytes)throw Error('Model integrity failed')
  const manager=new T.LoadingManager();manager.setURLModifier(url=>{if(!url.startsWith('data:')&&!url.startsWith('blob:'))throw Error('External resource denied');return url});const gltf=await new GLTFLoader(manager).parseAsync(model.upload.mediaType==='model/gltf+json'?new TextDecoder().decode(bytes):bytes,'');scene=new T.Scene();scene.add(gltf.scene);const box=new T.Box3().setFromObject(gltf.scene),center=box.getCenter(new T.Vector3()),size=Math.max(.1,box.getSize(new T.Vector3()).length());const camera=new T.PerspectiveCamera(40,1.5,size/1000,size*10);camera.position.copy(center).add(new T.Vector3(size*.7,size*.6,size*.8));camera.lookAt(center);scene.add(new T.HemisphereLight(0xffffff,0x46524d,3));const light=new T.DirectionalLight(0xffffff,2);light.position.set(20,40,30);scene.add(light);renderer=new T.WebGLRenderer({antialias:true,preserveDrawingBuffer:true});renderer.setSize(300,200);renderer.setClearColor(0x24343a);renderer.render(scene,camera);if(!cancelled)setSrc(renderer.domElement.toDataURL('image/png'))
 }catch{if(!cancelled)setError('Preview unavailable; model dimensions and validation remain available.')}finally{scene?.traverse(o=>{const mesh=o as T.Mesh;mesh.geometry?.dispose();for(const m of mesh.material?Array.isArray(mesh.material)?mesh.material:[mesh.material]:[]){for(const v of Object.values(m))if(v instanceof T.Texture)v.dispose();m.dispose()}});renderer?.dispose();renderer?.forceContextLoss()}}
 queue=queue.then(render,render);return()=>{cancelled=true}
 },[model.id,load])
 return src?<img src={src} width="300" height="200" alt={model.upload.name+' · rendered validated geometry'} className="aspect-[3/2] w-full rounded"/>:<p role="status" className="flex aspect-[3/2] items-center p-3 text-sm">{error||'Rendering model preview…'}</p>
}
