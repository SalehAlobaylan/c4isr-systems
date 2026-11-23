package geoenv

import (
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

type portDay struct {
	Date           string   `json:"date"`
	PortCalls      *int     `json:"portCalls"`
	BaselineDays   int      `json:"baselineDays"`
	BaselineMedian *float64 `json:"baselineMedian"`
	BaselineRange  *[2]int  `json:"baselineRange"`
	Departure      *float64 `json:"departureFromMedian"`
	Comparison     string   `json:"comparison"`
}

type portActivity struct {
	PortID       string                 `json:"portId"`
	BaselineFrom string                 `json:"baselineFrom"`
	From         string                 `json:"from"`
	ToExclusive  string                 `json:"toExclusive"`
	MissingDays  int                    `json:"missingDays"`
	Days         []portDay              `json:"days"`
	Recovery     portRecovery           `json:"recovery"`
	Validation   portBaselineValidation `json:"baselineValidation"`
}

type portRecovery struct {
	MinimumWeekdays    int     `json:"minimumWeekdays"`
	ConsecutiveDays    int     `json:"consecutiveDays"`
	FirstBelowDate     *string `json:"firstBelowDate"`
	ReturnConfirmedOn  *string `json:"returnConfirmedOn"`
	DaysToConfirmation *int    `json:"daysToConfirmation"`
	Status             string  `json:"status"`
}

type portBaselineValidation struct {
	ComparedDays       int      `json:"comparedDays"`
	WeekdayMAE         *float64 `json:"weekdayMeanAbsoluteError"`
	TrailingMAE        *float64 `json:"trailingMeanAbsoluteError"`
	WithinWeekdayRange int      `json:"withinWeekdayRange"`
}

type portReviewSettings struct{ minimum, consecutive int }

func (s Service) reviewPortActivity(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	port, earlier, from, to := query.Get("portId"), query.Get("baselineFrom"), query.Get("from"), query.Get("toExclusive")
	settings, err := parsePortSettings(query.Get("minimumWeekdays"), query.Get("consecutiveDays"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	baseline, start, end, err := parsePortWindow(port, earlier, from, to)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	s.reviewSimulationSource(w, r, "imf-portwatch", "daily-ports-v1", query.Get("inputSha256"),
		func(raw []byte) (string, any, error) {
			activity, err := summarizePortWindow(raw, port, baseline, start, end, settings)
			return "activity", activity, err
		})
}

func parsePortSettings(minimum, consecutive string) (portReviewSettings, error) {
	settings := portReviewSettings{minimum: 4, consecutive: 3}
	for _, item := range []struct {
		value string
		low   int
		out   *int
	}{{minimum, 2, &settings.minimum}, {consecutive, 1, &settings.consecutive}} {
		if item.value != "" {
			value, err := strconv.Atoi(item.value)
			if err != nil || value < item.low || value > 30 {
				return settings, apperr.Validation("provide 2–30 baseline weekdays and 1–30 consecutive recovery days")
			}
			*item.out = value
		}
	}
	return settings, nil
}

func parsePortWindow(port, earlier, start, end string) (time.Time, time.Time, time.Time, error) {
	baseline, errBaseline := time.Parse(time.DateOnly, earlier)
	from, errFrom := time.Parse(time.DateOnly, start)
	to, errTo := time.Parse(time.DateOnly, end)
	if (port != "port24" && port != "port2074") || errBaseline != nil || errFrom != nil || errTo != nil ||
		!baseline.Before(from) || !from.Before(to) || from.Sub(baseline) > 732*24*time.Hour || to.Sub(from) > 366*24*time.Hour {
		return baseline, from, to, apperr.Validation("select Jubail or Jazan, an earlier baseline of up to 732 days, and a study window of up to 366 days")
	}
	return baseline, from, to, nil
}

// ReadPortReview uses the Atlas method and settings for a retained PortWatch file.
func ReadPortReview(raw []byte, digest, port, baselineFrom, studyFrom, studyTo, minimum, consecutive string) (any, error) {
	if len(raw) == 0 || len(raw) > simulationReviewMaxBytes || !digestPattern.MatchString(digest) || artifacts.Hash(raw) != digest {
		return nil, apperr.Validation("PortWatch export differs from its trusted digest or exceeds the input budget")
	}
	baseline, from, to, err := parsePortWindow(port, baselineFrom, studyFrom, studyTo)
	if err != nil {
		return nil, err
	}
	settings, err := parsePortSettings(minimum, consecutive)
	if err != nil {
		return nil, err
	}
	activity, err := summarizePortWindow(raw, port, baseline, from, to, settings)
	if err != nil {
		return nil, err
	}
	return map[string]any{"schema": "emad.portwatch-review.v1", "inputSha256": digest, "activity": activity}, nil
}

func summarizePortActivity(raw []byte, port string, baseline, from, to time.Time) (portActivity, error) {
	return summarizePortWindow(raw, port, baseline, from, to, portReviewSettings{4, 3})
}

func summarizePortWindow(raw []byte, port string, baseline, from, to time.Time, settings portReviewSettings) (portActivity, error) {
	out := portActivity{PortID: port, BaselineFrom: baseline.Format(time.DateOnly),
		From: from.Format(time.DateOnly), ToExclusive: to.Format(time.DateOnly), Days: []portDay{},
		Recovery: portRecovery{MinimumWeekdays: settings.minimum, ConsecutiveDays: settings.consecutive, Status: "insufficient-coverage"}}
	var page struct {
		Error                 json.RawMessage `json:"error"`
		ExceededTransferLimit bool            `json:"exceededTransferLimit"`
		Features              []struct {
			Attributes struct {
				PortID string `json:"portid"`
				Year   int    `json:"year"`
				Month  int    `json:"month"`
				Day    int    `json:"day"`
				Calls  *int   `json:"portcalls"`
			} `json:"attributes"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &page); err != nil || page.Features == nil ||
		len(page.Error) != 0 || page.ExceededTransferLimit {
		return out, apperr.Validation("expected a complete PortWatch JSON response for one port")
	}
	values := map[time.Time]int{}
	var history [7][]int
	for _, feature := range page.Features {
		row := feature.Attributes
		at := time.Date(row.Year, time.Month(row.Month), row.Day, 0, 0, 0, 0, time.UTC)
		if row.PortID != port || at.Year() != row.Year || int(at.Month()) != row.Month || at.Day() != row.Day ||
			at.Before(baseline) || !at.Before(to) || row.Calls == nil || *row.Calls < 0 {
			return out, apperr.Validation("port-day row has a different port, missing or invalid data, or falls outside the declared window")
		}
		if _, exists := values[at]; exists {
			return out, apperr.Validation("port response repeats a day")
		}
		values[at] = *row.Calls
		if at.Before(from) {
			history[at.Weekday()] = append(history[at.Weekday()], *row.Calls)
		}
	}
	for _, weekday := range history {
		slices.Sort(weekday)
	}
	var firstBelow time.Time
	run := 0
	for at := from; at.Before(to); at = at.AddDate(0, 0, 1) {
		weekday := history[at.Weekday()]
		item := portDay{Date: at.Format(time.DateOnly), BaselineDays: len(weekday), Comparison: "missing-day"}
		if len(weekday) >= settings.minimum {
			median := medianPortCalls(weekday)
			item.BaselineMedian = &median
			item.BaselineRange = &[2]int{weekday[0], weekday[len(weekday)-1]}
		}
		if count, exists := values[at]; exists {
			item.PortCalls = &count
			if item.BaselineMedian != nil {
				departure := float64(count) - *item.BaselineMedian
				item.Departure = &departure
				if out.Recovery.Status == "insufficient-coverage" {
					out.Recovery.Status = "no-below-range-day"
				}
			}
			switch {
			case item.BaselineMedian == nil:
				item.Comparison = "insufficient-history"
			case count < weekday[0]:
				item.Comparison = "below-history"
			case count > weekday[len(weekday)-1]:
				item.Comparison = "above-history"
			default:
				item.Comparison = "within-history"
			}
		} else {
			out.MissingDays++
		}
		if item.Comparison == "below-history" && firstBelow.IsZero() {
			firstBelow = at
			out.Recovery.FirstBelowDate, out.Recovery.Status = &item.Date, "unresolved"
		}
		if !firstBelow.IsZero() && out.Recovery.ReturnConfirmedOn == nil {
			if item.Comparison == "within-history" {
				run++
			} else {
				run = 0
			}
			if run == settings.consecutive {
				days := int(at.Sub(firstBelow).Hours() / 24)
				out.Recovery.ReturnConfirmedOn, out.Recovery.DaysToConfirmation, out.Recovery.Status = &item.Date, &days, "confirmed"
			}
		}
		out.Days = append(out.Days, item)
	}
	out.Validation = validatePortBaselines(values, from, to, settings.minimum)
	return out, nil
}

func medianPortCalls(sorted []int) float64 {
	return float64(sorted[(len(sorted)-1)/2])/2 + float64(sorted[len(sorted)/2])/2
}

// Score both baselines on the same dates and use observations strictly earlier
// than each scored date. The interval is the observed weekday sample range.
func validatePortBaselines(values map[time.Time]int, from, to time.Time, minimum int) portBaselineValidation {
	var out portBaselineValidation
	var weekdayError, trailingError float64
	for date := from; date.Before(to); date = date.AddDate(0, 0, 1) {
		count, ok := values[date]
		if !ok {
			continue
		}
		var weekday, trailing []int
		for days := 1; days <= max(56, minimum*7); days++ {
			if prior, ok := values[date.AddDate(0, 0, -days)]; ok {
				if days%7 == 0 {
					weekday = append(weekday, prior)
				}
				if days <= 28 {
					trailing = append(trailing, prior)
				}
			}
		}
		if len(weekday) < minimum || len(trailing) < 14 {
			continue
		}
		slices.Sort(weekday)
		slices.Sort(trailing)
		weekdayError += math.Abs(float64(count) - medianPortCalls(weekday))
		trailingError += math.Abs(float64(count) - medianPortCalls(trailing))
		out.ComparedDays++
		if count >= weekday[0] && count <= weekday[len(weekday)-1] {
			out.WithinWeekdayRange++
		}
	}
	if out.ComparedDays > 0 {
		weekdayMAE, trailingMAE := weekdayError/float64(out.ComparedDays), trailingError/float64(out.ComparedDays)
		out.WeekdayMAE, out.TrailingMAE = &weekdayMAE, &trailingMAE
	}
	return out
}
