package investigations

import (
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"testing"
)

func TestExplicitPurposeRightsFailClosed(t *testing.T) {
	yes, no := true, false
	legacy := publicingest.Rights{RetainRaw: true, AllowAnalysis: true, AllowSimulationExport: true}
	for _, use := range []string{"reference", "internal", "imagery", "raw", "environment"} {
		if outputAllowed(legacy, use) {
			t.Fatalf("legacy simulation flag authorized %s", use)
		}
	}
	original := &publicingest.OutputPermissions{Citation: &yes, InternalAnalysis: &yes, IncludedImagery: &yes, RawDistribution: &yes, GeographicEnvironment: &yes, MetadataRetention: &yes}
	current := &publicingest.OutputPermissions{Citation: &yes, InternalAnalysis: &yes, IncludedImagery: &no, RawDistribution: &no, GeographicEnvironment: &yes, MetadataRetention: &yes}
	legacy.OutputPermissions = publicingest.IntersectOutputPermissions(original, current)
	for _, use := range []string{"reference", "internal", "environment"} {
		if !outputAllowed(legacy, use) {
			t.Fatalf("explicit permission rejected %s", use)
		}
	}
	for _, use := range []string{"imagery", "raw", "unknown"} {
		if outputAllowed(legacy, use) {
			t.Fatalf("denied permission allowed %s", use)
		}
	}
	legacy.OutputPermissions = publicingest.IntersectOutputPermissions(nil, current)
	if outputAllowed(legacy, "reference") {
		t.Fatal("current rights widened unknown original rights")
	}
	legacy.OutputPermissions = &publicingest.OutputPermissions{Citation: &yes, MetadataRetention: &no}
	if outputAllowed(legacy, "reference") {
		t.Fatal("metadata retention bypassed")
	}
}
