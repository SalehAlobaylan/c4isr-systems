package artifacts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGeographicStreamingRoundTripBeyondLegacyLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw := bytes.Repeat([]byte("reference-data"), MaxBytes/14+32768)
	digest := Hash(raw)
	var cfg S3Config
	if os.Getenv("EMAD_GEOGRAPHIC_TEST_REAL_S3") == "1" {
		cfg = S3Config{Endpoint: "http://127.0.0.1:3900", Bucket: os.Getenv("GARAGE_DEFAULT_BUCKET"),
			Region: "garage", AccessKey: os.Getenv("GARAGE_DEFAULT_ACCESS_KEY"), SecretKey: os.Getenv("GARAGE_DEFAULT_SECRET_KEY"), AllowHTTP: true}
	} else {
		var retained []byte
		var lock sync.Mutex
		fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lock.Lock()
			defer lock.Unlock()
			switch r.Method {
			case "PUT":
				if r.ContentLength != int64(len(raw)) || r.Header.Get("X-Amz-Content-Sha256") != digest {
					http.Error(w, "wrong content identity", 400)
					return
				}
				var err error
				retained, err = io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "read", 400)
					return
				}
				w.WriteHeader(200)
			case "GET":
				if retained == nil {
					w.WriteHeader(404)
					return
				}
				if value := r.Header.Get("Range"); value != "" {
					if value != "bytes=3-18" {
						http.Error(w, "wrong range", 416)
						return
					}
					w.Header().Set("Content-Range", fmt.Sprintf("bytes 3-18/%d", len(retained)))
					w.Header().Set("Content-Length", "16")
					w.WriteHeader(206)
					_, _ = w.Write(retained[3:19])
					return
				}
				w.Header().Set("Content-Length", fmt.Sprint(len(retained)))
				_, _ = w.Write(retained)
			case "DELETE":
				retained = nil
				w.WriteHeader(204)
			}
		}))
		defer fake.Close()
		cfg = S3Config{Endpoint: fake.URL, Bucket: "geo-test", AccessKey: "fixture", SecretKey: "fixture", AllowHTTP: true}
	}
	store, err := NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	testID := Hash([]byte(t.Name() + time.Now().String()))
	key := "emad/geographic/v1/" + testID + "/geoasset_" + testID + "/" + digest
	defer func() {
		if err := store.Delete(context.Background(), key); err != nil {
			t.Error(err)
		}
	}()
	if err = store.Put(ctx, key, raw, "image/tiff"); err == nil {
		t.Fatal("legacy object path exceeded its original limit")
	}
	bad := bytes.NewReader([]byte(strings.Repeat("a", 64)))
	if err = store.PutGeographic(ctx, key, bad, int64(len(raw)), digest, "image/tiff"); err == nil {
		t.Fatal("short source passed streaming verification")
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err = store.PutGeographic(ctx, key, bytes.NewReader(raw), int64(len(raw)), digest, "image/tiff"); err != nil {
			t.Fatal(err)
		}
	}
	read, err := store.OpenGeographic(ctx, key, int64(len(raw)), &ByteRange{Start: 3, End: 18})
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(read)
	read.Close()
	if err != nil || !bytes.Equal(got, raw[3:19]) {
		t.Fatal("range bytes differ", err)
	}
}
