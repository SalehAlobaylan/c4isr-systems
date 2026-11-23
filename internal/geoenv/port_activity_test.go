package geoenv

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPortRecoveryNeedsAComparableStudyDay(t *testing.T) {
	from := time.Date(2024, 1, 29, 0, 0, 0, 0, time.UTC)
	rows := []map[string]any{testPortRow(from, 0)}
	for weekday := 0; weekday < 4; weekday++ {
		raw, err := json.Marshal(map[string]any{"features": rows})
		if err != nil {
			t.Fatal(err)
		}
		report, err := summarizePortActivity(raw, "port24", from.AddDate(0, 0, -28), from, from.AddDate(0, 0, 1))
		if err != nil || report.Recovery.Status != "insufficient-coverage" {
			t.Fatalf("weekday history has only %d observations: %+v %v", weekday, report, err)
		}
		rows = append(rows, testPortRow(from.AddDate(0, 0, -7*(weekday+1)), 0))
	}
	raw, err := json.Marshal(map[string]any{"features": rows})
	if err != nil {
		t.Fatal(err)
	}
	report, err := summarizePortActivity(raw, "port24", from.AddDate(0, 0, -28), from, from.AddDate(0, 0, 1))
	if err != nil || report.Recovery.Status != "no-below-range-day" {
		t.Fatalf("a real zero with a qualifying zero baseline is comparable: %+v %v", report, err)
	}
}

func TestPortActivityFreezesBaselineAndDistinguishesZero(t *testing.T) {
	from := time.Date(2024, 1, 29, 0, 0, 0, 0, time.UTC)
	baseline := from.AddDate(0, 0, -28)
	to := from.AddDate(0, 0, 2)
	rows := []map[string]any{}
	for at := baseline; at.Before(from); at = at.AddDate(0, 0, 7) {
		rows = append(rows, testPortRow(at, 4))
	}
	rows = append(rows, testPortRow(from, 0))
	raw, err := json.Marshal(map[string]any{"features": rows})
	if err != nil {
		t.Fatal(err)
	}
	report, err := summarizePortActivity(raw, "port24", baseline, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Days) != 2 || report.MissingDays != 1 {
		t.Fatalf("study coverage: %+v", report)
	}
	first, second := report.Days[0], report.Days[1]
	if first.PortCalls == nil || *first.PortCalls != 0 || first.Comparison != "below-history" ||
		first.BaselineMedian == nil || *first.BaselineMedian != 4 ||
		second.PortCalls != nil || second.Comparison != "missing-day" {
		t.Fatalf("distinct zero and unavailable record required: %+v", report.Days)
	}
}

func TestPortActivityRejectsPartialAndDuplicatePages(t *testing.T) {
	from := time.Date(2024, 1, 29, 0, 0, 0, 0, time.UTC)
	row := testPortRow(from, 1)
	for _, tc := range []struct {
		page map[string]any
		want string
	}{
		{map[string]any{"features": []any{}, "exceededTransferLimit": true}, "complete"},
		{map[string]any{"features": []any{row, row}}, "repeats a day"},
	} {
		raw, err := json.Marshal(tc.page)
		if err != nil {
			t.Fatal(err)
		}
		_, err = summarizePortActivity(raw, "port24", from.AddDate(0, 0, -28), from, from.AddDate(0, 0, 1))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("expected %q, got %v", tc.want, err)
		}
	}
}

func TestPortRecoveryRequiresUninterruptedDaysAndMatchedValidation(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	baseline, end := start.AddDate(0, 0, -56), start.AddDate(0, 0, 7)
	var rows []map[string]any
	for at := baseline; at.Before(start); at = at.AddDate(0, 0, 1) {
		rows = append(rows, testPortRow(at, 4))
	}
	for day, value := range []int{0, 4, -1, 4, 4, 4, 30} {
		if value >= 0 {
			rows = append(rows, testPortRow(start.AddDate(0, 0, day), value))
		}
	}
	raw, err := json.Marshal(map[string]any{"features": rows})
	if err != nil {
		t.Fatal(err)
	}
	out, err := summarizePortWindow(raw, "port24", baseline, start, end, portReviewSettings{4, 3})
	if err != nil {
		t.Fatal(err)
	}
	if out.Recovery.Status != "confirmed" || *out.Recovery.ReturnConfirmedOn != "2024-01-06" || *out.Recovery.DaysToConfirmation != 5 ||
		out.Validation.ComparedDays != 6 || *out.Validation.WeekdayMAE != 5 || *out.Validation.TrailingMAE != 5 || out.Validation.WithinWeekdayRange != 4 ||
		*out.Days[0].Departure != -4 || out.Days[2].PortCalls != nil || *out.Days[6].BaselineMedian != 4 {
		t.Fatalf("unexpected validation or recovery: %+v", out)
	}
}

func testPortRow(day time.Time, calls int) map[string]any {
	return map[string]any{"attributes": map[string]any{
		"portid": "port24", "year": day.Year(), "month": int(day.Month()), "day": day.Day(), "portcalls": calls,
	}}
}
