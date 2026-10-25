// A small identical scene for both shortlisted rendering adapters. This file is
// built only by the evaluation harness; Babylon is not shipped with Operator UI.
const candidate=new URLSearchParams(location.search).get('engine')
const started=performance.now()
const fixtures=await (await fetch('/fixture.json')).json()
const canvas=document.querySelector('canvas')
let render,dispose,move,gl,counts
if(candidate==='three'){
 const T=await import('three'),{GLTFLoader}=await import('three/addons/loaders/GLTFLoader.js')
 const renderer=new T.WebGLRenderer({canvas,antialias:false});renderer.setSize(960,640);renderer.setPixelRatio(1);gl=renderer.getContext()
 const scene=new T.Scene();scene.background=new T.Color(0x14212a)
 const camera=new T.PerspectiveCamera(50,1.5,0.1,20000);camera.position.set(300,400,500);camera.lookAt(100,0,0)
 scene.add(new T.HemisphereLight(0xffffff,0x334455,2))
 const objects=[]
 for(const entry of fixtures.assets){const raw=await(await fetch('/models/'+entry.file)).arrayBuffer();const model=await new GLTFLoader().parseAsync(entry.file.endsWith('.gltf')?new TextDecoder().decode(raw):raw,'');model.scene.position.set(entry.local.x,entry.local.z,-entry.local.y);scene.add(model.scene);objects.push(model.scene)}
 const geometry=new T.BufferGeometry();geometry.setAttribute('position',new T.Float32BufferAttribute(fixtures.terrain.flatMap(v=>[v.x,v.z,-v.y]),3));geometry.setIndex(fixtures.triangles);geometry.computeVertexNormals();scene.add(new T.Mesh(geometry,new T.MeshStandardMaterial({color:0x5b7362,side:T.DoubleSide})))
 render=()=>renderer.render(scene,camera);move=(x)=>{objects[1].position.x=x;objects[1].updateMatrixWorld(true)};counts=()=>({triangles:renderer.info.render.triangles,drawCalls:renderer.info.render.calls});dispose=()=>renderer.dispose()
}else{
 const B=await import('@babylonjs/core');await import('@babylonjs/loaders/glTF')
 const engine=new B.Engine(canvas,false,{preserveDrawingBuffer:false,stencil:false});engine.setSize(960,640);gl=engine._gl
 const scene=new B.Scene(engine);scene.useRightHandedSystem=true;scene.clearColor=new B.Color4(0.078,0.129,0.165,1)
 const camera=new B.FreeCamera('camera',new B.Vector3(300,400,500),scene);camera.setTarget(new B.Vector3(100,0,0));camera.fov=50*Math.PI/180;camera.minZ=0.1;camera.maxZ=20000
 new B.HemisphericLight('light',new B.Vector3(0,1,0),scene).intensity=2
 const objects=[]
 for(const entry of fixtures.assets){const raw=await(await fetch('/models/'+entry.file)).arrayBuffer();const result=await B.SceneLoader.ImportMeshAsync(null,'',entry.file.endsWith('.gltf')?'data:'+new TextDecoder().decode(raw):new Uint8Array(raw),scene,null,entry.file.endsWith('.gltf')?'.gltf':'.glb');const root=result.meshes[0];root.position.set(entry.local.x,entry.local.z,-entry.local.y);objects.push(root)}
 const data=new B.VertexData();data.positions=fixtures.terrain.flatMap(v=>[v.x,v.z,-v.y]);data.indices=fixtures.triangles;data.normals=[];B.VertexData.ComputeNormals(data.positions,data.indices,data.normals);const mesh=new B.Mesh('terrain',scene);data.applyToMesh(mesh);const material=new B.StandardMaterial('terrain',scene);material.diffuseColor=new B.Color3(.36,.45,.38);material.backFaceCulling=false;mesh.material=material
 await scene.whenReadyAsync();
 let lastDrawCalls=0;render=()=>{const before=engine._drawCalls.current;scene.render();lastDrawCalls=engine._drawCalls.current-before};move=(x)=>{objects[1].position.x=x;objects[1].computeWorldMatrix(true)};counts=()=>({triangles:scene.getActiveMeshes().data.slice(0,scene.getActiveMeshes().length).reduce((sum,m)=>sum+(m.getTotalIndices()||m.getTotalVertices())/3,0),drawCalls:lastDrawCalls});dispose=()=>engine.dispose()
}
render();gl.finish()
const loadMs=performance.now()-started
const debug=gl.getExtension('WEBGL_debug_renderer_info'),timer=gl.getExtension('EXT_disjoint_timer_query_webgl2')
const renderer=debug?gl.getParameter(debug.UNMASKED_RENDERER_WEBGL):gl.getParameter(gl.RENDERER)
let query;if(timer){query=gl.createQuery();gl.beginQuery(timer.TIME_ELAPSED_EXT,query)}
const frames=120,t0=performance.now()
for(let i=0;i<frames;i++){move(i*10/60);render()}
gl.finish();const renderBatchMs=performance.now()-t0
if(timer)gl.endQuery(timer.TIME_ELAPSED_EXT)
let gpuMs=null
if(query){for(let i=0;i<100;i++){await new Promise(r=>setTimeout(r,10));if(gl.getQueryParameter(query,gl.QUERY_RESULT_AVAILABLE)){if(!gl.getParameter(timer.GPU_DISJOINT_EXT))gpuMs=gl.getQueryParameter(query,gl.QUERY_RESULT)/1e6;break}}gl.deleteQuery(query)}
window.evaluation={candidate,loadMs,frames,renderBatchMs,gpuBatchMs:gpuMs,gpuTimerAvailable:!!timer,renderer,...counts(),maxFloat32PositionErrorM:Math.max(...fixtures.coordinates.map(v=>Math.hypot(Math.fround(v.x)-v.x,Math.fround(v.y)-v.y,Math.fround(v.z)-v.z))),headlessExpectedFinalX:(frames-1)*10/60,jsHeapBytes:performance.memory?.usedJSHeapSize??null}
window.disposeEvaluation=dispose
