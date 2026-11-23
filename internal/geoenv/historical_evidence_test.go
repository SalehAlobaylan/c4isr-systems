package geoenv

import (
	"testing"
	"time"
)

func TestFloodProductsSelectsSourceVersionsAtCutoff(t *testing.T) {
	const raw = `{"count":1,"next":null,"results":[{"code":"EMSR761","category":"Flood","sensitive":false,"aois":[{"number":1,"name":"Dresden","products":[
	{"id":10,"type":"DEL","activationCode":"EMSR761","aoiNumber":1,"monitoringNumber":0,"version":{"uuid":"version-earlier","number":2,"statusCode":"F","deliveryTime":"2024-09-20T13:13:21.7"},"images":[{"uuid":"image-1","sensorName":"Radar","acquisitionTime":"2024-09-20T05:00:00"}]},
	{"id":11,"type":"GRA","activationCode":"EMSR761","aoiNumber":1,"monitoringNumber":1,"version":{"uuid":"version-later","number":1,"statusCode":"F","deliveryTime":"2024-09-21T05:29:29Z"},"images":[]}
	]}]}]}`
	before := time.Date(2024, 9, 20, 13, 13, 21, 0, time.UTC)
	out, err := summarizeFloodProducts([]byte(raw), "EMSR761", before)
	if err != nil || len(out.Products) != 0 || out.Products == nil {
		t.Fatalf("earlier view should have an empty product list: %+v, %v", out, err)
	}
	out, err = summarizeFloodProducts([]byte(raw), "EMSR761", before.Add(time.Second))
	if err != nil || len(out.Products) != 1 {
		t.Fatalf("expected one delivered version: %+v, %v", out, err)
	}
	first := out.Products[0]
	if first.VersionNumber != 2 || first.SourceImages[0].AcquisitionAt == nil ||
		first.SourceImages[0].AcquisitionAt.Hour() != 5 || out.VersionCoverage != "versions-in-supplied-response" {
		t.Fatalf("provider delivery and acquisition semantics were lost: %+v", out)
	}
	if _, err := summarizeFloodProducts([]byte(raw), "EMSR000", before); err == nil {
		t.Fatal("a different activation must be refused")
	}
	if _, err := summarizeFloodProducts([]byte(`{"count":1,"next":"next","results":[{}]}`), "EMSR761", before); err == nil {
		t.Fatal("a partial response must be refused")
	}
}

func TestRadarOutagesReportsUnknownEndAndAvailability(t *testing.T) {
	const raw = `{"success":true,"result":{"annotations":[
	{"id":"ongoing","eventType":"OUTAGE","locations":["SA"],"startDate":"2024-09-20T01:00:00Z","endDate":null,"outage":{"outageType":"REGIONAL"}},
	{"id":"boundary","eventType":"OUTAGE","locations":["SA"],"startDate":"2024-09-19T01:00:00Z","endDate":"2024-09-20T00:00:00Z","outage":{"outageType":"NATIONWIDE"}},
	{"id":"other-country","eventType":"OUTAGE","locations":["BE"],"startDate":"2024-09-20T00:00:00Z","outage":{"outageType":"REGIONAL"}},
	{"id":"other-event","eventType":"ANOMALY","locations":["SA"]}
	]}}`
	from := time.Date(2024, 9, 20, 0, 0, 0, 0, time.UTC)
	out, err := summarizeRadarOutages([]byte(raw), "SA", from, from.Add(24*time.Hour))
	if err != nil || len(out.Annotations) != 1 || out.Annotations[0].ID != "ongoing" {
		t.Fatalf("expected one overlapping outage: %+v, %v", out, err)
	}
	if out.AvailabilityAt != nil || out.Annotations[0].EndAt != nil {
		t.Fatalf("source did not establish availability or an end: %+v", out)
	}
	if _, err := summarizeRadarOutages([]byte(`{"success":false,"result":{"annotations":[]}}`), "SA", from, from.Add(time.Hour)); err == nil {
		t.Fatal("provider failure must not become an empty result")
	}
}
