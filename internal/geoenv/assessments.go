package geoenv

// Release-scoped physical-accuracy assessments and consumer suitability.
//
// An assessment is an immutable reviewer decision about one exact release
// member digest, area, use and control set. Corrections are new records that
// name the record they supersede. Suitability pins the assessment sequence and
// evidence digest it used so an earlier decision can still be explained.

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"regexp"
	"slices"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

const (
	assessmentAccepted = "ACCEPTED"
	assessmentRejected = "REJECTED"
)

var usePattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)
var layerPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type AssessmentInput struct {
	ManifestSHA256   string          `json:"manifestSha256"`
	MemberSHA256     string          `json:"memberSha256"`
	Layer            string          `json:"layer"`
	Use              string          `json:"use"`
	Method           string          `json:"method"`
	ControlSetSHA256 string          `json:"controlSetSha256"`
	StatisticM       float64         `json:"statisticM"`
	UncertaintyM     float64         `json:"uncertaintyM"`
	Area             json.RawMessage `json:"area"`
	Decision         string          `json:"decision"`
	Note             string          `json:"note"`
}

// AssessmentCorrection keeps the release, member, layer and use of the record
// it supersedes; only the evidence and reviewer decision can change.
type AssessmentCorrection struct {
	Method           string          `json:"method"`
	ControlSetSHA256 string          `json:"controlSetSha256"`
	StatisticM       float64         `json:"statisticM"`
	UncertaintyM     float64         `json:"uncertaintyM"`
	Area             json.RawMessage `json:"area"`
	Decision         string          `json:"decision"`
	Note             string          `json:"note"`
}

type Assessment struct {
	AssessmentInput
	ID                    string    `json:"id"`
	ReleaseID             string    `json:"releaseId"`
	ReleaseManifestSHA256 string    `json:"releaseManifestSha256"`
	Sequence              int       `json:"sequence"`
	SupersedesID          string    `json:"supersedesId,omitempty"`
	SupersededBy          string    `json:"supersededBy,omitempty"`
	ReviewerHumanID       string    `json:"reviewerHumanId"`
	ReviewedAt            time.Time `json:"reviewedAt"`
	EvidenceSHA256        string    `json:"evidenceSha256"`
}

const assessmentSelect = `SELECT a.id,a.release_id,a.release_manifest_sha256,a.member_sha256,a.layer,a.use_key,
	a.method,a.control_set_sha256,a.statistic_m,a.uncertainty_m,a.area,a.decision,a.sequence,
	COALESCE(a.supersedes_id,''),
	COALESCE((SELECT s.id FROM geographic_environment_assessments s
		WHERE s.supersedes_id=a.id AND s.scope_id=a.scope_id ORDER BY s.sequence DESC LIMIT 1),''),
	a.reviewer_human_id,a.note,a.evidence_sha256,a.reviewed_at
	FROM geographic_environment_assessments a `

func scanAssessment(row pgx.Row) (Assessment, error) {
	var a Assessment
	var raw []byte
	err := row.Scan(&a.ID, &a.ReleaseID, &a.ReleaseManifestSHA256, &a.MemberSHA256, &a.Layer, &a.Use,
		&a.Method, &a.ControlSetSHA256, &a.StatisticM, &a.UncertaintyM, &raw, &a.Decision, &a.Sequence,
		&a.SupersedesID, &a.SupersededBy, &a.ReviewerHumanID, &a.Note, &a.EvidenceSHA256, &a.ReviewedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, apperr.NotFound("environment assessment", "unavailable")
	}
	if err != nil {
		return a, err
	}
	a.ManifestSHA256 = a.ReleaseManifestSHA256
	a.Area = json.RawMessage(raw)
	return a, nil
}

// assessmentEvidenceDigest pins the immutable reviewer record. Derived fields
// such as supersededBy are excluded so a correction does not rewrite it.
func assessmentEvidenceDigest(a Assessment, area map[string]any) string {
	return canonicalDigest(map[string]any{
		"id": a.ID, "releaseId": a.ReleaseID, "releaseManifestSha256": a.ReleaseManifestSHA256,
		"memberSha256": a.MemberSHA256, "layer": a.Layer, "use": a.Use, "method": a.Method,
		"controlSetSha256": a.ControlSetSHA256, "statisticM": a.StatisticM,
		"uncertaintyM": a.UncertaintyM, "area": area, "decision": a.Decision,
		"sequence": a.Sequence, "supersedesId": a.SupersedesID,
	})
}

