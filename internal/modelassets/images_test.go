package modelassets

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"testing"
)

func TestTexturePayloadIntegrityAndCanceledProcessing(t *testing.T) {
	var b bytes.Buffer
	if e := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); e != nil {
		t.Fatal(e)
	}
	makeDoc := func(raw []byte) []byte {
		doc, _ := json.Marshal(map[string]any{"images": []any{map[string]any{"uri": "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)}}})
		return doc
	}
	if e := validateTextures(context.Background(), makeDoc(b.Bytes()), "model/gltf+json"); e != nil {
		t.Fatal(e)
	}
	corrupt := append([]byte(nil), b.Bytes()...)
	corrupt[len(corrupt)-1] ^= 1
	if e := validateTextures(context.Background(), makeDoc(corrupt), "model/gltf+json"); e == nil {
		t.Fatal("corrupt PNG accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := validateTextures(ctx, makeDoc(b.Bytes()), "model/gltf+json"); e == nil {
		t.Fatal("canceled image processing continued")
	}
}
