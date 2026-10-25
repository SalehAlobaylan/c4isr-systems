package modelassets

import (
	"strings"
	"testing"
)

func TestModelAdmissionRequiresArchiveCompatibleRightsAndPortablePurpose(t *testing.T) {
	in := UploadInput{SourceVersion: "v1", RequestKey: "test", Name: "Synthetic model", Kind: "MODEL", SHA256: strings.Repeat("a", 64), Bytes: 100, MediaType: "model/gltf-binary", SourceReference: "https://example.invalid/model", Provenance: "Own synthetic geometry", License: License{ID: "own", LicenseID: "LicenseRef-EMAD-TestFixture", TermsURL: "https://example.invalid/terms", Attribution: "Synthetic fixture", LegalBasis: "Own project-authored geometry", AllowStorage: true, AllowSimulation: true, ArchivePermission: "perpetual"}}
	if e := in.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*UploadInput){func(v *UploadInput) { v.Bytes = MaxBytes + 1 }, func(v *UploadInput) { v.SourceVersion = " " }, func(v *UploadInput) { v.SourceReference = "http://example.invalid/model" }, func(v *UploadInput) { v.License.AllowSimulation = false }, func(v *UploadInput) { v.License.ArchivePermission = "unknown" }, func(v *UploadInput) { v.MediaType = "application/zip" }, func(v *UploadInput) { v.ParentVersionID = "unsupported-parent" }} {
		bad := in
		change(&bad)
		if e := bad.Validate(); e == nil {
			t.Fatal("invalid admission accepted", bad)
		}
	}
}
func TestValidatorOutputCannotExceedBound(t *testing.T) {
	b := &cappedBuffer{limit: 4}
	if _, e := b.Write([]byte("valid")); e == nil || b.Len() != 0 {
		t.Fatal("oversized process output retained")
	}
}
