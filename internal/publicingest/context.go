package publicingest

import (
	"math"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

// ContextRecord describes a station sample or a provider claim, never an actor,
// trajectory, spatial coverage assertion or analyst assessment.
type ContextRecord struct {
	Kind             string          `json:"kind"`
	Title            string          `json:"title"`
	Position         geo.Point       `json:"position"`
	LocationBasis    string          `json:"locationBasis"`
	TimeBasis        string          `json:"timeBasis"`
	From             time.Time       `json:"from"`
	To               time.Time       `json:"to"`
	ExtractionMethod string          `json:"extractionMethod"`
	Uncertainty      string          `json:"uncertainty"`
	Weather          *StationWeather `json:"weather,omitempty"`
	Report           *SourceReport   `json:"report,omitempty"`
}
type StationWeather struct {
	StationID              string   `json:"stationId"`
	RawMETAR               string   `json:"rawMetar"`
	ReportType             string   `json:"reportType"`
	TemperatureC           *float64 `json:"temperatureC"`
	DewpointC              *float64 `json:"dewpointC"`
	WindDirectionDegrees   *float64 `json:"windDirectionDegrees"`
	VariableWind           bool     `json:"variableWind"`
	WindSpeedMPS           *float64 `json:"windSpeedMps"`
	WindGustMPS            *float64 `json:"windGustMps"`
	VisibilityStatuteMiles *float64 `json:"visibilityStatuteMiles"`
	VisibilityLowerBound   bool     `json:"visibilityLowerBound"`
	AltimeterHPa           *float64 `json:"altimeterHPa"`
}
type SourceReport struct {
	Claim                string   `json:"claim"`
	Country              string   `json:"country"`
	ProviderAlertLevel   string   `json:"providerAlertLevel"`
	CitedSource          string   `json:"citedSource"`
	CitedSourceID        string   `json:"citedSourceId"`
	CitedFrom            string   `json:"citedFrom"`
	CitedTo              string   `json:"citedTo"`
	DocumentURL          string   `json:"documentUrl"`
	LineageKeys          []string `json:"lineageKeys"`
	CorroborationMeaning string   `json:"corroborationMeaning"`
}

func (v ContextRecord) Validate(now time.Time) error {
	bad := func() error { return apperr.Validation("invalid weather/report context") }
	if !v.Position.Valid() || v.From.IsZero() || v.To.Before(v.From) || v.To.After(now.Add(5*time.Minute)) || len(v.Title) < 1 || len(v.Title) > 1000 || len(v.Uncertainty) > 4000 {
		return bad()
	}
	if v.LocationBasis == "" || v.TimeBasis == "" || v.ExtractionMethod == "" {
		return bad()
	}
	switch v.Kind {
	case "WEATHER":
		if v.Weather == nil || v.Report != nil {
			return bad()
		}
		w := v.Weather
		if len(w.StationID) < 3 || len(w.StationID) > 8 || len(w.RawMETAR) > 4096 || w.RawMETAR == "" || (w.ReportType != "METAR" && w.ReportType != "SPECI") {
			return bad()
		}
		for _, n := range []*float64{w.TemperatureC, w.DewpointC, w.WindDirectionDegrees, w.WindSpeedMPS, w.WindGustMPS, w.VisibilityStatuteMiles, w.AltimeterHPa} {
			if n != nil && (math.IsNaN(*n) || math.IsInf(*n, 0)) {
				return bad()
			}
		}
		if w.WindDirectionDegrees != nil && (*w.WindDirectionDegrees < 0 || *w.WindDirectionDegrees > 360 || w.VariableWind) {
			return bad()
		}
		for _, n := range []*float64{w.WindSpeedMPS, w.WindGustMPS, w.VisibilityStatuteMiles, w.AltimeterHPa} {
			if n != nil && *n < 0 {
				return bad()
			}
		}
	case "REPORT":
		if v.Report == nil || v.Weather != nil {
			return bad()
		}
		r := v.Report
		if len(strings.TrimSpace(r.Claim)) < 1 || len(r.Claim) > 8000 || len(r.LineageKeys) < 1 || len(r.LineageKeys) > 8 || len(r.DocumentURL) > 2048 || r.CorroborationMeaning == "" {
			return bad()
		}
	default:
		return bad()
	}
	return nil
}
