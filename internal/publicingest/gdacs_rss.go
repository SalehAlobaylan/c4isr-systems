package publicingest

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"io"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const GDACSRSSAdapter = "gdacs-eq-rss-v1"
const GDACSRSSURL = "https://www.gdacs.org/xml/rss.xml"

func isGDACS(adapter string) bool { return adapter == GDACSAdapter || adapter == GDACSRSSAdapter }
func DecodeGDACSRSS(raw []byte, in FetchRequest) (Page, error) {
	fail := func() (Page, error) { return Page{}, &RetrievalError{Code: "invalid_response"} }
	if bytes.Contains(bytes.ToUpper(raw), []byte("<!DOCTYPE")) || bytes.Contains(bytes.ToUpper(raw), []byte("<!ENTITY")) {
		return fail()
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	page := Page{Records: []Record{}, CoverageFrom: in.From, CoverageTo: in.To}
	count := 0
	for {
		begin := decoder.InputOffset()
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return fail()
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "item" {
			continue
		}
		count++
		if count > 2000 {
			return fail()
		}
		var item struct {
			Fields []struct {
				XMLName xml.Name
				Text    string `xml:",chardata"`
			} `xml:",any"`
		}
		if decoder.DecodeElement(&item, &start) != nil {
			return fail()
		}
		fields := map[string]string{}
		for _, v := range item.Fields {
			fields[v.XMLName.Local] = strings.TrimSpace(v.Text)
		}
		if fields["eventtype"] != "EQ" {
			page.ExcludedRecords++
			continue
		}
		id, e := strconv.ParseInt(fields["eventid"], 10, 64)
		if e != nil || id < 1 {
			return fail()
		}
		at, e := mail.ParseDate(fields["fromdate"])
		if e != nil {
			return fail()
		}
		to, e := mail.ParseDate(fields["todate"])
		if e != nil {
			return fail()
		}
		modified, e := mail.ParseDate(fields["datemodified"])
		if e != nil {
			return fail()
		}
		point := strings.Fields(fields["point"])
		if len(point) != 2 {
			return fail()
		}
		lat, e := strconv.ParseFloat(point[0], 64)
		if e != nil {
			return fail()
		}
		lon, e := strconv.ParseFloat(point[1], 64)
		if e != nil {
			return fail()
		}
		b := in.Admission.Policy.AOI
		if at.Before(in.From) || at.After(in.To) || lat < b.South || lat > b.North || lon < b.West || lon > b.East {
			page.ExcludedRecords++
			continue
		}
		if len(page.Records) >= in.Admission.Policy.MaxRecordsPerPage {
			page.PartialReason = "record_budget"
			page.ExcludedRecords++
			continue
		}
		link, e := url.Parse(fields["link"])
		if e != nil || (link.Scheme != "http" && link.Scheme != "https") || link.Host != "www.gdacs.org" || link.Path != "/report.aspx" || link.User != nil {
			return fail()
		}
		// RSS currently supplies HTTP links. Retain the exact original XML and make
		// the separately labeled clickable reference use the same host over HTTPS.
		link.Scheme = "https"
		description := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(fields["description"], " ")
		description = strings.Join(strings.Fields(description), " ")
		if len(description) > 8000 {
			return fail()
		}
		fromText, toText := fields["fromdate"], fields["todate"]
		context := ContextRecord{Kind: "REPORT", Title: fields["title"], Position: geo.Point{Lng: lon, Lat: lat}, LocationBasis: "GDACS cited event centroid, not an independently verified affected area", TimeBasis: "GDACS RSS explicit timezone; original time strings retained", From: at.UTC(), To: to.UTC(), ExtractionMethod: "gdacs-georss-eq-fields-v1; no AI or linked HTML extraction", Uncertainty: "Automatic provider impact model; not expert reviewed. Alert level is a GDACS claim, not EMAD confidence.", Report: &SourceReport{Claim: description, Country: fields["country"], ProviderAlertLevel: fields["alertlevel"], CitedSource: "GDACS RSS; originating scientific identifier not supplied", CitedFrom: fromText, CitedTo: toText, DocumentURL: link.String(), LineageKeys: []string{"GDACS:EQ:" + fields["eventid"]}, CorroborationMeaning: "GDACS revisions/copies share one event lineage; no independent corroboration inferred."}}
		document, _ := json.Marshal(fields)
		original := append([]byte(nil), raw[begin:decoder.InputOffset()]...)
		observed, updated := at.UTC(), modified.UTC()
		page.Records = append(page.Records, Record{ProviderRecordID: "EQ:" + fields["eventid"], ObservedAt: &observed, ProviderUpdatedAt: &updated, ReferenceURL: "https://www.gdacs.org/", OriginalRecord: document, OriginalBytes: original, OriginalMediaType: "application/xml", Context: &context})
	}
	page.Checkpoint = "gdacs-rss:" + in.To.UTC().Format(time.RFC3339Nano)
	return page, nil
}
