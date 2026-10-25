package publicingest

import (
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"net/url"
	"strconv"
	"time"
)

const USGSAdapter = "usgs-earthquakes-v1"
const USGSQueryURL = "https://earthquake.usgs.gov/fdsnws/event/1/query"

type usgsCursor struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Offset   int       `json:"offset"`
	Complete bool      `json:"complete"`
}

func usgsURL(location Location, in FetchRequest) (string, error) {
	bad := func() (string, error) { return "", &RetrievalError{Code: "invalid_response"} }
	cursor := usgsCursor{From: in.From, To: in.To, Offset: 1}
	if in.Checkpoint != "" {
		var old usgsCursor
		if json.Unmarshal([]byte(in.Checkpoint), &old) != nil {
			return bad()
		}
		if !old.Complete {
			if !old.From.Equal(in.From) || !old.To.Equal(in.To) || old.Offset < 1 || old.Offset > 20000 {
				return bad()
			}
			cursor = old
		}
	}
	u, err := url.Parse(location.URL)
	if err != nil {
		return bad()
	}
	p := in.Admission.Policy
	q := u.Query()
	q.Set("format", "geojson")
	q.Set("nodata", "204")
	q.Set("eventtype", "earthquake")
	q.Set("includedeleted", "true")
	q.Set("orderby", "time-asc")
	q.Set("starttime", in.From.UTC().Format(time.RFC3339Nano))
	q.Set("endtime", in.To.UTC().Format(time.RFC3339Nano))
	q.Set("offset", strconv.Itoa(cursor.Offset))
	q.Set("limit", strconv.Itoa(p.MaxRecordsPerPage))
	q.Set("minlongitude", strconv.FormatFloat(p.AOI.West, 'f', -1, 64))
	q.Set("maxlongitude", strconv.FormatFloat(p.AOI.East, 'f', -1, 64))
	q.Set("minlatitude", strconv.FormatFloat(p.AOI.South, 'f', -1, 64))
	q.Set("maxlatitude", strconv.FormatFloat(p.AOI.North, 'f', -1, 64))
	u.RawQuery = q.Encode()
	return u.String(), nil
}
func DecodeUSGS(raw []byte, in FetchRequest) (Page, error) {
	fail := func() (Page, error) { return Page{}, &RetrievalError{Code: "invalid_response"} }
	var collection struct {
		Type     string            `json:"type"`
		Features []json.RawMessage `json:"features"`
		Metadata struct {
			Count  *int `json:"count"`
			Status int  `json:"status"`
		} `json:"metadata"`
	}
	if json.Unmarshal(raw, &collection) != nil || collection.Type != "FeatureCollection" || len(collection.Features) > in.Admission.Policy.MaxRecordsPerPage || (collection.Metadata.Count != nil && *collection.Metadata.Count != len(collection.Features)) || collection.Metadata.Status != 200 {
		return fail()
	}
	page := Page{Records: []Record{}, CoverageFrom: in.From, CoverageTo: in.To}
	cursor := usgsCursor{From: in.From, To: in.To, Offset: 1}
	if in.Checkpoint != "" {
		var old usgsCursor
		if json.Unmarshal([]byte(in.Checkpoint), &old) != nil {
			return fail()
		}
		if !old.Complete {
			cursor = old
		}
	}
	for _, original := range collection.Features {
		var f struct {
			Type     string `json:"type"`
			ID       string `json:"id"`
			Geometry *struct {
				Type        string     `json:"type"`
				Coordinates []*float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				Time    *int64   `json:"time"`
				Updated *int64   `json:"updated"`
				Status  string   `json:"status"`
				Type    string   `json:"type"`
				Title   string   `json:"title"`
				Place   string   `json:"place"`
				URL     string   `json:"url"`
				Mag     *float64 `json:"mag"`
				MagType string   `json:"magType"`
				Net     string   `json:"net"`
				Code    string   `json:"code"`
				RMS     *float64 `json:"rms"`
				Gap     *float64 `json:"gap"`
				NST     *int     `json:"nst"`
				DMin    *float64 `json:"dmin"`
			} `json:"properties"`
		}
		if json.Unmarshal(original, &f) != nil || f.Type != "Feature" || f.ID == "" || f.Properties.Updated == nil || *f.Properties.Updated <= 0 {
			return fail()
		}
		p := f.Properties
		updated := time.UnixMilli(*p.Updated).UTC()
		withdrawn := p.Status == "deleted"
		if p.Status != "automatic" && p.Status != "reviewed" && !withdrawn {
			return fail()
		}
		o := ReportedObservation{SemanticKind: "point_event", Type: "event.earthquake", Title: p.Title, GeometryRole: "reported_location", Withdrawn: withdrawn,
			Properties: map[string]any{"magnitude": p.Mag, "magnitudeType": p.MagType, "place": p.Place, "status": p.Status, "network": p.Net, "code": p.Code, "providerEventType": p.Type},
			Quality:    map[string]any{"reviewStatus": p.Status, "rmsSeconds": p.RMS, "azimuthalGapDegrees": p.Gap, "stationCount": p.NST, "nearestStationDegrees": p.DMin}}
		var observed *time.Time
		if p.Time != nil && *p.Time > 0 {
			t := time.UnixMilli(*p.Time).UTC()
			observed = &t
			o.ObservedAt = t
		}
		if !withdrawn {
			if observed == nil || p.Type != "earthquake" || f.Geometry == nil || f.Geometry.Type != "Point" || len(f.Geometry.Coordinates) != 3 {
				return fail()
			}
			if (!in.From.IsZero() && observed.Before(in.From)) || (!in.To.IsZero() && observed.After(in.To)) {
				return fail()
			}
			for _, v := range f.Geometry.Coordinates {
				if v == nil {
					return fail()
				}
			}
			lng, lat, depth := *f.Geometry.Coordinates[0], *f.Geometry.Coordinates[1], *f.Geometry.Coordinates[2]
			if lng < in.Admission.Policy.AOI.West || lng > in.Admission.Policy.AOI.East || lat < in.Admission.Policy.AOI.South || lat > in.Admission.Policy.AOI.North || depth < -100 || depth > 1000 {
				return fail()
			}
			o.Position = &geo.Point{Lat: lat, Lng: lng}
			o.Properties["depthKm"] = depth
			o.Properties["depthConvention"] = "Provider-reported hypocentral depth in kilometres, positive down; reference level varies by network. Not scene altitude or terrain height."
		}
		reference := p.URL
		if reference == "" {
			reference = "https://earthquake.usgs.gov/earthquakes/eventpage/" + url.PathEscape(f.ID)
		}
		page.Records = append(page.Records, Record{ProviderRecordID: f.ID, ProviderRevision: strconv.FormatInt(*p.Updated, 10), ProviderUpdatedAt: &updated, ObservedAt: observed, ReferenceURL: reference, OriginalRecord: original, Observation: &o})
	}
	page.HasMore = len(page.Records) == in.Admission.Policy.MaxRecordsPerPage
	cursor.Offset += len(page.Records)
	cursor.Complete = !page.HasMore
	if cursor.Offset > 20000 && page.HasMore {
		return fail()
	}
	checkpoint, _ := json.Marshal(cursor)
	page.Checkpoint = string(checkpoint)
	return page, nil
}
