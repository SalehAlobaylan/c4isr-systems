package publicingest

import (
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"math"
	"regexp"
	"time"
)

type MovingObservation struct {
	Kind          string         `json:"kind"`
	IdentityClaim string         `json:"identityClaim"`
	ObservedAt    time.Time      `json:"observedAt"`
	Position      geo.Point      `json:"position"`
	SpeedMPS      *float64       `json:"speedMps"`
	Course        *float64       `json:"course"`
	Claims        map[string]any `json:"claims"`
	Quality       map[string]any `json:"quality"`
}

var aircraftIdentity = regexp.MustCompile(`^[0-9a-f]{6}$`)
var vesselIdentity = regexp.MustCompile(`^[1-9][0-9]{8}$`)

func (o MovingObservation) Validate(now time.Time) error {
	raw, err := json.Marshal(o)
	if err != nil || len(raw) > 16384 || !o.Position.Valid() || o.ObservedAt.IsZero() || o.ObservedAt.After(now.Add(5*time.Second)) || (o.Kind != "aircraft" && o.Kind != "vessel") {
		return apperr.Validation("invalid moving provider observation")
	}
	if (o.Kind == "aircraft" && !aircraftIdentity.MatchString(o.IdentityClaim)) || (o.Kind == "vessel" && !vesselIdentity.MatchString(o.IdentityClaim)) {
		return apperr.Validation("invalid claimed external identity")
	}
	if o.SpeedMPS != nil && (math.IsNaN(*o.SpeedMPS) || math.IsInf(*o.SpeedMPS, 0) || *o.SpeedMPS < 0 || *o.SpeedMPS > 2000) {
		return apperr.Validation("invalid reported speed")
	}
	if o.Course != nil && (math.IsNaN(*o.Course) || math.IsInf(*o.Course, 0) || *o.Course < 0 || *o.Course >= 360) {
		return apperr.Validation("invalid reported course")
	}
	return nil
}
func within(b Bounds, p geo.Point) bool {
	return p.Lng >= b.West && p.Lng <= b.East && p.Lat >= b.South && p.Lat <= b.North
}
