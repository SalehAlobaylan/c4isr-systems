"""Small project-owned static glTF generator; geometry uses metres and Y up."""
import json
import struct
import math
import numpy as np


class GLB:
    def __init__(self):
        self.binary = bytearray()
        self.document = {'asset': {'version': '2.0', 'generator': 'EMAD static geometry v1'}, 'scene': 0,
                         'scenes': [{'nodes': [0]}], 'nodes': [{'mesh': 0}], 'meshes': [{'primitives': []}],
                         'buffers': [{'byteLength': 0}], 'bufferViews': [], 'accessors': [], 'materials': []}

    def view(self, raw, target=None):
        while len(self.binary) % 4: self.binary.append(0)
        item = {'buffer': 0, 'byteOffset': len(self.binary), 'byteLength': len(raw)}
        if target: item['target'] = target
        index = len(self.document['bufferViews'])
        self.document['bufferViews'].append(item)
        self.binary.extend(raw)
        return index

    def accessor(self, array, kind, component=5126, target=34962):
        array = np.asarray(array, dtype='<f4' if component == 5126 else '<u4')
        view = self.view(array.tobytes(), target)
        value = {'bufferView': view, 'componentType': component, 'count': len(array), 'type': kind}
        if kind == 'VEC3':
            value.update({'min': array.min(axis=0).astype(float).tolist(), 'max': array.max(axis=0).astype(float).tolist()})
        index = len(self.document['accessors']); self.document['accessors'].append(value)
        return index

    def material(self, name, colour, texture=None):
        pbr = {'baseColorFactor': list(colour), 'metallicFactor': .05, 'roughnessFactor': .85}
        if texture is not None:
            image = {'bufferView': self.view(texture), 'mimeType': 'image/png'}
            self.document.setdefault('images', []).append(image)
            self.document.setdefault('samplers', []).append({'magFilter': 9729, 'minFilter': 9729, 'wrapS': 33071, 'wrapT': 33071})
            textures = self.document.setdefault('textures', [])
            textures.append({'source': len(self.document['images'])-1, 'sampler': len(self.document['samplers'])-1})
            pbr['baseColorTexture'] = {'index': len(textures)-1}
        index = len(self.document['materials'])
        self.document['materials'].append({'name': name, 'doubleSided': True, 'pbrMetallicRoughness': pbr})
        return index

    def mesh(self, positions, triangles, material, uv=None):
        positions = np.asarray(positions, dtype=np.float32)
        triangles = np.asarray(triangles, dtype=np.uint32).reshape((-1, 3))
        if len(triangles) > 80000:
            for start in range(0, len(triangles), 80000):
                used, remapped = np.unique(triangles[start:start+80000].ravel(), return_inverse=True)
                self.mesh(positions[used], remapped.reshape((-1,3)), material, np.asarray(uv)[used] if uv is not None else None)
            return
        normals = np.zeros_like(positions)
        cross = np.cross(positions[triangles[:, 1]]-positions[triangles[:, 0]], positions[triangles[:, 2]]-positions[triangles[:, 0]])
        for i in range(3): np.add.at(normals, triangles[:, i], cross)
        length = np.linalg.norm(normals, axis=1)
        normals /= np.maximum(length[:, None], 1e-12)
        empty = length < 1e-12; normals[empty] = [0, 1, 0]
        attributes = {'POSITION': self.accessor(positions, 'VEC3'), 'NORMAL': self.accessor(normals, 'VEC3')}
        if uv is not None: attributes['TEXCOORD_0'] = self.accessor(uv, 'VEC2')
        self.document['meshes'][0]['primitives'].append({'attributes': attributes,
            'indices': self.accessor(triangles.ravel(), 'SCALAR', 5125, 34963), 'material': material, 'mode': 4})

    def bytes(self):
        while len(self.binary) % 4: self.binary.append(0)
        self.document['buffers'][0]['byteLength'] = len(self.binary)
        document = json.dumps(self.document, separators=(',', ':'), allow_nan=False).encode()
        document += b' ' * (-len(document) % 4)
        return struct.pack('<4sII', b'glTF', 2, 12+8+len(document)+8+len(self.binary)) + struct.pack('<I4s', len(document), b'JSON') + document + struct.pack('<I4s', len(self.binary), b'BIN\0') + self.binary


