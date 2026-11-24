package coverage

import (
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

func TestDeriveFootprintNameAndUnionBounds(t *testing.T) {
	jeddah := publicingest.Bounds{West: 39.144, South: 21.464, East: 39.216, North: 21.536}
	saudi := publicingest.Bounds{West: 34.0, South: 16.0, East: 56.0, North: 33.0}

	if got := deriveFootprintName(jeddah, []string{"Jeddah surface elevation · Copernicus DEM", "Jeddah roads and buildings · OpenStreetMap"}); got != "Jeddah corridor" {
		t.Fatalf("expected Jeddah corridor, got %q", got)
	}
	if got := deriveFootprintName(saudi, []string{"USGS earthquakes · Saudi region", "Official station weather · Saudi region"}); got != "Saudi region" {
		t.Fatalf("expected Saudi region, got %q", got)
	}
	if got := deriveFootprintName(publicingest.Bounds{West: 2.0, South: 48.0, East: 3.0, North: 49.0}, []string{"Fixture source"}); got != "Fixture source" {
		t.Fatalf("expected single source fallback name, got %q", got)
	}

	u := unionBounds(&jeddah, &saudi)
	if u == nil || u.West != 34.0 || u.South != 16.0 || u.East != 56.0 || u.North != 33.0 {
		t.Fatalf("unexpected union bounds: %+v", u)
	}
	if footprintKey(jeddah) != "39.14,21.46,39.22,21.54" {
		t.Fatalf("unexpected footprint key: %s", footprintKey(jeddah))
	}
}
