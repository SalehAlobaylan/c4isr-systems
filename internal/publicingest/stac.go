package publicingest

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const STACAdapter = "cdse-sentinel2-stac-v1"
const STACURL = "https://stac.dataspace.copernicus.eu/v1/search"
const STACDataset = "sentinel-2-l2a"

type SceneAsset struct {
	Href      string `json:"href"`
	MediaType string `json:"mediaType"`
}
type SceneMetadata struct {
	RasterBands        map[string]RasterBand `json:"rasterBands,omitempty"`
	CatalogueReference string                `json:"catalogueReference,omitempty"`
	Schema             string                `json:"schema"`
	ProviderID         string                `json:"providerId"`
	Collection         string                `json:"collection"`
	AcquiredAt         time.Time             `json:"acquiredAt"`
	Platform           string                `json:"platform"`
	Tile               string                `json:"tile"`
	Orbit              int                   `json:"orbit"`
	ProcessingVersion  string                `json:"processingVersion"`
	CloudCover         *float64              `json:"cloudCover"`
	Geometry           json.RawMessage       `json:"geometry"`
	BBox               []float64             `json:"bbox"`
	Assets             map[string]SceneAsset `json:"assets"`
	STACVersion        string                `json:"stacVersion"`
}
type stacCursor struct {
	From, To time.Time
	Token    string
	Complete bool
}

