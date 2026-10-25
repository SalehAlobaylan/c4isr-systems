package publicingest

import (
	"encoding/json"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"net/url"
	"strconv"
	"time"
)

const GDACSAdapter = "gdacs-eq-bulletins-v1"
const GDACSURL = "https://www.gdacs.org/gdacsapi/api/events/geteventlist/SEARCH"
const GDACSDataset = "gdacs-earthquake-bulletins"

func gdacsURL(in FetchRequest) string {
	u, _ := url.Parse(GDACSURL)
	q := u.Query()
	q.Set("eventlist", "EQ")
	q.Set("fromDate", in.From.Format(time.RFC3339))
	q.Set("toDate", in.To.Format(time.RFC3339))
	q.Set("pageSize", strconv.Itoa(in.Admission.Policy.MaxRecordsPerPage))
	q.Set("pageNumber", "1")
	u.RawQuery = q.Encode()
	return u.String()
}
func gdacsTime(v string) (time.Time, error) {
	if t, e := time.Parse(time.RFC3339, v); e == nil {
		return t.UTC(), nil
	}
	// The API also returns timezone-free timestamps. Keep that uncertainty visible.
	return time.Parse("2006-01-02T15:04:05", v)
}
func DecodeGDACS(raw []byte, in FetchRequest) (Page, error) {
	fail := func() (Page, error) { return Page{}, &RetrievalError{Code: "invalid_gdacs_response"} }
	var doc struct {
		Type     string
		Features []json.RawMessage
	}
	if len(raw) > in.Admission.Policy.MaxResponseBytes || json.Unmarshal(raw, &doc) != nil || doc.Type != "FeatureCollection" || doc.Features == nil || len(doc.Features) > in.Admission.Policy.MaxRecordsPerPage {
		return fail()
	}
	p := Page{Records: []Record{}, CoverageFrom: in.From, CoverageTo: in.To}
	if len(doc.Features) == in.Admission.Policy.MaxRecordsPerPage {
		p.PartialReason = "provider_result_limit"
	} // Explicitly one page: provider ordering may change between calls.
	for _, feature := range doc.Features {
		var f struct {
			Type     string
			Geometry struct {
				Type        string
				Coordinates []float64
			}
			Properties struct {
				EventType                          string `json:"eventtype"`
				ID                                 int64  `json:"eventid"`
				Episode                            int64  `json:"episodeid"`
				Name, Description, Country, Source string
				SourceID                           string `json:"sourceid"`
				Alert                              string `json:"alertlevel"`
				From                               string `json:"fromdate"`
				To                                 string `json:"todate"`
				Modified                           string `json:"datemodified"`
				URL                                struct{ Report string }
			}
		}
		if json.Unmarshal(feature, &f) != nil || f.Type != "Feature" || f.Properties.ID <= 0 || f.Geometry.Type != "Point" || len(f.Geometry.Coordinates) != 2 {
			return fail()
		}
		v := f.Properties
		if v.EventType != "EQ" {
			p.ExcludedRecords++
			continue
		}
		from, e := gdacsTime(v.From)
		if e != nil {
			return fail()
		}
		to, e := gdacsTime(v.To)
		if e != nil || to.Before(from) {
			return fail()
		}
		modified, e := gdacsTime(v.Modified)
		if e != nil {
			return fail()
		}
		point := geo.Point{Lng: f.Geometry.Coordinates[0], Lat: f.Geometry.Coordinates[1]}
		if !point.Valid() {
			return fail()
		}
		// Selection is by cited event interval and centroid, not complete hazard footprint.
		if !within(in.Admission.Policy.AOI, point) || to.Before(in.From) || from.After(in.To) {
			p.ExcludedRecords++
			continue
		}
		docURL, e := url.Parse(v.URL.Report)
		if e != nil || docURL.Scheme != "https" || docURL.Host != "www.gdacs.org" || docURL.Path != "/report.aspx" || docURL.User != nil || docURL.Fragment != "" {
			return fail()
		}
		lineage := []string{fmt.Sprintf("gdacs:EQ:%d", v.ID)}
		if v.Source == "NEIC" || v.Source == "USGS" {
			lineage = append(lineage, "upstream:usgs-neic")
		} else if v.Source != "" {
			lineage = append(lineage, "provider-cited:"+v.Source)
		}
		c := ContextRecord{Kind: "REPORT", Title: v.Name, Position: point, LocationBasis: "Provider cited event centroid; not independently verified or an affected-area boundary", TimeBasis: "Provider cited event interval; timezone-free values interpreted as UTC for filtering, exact source strings retained", From: from, To: to, ExtractionMethod: "gdacs-eq-structured-fields-v1; no AI or HTML interpretation", Uncertainty: "Automatic provider bulletin, not an expert-reviewed assessment. Cited times/location may be approximate. Alert level is a GDACS claim and is not EMAD confidence.", Report: &SourceReport{Claim: v.Description, Country: v.Country, ProviderAlertLevel: v.Alert, CitedSource: v.Source, CitedSourceID: v.SourceID, CitedFrom: v.From, CitedTo: v.To, DocumentURL: docURL.String(), LineageKeys: lineage, CorroborationMeaning: "Copies and revisions share the GDACS event lineage. A shared NEIC/USGS origin is not independent evidence; absent source identifiers leave event-level independence unresolved. No corroboration score is inferred."}}
		if c.Validate(in.Now) != nil {
			return fail()
		}
		p.Records = append(p.Records, Record{ProviderRecordID: fmt.Sprintf("EQ:%d", v.ID), ObservedAt: &from, ProviderUpdatedAt: &modified, ReferenceURL: "https://www.gdacs.org/", OriginalRecord: feature, Context: &c})
	}
	return p, nil
}
