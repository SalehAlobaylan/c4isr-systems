package modelassets

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

// Images are decoded only in the independent import worker, never in API paths.
// Bound compressed bytes, declared dimensions and aggregate decoded pixels first.
func validateTextures(ctx context.Context, raw []byte, media string) error {
	if len(raw) > MaxBytes {
		return fmt.Errorf("image_source_budget")
	}
	data := raw
	var bin []byte
	if media == "model/gltf-binary" {
		if len(raw) < 20 {
			return fmt.Errorf("image_glb_header")
		}
		n := int(binary.LittleEndian.Uint32(raw[12:]))
		if n < 0 || n > len(raw)-20 {
			return fmt.Errorf("image_glb_range")
		}
		data = raw[20 : 20+n]
		offset := 20 + n
		if offset < len(raw) {
			if offset+8 > len(raw) {
				return fmt.Errorf("image_glb_range")
			}
			size := int(binary.LittleEndian.Uint32(raw[offset:]))
			if size < 0 || size > len(raw)-offset-8 {
				return fmt.Errorf("image_glb_range")
			}
			bin = raw[offset+8 : offset+8+size]
		}
	}
	var doc struct {
		Buffers []struct {
			URI string `json:"uri"`
		}
		Views  []struct{ Buffer, ByteOffset, ByteLength int } `json:"bufferViews"`
		Images []struct {
			URI  string `json:"uri"`
			View *int   `json:"bufferView"`
		}
	}
	if len(data) > 4*1024*1024 || json.Unmarshal(data, &doc) != nil || len(doc.Buffers) > 4 || len(doc.Images) > 16 {
		return fmt.Errorf("image_document_budget")
	}
	if len(doc.Images) == 0 {
		return ctx.Err()
	}
	decode := func(uri string) ([]byte, error) {
		parts := strings.SplitN(uri, ",", 2)
		if len(parts) != 2 || !strings.HasPrefix(parts[0], "data:") || !strings.HasSuffix(parts[0], ";base64") || len(parts[1]) > ((MaxBytes+2)/3)*4 {
			return nil, fmt.Errorf("image_uri_budget")
		}
		return base64.StdEncoding.Strict().DecodeString(parts[1])
	}
	buffers := [][]byte{}
	total := 0
	for i, b := range doc.Buffers {
		var buf []byte
		var e error
		if i == 0 && b.URI == "" {
			buf = bin
		} else {
			buf, e = decode(b.URI)
		}
		if e != nil {
			return e
		}
		total += len(buf)
		if total > MaxBytes {
			return fmt.Errorf("image_buffer_budget")
		}
		buffers = append(buffers, buf)
	}
	pixels := int64(0)
	for _, entry := range doc.Images {
		if e := ctx.Err(); e != nil {
			return e
		}
		var b []byte
		var e error
		if entry.URI != "" {
			b, e = decode(entry.URI)
		} else {
			if entry.View == nil || *entry.View < 0 || *entry.View >= len(doc.Views) {
				return fmt.Errorf("image_view_range")
			}
			v := doc.Views[*entry.View]
			if v.Buffer < 0 || v.Buffer >= len(buffers) || v.ByteOffset < 0 || v.ByteLength < 1 || v.ByteOffset > len(buffers[v.Buffer])-v.ByteLength {
				return fmt.Errorf("image_buffer_range")
			}
			b = buffers[v.Buffer][v.ByteOffset : v.ByteOffset+v.ByteLength]
		}
		if e != nil {
			return e
		}
		if len(b) > MaxBytes {
			return fmt.Errorf("image_encoded_budget")
		}
		config, format, e := image.DecodeConfig(bytes.NewReader(b))
		if e != nil || format != "png" && format != "jpeg" || config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 {
			return fmt.Errorf("image_dimensions_or_format")
		}
		pixels += int64(config.Width) * int64(config.Height)
		if pixels > 16*1024*1024 {
			return fmt.Errorf("image_decoded_pixel_budget")
		}
		decoded, _, e := image.Decode(bytes.NewReader(b))
		if e != nil || decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
			return fmt.Errorf("invalid_image_payload")
		}
		if e := ctx.Err(); e != nil {
			return e
		}
	}
	return nil
}
