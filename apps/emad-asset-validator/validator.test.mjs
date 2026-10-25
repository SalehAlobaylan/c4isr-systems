import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { validate } from './validate.mjs'
const source = new URL('../../testdata/models/',import.meta.url)
test('project-authored GLB and embedded glTF retain verified geometry/material metadata',async()=>{
 for(const [name,media,triangles] of [['abstract-wing.glb','model/gltf-binary',1],['marker.glb','model/gltf-binary',2],['ground-tile.gltf','model/gltf+json',2]]){
  const v=await validate(await readFile(new URL(name,source)),media)
  assert.equal(v.triangles,triangles);assert.equal(v.fidelity,'visual_only');assert.equal(v.units,'metres');assert.equal(v.materials.length,1);assert.ok(v.validatorVersion)
 }
})
test('actual node transforms define scene bounds; claimed accessor bounds cannot hide geometry',async()=>{
 const d=JSON.parse(await readFile(new URL('ground-tile.gltf',source),'utf8'));d.nodes[0].translation=[1,2,3]
 const v=await validate(Buffer.from(JSON.stringify(d)),'model/gltf+json');assert.deepEqual(v.bounds,{min:[-9,2,-7],max:[11,2,13]})
 d.accessors[0].max[0]=0
 await assert.rejects(()=>validate(Buffer.from(JSON.stringify(d)),'model/gltf+json'),/gltf_validation_failed/)
})
test('external references, archives, compression, expansion and invalid indices fail closed',async()=>{
 const original=JSON.parse(await readFile(new URL('ground-tile.gltf',source),'utf8'))
 for(const change of [d=>d.buffers[0].uri='https://example.invalid/private.bin',d=>d.buffers[0].uri='../../private.bin',d=>d.extensionsUsed=['KHR_draco_mesh_compression'],d=>d.accessors[0].count=100000000,d=>d.bufferViews[0].byteLength=100000000,d=>d.nodes[0].children=[0],d=>d.meshes[0].primitives[0].indices=100,d=>d.skins=[{}]]){
  const d=structuredClone(original);change(d);await assert.rejects(()=>validate(Buffer.from(JSON.stringify(d)),'model/gltf+json'))
 }
 await assert.rejects(()=>validate(Buffer.alloc(16*1024*1024+1),'model/gltf-binary'),/file_budget/)
 const raw=await readFile(new URL('abstract-wing.glb',source));raw.writeUInt32LE(20,8)
 await assert.rejects(()=>validate(raw,'model/gltf-binary'),/invalid_glb_header/)
 await assert.rejects(()=>validate(Buffer.from('PK ZIP'),'application/zip'),/unsupported_media/)
})

test('embedded PNG metadata bounds future decoded textures without external IO',async()=>{
 const d=JSON.parse(await readFile(new URL('ground-tile.gltf',source),'utf8'))
 const image=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=','base64')
 d.images=[{uri:'data:image/png;base64,'+image.toString('base64')}];d.textures=[{source:0}]
 const v=await validate(Buffer.from(JSON.stringify(d)),'model/gltf+json');assert.equal(v.images[0].width,1);assert.equal(v.decodedTexturePixels,1)
 image.writeUInt32BE(100000,16);d.images[0].uri='data:image/png;base64,'+image.toString('base64')
 await assert.rejects(()=>validate(Buffer.from(JSON.stringify(d)),'model/gltf+json'),/texture_dimensions_or_media/)
})
test('rendered instance budgets count repeated meshes',async()=>{
 const d=JSON.parse(await readFile(new URL('ground-tile.gltf',source),'utf8'))
 const data=Buffer.alloc(125001*12);d.buffers[0]={byteLength:data.length,uri:'data:application/octet-stream;base64,'+data.toString('base64')};d.bufferViews[0].byteLength=data.length;d.accessors[0].count=125001;d.accessors[0].min=[0,0,0];d.accessors[0].max=[0,0,0];d.nodes=[{mesh:0},{mesh:0},{mesh:0}];d.scenes[0].nodes=[0,1,2]
 await assert.rejects(()=>validate(Buffer.from(JSON.stringify(d)),'model/gltf+json'),/instanced_mesh_budget/)
})

test('truncated issue reports never certify an incompletely reported model',async()=>{
 const d=JSON.parse(await readFile(new URL('ground-tile.gltf',source),'utf8'))
 for(let i=0;i<24;i++)d.materials.push({name:'Unused fixture material '+i})
 await assert.rejects(()=>validate(Buffer.from(JSON.stringify(d)),'model/gltf+json'),/validation_issue_budget/)
})
