package publicingest

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const EarthSearchAdapter = "earthsearch-sentinel2-c1-v1"
const EarthSearchURL = "https://earth-search.aws.element84.com/v1/search"
const EarthSearchDataset = "sentinel-2-c1-l2a"

type RasterBand struct {
	Scale      *float64 `json:"scale,omitempty"`
	Offset     *float64 `json:"offset,omitempty"`
	NoData     *float64 `json:"nodata,omitempty"`
	Resolution float64  `json:"spatial_resolution"`
}

func earthSearchURL(l Location, in FetchRequest) (string, error) {
	if l.URL != EarthSearchURL {
		return "", &RetrievalError{Code: "invalid_location"}
	}
	b := in.Admission.Policy.AOI
	q := url.Values{"collections": {EarthSearchDataset}, "bbox": {strings.Join([]string{strconv.FormatFloat(b.West, 'f', -1, 64), strconv.FormatFloat(b.South, 'f', -1, 64), strconv.FormatFloat(b.East, 'f', -1, 64), strconv.FormatFloat(b.North, 'f', -1, 64)}, ",")}, "datetime": {in.From.Format(time.RFC3339Nano) + "/" + in.To.Format(time.RFC3339Nano)}, "limit": {strconv.Itoa(in.Admission.Policy.MaxRecordsPerPage)}}
	if in.Checkpoint != "" {
		var c stacCursor
		if json.Unmarshal([]byte(in.Checkpoint), &c) != nil || !c.From.Equal(in.From) || !c.To.Equal(in.To) {
			return "", &RetrievalError{Code: "invalid_response"}
		}
		if !c.Complete {
			q.Set("next", c.Token)
		}
	}
	return EarthSearchURL + "?" + q.Encode(), nil
}
func DecodeEarthSearch(raw []byte, in FetchRequest) (Page, error) {
	fail := func() (Page, error) { return Page{}, &RetrievalError{Code: "invalid_response"} }
	var c struct {
		Type     string
		Features []json.RawMessage
		Links    []struct{ Rel, Href, Method string }
	}
	if json.Unmarshal(raw, &c) != nil || c.Type != "FeatureCollection" || len(c.Features) > in.Admission.Policy.MaxRecordsPerPage {
		return fail()
	}
	out := Page{Records: []Record{}, CoverageFrom: in.From, CoverageTo: in.To}
	for _, original := range c.Features {
		var f struct {
			Type, ID, Collection string
			STACVersion          string `json:"stac_version"`
			BBox                 []float64
			Geometry             json.RawMessage
			Properties           struct {
				At         time.Time  `json:"datetime"`
				Updated    *time.Time `json:"updated"`
				Platform   string     `json:"platform"`
				Tile       string     `json:"grid:code"`
				Cloud      *float64   `json:"eo:cloud_cover"`
				Processing string     `json:"s2:processing_baseline"`
				Product    string     `json:"s2:product_type"`
				URI        string     `json:"s2:product_uri"`
			}
			Assets map[string]struct {
				Href, Type string
				Bands      []RasterBand `json:"raster:bands"`
			}
		}
		if json.Unmarshal(original, &f) != nil || f.Type != "Feature" || f.Collection != EarthSearchDataset || !stacID.MatchString(f.ID) || (f.STACVersion != "1.0.0" && f.STACVersion != "1.1.0") || f.Properties.Product != "S2MSI2A" || f.Properties.At.Before(in.From) || f.Properties.At.After(in.To) || f.Properties.Cloud == nil || *f.Properties.Cloud < 0 || *f.Properties.Cloud > 100 || f.Properties.Processing == "" || len(f.BBox) != 4 {
			return fail()
		}
		if f.Properties.Platform != "sentinel-2a" && f.Properties.Platform != "sentinel-2b" && f.Properties.Platform != "sentinel-2c" {
			return fail()
		}
		orbitMatch := regexp.MustCompile(`_R([0-9]{3})_`).FindStringSubmatch(f.Properties.URI)
		if len(orbitMatch) != 2 {
			return fail()
		}
		orbit, _ := strconv.Atoi(orbitMatch[1])
		if orbit < 1 || orbit > 143 {
			return fail()
		}
		b := in.Admission.Policy.AOI
		fb := Bounds{f.BBox[0], f.BBox[1], f.BBox[2], f.BBox[3]}
		if fb.Validate() != nil || fb.East < b.West || fb.West > b.East || fb.North < b.South || fb.South > b.North {
			return fail()
		}
		var geom struct{ Type string }
		if json.Unmarshal(f.Geometry, &geom) != nil || (geom.Type != "Polygon" && geom.Type != "MultiPolygon") || len(f.Geometry) > 32000 {
			return fail()
		}
		if *f.Properties.Cloud > 30 {
			out.ExcludedRecords++
			continue
		}
		reference := "https://earth-search.aws.element84.com/v1/collections/" + EarthSearchDataset + "/items/" + url.PathEscape(f.ID)
		m := SceneMetadata{Schema: "emad.scene.v1", ProviderID: f.ID, Collection: f.Collection, AcquiredAt: f.Properties.At, Platform: f.Properties.Platform, Tile: f.Properties.Tile, Orbit: orbit, ProcessingVersion: f.Properties.Processing, CloudCover: f.Properties.Cloud, Geometry: f.Geometry, BBox: f.BBox, Assets: map[string]SceneAsset{}, RasterBands: map[string]RasterBand{}, STACVersion: f.STACVersion, CatalogueReference: reference}
		for key, target := range map[string]string{"red": "B04_10m", "green": "B03_10m", "blue": "B02_10m", "nir": "B08_10m", "scl": "SCL_20m", "product_metadata": "product_metadata"} {
			a, ok := f.Assets[key]
			if !ok {
				return fail()
			}
			u, e := url.Parse(a.Href)
			if e != nil || u.Scheme != "https" || u.Host != "e84-earth-search-sentinel-data.s3.us-west-2.amazonaws.com" || !strings.HasPrefix(u.Path, "/sentinel-2-c1-l2a/") || strings.Contains(u.Path, "..") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
				return fail()
			}
			m.Assets[target] = SceneAsset{a.Href, a.Type}
			if key != "product_metadata" {
				if len(a.Bands) != 1 || a.Bands[0].NoData == nil {
					return fail()
				}
				band := a.Bands[0]
				if key != "scl" && (band.Scale == nil || *band.Scale <= 0 || band.Offset == nil) {
					return fail()
				}
				m.RasterBands[target] = band
			}
		}
		doc, _ := json.Marshal(m)
		at := f.Properties.At
		out.Records = append(out.Records, Record{ProviderRecordID: f.ID, OriginalRecord: original, ObservedAt: &at, ProviderUpdatedAt: f.Properties.Updated, ReferenceURL: reference, Scene: doc})
	}
	cursor := stacCursor{From: in.From, To: in.To, Complete: true}
	for _, link := range c.Links {
		if link.Rel != "next" {
			continue
		}
		u, e := url.Parse(link.Href)
		if e != nil || u.Scheme != "https" || u.Host != "earth-search.aws.element84.com" || u.Path != "/v1/search" || u.User != nil || u.Fragment != "" || (link.Method != "" && link.Method != "GET") {
			return fail()
		}
		token := u.Query().Get("next")
		if token == "" || len(token) > 2000 || strings.ContainsAny(token, "\r\n") {
			return fail()
		}
		cursor.Token = token
		cursor.Complete = false
		out.HasMore = true
	}
	encoded, _ := json.Marshal(cursor)
	out.Checkpoint = string(encoded)
	return out, nil
}