var stacID = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,200}$`)

func stacURL(l Location, in FetchRequest) (string, error) {
	if l.URL != STACURL {
		return "", &RetrievalError{Code: "invalid_location"}
	}
	b := in.Admission.Policy.AOI
	q := url.Values{"collections": {STACDataset}, "bbox": {strings.Join([]string{strconv.FormatFloat(b.West, 'f', -1, 64), strconv.FormatFloat(b.South, 'f', -1, 64), strconv.FormatFloat(b.East, 'f', -1, 64), strconv.FormatFloat(b.North, 'f', -1, 64)}, ",")}, "datetime": {in.From.UTC().Format(time.RFC3339Nano) + "/" + in.To.UTC().Format(time.RFC3339Nano)}, "limit": {strconv.Itoa(in.Admission.Policy.MaxRecordsPerPage)}, "filter-lang": {"cql2-text"}, "filter": {"eo:cloud_cover <= 30"}}
	if in.Checkpoint != "" {
		var c stacCursor
		if json.Unmarshal([]byte(in.Checkpoint), &c) != nil {
			return "", &RetrievalError{Code: "invalid_response"}
		}
		if !c.Complete {
			if !c.From.Equal(in.From) || !c.To.Equal(in.To) || c.Token == "" || len(c.Token) > 1000 {
				return "", &RetrievalError{Code: "invalid_response"}
			}
			q.Set("token", c.Token)
		}
	}
	return STACURL + "?" + q.Encode(), nil
}
func DecodeSTAC(raw []byte, in FetchRequest) (Page, error) {
	fail := func() (Page, error) { return Page{}, &RetrievalError{Code: "invalid_response"} }
	if in.Admission.Dataset != STACDataset || in.Admission.ProductVersion != "stac-1.1-l2a" || in.Admission.RevisionPolicy != "content_hash" || len(raw) > in.Admission.Policy.MaxResponseBytes {
		return fail()
	}
	var c struct {
		Type     string
		Features []json.RawMessage
		Links    []struct{ Rel, Href, Method string }
	}
	if json.Unmarshal(raw, &c) != nil || c.Type != "FeatureCollection" || len(c.Features) > in.Admission.Policy.MaxRecordsPerPage {
		return fail()
	}
	page := Page{Records: []Record{}, CoverageFrom: in.From, CoverageTo: in.To}
	for _, original := range c.Features {
		var f struct {
			Type, ID, Collection string
			STACVersion          string `json:"stac_version"`
			BBox                 []float64
			Geometry             json.RawMessage
			Properties           struct {
				Acquired   time.Time  `json:"datetime"`
				Updated    *time.Time `json:"updated"`
				Platform   string     `json:"platform"`
				Tile       string     `json:"grid:code"`
				Orbit      int        `json:"sat:relative_orbit"`
				Processing string     `json:"processing:version"`
				Cloud      *float64   `json:"eo:cloud_cover"`
				Product    string     `json:"product:type"`
			}
			Assets map[string]struct{ Href, Type string }
		}
		if json.Unmarshal(original, &f) != nil || f.Type != "Feature" || f.Collection != STACDataset || f.STACVersion != "1.1.0" || !stacID.MatchString(f.ID) || f.Properties.Product != "S2MSI2A" || f.Properties.Acquired.Before(in.From) || f.Properties.Acquired.After(in.To) || len(f.BBox) != 4 || f.Properties.Cloud == nil || *f.Properties.Cloud < 0 || *f.Properties.Cloud > 100 || f.Properties.Processing == "" || f.Properties.Tile == "" || f.Properties.Orbit < 1 || f.Properties.Orbit > 143 {
			return fail()
		}
		if f.Properties.Platform != "sentinel-2a" && f.Properties.Platform != "sentinel-2b" && f.Properties.Platform != "sentinel-2c" {
			return fail()
		}
		b := in.Admission.Policy.AOI
		fb := f.BBox
		if (Bounds{fb[0], fb[1], fb[2], fb[3]}).Validate() != nil || fb[2] < b.West || fb[0] > b.East || fb[3] < b.South || fb[1] > b.North {
			return fail()
		}
		var geometry struct {
			Type        string
			Coordinates json.RawMessage
		}
		if json.Unmarshal(f.Geometry, &geometry) != nil || (geometry.Type != "Polygon" && geometry.Type != "MultiPolygon") || len(f.Geometry) > 32000 {
			return fail()
		}
		// STAC's query is only an optimization. Enforce the reviewed first-product
		// cloud selection independently and retain exclusion counts.
		if *f.Properties.Cloud > 30 {
			page.ExcludedRecords++
			continue
		}
		m := SceneMetadata{Schema: "emad.scene.v1", ProviderID: f.ID, Collection: f.Collection, AcquiredAt: f.Properties.Acquired, Platform: f.Properties.Platform, Tile: f.Properties.Tile, Orbit: f.Properties.Orbit, ProcessingVersion: f.Properties.Processing, CloudCover: f.Properties.Cloud, Geometry: f.Geometry, BBox: f.BBox, Assets: map[string]SceneAsset{}, STACVersion: f.STACVersion}
		for _, key := range []string{"B04_10m", "B08_10m", "SCL_20m", "product_metadata", "TCI_10m"} {
			a, ok := f.Assets[key]
			if !ok {
				return fail()
			}
			u, e := url.Parse(a.Href)
			if e != nil || u.Scheme != "s3" || u.Host != "eodata" || !strings.HasPrefix(u.Path, "/Sentinel-2/MSI/L2A/") || strings.Contains(u.Path, "..") || u.RawQuery != "" || u.Fragment != "" || u.User != nil || len(a.Href) > 1500 {
				return fail()
			}
			m.Assets[key] = SceneAsset{a.Href, a.Type}
		}
		doc, _ := json.Marshal(m)
		observed := f.Properties.Acquired
		page.Records = append(page.Records, Record{ProviderRecordID: f.ID, OriginalRecord: original, ObservedAt: &observed, ProviderUpdatedAt: f.Properties.Updated, ReferenceURL: "https://stac.dataspace.copernicus.eu/v1/collections/" + STACDataset + "/items/" + url.PathEscape(f.ID), Scene: doc})
	}
	cursor := stacCursor{From: in.From, To: in.To, Complete: true}
	for _, link := range c.Links {
		if link.Rel != "next" {
			continue
		}
		u, e := url.Parse(link.Href)
		if e != nil || u.Scheme != "https" || u.Host != "stac.dataspace.copernicus.eu" || u.Path != "/v1/search" || u.User != nil || u.Fragment != "" || (link.Method != "" && link.Method != "GET") {
			return fail()
		}
		token := u.Query().Get("token")
		if token == "" || len(token) > 1000 || strings.ContainsAny(token, "\r\n") {
			return fail()
		}
		cursor.Token = token
		cursor.Complete = false
		page.HasMore = true
	}
	encoded, _ := json.Marshal(cursor)
	page.Checkpoint = string(encoded)
	return page, nil
}
