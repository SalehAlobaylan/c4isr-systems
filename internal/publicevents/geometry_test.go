package publicevents

import "testing"

func TestStudyGeometryRejectsAmbiguousAndUnboundedAreas(t *testing.T) {
	for _, raw := range []string{
		`{"type":"Polygon","coordinates":[[[0,0],[1,1],[1,0],[0,1],[0,0]]]}`,
		`{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1]]]}`,
		`{"type":"Polygon","coordinates":[[[0,0],[40,0],[40,40],[0,40],[0,0]]]}`,
		`{"type":"Polygon","coordinates":[[[0,0],[1,1],[2,2],[0,0]]]}`,
		`{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[1,0],[0,0]]]}`,
		`{"type":"Point","coordinates":[46,24]}`,
		`{"type":"Polygon","crs":{},"coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}`,
	} {
		if ValidateStudyGeometry(raw, false) == nil {
			t.Errorf("invalid analyst polygon accepted: %s", raw)
		}
	}
	for _, raw := range []string{`{"type":"Polygon","coordinates":[[[46,24],[47,24],[46,25],[46,24]]]}`, `{"type":"Polygon","coordinates":[[[46,24],[46,25],[47,24],[46,24]]]}`} {
		if e := ValidateStudyGeometry(raw, false); e != nil {
			t.Error(e)
		}
	}
	line := `{"type":"LineString","coordinates":[[46,24],[47,24]]}`
	if ValidateStudyGeometry(line, false) == nil || ValidateStudyGeometry(line, true) != nil {
		t.Fatal("measurement line confused with geographic area")
	}
}
