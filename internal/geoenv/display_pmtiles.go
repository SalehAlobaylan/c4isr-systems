package geoenv

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
)

// The bounded street profile uses a gzip root, metadata and vector tiles.
// Every tile is checked before a scoped browser can open the archive.
func validStreetArchive(raw []byte, bounds [4]float64) bool {
	if len(raw) < 127 || string(raw[:7]) != "PMTiles" || raw[7] != 3 ||
		raw[96] != 1 || raw[97] != 2 || raw[98] != 2 || raw[99] != 1 || raw[100] != 12 || raw[101] != 16 {
		return false
	}
	for i, edge := range bounds {
		actual := float64(int32(binary.LittleEndian.Uint32(raw[102+4*i:]))) / 1e7
		if math.Abs(actual-edge) > 1e-6 {
			return false
		}
	}
	var sizes [8]uint64
	for i := range sizes {
		sizes[i] = binary.LittleEndian.Uint64(raw[8+8*i:])
		if sizes[i] > uint64(len(raw)) {
			return false
		}
	}
	root, rootLen, meta, metaLen, leaves, leavesLen, tiles, tilesLen := sizes[0], sizes[1], sizes[2], sizes[3], sizes[4], sizes[5], sizes[6], sizes[7]
	if root != 127 || rootLen == 0 || meta != root+rootLen || meta > 16384 || metaLen == 0 || leaves != meta+metaLen ||
		leavesLen != 0 || tiles != leaves || tilesLen == 0 || tiles+tilesLen != uint64(len(raw)) {
		return false
	}
	metadata, ok := inflateDisplay(raw[meta:leaves], 512<<10)
	var document struct {
		Type, Format string
		Layers       []struct {
			ID     string
			Fields map[string]string
		} `json:"vector_layers"`
	}
	if !ok || json.Unmarshal(metadata, &document) != nil || document.Type != "baselayer" || document.Format != "pbf" || len(document.Layers) != 3 {
		return false
	}
	seen := map[string]bool{}
	for _, layer := range document.Layers {
		if (layer.ID != "roads" && layer.ID != "buildings" && layer.ID != "water") || seen[layer.ID] || layer.Fields["source_id"] != "String" {
			return false
		}
		seen[layer.ID] = true
	}
	directory, ok := inflateDisplay(raw[root:meta], 256<<10)
	if !ok {
		return false
	}
	r := bytes.NewReader(directory)
	count, err := binary.ReadUvarint(r)
	if err != nil || count == 0 || count > 4096 || count != binary.LittleEndian.Uint64(raw[80:88]) {
		return false
	}
	var lastID, addressed uint64
	for i := range count {
		delta, err := binary.ReadUvarint(r)
		if err != nil || delta >= 5726623061 || delta == 0 && i > 0 || lastID+delta >= 5726623061 || lastID+delta < 5592405 {
			return false
		}
		lastID += delta
	}
	for range count {
		run, err := binary.ReadUvarint(r)
		if err != nil || run == 0 || run > 4096 || addressed+run > 4096 {
			return false
		}
		addressed += run
	}
	if addressed != binary.LittleEndian.Uint64(raw[72:80]) {
		return false
	}
	lengths := make([]uint64, count)
	for i := range lengths {
		length, err := binary.ReadUvarint(r)
		if err != nil || length == 0 || length > 1000000 {
			return false
		}
		lengths[i] = length
	}
	var end uint64
	var total int
	verified := make(map[[2]uint64]bool)
	for i, length := range lengths {
		encoded, err := binary.ReadUvarint(r)
		if err != nil || encoded == 0 && i == 0 || encoded > tilesLen+1 {
			return false
		}
		offset := end
		if encoded != 0 {
			offset = encoded - 1
		}
		end = offset + length
		if end > tilesLen {
			return false
		}
		key := [2]uint64{offset, end}
		if !verified[key] {
			data, ok := inflateDisplay(raw[tiles+offset:tiles+end], 4<<20)
			total += len(data)
			if !ok || len(data) == 0 || data[0] != 0x1a || total > 128<<20 {
				return false
			}
			verified[key] = true
		}
	}
	return r.Len() == 0 && uint64(len(verified)) == binary.LittleEndian.Uint64(raw[88:96])
}

func inflateDisplay(raw []byte, maxBytes int64) ([]byte, bool) {
	r, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, false
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	return data, err == nil && int64(len(data)) <= maxBytes
}
