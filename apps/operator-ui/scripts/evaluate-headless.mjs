import {fixture} from '../engine-evaluation/fixture.mjs'
import {readFile} from 'node:fs/promises'
import {performance} from 'node:perf_hooks'
import {resolve,dirname} from 'node:path'
import {fileURLToPath} from 'node:url'
const candidate=process.argv[2],root=resolve(dirname(fileURLToPath(import.meta.url)),'../../..')
const before=process.cpuUsage(),start=performance.now(),names=['ground-tile.gltf','abstract-wing.glb','marker.glb']
let roots=[],step,dispose
if(candidate==='three'){
 globalThis.ProgressEvent??=class{constructor(type,init){this.type=type;Object.assign(this,init)}}
 const T=await import('three'),{GLTFLoader}=await import('three/addons/loaders/GLTFLoader.js'),scene=new T.Scene()
 for(const name of names){const bytes=await readFile(resolve(root,'testdata/models',name)),raw=bytes.buffer.slice(bytes.byteOffset,bytes.byteOffset+bytes.byteLength),loaded=await new GLTFLoader().parseAsync(name.endsWith('.gltf')?bytes.toString():raw,'');scene.add(loaded.scene);roots.push(loaded.scene)}
 for(const [i,a]of fixture.assets.entries())roots[i].position.set(a.local.x,a.local.z,-a.local.y)
 const geometry=new T.BufferGeometry();geometry.setAttribute('position',new T.Float32BufferAttribute(fixture.terrain.flatMap(v=>[v.x,v.z,-v.y]),3));geometry.setIndex(fixture.triangles);scene.add(new T.Mesh(geometry,new T.MeshBasicMaterial()))
 step=x=>{roots[1].position.set(x,10,0);scene.updateMatrixWorld(true);return roots[1].matrixWorld.elements[12]};dispose=()=>scene.clear()
}else{
 const B=await import('@babylonjs/core');await import('@babylonjs/loaders/glTF/index.js')
 const engine=new B.NullEngine(),scene=new B.Scene(engine);scene.useRightHandedSystem=true
 for(const name of names){const bytes=await readFile(resolve(root,'testdata/models',name)),loaded=await B.SceneLoader.ImportMeshAsync(null,'',name.endsWith('.gltf')?'data:'+bytes.toString():new Uint8Array(bytes),scene,null,name.endsWith('.gltf')?'.gltf':'.glb');roots.push(loaded.meshes[0])}
 for(const [i,a]of fixture.assets.entries())roots[i].position.set(a.local.x,a.local.z,-a.local.y)
 const data=new B.VertexData();data.positions=fixture.terrain.flatMap(v=>[v.x,v.z,-v.y]);data.indices=fixture.triangles;data.applyToMesh(new B.Mesh('terrain',scene))
 step=x=>{roots[1].position.set(x,10,0);return roots[1].computeWorldMatrix(true).getTranslation().x};dispose=()=>engine.dispose()
}
const loadMs=performance.now()-start;let error=0
for(let i=0;i<20000;i++){const elapsedMs=i*50,expected=Math.min(100,elapsedMs*10/1000);error=Math.max(error,Math.abs(step(expected)-expected))}
const cpu=process.cpuUsage(before),memory=process.memoryUsage();dispose()
console.log('EMAD_RESULT '+JSON.stringify({candidate,loadMs,totalMs:performance.now()-start,cpuMs:(cpu.user+cpu.system)/1000,rssBytes:memory.rss,heapUsedBytes:memory.heapUsed,steps:20000,terrainVertices:fixture.terrain.length,coordinateFloat32ErrorM:Math.max(...fixture.coordinates.map(v=>Math.hypot(Math.fround(v.x)-v.x,Math.fround(v.y)-v.y,Math.fround(v.z)-v.z))),maxPositionErrorM:error}))
if(error>0.001)process.exitCode=1
