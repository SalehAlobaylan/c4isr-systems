package contextpackage

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func fixture(t *testing.T) Bundle {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/context-packages/hand-authored.json")
	if err != nil {
		t.Fatal(err)
	}
	var b Bundle
	if err = json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	return b
}
func TestRetainedFixtureOfflineIntegrityAndImmutableDependencies(t *testing.T) {
	original := fixture(t)
	if err := original.Validate(time.Now(), true); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Bundle){
		"corrupt_bytes": func(b *Bundle) {
			for k, v := range b.Assets {
				v[0] ^= 1
				b.Assets[k] = v
				break
			}
		},
		"missing_asset":          func(b *Bundle) { b.Assets = map[string][]byte{} },
		"changed_source_version": func(b *Bundle) { b.Manifest.Assets[0].Version = "new upstream version" },
		"unapproved":             func(b *Bundle) { b.Manifest.Approval.By = ""; b.ID = ID(b.Manifest) },
		"changed_rights":         func(b *Bundle) { b.Manifest.Assets[0].Rights.AllowSimulationExport = false; b.ID = ID(b.Manifest) },
		"expired_rights": func(b *Bundle) {
			past := time.Now().Add(-time.Hour)
			b.Manifest.Assets[0].Rights.ExpiresAt = &past
			b.ID = ID(b.Manifest)
		},
		"wrong_transform": func(b *Bundle) { b.Manifest.TransformVersion = "unversioned"; b.ID = ID(b.Manifest) },
		"unknown_crs":     func(b *Bundle) { b.Manifest.Assets[0].CRS = "EPSG:3857"; b.ID = ID(b.Manifest) },
		"extra_asset":     func(b *Bundle) { b.Assets["unlisted"] = []byte("x") },
	} {
		t.Run(name, func(t *testing.T) {
			b := fixture(t)
			mutate(&b)
			if b.Validate(time.Now(), true) == nil {
				t.Fatal("invalid package accepted")
			}
		})
	}
	if err := original.Validate(time.Now(), true); err != nil {
		t.Fatalf("mutation affected frozen fixture: %v", err)
	}
}
func TestReferenceOnlyDeclaresWeakerReproduction(t *testing.T) {
	b := fixture(t)
	b.Assets = map[string][]byte{}
	b.Manifest.Assets[0].Retention = "reference_only"
	b.Manifest.Reproduction = "external_dependencies"
	b.ID = ID(b.Manifest)
	if err := b.Validate(time.Now(), false); err != nil {
		t.Fatal(err)
	}
	if b.Validate(time.Now(), true) == nil {
		t.Fatal("external dependency claimed offline reproduction")
	}
}
