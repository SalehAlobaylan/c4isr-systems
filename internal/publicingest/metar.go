package publicingest

import (
	"encoding/json"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const METARAdapter = "noaa-metar-v1"
const METARURL = "https://aviationweather.gov/api/data/metar"
const METARDataset = "noaa-awc-metar"

func metarURL(in FetchRequest) string {
	u, _ := url.Parse(METARURL)
	q := u.Query()
	b := in.Admission.Policy.AOI
	q.Set("bbox", fmt.Sprintf("%g,%g,%g,%g", b.South, b.West, b.North, b.East))
	q.Set("format", "json")
	q.Set("hours", strconv.Itoa(int(math.Ceil(float64(in.Admission.Policy.LookbackSeconds)/3600))))
	u.RawQuery = q.Encode()
	return u.String()
}
func DecodeMETAR(raw []byte, in FetchRequest) (Page, error) {
	fail := func() (Page, error) { return Page{}, &RetrievalError{Code: "invalid_metar_response"} }
	var rows []json.RawMessage
	if len(raw) > in.Admission.Policy.MaxResponseBytes || json.Unmarshal(raw, &rows) != nil || rows == nil || len(rows) > in.Admission.Policy.MaxRecordsPerPage {
		return fail()
	}
	p := Page{Records: []Record{}, CoverageFrom: in.From, CoverageTo: in.To}
	if len(rows) == 400 {
		p.PartialReason = "provider_result_limit"
	}
	for _, rawRow := range rows {
		var r struct {
			ICAO     string `json:"icaoId"`
			Time     int64  `json:"obsTime"`
			Type     string `json:"metarType"`
			Raw      string `json:"rawOb"`
			Lat, Lon *float64
			Temp     *float64        `json:"temp"`
			Dewp     *float64        `json:"dewp"`
			Wind     json.RawMessage `json:"wdir"`
			Speed    *float64        `json:"wspd"`
			Gust     *float64        `json:"wgst"`
			Vis      json.RawMessage `json:"visib"`
			Alt      *float64        `json:"altim"`
		}
		if json.Unmarshal(rawRow, &r) != nil {
			return fail()
		}
		if r.Type != "METAR" && r.Type != "SPECI" {
			p.ExcludedRecords++
			continue
		}
		if r.Lat == nil || r.Lon == nil {
			p.ExcludedRecords++
			continue
		}
		at := time.Unix(r.Time, 0).UTC()
		point := geo.Point{Lng: *r.Lon, Lat: *r.Lat}
		if r.Time <= 0 || !point.Valid() {
			return fail()
		}
		if !within(in.Admission.Policy.AOI, point) || at.Before(in.From) || at.After(in.To) {
			p.ExcludedRecords++
			continue
		}
		w := StationWeather{StationID: r.ICAO, RawMETAR: r.Raw, ReportType: r.Type, TemperatureC: r.Temp, DewpointC: r.Dewp, AltimeterHPa: r.Alt}
		convert := func(v *float64) *float64 {
			if v == nil {
				return nil
			}
			n := *v * 0.5144444444444445
			return &n
		}
		w.WindSpeedMPS = convert(r.Speed)
		w.WindGustMPS = convert(r.Gust)
		if len(r.Wind) > 0 && string(r.Wind) != "null" {
			if string(r.Wind) == `"VRB"` {
				w.VariableWind = true
			} else {
				var n float64
				if json.Unmarshal(r.Wind, &n) != nil {
					return fail()
				}
				w.WindDirectionDegrees = &n
			}
		}
		if len(r.Vis) > 0 && string(r.Vis) != "null" {
			var n float64
			if json.Unmarshal(r.Vis, &n) != nil {
				var v string
				if json.Unmarshal(r.Vis, &v) != nil {
					return fail()
				}
				w.VisibilityLowerBound = strings.HasSuffix(v, "+")
				var e error
				n, e = strconv.ParseFloat(strings.TrimSuffix(v, "+"), 64)
				if e != nil {
					return fail()
				}
			}
			w.VisibilityStatuteMiles = &n
		}
		c := ContextRecord{Kind: "WEATHER", Title: r.ICAO + " station weather", Position: point, LocationBasis: "Reporting station point; no area or route coverage is implied", TimeBasis: "Provider obsTime Unix seconds UTC; receipt time is not observation time", From: at, To: at, ExtractionMethod: "noaa-awc-metar-json-v1", Uncertainty: "Station sample only. Missing values remain unknown; variable wind has no fixed bearing; visibility may be a lower bound. No interpolation, forecast or operational clearance.", Weather: &w}
		if c.Validate(in.Now) != nil {
			return fail()
		}
		p.Records = append(p.Records, Record{ProviderRecordID: fmt.Sprintf("%s:%d:%s", r.ICAO, r.Time, r.Type), ObservedAt: &at, ReferenceURL: METARURL, OriginalRecord: rawRow, Context: &c})
	}
	return p, nil
}
