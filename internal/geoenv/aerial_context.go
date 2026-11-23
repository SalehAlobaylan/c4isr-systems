package geoenv

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

type aerialEnergy struct {
	SourceID        string    `json:"sourceId"`
	AvailableAt     time.Time `json:"availableAt"`
	Platform        string    `json:"platform"`
	TestType        string    `json:"testType"`
	TrainingFlights int       `json:"trainingFlights"`
	HeldOutFlights  int       `json:"heldOutFlights"`
	ScoredFlights   int       `json:"scoredFlights"`
	MedianErrorWh   *float64  `json:"medianAbsoluteErrorWh"`
	P90ErrorWh      *float64  `json:"p90AbsoluteErrorWh"`
	ExcludedSeconds float64   `json:"excludedHeldOutSeconds"`
}

func (e aerialExercise) reviewContext(cutoff time.Time, sources map[string]bool, out *aerialReview) error {
	type revisionKey struct {
		id      string
		version int
	}
	productSeen := map[revisionKey]bool{}
	for _, item := range e.Assessments {
		id := revisionKey{item.ID, item.Version}
		if !keyPattern.MatchString(item.ID) || !sources[item.SourceID] || productSeen[id] || item.Version < 1 ||
			item.AvailableAt.IsZero() || (item.Kind != "delineation" && item.Kind != "grading") ||
			len(item.AssessedAreaWkt) > 32000 || len(item.Classes) > 100 || len(item.SourceImageTimes) > 100 ||
			!slices.Contains([]string{"inside", "partial", "outside", "unknown"}, item.Coverage) ||
			(item.Coverage != "inside" && item.Affected != nil) {
			return apperr.Validation("invalid mapping product identity, version or assessment extent")
		}
		productSeen[id] = true
		if item.AvailableAt.After(cutoff) {
			continue
		}
		shared := out.Before != nil && slices.Contains(item.SceneRefs, out.Before.ID) ||
			out.After != nil && slices.Contains(item.SceneRefs, out.After.ID)
		out.Assessments = append(out.Assessments, assessmentReview{item, shared})
	}
	slices.SortFunc(out.Assessments, func(a, b assessmentReview) int {
		if order := a.AvailableAt.Compare(b.AvailableAt); order != 0 {
			return order
		}
		if order := cmp.Compare(a.ID, b.ID); order != 0 {
			return order
		}
		return cmp.Compare(a.Version, b.Version)
	})
	briefingSeen := map[string]bool{}
	for _, cue := range e.Briefing {
		if !keyPattern.MatchString(cue.ID) || !sources[cue.SourceID] || briefingSeen[cue.ID] ||
			cue.AvailableAt.IsZero() || strings.TrimSpace(cue.Briefing) == "" {
			return apperr.Validation("invalid briefing cue identity, source or availability")
		}
		briefingSeen[cue.ID] = true
		if !cue.AvailableAt.After(cutoff) {
			out.Briefing = append(out.Briefing, cue)
		}
	}
	slices.SortFunc(out.Briefing, func(a, b handoffCue) int {
		if order := a.AvailableAt.Compare(b.AvailableAt); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	trafficSeen := map[[2]string]bool{}
	for _, row := range e.Traffic {
		day, err := time.Parse(time.DateOnly, row.Day)
		next := day.AddDate(0, 0, 1)
		key := [2]string{row.SourceID, row.Day}
		if err != nil || !sources[row.SourceID] || trafficSeen[key] || row.AvailableAt.Before(next) ||
			row.UsableHours < 0 || row.UsableHours > 24 ||
			(row.UsableHours == 0 && (row.CivilCount != nil || row.Unclassified != nil || row.OldestAt != nil)) ||
			(row.CivilCount != nil && *row.CivilCount < 0) || (row.Unclassified != nil && *row.Unclassified < 0) ||
			(row.OldestAt != nil && (row.OldestAt.Before(day) || !row.OldestAt.Before(next))) {
			return apperr.Validation("invalid completed-day civil traffic coverage")
		}
		trafficSeen[key] = true
		if !row.AvailableAt.After(cutoff) {
			out.Traffic = append(out.Traffic, row)
		}
	}
	slices.SortFunc(out.Traffic, func(a, b civilTrafficDay) int {
		if order := cmp.Compare(a.Day, b.Day); order != 0 {
			return order
		}
		return cmp.Compare(a.SourceID, b.SourceID)
	})
	notices, err := airportNoticesAt(e.Notices, sources, cutoff)
	if err != nil {
		return err
	}
	out.Notices = notices
	return e.reviewEnergy(cutoff, sources, out)
}

func (e aerialExercise) reviewEnergy(cutoff time.Time, sources map[string]bool, out *aerialReview) error {
	if len(e.Energy) > 30 {
		return apperr.Validation("too many retrospective energy comparison groups")
	}
	seen := map[[3]string]bool{}
	for _, row := range e.Energy {
		key := [3]string{row.SourceID, row.Platform, row.TestType}
		if !sources[row.SourceID] || row.AvailableAt.IsZero() || strings.TrimSpace(row.Platform) == "" ||
			strings.TrimSpace(row.TestType) == "" || seen[key] || row.ExcludedSeconds < 0 ||
			row.TrainingFlights < 0 || row.HeldOutFlights < 0 || row.ScoredFlights < 0 || row.ScoredFlights > row.HeldOutFlights ||
			(row.ScoredFlights == 0) != (row.MedianErrorWh == nil) || (row.ScoredFlights == 0) != (row.P90ErrorWh == nil) ||
			(row.ScoredFlights > 0 && (row.TrainingFlights == 0 || *row.MedianErrorWh < 0 || *row.P90ErrorWh < *row.MedianErrorWh)) {
			return apperr.Validation("invalid completed-flight energy comparison")
		}
		seen[key] = true
		if !row.AvailableAt.After(cutoff) {
			out.Energy = append(out.Energy, row)
		}
	}
	return nil
}

func airportNoticesAt(items []airportNotice, sources map[string]bool, cutoff time.Time) ([]noticeReview, error) {
	type revisionKey struct {
		issuer, id string
		revision   int
	}
	byVersion := map[revisionKey]airportNotice{}
	visible := map[revisionKey]noticeReview{}
	deliveryIDs := map[string]bool{}
	for _, notice := range items {
		key := revisionKey{notice.Publisher, notice.ID, notice.Revision}
		if !keyPattern.MatchString(notice.ID) || !keyPattern.MatchString(notice.DeliveryID) || !sources[notice.SourceID] ||
			deliveryIDs[notice.DeliveryID] || strings.TrimSpace(notice.Title) == "" || strings.TrimSpace(notice.Publisher) == "" ||
			notice.Revision < 1 || (notice.Kind != "aip-edition" && notice.Kind != "service-notice") ||
			(notice.Status != "active" && notice.Status != "cancelled") || notice.PublishedAt.IsZero() ||
			notice.EffectiveAt.IsZero() || notice.AvailableAt.Before(notice.PublishedAt) ||
			(notice.ExpiresAt != nil && !notice.ExpiresAt.After(notice.EffectiveAt)) {
			return nil, apperr.Validation("invalid historical airport notice or delivery")
		}
		if first, exists := byVersion[key]; exists && !sameAirportNotice(first, notice) {
			return nil, apperr.Validation("delivery copies disagree about the issuer's notice")
		}
		byVersion[key] = notice
		deliveryIDs[notice.DeliveryID] = true
		if notice.AvailableAt.After(cutoff) {
			continue
		}
		review, exists := visible[key]
		if !exists || notice.AvailableAt.Before(review.AvailableAt) ||
			notice.AvailableAt.Equal(review.AvailableAt) && notice.DeliveryID < review.DeliveryID {
			review.airportNotice = notice
		}
		review.DeliveryIDs = append(review.DeliveryIDs, notice.DeliveryID)
		visible[key] = review
	}
	type noticeHeadKey struct {
		publisher, kind, id string
		scheduled           bool
	}
	heads := map[noticeHeadKey]noticeReview{}
	for _, notice := range visible {
		key := noticeHeadKey{notice.Publisher, notice.Kind, notice.ID, notice.EffectiveAt.After(cutoff)}
		if notice.Kind == "aip-edition" && !key.scheduled {
			key.id = ""
		}
		if key.scheduled {
			notice.Status = "scheduled"
		} else if notice.Status == "active" && notice.ExpiresAt != nil && !notice.ExpiresAt.After(cutoff) {
			notice.Status = "expired"
		}
		head := heads[key]
		if head.ID == "" || noticeHeadOrder(head, notice) < 0 {
			heads[key] = notice
		}
	}
	out := []noticeReview{}
	for _, notice := range heads {
		slices.Sort(notice.DeliveryIDs)
		out = append(out, notice)
	}
	slices.SortFunc(out, func(a, b noticeReview) int {
		if order := cmp.Compare(a.Publisher, b.Publisher); order != 0 {
			return order
		}
		if order := a.EffectiveAt.Compare(b.EffectiveAt); order != 0 {
			return order
		}
		if order := cmp.Compare(a.ID, b.ID); order != 0 {
			return order
		}
		return cmp.Compare(a.Revision, b.Revision)
	})
	return out, nil
}

func noticeHeadOrder(a, b noticeReview) int {
	if a.Kind == "service-notice" && b.Kind == "service-notice" && a.ID == b.ID && a.Revision != b.Revision {
		return cmp.Compare(a.Revision, b.Revision)
	}
	if order := a.EffectiveAt.Compare(b.EffectiveAt); order != 0 {
		return order
	}
	if order := cmp.Compare(a.ID, b.ID); order != 0 {
		return order
	}
	return cmp.Compare(a.Revision, b.Revision)
}

func sameAirportNotice(a, b airportNotice) bool {
	expiryMatches := a.ExpiresAt == nil && b.ExpiresAt == nil ||
		a.ExpiresAt != nil && b.ExpiresAt != nil && a.ExpiresAt.Equal(*b.ExpiresAt)
	return a.Kind == b.Kind && a.Title == b.Title && a.PublishedAt.Equal(b.PublishedAt) &&
		a.EffectiveAt.Equal(b.EffectiveAt) && expiryMatches && a.Status == b.Status
}

func (e aerialExercise) reviewSurvey(cutoff time.Time, out *aerialReview) error {
	if e.Survey == nil {
		return nil
	}
	if len(e.Survey.RequiredCells) > 10000 || len(e.Survey.Assignments) > 250 {
		return apperr.Validation("survey exceeds the bounded cell or assignment budget")
	}
	needed, covered, seen := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, id := range e.Survey.RequiredCells {
		if !keyPattern.MatchString(id) || needed[id] {
			return apperr.Validation("invalid or repeated survey reference cell")
		}
		needed[id] = true
	}
	s := surveyReview{Required: len(needed)}
	for _, task := range e.Survey.Assignments {
		if !keyPattern.MatchString(task.ID) || seen[task.ID] || task.AvailableAt.IsZero() ||
			(task.Status != "completed" && task.Status != "aborted") {
			return apperr.Validation("invalid survey assignment")
		}
		seen[task.ID] = true
		if task.AvailableAt.After(cutoff) {
			continue
		}
		s.Assignments++
		if task.Status == "aborted" {
			s.Aborted++
		}
		perTask := map[string]bool{}
		for _, cell := range task.Cells {
			if !needed[cell] || perTask[cell] {
				return apperr.Validation("survey footprint has unknown or repeated reference cells")
			}
			perTask[cell] = true
			if covered[cell] {
				s.Duplicate++
			}
			covered[cell] = true
		}
	}
	s.Covered = len(covered)
	out.Survey = &s
	return nil
}
