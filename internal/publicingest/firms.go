package publicingest

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

const FIRMSAdapter = "firms-viirs-noaa20-v1"
const FIRMSAreaURL = "https://firms.modaps.eosdis.nasa.gov/api/area/csv/"
const FIRMSDataset = "VIIRS_NOAA20_NRT"
const FIRMSProductVersion = "2.0NRT"

var firmsKey = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)
var csvName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

type firmsCursor struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Day      time.Time `json:"day"`
	Complete bool      `json:"complete"`
}

func firmsWindow(in FetchRequest) (firmsCursor, error) {
	c := firmsCursor{From: in.From, To: in.To, Day: in.From.UTC().Truncate(24 * time.Hour)}
	if in.From.IsZero() || in.To.Before(in.From) || in.To.Sub(in.From) > 10*24*time.Hour {
		return c, &RetrievalError{Code: "invalid_response"}
	}
	if in.Checkpoint != "" {
		var old firmsCursor
		if json.Unmarshal([]byte(in.Checkpoint), &old) != nil {
			return c, &RetrievalError{Code: "invalid_response"}
		}
		if !old.Complete {
			if !old.From.Equal(in.From) || !old.To.Equal(in.To) || old.Day.Before(c.Day) || old.Day.After(in.To) || !old.Day.Equal(old.Day.UTC().Truncate(24*time.Hour)) {
				return c, &RetrievalError{Code: "invalid_response"}
			}
			c = old
		}
	}
	return c, nil
}
func firmsURL(location Location, in FetchRequest) (string, error) {
	if location.URL != FIRMSAreaURL || !firmsKey.MatchString(location.token) {
		return "", &RetrievalError{Code: "invalid_location"}
	}
	c, err := firmsWindow(in)
	if err != nil {
		return "", err
	}
	b := in.Admission.Policy.AOI
	bbox := strings.Join([]string{strconv.FormatFloat(b.West, 'f', -1, 64), strconv.FormatFloat(b.South, 'f', -1, 64), strconv.FormatFloat(b.East, 'f', -1, 64), strconv.FormatFloat(b.North, 'f', -1, 64)}, ",")
	return FIRMSAreaURL + url.PathEscape(location.token) + "/" + FIRMSDataset + "/" + bbox + "/1/" + c.Day.Format("2006-01-02"), nil
}
func DecodeFIRMS(raw []byte, in FetchRequest) (Page, error) {
	if in.Admission.Policy.MaxResponseBytes > 0 && len(raw) > in.Admission.Policy.MaxResponseBytes {
		return Page{}, &RetrievalError{Code: "response_too_large"}
	}
	fail := func() (Page, error) { return Page{}, &RetrievalError{Code: "invalid_response"} }
	if len(raw) < 100 && strings.Contains(strings.ToLower(string(raw)), "invalid map_key") {
		return Page{}, &RetrievalError{Code: "provider_auth"}
	}
	if in.Admission.Dataset != FIRMSDataset || in.Admission.ProductVersion != FIRMSProductVersion || in.Admission.RevisionPolicy != "content_hash" {
		return fail()
	}
	c, err := firmsWindow(in)
	if err != nil {
		return Page{}, err
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil || len(header) < 13 || len(header) > 24 {
		return fail()
	}
	endHeader := reader.InputOffset()
	names := map[string]int{}
	for i, key := range header {
		if !csvName.MatchString(key) {
			return fail()
		}
		if _, ok := names[key]; ok {
			return fail()
		}
		names[key] = i
	}
	for _, key := range []string{"latitude", "longitude", "bright_ti4", "scan", "track", "acq_date", "acq_time", "satellite", "confidence", "version", "bright_ti5", "frp", "daynight"} {
		if _, ok := names[key]; !ok {
			return fail()
		}
	}
	reader.FieldsPerRecord = len(header)
	page := Page{Records: []Record{}, CoverageFrom: in.From, CoverageTo: in.To}
	rows := 0
	for {
		start := reader.InputOffset()
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fail()
		}
		rows++
		if rows > in.Admission.Policy.MaxRecordsPerPage {
			return Page{}, &RetrievalError{Code: "response_too_large"}
		}
		fields := map[string]string{}
		for i, key := range header {
			fields[key] = row[i]
		}
		get := func(key string) string { return fields[key] }
		parse := func(key string, min, max float64) (float64, bool) {
			v, e := strconv.ParseFloat(get(key), 64)
			return v, e == nil && !math.IsNaN(v) && !math.IsInf(v, 0) && v >= min && v <= max
		}
		lat, ok := parse("latitude", -90, 90)
		if !ok {
			return fail()
		}
		lng, ok := parse("longitude", -180, 180)
		if !ok {
			return fail()
		}
		scan, ok := parse("scan", 0.01, 10)
		if !ok {
			return fail()
		}
		track, ok := parse("track", 0.01, 10)
		if !ok {
			return fail()
		}
		ti4, ok := parse("bright_ti4", 0, 1000)
		if !ok {
			return fail()
		}
		ti5, ok := parse("bright_ti5", 0, 1000)
		if !ok {
			return fail()
		}
		frp, ok := parse("frp", 0, 1000000)
		if !ok {
			return fail()
		}
		clock := get("acq_time")
		if len(clock) < 1 || len(clock) > 4 {
			return fail()
		}
		for _, ch := range clock {
			if ch < '0' || ch > '9' {
				return fail()
			}
		}
		clock = strings.Repeat("0", 4-len(clock)) + clock
		acquired, err := time.Parse("2006-01-02 1504", get("acq_date")+" "+clock)
		if err != nil || !acquired.Equal(acquired.UTC()) || !acquired.Truncate(24*time.Hour).Equal(c.Day) {
			return fail()
		}
		if get("satellite") != "N20" || get("version") != FIRMSProductVersion || (get("instrument") != "" && get("instrument") != "VIIRS") || (get("daynight") != "D" && get("daynight") != "N") {
			return fail()
		}
		class := map[string]string{"l": "low", "low": "low", "n": "nominal", "nominal": "nominal", "h": "high", "high": "high"}[get("confidence")]
		if class == "" {
			return fail()
		}
		aoi := in.Admission.Policy.AOI
		if lng < aoi.West || lng > aoi.East || lat < aoi.South || lat > aoi.North {
			return fail()
		}
		// Calendar-day API queries deliberately include a wider interval. Keep
		// only the exact admitted frozen interval, with explicit exclusion counts.
		if acquired.Before(in.From) || acquired.After(in.To) {
			page.ExcludedRecords++
			continue
		}
		original, err := json.Marshal(map[string]any{"csvColumns": fields})
		if err != nil {
			return fail()
		}
		recordBytes := append(append([]byte{}, raw[:endHeader]...), raw[start:reader.InputOffset()]...)
		// FIRMS publishes no stable UUID for a pixel in this feed. This is a
		// derived acquisition/center key, not an inferred fire identity.
		key := strings.Join([]string{FIRMSDataset, "N20", "VIIRS", acquired.Format(time.RFC3339), strconv.FormatFloat(lat, 'f', -1, 64), strconv.FormatFloat(lng, 'f', -1, 64)}, "|")
		observed := acquired
		o := ReportedObservation{SemanticKind: "point_event", Type: "thermal.detection", Title: "Reported VIIRS thermal detection", ObservedAt: acquired, Position: &geo.Point{Lat: lat, Lng: lng}, GeometryRole: "reported_location", Properties: map[string]any{"instrument": "VIIRS", "satellite": "NOAA-20", "product": FIRMSDataset, "productVersion": get("version"), "scanKm": scan, "trackKm": track, "nominalResolutionM": 375, "brightTi4Kelvin": ti4, "brightTi5Kelvin": ti5, "frpMegawatts": frp, "daynight": get("daynight"), "footprintStatus": "dimensions_only_orientation_not_provided", "identityBasis": "derived_acquisition_center_v1", "interpretation": "Reported thermal anomaly; not a confirmed fire or perimeter"}, Quality: map[string]any{"providerConfidence": class, "providerConfidenceCode": get("confidence"), "basis": "FIRMS VIIRS confidence class; not an EMAD assessment confidence", "processing": "NRT"}}
		page.Records = append(page.Records, Record{ProviderRecordID: "firms_pixel_" + firmsHash([]byte(key)), ObservedAt: &observed, ReferenceURL: "https://firms.modaps.eosdis.nasa.gov/", OriginalRecord: original, OriginalBytes: recordBytes, OriginalMediaType: "text/csv", Observation: &o})
	}
	c.Day = c.Day.Add(24 * time.Hour)
	c.Complete = c.Day.After(in.To)
	page.HasMore = !c.Complete
	cursor, _ := json.Marshal(c)
	page.Checkpoint = string(cursor)
	return page, nil
}

func firmsHash(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