func validAssessmentEvidence(in AssessmentInput) bool {
	return digestPattern.MatchString(in.ManifestSHA256) && digestPattern.MatchString(in.MemberSHA256) &&
		layerPattern.MatchString(in.Layer) && usePattern.MatchString(in.Use) && boundedText(in.Method, 1000) &&
		digestPattern.MatchString(in.ControlSetSHA256) && validStatistic(in.StatisticM) &&
		validStatistic(in.UncertaintyM) && (in.Decision == assessmentAccepted || in.Decision == assessmentRejected) &&
		boundedText(in.Note, 2000) && len(in.Area) > 0 && len(in.Area) <= 65536 && json.Valid(in.Area)
}

func validStatistic(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 100000
}

func validAssessmentCorrection(in AssessmentCorrection) bool {
	return boundedText(in.Method, 1000) && digestPattern.MatchString(in.ControlSetSHA256) &&
		validStatistic(in.StatisticM) && validStatistic(in.UncertaintyM) &&
		(in.Decision == assessmentAccepted || in.Decision == assessmentRejected) &&
		boundedText(in.Note, 2000) && len(in.Area) > 0 && len(in.Area) <= 65536 && json.Valid(in.Area)
}

func (s Service) validAssessmentArea(ctx context.Context, tx pgx.Tx, area, releaseArea json.RawMessage) error {
	var covered bool
	query := `SELECT CASE WHEN ST_GeometryType(g) IN ('ST_Polygon','ST_MultiPolygon') AND ST_IsValid(g)
		AND NOT ST_IsEmpty(g) AND ST_CoveredBy(g,r)
		THEN true ELSE false END
		FROM (SELECT ST_SetSRID(ST_GeomFromGeoJSON($1),4326) g,
			ST_SetSRID(ST_GeomFromGeoJSON($2),4326) r) x`
	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, query, string(area), string(releaseArea)).Scan(&covered)
	} else {
		err = db.Scoped(s.Pool).QueryRow(ctx, query, string(area), string(releaseArea)).Scan(&covered)
	}
	if err != nil || !covered {
		return apperr.Validation("assessment area must be a valid WGS84 polygon inside the release area")
	}
	return nil
}

func manifestMemberFor(m Manifest, digest, layer string) (Member, bool) {
	for _, member := range m.Members {
		if member.SHA256 == digest && member.Layer == layer {
			return member, true
		}
	}
	return Member{}, false
}

func (s Service) assessmentParent(ctx context.Context, tx pgx.Tx, id string) (EnvironmentRelease, Manifest, error) {
	rel, err := s.displayParentFor(ctx, tx, id, access.Allows(ctx, Qualify))
	if err != nil {
		return rel, Manifest{}, err
	}
	var m Manifest
	if err = json.Unmarshal(rel.Manifest, &m); err != nil {
		return rel, m, apperr.Conflict("release manifest is unavailable")
	}
	return rel, m, nil
}

