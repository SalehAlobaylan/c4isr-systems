package jsoncanon

import "testing"

func TestJSONFingerprintSurvivesObjectAndDecimalRepresentation(t *testing.T) {
	a, e := Canonical([]byte(`{"type":"Polygon","coordinates":[21.50,0.000001,-0,9007199254740993]}`))
	if e != nil {
		t.Fatal(e)
	}
	b, e := Canonical([]byte(`{"coordinates":[2.150e1,1e-6,0.0,9007199254740993.0],"type":"Polygon"}`))
	if e != nil || string(a) != string(b) {
		t.Fatal("equivalent JSON changed identity", string(a), string(b), e)
	}
	c, e := Canonical([]byte(`{"coordinates":[2.150e1,1e-6,0.0,9007199254740992],"type":"Polygon"}`))
	if e != nil || string(c) == string(b) {
		t.Fatal("large distinct integers collapsed", e)
	}
	huge, e := Canonical([]byte(`1.00e999999999999999999999999999999999999`))
	if e != nil || len(huge) > 50 {
		t.Fatal("exponent expanded or overflowed", string(huge), e)
	}
	if _, e = Canonical([]byte(`{} {}`)); e == nil {
		t.Fatal("multiple JSON values accepted")
	}
}
