package geoenv

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
)

const (
	municipalQ1Dataset       = "riyadh-940"
	municipalQ1Edition       = "2026-q1"
	municipalQ1Digest        = "b3619c468a0e91c06d718ed7421b7f18deb5c7d78535f54046f7f807b8b60a21"
	simulationReviewMaxBytes = 3 << 20
)

type municipalWorkload struct {
	RowCount         int            `json:"rowCount"`
	RowsWithoutPlace int            `json:"rowsWithoutPlaceLabel"`
	CategoryCounts   map[string]int `json:"categoryCounts"`
	UTCDayCounts     map[string]int `json:"utcDayCounts"`
	TemporalMeaning  string         `json:"temporalMeaning"`
}

func summarizeMunicipalQ1(raw []byte) (municipalWorkload, error) {
	result := municipalWorkload{
		CategoryCounts: map[string]int{}, UTCDayCounts: map[string]int{},
		TemporalMeaning: "retrospective-export",
	}
	var rows []struct {
		Category      *string         `json:"نوع البلاغ"`
		Place         json.RawMessage `json:"الموقع"`
		Status        *string         `json:"حالة البلاغ"`
		Closure       *int64          `json:"زمن الإغلاق"`
		ReceiptMillis *int64          `json:"تاريخ الاستلام"`
		Repeat        *string         `json:"تكرار"`
	}
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rows); err != nil || len(rows) == 0 || decoder.Decode(new(any)) != io.EOF {
		return result, apperr.Validation("expected a municipal Q1 JSON array")
	}
	begin := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := begin.AddDate(0, 3, 0)
	for _, row := range rows {
		if row.Category == nil || row.ReceiptMillis == nil || row.Status == nil || row.Closure == nil || row.Repeat == nil || len(row.Place) == 0 {
			return result, apperr.Validation("municipal row is missing a published field")
		}
		category := strings.Join(strings.Fields(*row.Category), " ")
		receipt := time.UnixMilli(*row.ReceiptMillis).UTC()
		if category == "" || receipt.Before(begin) || !receipt.Before(end) {
			return result, apperr.Validation("municipal receipt is outside the edition or lacks a category")
		}
		var place string
		if json.Unmarshal(row.Place, &place) != nil || strings.TrimSpace(place) == "" {
			result.RowsWithoutPlace++
		}
		result.CategoryCounts[category]++
		result.UTCDayCounts[receipt.Format(time.DateOnly)]++
		result.RowCount++
	}
	return result, nil
}

func (s Service) reviewMunicipalQ1(w http.ResponseWriter, r *http.Request) {
	s.reviewSimulationSource(w, r, municipalQ1Dataset, municipalQ1Edition, municipalQ1Digest,
		func(raw []byte) (string, any, error) {
			workload, err := summarizeMunicipalQ1(raw)
			return "workload", workload, err
		})
}

func (s Service) reviewSimulationSource(
	w http.ResponseWriter, r *http.Request, dataset, edition, expectedDigest string,
	summarize func([]byte) (string, any, error),
) {
	rel, err := s.getVisible(r, Read)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	query := r.URL.Query()
	sourceID, admissionID := query.Get("sourceId"), query.Get("admissionId")
	if !keyPattern.MatchString(sourceID) || !keyPattern.MatchString(admissionID) ||
		!digestPattern.MatchString(query.Get("manifestSha256")) || !digestPattern.MatchString(expectedDigest) {
		httpx.Error(w, apperr.Validation("provide exact release, source, admission and input identities"))
		return
	}
	if rel.State == "WITHDRAWN" || rel.ManifestSHA256 != query.Get("manifestSha256") {
		httpx.Error(w, apperr.Conflict("environment release is withdrawn or the selected manifest changed"))
		return
	}
	resolve := func() (publicingest.Admission, error) {
		policy, err := publicingest.ResolvePolicy(r.Context(), s.Pool, sourceID, admissionID)
		if err != nil {
			return policy, err
		}
		if policy.Dataset != dataset || policy.ProductVersion != edition {
			return policy, apperr.Validation("the admission must name the requested dataset and edition")
		}
		permissions := policy.Rights.OutputPermissions
		if !policy.Rights.AllowAnalysis || permissions == nil || permissions.InternalAnalysis == nil || !*permissions.InternalAnalysis {
			return policy, apperr.Forbidden("the current source policy does not allow internal analysis")
		}
		return policy, nil
	}
	if _, err := resolve(); err != nil {
		httpx.Error(w, err)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, simulationReviewMaxBytes))
	if err != nil {
		httpx.Error(w, apperr.BadRequest("the input is unreadable or exceeds 3 MiB"))
		return
	}
	if artifacts.Hash(raw) != expectedDigest {
		httpx.Error(w, apperr.Validation("input SHA-256 differs from the requested edition"))
		return
	}
	field, result, err := summarize(raw)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	// A slow upload may overlap a policy, visibility or release decision change.
	current, err := s.getVisible(r, Read)
	if err == nil && (current.State == "WITHDRAWN" || current.ManifestSHA256 != rel.ManifestSHA256) {
		err = apperr.Conflict("the selected environment release changed while reading the file")
	}
	var policy publicingest.Admission
	if err == nil {
		policy, err = resolve()
	}
	respond(w, http.StatusOK, map[string]any{
		"releaseId": rel.ID, "manifestSha256": rel.ManifestSHA256, "inputSha256": expectedDigest,
		"sourceId": sourceID, "admissionId": admissionID, "currentAdmissionId": policy.CurrentAdmissionID,
		"productVersion": policy.ProductVersion, "attribution": policy.Rights.Attribution, field: result,
	}, err)
}
