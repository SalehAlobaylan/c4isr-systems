package publicingest

import (
	"bytes"
	"testing"
	"time"
)

func TestGDACSRSSRetainsBytesAndBounds(t *testing.T) {
	item := `<item><title>Provider earthquake report</title><link>http://www.gdacs.org/report.aspx?eventid=123</link><eventtype>EQ</eventtype><eventid>123</eventid><fromdate>Fri, 23 Oct 2026 03:00:00 GMT</fromdate><todate>Fri, 23 Oct 2026 04:00:00 GMT</todate><datemodified>Fri, 23 Oct 2026 05:00:00 GMT</datemodified><point>22 39</point><description>Automatic provider claim</description></item>`
	raw := []byte(`<rss><channel>` + item + `</channel></rss>`)
	from := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	in := FetchRequest{From: from, To: from.Add(24 * time.Hour), Admission: Admission{AdmissionInput: AdmissionInput{Policy: Policy{AOI: Bounds{West: 34, South: 16, East: 56, North: 33}, MaxRecordsPerPage: 100}}}}
	page, err := DecodeGDACSRSS(raw, in)
	if err != nil || len(page.Records) != 1 {
		t.Fatal(page, err)
	}
	if !bytes.Contains(page.Records[0].OriginalBytes, []byte(item)) || page.Records[0].OriginalMediaType != "application/xml" {
		t.Fatal("original provider XML was not retained")
	}
	in.Admission.Policy.AOI = Bounds{West: 40, South: 20, East: 45, North: 25}
	page, err = DecodeGDACSRSS(raw, in)
	if err != nil || len(page.Records) != 0 || page.ExcludedRecords != 1 {
		t.Fatal("outside report retained", page, err)
	}
	if _, err = DecodeGDACSRSS([]byte(`<!DOCTYPE rss><rss/>`), in); err == nil {
		t.Fatal("DTD accepted")
	}
}