func (s Service) recordAssessment(ctx context.Context, releaseID string, in AssessmentInput) (Assessment, error) {
	var a Assessment
	if err := require(ctx, Qualify, false); err != nil {
		return a, err
	}
	if !validAssessmentEvidence(in) {
		return a, apperr.Validation("assessment requires an exact release member, area, method, control set and reviewer decision")
	}
	p, resource, _ := access.From(ctx)
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+":georelease:"+releaseID); err != nil {
			return err
		}
		rel, manifest, err := s.assessmentParent(ctx, tx, releaseID)
		if err != nil {
			return err
		}
		if rel.ManifestSHA256 != in.ManifestSHA256 {
			return apperr.Conflict("assessment does not bind the exact retained manifest")
		}
		if _, ok := manifestMemberFor(manifest, in.MemberSHA256, in.Layer); !ok {
			return apperr.Validation("assessment member digest and layer are not present in the release manifest")
		}
		if err = s.validAssessmentArea(ctx, tx, in.Area, manifest.Area); err != nil {
			return err
		}
		var sequence int
		if err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(sequence),0)+1 FROM geographic_environment_assessments
			WHERE release_id=$1 AND layer=$2 AND use_key=$3`, releaseID, in.Layer, in.Use).Scan(&sequence); err != nil {
			return err
		}
		a = Assessment{AssessmentInput: in, ID: ids.New("geoassess"), ReleaseID: releaseID,
			ReleaseManifestSHA256: rel.ManifestSHA256, Sequence: sequence,
			ReviewerHumanID: p.HumanID, ReviewedAt: time.Now().UTC()}
		decoded, err := DecodeEvidenceDocument(in.Area)
		if err != nil {
			return err
		}
		a.EvidenceSHA256 = assessmentEvidenceDigest(a, decoded)
		_, err = tx.Exec(ctx, `INSERT INTO geographic_environment_assessments(id,release_id,release_manifest_sha256,
			member_sha256,layer,use_key,method,control_set_sha256,statistic_m,uncertainty_m,area,decision,sequence,
			supersedes_id,reviewer_human_id,note,evidence_sha256,required_sources,reviewed_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
			a.ID, a.ReleaseID, a.ReleaseManifestSHA256, a.MemberSHA256, a.Layer, a.Use, a.Method,
			a.ControlSetSHA256, a.StatisticM, a.UncertaintyM, string(a.Area), a.Decision, a.Sequence,
			nullString(a.SupersedesID), a.ReviewerHumanID, a.Note, a.EvidenceSHA256, rel.sources, a.ReviewedAt)
		return err
	})
	return a, err
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (s Service) supersedeAssessment(ctx context.Context, id string, in AssessmentCorrection) (Assessment, error) {
	var a Assessment
	if err := require(ctx, Qualify, false); err != nil {
		return a, err
	}
	if !validAssessmentCorrection(in) {
		return a, apperr.Validation("correction requires an area, method, control set and reviewer decision")
	}
	p, resource, _ := access.From(ctx)
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		prior, err := scanAssessment(tx.QueryRow(ctx, assessmentSelect+`WHERE a.id=$1`, id))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+":georelease:"+prior.ReleaseID); err != nil {
			return err
		}
		reloaded, err := scanAssessment(tx.QueryRow(ctx, assessmentSelect+`WHERE a.id=$1`, id))
		if err != nil {
			return err
		}
		if reloaded.SupersededBy != "" {
			return apperr.Conflict("assessment already has a recorded correction")
		}
		rel, manifest, err := s.assessmentParent(ctx, tx, prior.ReleaseID)
		if err != nil {
			return err
		}
		if rel.ManifestSHA256 != prior.ReleaseManifestSHA256 {
			return apperr.Conflict("assessment no longer binds the retained manifest")
		}
		if _, ok := manifestMemberFor(manifest, prior.MemberSHA256, prior.Layer); !ok {
			return apperr.Conflict("assessment member is no longer retained by the release")
		}
		if err = s.validAssessmentArea(ctx, tx, in.Area, manifest.Area); err != nil {
			return err
		}
		var sequence int
		if err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(sequence),0)+1 FROM geographic_environment_assessments
			WHERE release_id=$1 AND layer=$2 AND use_key=$3`, prior.ReleaseID, prior.Layer, prior.Use).Scan(&sequence); err != nil {
			return err
		}
		a = Assessment{AssessmentInput: AssessmentInput{ManifestSHA256: prior.ReleaseManifestSHA256,
			MemberSHA256: prior.MemberSHA256, Layer: prior.Layer, Use: prior.Use, Method: in.Method,
			ControlSetSHA256: in.ControlSetSHA256, StatisticM: in.StatisticM, UncertaintyM: in.UncertaintyM,
			Area: in.Area, Decision: in.Decision, Note: in.Note},
			ID: ids.New("geoassess"), ReleaseID: prior.ReleaseID, ReleaseManifestSHA256: prior.ReleaseManifestSHA256,
			Sequence: sequence, SupersedesID: prior.ID, ReviewerHumanID: p.HumanID, ReviewedAt: time.Now().UTC()}
		decoded, err := DecodeEvidenceDocument(in.Area)
		if err != nil {
			return err
		}
		a.EvidenceSHA256 = assessmentEvidenceDigest(a, decoded)
		_, err = tx.Exec(ctx, `INSERT INTO geographic_environment_assessments(id,release_id,release_manifest_sha256,
			member_sha256,layer,use_key,method,control_set_sha256,statistic_m,uncertainty_m,area,decision,sequence,
			supersedes_id,reviewer_human_id,note,evidence_sha256,required_sources,reviewed_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
			a.ID, a.ReleaseID, a.ReleaseManifestSHA256, a.MemberSHA256, a.Layer, a.Use, a.Method,
			a.ControlSetSHA256, a.StatisticM, a.UncertaintyM, string(a.Area), a.Decision, a.Sequence,
			prior.ID, a.ReviewerHumanID, a.Note, a.EvidenceSHA256, rel.sources, a.ReviewedAt)
		return err
	})
	return a, err
}