class Parts:
    def __init__(self): self.positions, self.triangles = [], []
    def add(self, points, faces):
        start = len(self.positions); self.positions.extend(points); self.triangles.extend([[start+a, start+b, start+c] for a,b,c in faces])
    def box(self, center, size):
        x,y,z = center; a,b,c = [v/2 for v in size]
        points = [[x+sx*a,y+sy*b,z+sz*c] for sx,sy,sz in [(-1,-1,-1),(1,-1,-1),(1,1,-1),(-1,1,-1),(-1,-1,1),(1,-1,1),(1,1,1),(-1,1,1)]]
        self.add(points, [[0,2,1],[0,3,2],[4,5,6],[4,6,7],[0,1,5],[0,5,4],[3,7,6],[3,6,2],[0,4,7],[0,7,3],[1,2,6],[1,6,5]])
    def ellipsoid(self, center, radii, sections=16, rings=8):
        points = []
        for j in range(rings+1):
            phi = math.pi*j/rings
            for i in range(sections):
                theta = 2*math.pi*i/sections
                points.append([center[0]+radii[0]*math.sin(phi)*math.cos(theta), center[1]+radii[1]*math.cos(phi), center[2]+radii[2]*math.sin(phi)*math.sin(theta)])
        faces=[]
        for j in range(rings):
            for i in range(sections):
                a=j*sections+i;b=j*sections+(i+1)%sections;c=(j+1)*sections+i;d=(j+1)*sections+(i+1)%sections
                if j>0: faces.append([a,b,c])
                if j<rings-1: faces.append([b,d,c])
        self.add(points, faces)


def model(kind):
    body, detail, dark = Parts(), Parts(), Parts()
    if kind == 'fixed-wing':
        body.ellipsoid((0,2,0),(1.3,1.2,7));body.box((0,2,0),(16,.3,2.4));body.box((0,2,-5.8),(5,.2,1.4));detail.box((0,3.1,-5.5),(.3,2.3,1.5));dark.ellipsoid((0,2.7,3),(1,.5,1.8))
    elif kind == 'uav':
        body.ellipsoid((0,.7,0),(.8,.35,1));body.box((0,.6,0),(4.5,.15,.25));body.box((0,.6,0),(.25,.15,4.5))
        for x,z in [(2,0),(-2,0),(0,2),(0,-2)]: dark.ellipsoid((x,.85,z),(.7,.06,.7),16,4)
        detail.box((0,.2,.6),(.4,.4,.4))
    elif kind == 'helicopter':
        body.ellipsoid((0,1.9,.5),(1.4,1.3,3.2));body.box((0,2,-4.5),(.5,.5,6));dark.ellipsoid((0,2.1,2.4),(1.2,1,1.2));detail.box((0,3.7,0),(11,.12,.35));detail.box((0,3.7,0),(.35,.12,11));detail.box((0,2,-7),(.15,2,1))
        for x in [-1.3,1.3]: dark.box((x,.2,0),(.15,.2,5))
    elif kind == 'vehicle':
        body.box((0,1.3,0),(2.2,1.4,4.6));detail.box((0,2.3,.7),(2,1.1,2));dark.box((0,2.4,1.73),(1.7,.6,.04))
        for x,z in [(-1.1,-1.5),(1.1,-1.5),(-1.1,1.5),(1.1,1.5)]: dark.ellipsoid((x,.55,z),(.35,.55,.55))
    elif kind == 'vessel':
        body.ellipsoid((0,.6,0),(3.2,1,12));body.box((0,1.4,-1),(5,.3,16));detail.box((0,3,-2),(3.5,3,5));dark.box((0,3.7,.55),(3.1,.7,.1));detail.box((0,6,-2),(.2,4,.2))
    elif kind == 'sensor':
        body.box((0,.3,0),(5,.6,5));body.box((0,4,0),(.5,7,.5));detail.ellipsoid((0,8,0),(3,.7,1.5));dark.box((0,1.2,1),(2,1.2,1))
    else: raise ValueError('Unknown starter model')
    output = GLB()
    for name, part, colour in [('body',body,(.68,.76,.79,1)),('detail',detail,(.1,.55,.65,1)),('dark',dark,(.055,.1,.15,1))]:
        if part.positions: output.mesh(part.positions,part.triangles,output.material(name,colour))
    return output.bytes()
