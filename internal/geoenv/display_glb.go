package geoenv

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
)

type terrainDocument struct {
	Asset struct {
		Version   string
		Generator string
	}
	Scene  int
	Scenes []struct{ Nodes []int }
	Nodes  []struct{ Mesh int }
	Meshes []struct {
		Primitives []struct {
			Attributes              struct{ Position, Normal int }
			Indices, Material, Mode int
		}
	}
	Materials []struct {
		Name                 string
		DoubleSided          bool
		PBRMetallicRoughness struct {
			BaseColorFactor                 [4]float64
			MetallicFactor, RoughnessFactor float64
		}
	}
	Buffers     []struct{ ByteLength int }
	BufferViews []struct{ Buffer, ByteOffset, ByteLength, Target int }
	Accessors   []struct {
		BufferView, ComponentType, Count int
		Type                             string
		Min, Max                         []float64
	}
}

// Terrain displays accept only the geometry subset produced by GLB.mesh.
// Strict decoding excludes external resources, extensions and scene graphs.
func validTerrainGLB(raw []byte) bool {
	if len(raw) < 28 || string(raw[:4]) != "glTF" || binary.LittleEndian.Uint32(raw[4:8]) != 2 ||
		int(binary.LittleEndian.Uint32(raw[8:12])) != len(raw) {
		return false
	}
	jsonLength := int(binary.LittleEndian.Uint32(raw[12:16]))
	if jsonLength < 2 || jsonLength%4 != 0 || jsonLength > 512<<10 || jsonLength > len(raw)-28 || string(raw[16:20]) != "JSON" {
		return false
	}
	start := 20 + jsonLength
	data := raw[start+8:]
	if string(raw[start+4:start+8]) != "BIN\x00" || int(binary.LittleEndian.Uint32(raw[start:start+4])) != len(data) || len(data)%4 != 0 {
		return false
	}
	var doc terrainDocument
	decoder := json.NewDecoder(bytes.NewReader(raw[20:start]))
	decoder.DisallowUnknownFields()
	var fields any
	if json.Unmarshal(raw[20:start], &fields) != nil || !terrainFields(fields) ||
		decoder.Decode(&doc) != nil || doc.Asset.Version != "2.0" ||
		doc.Scene != 0 || len(doc.Scenes) != 1 || len(doc.Scenes[0].Nodes) != 1 || doc.Scenes[0].Nodes[0] != 0 ||
		len(doc.Nodes) != 1 || doc.Nodes[0].Mesh != 0 || len(doc.Meshes) != 1 || len(doc.Materials) != 1 ||
		len(doc.Buffers) != 1 || doc.Buffers[0].ByteLength != len(data) {
		return false
	}
	mesh := doc.Meshes[0].Primitives
	if len(mesh) < 1 || len(mesh) > 16 || len(doc.BufferViews) != len(mesh)*3 || len(doc.Accessors) != len(mesh)*3 {
		return false
	}
	triangles := 0
	for _, p := range mesh {
		if p.Mode != 4 || p.Material != 0 {
			return false
		}
		positions, vertices := doc.terrainAccessor(data, p.Attributes.Position, 5126, 34962, "VEC3", 12)
		normals, normalCount := doc.terrainAccessor(data, p.Attributes.Normal, 5126, 34962, "VEC3", 12)
		indices, indexCount := doc.terrainAccessor(data, p.Indices, 5125, 34963, "SCALAR", 4)
		if vertices < 3 || vertices != normalCount || indexCount < 3 || indexCount%3 != 0 ||
			!boundedFloats(positions, 1e7) || !boundedFloats(normals, 1.001) {
			return false
		}
		triangles += indexCount / 3
		if triangles > 500_000 {
			return false
		}
		for i := 0; i < len(indices); i += 4 {
			if binary.LittleEndian.Uint32(indices[i:]) >= uint32(vertices) {
				return false
			}
		}
	}
	return true
}

func terrainFields(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, field := range v {
			switch key {
			case "asset", "version", "generator", "scene", "scenes", "nodes", "mesh", "meshes", "primitives",
				"materials", "name", "doubleSided", "pbrMetallicRoughness", "baseColorFactor", "metallicFactor", "roughnessFactor",
				"buffers", "byteLength", "bufferViews", "buffer", "byteOffset", "target", "accessors", "bufferView", "componentType",
				"count", "type", "min", "max", "attributes", "POSITION", "NORMAL", "indices", "material", "mode":
			default:
				return false
			}
			if !terrainFields(field) {
				return false
			}
		}
	case []any:
		for _, field := range v {
			if !terrainFields(field) {
				return false
			}
		}
	}
	return true
}

func (d terrainDocument) terrainAccessor(data []byte, index, component, target int, kind string, width int) ([]byte, int) {
	if index < 0 || index >= len(d.Accessors) {
		return nil, 0
	}
	a := d.Accessors[index]
	if a.ComponentType != component || a.Type != kind || a.Count < 1 || a.Count > len(data)/width ||
		a.BufferView < 0 || a.BufferView >= len(d.BufferViews) {
		return nil, 0
	}
	v := d.BufferViews[a.BufferView]
	if v.Buffer != 0 || v.Target != target || v.ByteOffset < 0 || v.ByteOffset%4 != 0 ||
		v.ByteLength != a.Count*width || v.ByteOffset > len(data)-v.ByteLength {
		return nil, 0
	}
	return data[v.ByteOffset : v.ByteOffset+v.ByteLength], a.Count
}

func boundedFloats(data []byte, limit float64) bool {
	for i := 0; i < len(data); i += 4 {
		value := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i:])))
		if !(math.Abs(value) <= limit) {
			return false
		}
	}
	return true
}