func (s Service) listAssessments(ctx context.Context, releaseID, after string) (page[Assessment], error) {
	out := page[Assessment]{Items: []Assessment{}}
	if err := require(ctx, Read, false); err != nil {
		return out, err
	}
	if _, err := s.displayParent(ctx, nil, releaseID); err != nil {
		return out, err
	}
	rows, err := db.Scoped(s.Pool).Query(ctx, assessmentSelect+
		`WHERE a.release_id=$1 AND a.id>$2 ORDER BY a.id LIMIT 51`, releaseID, after)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanAssessment(rows)
		if err != nil {
			return out, err
		}
		if len(out.Items) == 50 {
			out.Next = out.Items[49].ID
			break
		}
		out.Items = append(out.Items, a)
	}
	return out, rows.Err()
}

func (s Service) getAssessment(ctx context.Context, id string) (Assessment, error) {
	if err := require(ctx, Read, false); err != nil {
		return Assessment{}, err
	}
	a, err := scanAssessment(db.Scoped(s.Pool).QueryRow(ctx, assessmentSelect+`WHERE a.id=$1`, id))
	if err != nil {
		return a, err
	}
	_, err = s.displayParent(ctx, nil, a.ReleaseID)
	return a, err
}

// releaseAssessments returns every retained assessment, including superseded
// records. Suitability filters current records from supersededBy.
func (s Service) releaseAssessments(ctx context.Context, releaseID string) ([]Assessment, error) {
	rows, err := db.Scoped(s.Pool).Query(ctx, assessmentSelect+`WHERE a.release_id=$1 ORDER BY a.sequence,a.id`, releaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Assessment{}
	for rows.Next() {
		a, err := scanAssessment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

type evidenceMember struct {
	Name      string          `json:"name"`
	Layer     string          `json:"layer"`
	Component *int            `json:"component"`
	SHA256    string          `json:"sha256"`
	Bytes     int64           `json:"bytes"`
	Details   json.RawMessage `json:"details"`
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func evidenceInputs(m Manifest) []map[string]any {
	inputs := make([]map[string]any, 0, len(m.Inputs))
	for _, input := range m.Inputs {
		rep := input.Representation
		inputs = append(inputs, map[string]any{
			"layer": rep.Layer, "verticalDatum": nullable(rep.VerticalDatum),
			"dataTime": nullable(rep.DataTime), "uncertainty": nullable(rep.Uncertainty),
			"crs": nullable(rep.CRS), "productEdition": nullable(rep.ProductEdition),
		})
	}
	return inputs
}

type evidenceManifestMember struct {
	Member
	Component *int            `json:"component"`
	Details   json.RawMessage `json:"details"`
}

// buildEvidence assembles the versioned evidence document shared with the
// offline reader. Keys and null values follow scripts/data/environment_reader.py
// so both readers produce the same release digest.
func buildEvidence(manifest releaseManifestEvidence, manifestSHA string, assessments []any, terms any, visual []string) (map[string]any, error) {
	members := make([]evidenceMember, 0, len(manifest.Members))
	for _, member := range manifest.Members {
		details := member.Details
		if len(details) == 0 || jsonEqual(details, json.RawMessage("null")) {
			details = json.RawMessage("{}")
		}
		members = append(members, evidenceMember{Name: member.Name, Layer: member.Layer,
			Component: member.Component, SHA256: member.SHA256, Bytes: member.Bytes, Details: details})
	}
	document := map[string]any{
		"schema": suitabilityEvidenceSchema, "manifestSha256": manifestSHA,
		"profileId": manifest.ProfileID, "areaRevisionId": manifest.AreaRevisionID,
		"area": manifest.Area, "members": members, "inputs": evidenceInputs(manifest.Manifest),
		"assessments": assessments, "terms": terms,
		"representationAvailability": map[string]any{"visual-derivative": visual},
	}
	raw, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	return DecodeEvidenceDocument(raw)
}

// releaseManifestEvidence adds the member component and details fields that the
// base release manifest type does not need for publication.
type releaseManifestEvidence struct {
	Manifest
	Members []evidenceManifestMember `json:"members"`
}

func (s Service) releaseEvidence(ctx context.Context, rel EnvironmentRelease) (map[string]any, error) {
	var manifest releaseManifestEvidence
	if err := json.Unmarshal(rel.Manifest, &manifest); err != nil {
		return nil, apperr.Conflict("release manifest is unavailable")
	}
	terms, err := s.currentTerms(ctx, manifest.Manifest)
	if err != nil {
		return nil, err
	}
	assessments, err := s.releaseAssessments(ctx, rel.ID)
	if err != nil {
		return nil, err
	}
	assessmentValues := make([]any, 0, len(assessments))
	for _, item := range assessments {
		assessmentValues = append(assessmentValues, map[string]any{
			"id": item.ID, "releaseManifestSha256": item.ReleaseManifestSHA256,
			"memberSha256": item.MemberSHA256, "layer": item.Layer, "use": item.Use,
			"decision": item.Decision, "method": item.Method, "controlSetSha256": item.ControlSetSHA256,
			"evidenceSha256": item.EvidenceSHA256, "statisticM": item.StatisticM,
			"uncertaintyM": item.UncertaintyM, "area": item.Area, "sequence": item.Sequence,
			"reviewedAt": item.ReviewedAt.UTC().Format(time.RFC3339), "supersededBy": item.SupersededBy,
		})
	}
	visual, err := s.visualOnlyLayers(ctx, rel.ID, manifest.Members)
	if err != nil {
		return nil, err
	}
	return buildEvidence(manifest, rel.ManifestSHA256, assessmentValues, terms, visual)
}

// currentTerms reevaluates each input's original and current source policy.
// A withdrawn, expired or disabled policy becomes a revoked grant rather than
// a failed request, so the caller receives an explicit decision.
func (s Service) currentTerms(ctx context.Context, m Manifest) (any, error) {
	if len(m.Inputs) == 0 {
		return nil, nil
	}
	grants := make([]map[string]any, 0, len(m.Inputs))
	var issued time.Time
	var expires *time.Time
	for _, input := range m.Inputs {
		policy, err := publicingest.ResolvePolicy(ctx, s.Pool, input.SourceID, input.AdmissionID)
		allowed := err == nil && policy.Rights.RetainRaw && policy.Rights.AllowAnalysis &&
			policy.Rights.OutputPermissions != nil && policy.Rights.OutputPermissions.GeographicEnvironment != nil &&
			*policy.Rights.OutputPermissions.GeographicEnvironment &&
			(policy.Rights.ExpiresAt == nil || policy.Rights.ExpiresAt.After(time.Now()))
		if err != nil && !apperr.Is(err, apperr.CodeNotFound) && !apperr.Is(err, apperr.CodeConflict) && !apperr.Is(err, apperr.CodeForbidden) {
			return nil, err
		}
		id := input.AdmissionID
		if err == nil && policy.Rights.ID != "" {
			id = policy.Rights.ID
			if policy.ApprovedAt.After(issued) {
				issued = policy.ApprovedAt
			}
			if policy.Rights.ExpiresAt != nil && (expires == nil || policy.Rights.ExpiresAt.Before(*expires)) {
				value := *policy.Rights.ExpiresAt
				expires = &value
			}
		}
		grants = append(grants, map[string]any{"id": id, "scope": "read-native", "revoked": !allowed})
	}
	terms := map[string]any{"grants": grants}
	if !issued.IsZero() {
		terms["issuedAt"] = issued.UTC().Format(time.RFC3339)
	}
	if expires != nil {
		terms["expiresAt"] = expires.UTC().Format(time.RFC3339)
	}
	return terms, nil
}

func (s Service) visualOnlyLayers(ctx context.Context, releaseID string, members []evidenceManifestMember) ([]string, error) {
	native := map[string]bool{}
	for _, member := range members {
		native[member.Layer] = true
	}
	visual := []string{}
	displays, err := s.readyDisplays(ctx, releaseID, "")
	if err != nil {
		return nil, err
	}
	for _, display := range displays.Items {
		layer := displayFormat(display.Recipe).layer
		if layer != "" && !native[layer] && !slices.Contains(visual, layer) {
			visual = append(visual, layer)
		}
	}
	slices.Sort(visual)
	return visual, nil
}

func validSuitabilityRequirement(requirement map[string]any) bool {
	if evidenceString(requirement["schema"]) != suitabilitySchema ||
		!boundedText(evidenceString(requirement["requirementId"]), 200) ||
		!boundedText(evidenceString(requirement["operation"]), 200) {
		return false
	}
	release := evidenceObject(requirement["release"])
	if release == nil || !digestPattern.MatchString(evidenceString(release["manifestSha256"])) ||
		!boundedText(evidenceString(release["areaRevisionId"]), 200) {
		return false
	}
	representations, ok := requirement["representations"].([]any)
	return ok && len(representations) > 0 && len(representations) <= 64
}

func (s Service) suitability(ctx context.Context, releaseID string, raw json.RawMessage) (SuitabilityResult, error) {
	var result SuitabilityResult
	if err := require(ctx, Read, false); err != nil {
		return result, err
	}
	if len(raw) == 0 || len(raw) > 1<<20 {
		return result, apperr.Validation("a bounded requirement document is required")
	}
	requirement, err := DecodeEvidenceDocument(raw)
	if err != nil {
		return result, apperr.Validation("requirement must be a JSON object")
	}
	if !validSuitabilityRequirement(requirement) {
		return result, apperr.Validation("requirement must declare its schema, identity, operation, release and representations")
	}
	rel, err := s.release(ctx, releaseID)
	if err != nil {
		return result, err
	}
	if err = s.visibleRelease(ctx, rel); err != nil {
		return result, err
	}
	evidence, err := s.releaseEvidence(ctx, rel)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC()
	result, err = EvaluateSuitability(requirement, evidence, &now)
	if err != nil {
		return SuitabilityResult{}, apperr.Validation(err.Error())
	}
	return result, nil
}

func (s Service) assessmentRoutes(r chi.Router) {
	r.Get("/environment-releases/{id}/assessments", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.listAssessments(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("after"))
		respond(w, http.StatusOK, out, err)
	})
	r.Post("/environment-releases/{id}/assessments", func(w http.ResponseWriter, r *http.Request) {
		request(w, r, http.StatusCreated, func(ctx context.Context, in AssessmentInput) (Assessment, error) {
			return s.recordAssessment(ctx, chi.URLParam(r, "id"), in)
		})
	})
	r.Get("/environment-assessments/{id}", func(w http.ResponseWriter, r *http.Request) {
		a, err := s.getAssessment(r.Context(), chi.URLParam(r, "id"))
		respond(w, http.StatusOK, a, err)
	})
	r.Post("/environment-assessments/{id}/supersede", func(w http.ResponseWriter, r *http.Request) {
		request(w, r, http.StatusCreated, func(ctx context.Context, in AssessmentCorrection) (Assessment, error) {
			return s.supersedeAssessment(ctx, chi.URLParam(r, "id"), in)
		})
	})
	r.Post("/environment-releases/{id}/suitability", func(w http.ResponseWriter, r *http.Request) {
		var raw json.RawMessage
		if err := httpx.DecodeJSON(r, &raw); err != nil {
			httpx.Error(w, err)
			return
		}
		result, err := s.suitability(r.Context(), chi.URLParam(r, "id"), raw)
		respond(w, http.StatusOK, result, err)
	})
}
