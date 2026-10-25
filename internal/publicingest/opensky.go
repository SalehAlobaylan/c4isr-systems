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

const OpenSkyAdapter = "opensky-states-v1"
const OpenSkyURL = "https://opensky-network.org/api/states/all"
const OpenSkyDataset = "opensky-state-vectors"

func openskyURL(l Location, in FetchRequest) (string, error) {
	u, _ := url.Parse(OpenSkyURL)
	b := in.Admission.Policy.AOI
	q := u.Query()
	for key, v := range map[string]float64{"lamin": b.South, "lamax": b.North, "lomin": b.West, "lomax": b.East} {
		q.Set(key, strconv.FormatFloat(v, 'f', -1, 64))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
func DecodeOpenSky(raw []byte, in FetchRequest) (Page, error) {
	fail := func() (Page, error) { return Page{}, &RetrievalError{Code: "invalid_opensky_response"} }
	var doc struct {
		Time   int64             `json:"time"`
		States []json.RawMessage `json:"states"`
	}
	if len(raw) > in.Admission.Policy.MaxResponseBytes || json.Unmarshal(raw, &doc) != nil || doc.Time <= 0 || len(doc.States) > in.Admission.Policy.MaxRecordsPerPage {
		return fail()
	}
	at := time.Unix(doc.Time, 0).UTC()
	if at.After(in.Now.Add(5*time.Second)) || at.Before(in.Now.Add(-5*time.Minute)) {
		return fail()
	}
	p := Page{Records: []Record{}, CoverageFrom: in.From, CoverageTo: in.To}
	for _, vector := range doc.States {
		var row []json.RawMessage
		if json.Unmarshal(vector, &row) != nil {
			return fail()
		}
		if len(row) < 17 || len(row) > 18 {
			return fail()
		}
		var identity, call, country string
		var observed *int64
		var lng, lat, speed, course, altitude *float64
		var positionSource int
		var onGround bool
		if json.Unmarshal(row[0], &identity) != nil || json.Unmarshal(row[1], &call) != nil || json.Unmarshal(row[2], &country) != nil || json.Unmarshal(row[3], &observed) != nil || json.Unmarshal(row[5], &lng) != nil || json.Unmarshal(row[6], &lat) != nil || json.Unmarshal(row[7], &altitude) != nil || json.Unmarshal(row[8], &onGround) != nil || json.Unmarshal(row[9], &speed) != nil || json.Unmarshal(row[10], &course) != nil || json.Unmarshal(row[16], &positionSource) != nil {
			return fail()
		}
		if lng == nil || lat == nil || observed == nil {
			p.ExcludedRecords++
			continue
		}
		t := time.Unix(*observed, 0).UTC()
		point := geo.Point{Lng: *lng, Lat: *lat}
		if !point.Valid() {
			return fail()
		}
		if !within(in.Admission.Policy.AOI, point) || t.Before(in.From) || t.After(in.To) || t.After(in.Now.Add(5*time.Second)) {
			p.ExcludedRecords++
			continue
		}
		claims := map[string]any{"icao24": identity, "callsign": strings.TrimSpace(call), "registrationCountryInferredByProvider": country, "barometricAltitudeMetres": altitude, "onGround": onGround, "affiliation": "unassessed", "identityStatus": "provider_claim_not_verified"}
		quality := map[string]any{"positionSourceCode": positionSource, "providerSnapshotAt": at, "analystConfidence": nil, "upstreamLineage": "OpenSky aggregated receiver network; repeat snapshots are not independent corroboration"}
		o := MovingObservation{Kind: "aircraft", IdentityClaim: identity, ObservedAt: t, Position: point, SpeedMPS: speed, Course: course, Claims: claims, Quality: quality}
		if o.Validate(in.Now) != nil || (altitude != nil && (math.IsNaN(*altitude) || math.IsInf(*altitude, 0))) {
			return fail()
		}
		// Stable content excludes the changing provider snapshot time. Exact vector
		// bytes remain available, while unchanged snapshots deduplicate correctly.
		original := vector
		p.Records = append(p.Records, Record{ProviderRecordID: fmt.Sprintf("icao24:%s:%d", identity, *observed), OriginalRecord: original, ObservedAt: &t, ReferenceURL: OpenSkyURL, Moving: &o})
	}
	return p, nil
}
