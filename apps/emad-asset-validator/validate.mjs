// Bounded static glTF profile. This process never fetches external resources.
import validator from 'gltf-validator'
const MAX = 16 * 1024 * 1024
const fail = (code) => { throw new Error(code) }
const integer = (n, min, max) => Number.isSafeInteger(n) && n >= min && n <= max
const list = (x, max) => { if (!Array.isArray(x) || x.length > max) fail('profile_array_budget'); return x }
const decode = (uri, types) => {
  if (typeof uri !== 'string') fail('external_resource_denied')
  const type = types.find((t) => uri.startsWith(`data:${t};base64,`))
  if (!type) fail('external_resource_denied')
  const text = uri.slice(`data:${type};base64,`.length)
  if (text.length > Math.ceil(MAX / 3) * 4 || !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(text)) fail('data_uri_budget_or_encoding')
  return Buffer.from(text, 'base64')
}
export async function validate(raw, media) {
  if (!Buffer.isBuffer(raw) || raw.length < 1 || raw.length > MAX) fail('file_budget')
  let json, bin
  if (media === 'model/gltf-binary') {
    if (raw.length < 20 || raw.readUInt32LE(0) !== 0x46546c67 || raw.readUInt32LE(4) !== 2 || raw.readUInt32LE(8) !== raw.length) fail('invalid_glb_header')
    let offset = 12, index = 0
    while (offset < raw.length) {
      if (offset + 8 > raw.length) fail('invalid_glb_chunk')
      const n = raw.readUInt32LE(offset), type = raw.readUInt32LE(offset + 4); offset += 8
      if (n % 4 || n > raw.length - offset) fail('invalid_glb_chunk')
      if (index === 0 && type === 0x4e4f534a) json = raw.subarray(offset, offset + n)
      else if (index === 1 && type === 0x004e4942) bin = raw.subarray(offset, offset + n)
      else fail('unsupported_glb_chunk')
      offset += n; index++
    }
  } else if (media === 'model/gltf+json') json = raw
  else fail('unsupported_media')
  if (!json || json.length > 4 * 1024 * 1024) fail('json_budget')
  const d = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(json))
  if (d.asset?.version !== '2.0' || (d.asset.minVersion && d.asset.minVersion !== '2.0')) fail('unsupported_gltf_version')
  // Reject executable/external/compressed/unsupported extension paths even when
  // optional. JSON depth and declared counts are bounded before the validator.
  const walk = (v, depth = 0) => {
    if (depth > 32) fail('json_depth_budget')
    if (!v || typeof v !== 'object') return
    for (const [k, x] of Object.entries(v)) {
      if ((k === 'extensions' && x && Object.keys(x).length) || ((k === 'extensionsUsed' || k === 'extensionsRequired') && x?.length)) fail('unsupported_extension')
      if (k === 'uri' && (typeof x !== 'string' || !x.startsWith('data:'))) fail('external_resource_denied')
      walk(x, depth + 1)
    }
  }
  walk(d)
  for (const name of ['animations', 'skins', 'cameras']) if (d[name]?.length) fail('static_profile_only')
  const buffers = list(d.buffers ?? [], 4).map((b, i) => {
    if (!integer(b.byteLength, 1, MAX)) fail('buffer_budget')
    const bytes = b.uri === undefined && i === 0 && bin ? bin : decode(b.uri, ['application/octet-stream', 'application/gltf-buffer'])
    if (bytes.length < b.byteLength || bytes.length - b.byteLength > (b.uri === undefined ? 3 : 0)) fail('buffer_length')
    return bytes.subarray(0, b.byteLength)
  })
  if (buffers.reduce((n, b) => n + b.length, 0) > MAX) fail('decoded_buffer_budget')
  const views = list(d.bufferViews ?? [], 1024)
  for (const v of views) if (!integer(v.buffer, 0, buffers.length - 1) || !integer(v.byteLength, 1, MAX) || !integer(v.byteOffset ?? 0, 0, MAX) || (v.byteOffset ?? 0) + v.byteLength > buffers[v.buffer].length) fail('buffer_view_range')
  const accessors = list(d.accessors ?? [], 1024)
  for (const a of accessors) if (!integer(a.count, 1, 250000) || a.sparse || !integer(a.bufferView, 0, views.length - 1)) fail('accessor_budget_or_sparse')
  const meshes = list(d.meshes ?? [], 128), nodes = list(d.nodes ?? [], 256), scenes = list(d.scenes ?? [], 16)
  if (!meshes.length || !scenes.length || !nodes.length) fail('renderable_scene_required')
  let vertices = 0, triangles = 0, primitives = 0
  for (const mesh of meshes) for (const p of list(mesh.primitives, 256)) {
    if ((p.mode ?? 4) !== 4 || p.targets?.length) fail('static_triangles_only')
    const a = accessors[p.attributes?.POSITION]
    if (!a || a.type !== 'VEC3' || a.componentType !== 5126 || !Array.isArray(a.min) || !Array.isArray(a.max)) fail('position_bounds_required')
    const count = p.indices === undefined ? a.count : accessors[p.indices]?.count
    if (!integer(count, 3, 750000) || count % 3) fail('triangle_index_count')
    vertices += a.count; triangles += count / 3; primitives++
  }
  if (vertices > 250000 || triangles > 250000 || primitives > 256) fail('mesh_budget')
  const materials = list(d.materials ?? [], 64), textures = list(d.textures ?? [], 32)
  let pixels = 0
  const imageInfo = list(d.images ?? [], 16).map((image) => {
    let b
    if (image.uri !== undefined) b = decode(image.uri, ['image/png', 'image/jpeg'])
    else { const v = views[image.bufferView]; if (!v) fail('image_view'); b = buffers[v.buffer].subarray(v.byteOffset ?? 0, (v.byteOffset ?? 0) + v.byteLength) }
    let width, height, mediaType
    if (b.length >= 24 && b.subarray(0, 8).equals(Buffer.from([137,80,78,71,13,10,26,10])) && b.toString('ascii',12,16) === 'IHDR') { width = b.readUInt32BE(16); height = b.readUInt32BE(20); mediaType = 'image/png' }
    else if (b.length >= 4 && b.readUInt16BE(0) === 0xffd8) {
      mediaType = 'image/jpeg'; let i = 2
      while (i + 4 <= b.length) {
        if (b[i] !== 255) fail('jpeg_header'); const marker = b[i + 1]; i += 2
        const n = b.readUInt16BE(i); if (n < 2 || i + n > b.length) fail('jpeg_range')
        if ([0xc0,0xc1,0xc2].includes(marker)) { if (n < 8) fail('jpeg_frame'); height = b.readUInt16BE(i+3); width = b.readUInt16BE(i+5); break }
        if (marker === 0xda || marker === 0xd9) break
        i += n
      }
    }
    if (!integer(width,1,4096) || !integer(height,1,4096) || (image.mimeType && image.mimeType !== mediaType) || (image.uri && !image.uri.startsWith(`data:${mediaType};base64,`))) fail('texture_dimensions_or_media')
    pixels += width * height
    return { width, height, mediaType, bytes: b.length }
  })
  if (pixels > 16 * 1024 * 1024) fail('decoded_texture_budget')
  const report = await validator.validateBytes(new Uint8Array(raw), { maxIssues: 20, externalResourceFunction: () => Promise.reject(new Error('external_resource_denied')) })
  if (report.issues.truncated) fail('validation_issue_budget')
  if (report.issues.numErrors) fail('gltf_validation_failed:' + report.issues.messages.filter((m) => m.severity === 0).map((m) => m.code).join(',').slice(0, 500))
  // Bounds use validated POSITION bounds and the actual selected scene hierarchy.
  const bounds = { min: [Infinity,Infinity,Infinity], max: [-Infinity,-Infinity,-Infinity] }
  const mul = (a,b) => Array.from({length:16},(_,i) => { const r=i%4,c=Math.floor(i/4);return a[r]*b[c*4]+a[r+4]*b[c*4+1]+a[r+8]*b[c*4+2]+a[r+12]*b[c*4+3] })
  const identity = () => [1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1]
  const matrix = (n) => {
    if(n.matrix) return n.matrix
    const [x,y,z,w]=n.rotation??[0,0,0,1],s=n.scale??[1,1,1],t=n.translation??[0,0,0]
    return [(1-2*y*y-2*z*z)*s[0],(2*x*y+2*z*w)*s[0],(2*x*z-2*y*w)*s[0],0,(2*x*y-2*z*w)*s[1],(1-2*x*x-2*z*z)*s[1],(2*y*z+2*x*w)*s[1],0,(2*x*z+2*y*w)*s[2],(2*y*z-2*x*w)*s[2],(1-2*x*x-2*y*y)*s[2],0,...t,1]
  }
  let instances=0, renderedTriangles=0, renderedVertices=0
  const visit = (id,parent,path) => {
    if(path.size >= 32 || path.has(id) || ++instances > 256) fail('scene_hierarchy_budget')
    const n=nodes[id],m=mul(parent,matrix(n)),next=new Set([...path,id])
    if(n.mesh !== undefined) for(const p of meshes[n.mesh].primitives) { const a=accessors[p.attributes.POSITION];renderedVertices+=a.count;renderedTriangles+=(p.indices===undefined?a.count:accessors[p.indices].count)/3;if(renderedVertices>250000||renderedTriangles>250000)fail('instanced_mesh_budget')
      for(let bits=0;bits<8;bits++) {const v=[0,1,2].map((k)=>(bits&(1<<k))?a.max[k]:a.min[k]);for(let k=0;k<3;k++){const x=m[k]*v[0]+m[k+4]*v[1]+m[k+8]*v[2]+m[k+12];if(!Number.isFinite(x)||Math.abs(x)>500000)fail('scene_bounds_budget');bounds.min[k]=Math.min(bounds.min[k],x);bounds.max[k]=Math.max(bounds.max[k],x)}}
    }
    for(const child of n.children??[])visit(child,m,next)
  }
  const scene=scenes[d.scene??0]; if(!scene)fail('default_scene_required')
  for(const id of scene.nodes??[])visit(id,identity(),new Set())
  if(!bounds.min.every(Number.isFinite))fail('empty_scene')
  return { schema:'emad.model-validation.v1', importer:'emad-static-gltf-v1', validatorVersion:validator.version(), units:'metres', axes:'right_handed_y_up_z_forward', fidelity:'visual_only', vertices:renderedVertices, triangles:renderedTriangles, primitives, nodes:nodes.length, instances, materials:materials.map((m)=>({name:(m.name??'').slice(0,256),alphaMode:m.alphaMode??'OPAQUE',doubleSided:m.doubleSided??false,pbr:{baseColorFactor:m.pbrMetallicRoughness?.baseColorFactor??[1,1,1,1],metallicFactor:m.pbrMetallicRoughness?.metallicFactor??1,roughnessFactor:m.pbrMetallicRoughness?.roughnessFactor??1,baseColorTexture:m.pbrMetallicRoughness?.baseColorTexture?{index:m.pbrMetallicRoughness.baseColorTexture.index,texCoord:m.pbrMetallicRoughness.baseColorTexture.texCoord??0}:null,metallicRoughnessTexture:m.pbrMetallicRoughness?.metallicRoughnessTexture?{index:m.pbrMetallicRoughness.metallicRoughnessTexture.index,texCoord:m.pbrMetallicRoughness.metallicRoughnessTexture.texCoord??0}:null},emissiveFactor:m.emissiveFactor??[0,0,0],normalTexture:m.normalTexture?{index:m.normalTexture.index,texCoord:m.normalTexture.texCoord??0,scale:m.normalTexture.scale??1}:null,occlusionTexture:m.occlusionTexture?{index:m.occlusionTexture.index,texCoord:m.occlusionTexture.texCoord??0,strength:m.occlusionTexture.strength??1}:null,emissiveTexture:m.emissiveTexture?{index:m.emissiveTexture.index,texCoord:m.emissiveTexture.texCoord??0}:null})), images:imageInfo, textureCount:textures.length, decodedTexturePixels:pixels, bounds, warnings:report.issues.messages.filter((m)=>m.severity>0).map((m)=>m.code) }
}
if (process.argv[1]?.endsWith('validate.mjs')) {
  try {
    const chunks=[];let bytes=0
    for await(const chunk of process.stdin){bytes+=chunk.length;if(bytes>MAX)fail('file_budget');chunks.push(chunk)}
    process.stdout.write(JSON.stringify(await validate(Buffer.concat(chunks),process.argv[2])))
  } catch(e){process.stderr.write(JSON.stringify({assetRejected:true,reason:String(e.message).slice(0,1000)}));process.exitCode=1}
}
