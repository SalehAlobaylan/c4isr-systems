"""Generate project-authored abstract static meshes for bounded import tests.
No external models, military performance claims or physical fidelity are used.
"""
from pathlib import Path
import base64, json, struct
ROOT = Path(__file__).resolve().parents[2] / "testdata" / "models"
ROOT.mkdir(parents=True, exist_ok=True)
def document(name, vertices):
    data = b"".join(struct.pack("<fff", *v) for v in vertices)
    lo = [min(v[k] for v in vertices) for k in range(3)]
    hi = [max(v[k] for v in vertices) for k in range(3)]
    return {"asset":{"version":"2.0","generator":"EMAD project-authored synthetic fixture"},"scene":0,"scenes":[{"nodes":[0]}],"nodes":[{"mesh":0}],"meshes":[{"name":name,"primitives":[{"attributes":{"POSITION":0},"material":0}]}],"buffers":[{"byteLength":len(data)}],"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":len(data),"target":34962}],"accessors":[{"bufferView":0,"componentType":5126,"count":len(vertices),"type":"VEC3","min":lo,"max":hi}],"materials":[{"name":"Synthetic neutral material","pbrMetallicRoughness":{"baseColorFactor":[.4,.5,.6,1],"metallicFactor":0,"roughnessFactor":1},"doubleSided":True}]}, data
samples = {
    "abstract-wing": [(-2,0,0),(2,0,0),(0,.5,3)],
    "marker": [(0,0,0),(1,0,0),(0,1,0),(0,0,0),(0,0,1),(1,0,0)],
    "ground-tile": [(-10,0,-10),(10,0,-10),(10,0,10),(-10,0,-10),(10,0,10),(-10,0,10)],
}
for name, vertices in samples.items():
    doc, data = document(name, vertices)
    if name == "ground-tile":
        doc["buffers"][0]["uri"] = "data:application/octet-stream;base64," + base64.b64encode(data).decode()
        (ROOT / (name + ".gltf")).write_text(json.dumps(doc, indent=2)+"\n", encoding="utf-8", newline="\n")
    else:
        raw = json.dumps(doc, separators=(",", ":")).encode(); raw += b" " * (-len(raw) % 4)
        data += b"\x00" * (-len(data) % 4)
        total = 12+8+len(raw)+8+len(data)
        (ROOT / (name + ".glb")).write_bytes(struct.pack("<III",0x46546c67,2,total)+struct.pack("<II",len(raw),0x4e4f534a)+raw+struct.pack("<II",len(data),0x004e4942)+data)
