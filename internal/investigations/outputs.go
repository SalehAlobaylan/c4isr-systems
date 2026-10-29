package investigations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/capacity"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/jsoncanon"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type OutputInput struct {
	DeliveryKind       string            `json:"deliveryKind,omitempty"`
	Environment        *EnvironmentInput `json:"environment,omitempty"`
	RequestKey         string            `json:"requestKey"`
	ExpectedRevisionID string            `json:"expectedRevisionId"`
	Purpose            string            `json:"purpose"`
	SourceUses         map[string]string `json:"sourceUses"`
	AssetIDs           []string          `json:"assetIds"`
	RendererVersion    string            `json:"rendererVersion"`
	StyleVersion       string            `json:"styleVersion"`
	RetainUntil        time.Time         `json:"retainUntil"`
	ExpectedBytes      int64             `json:"expectedBytes"`
}
type OutputDependency struct {
	CurrentPolicyID string              `json:"currentPolicyId"`
	EvaluatedAt     time.Time           `json:"evaluatedAt"`
	OriginalRights  publicingest.Rights `json:"originalRights"`
	SourceID        string              `json:"sourceId"`
	AdmissionID     string              `json:"admissionId,omitempty"`
	ReferenceID     string              `json:"referenceId,omitempty"`
	Use             string              `json:"use"`
	Rights          publicingest.Rights `json:"effectiveRights"`
}
type OutputManifest struct {
	Brief           *BriefSnapshot       `json:"brief,omitempty"`
	Environment     *EnvironmentSnapshot `json:"environment,omitempty"`
	RetainUntil     time.Time            `json:"retainUntil"`
	Schema          string               `json:"schema"`
	Case            Revision             `json:"case"`
	Dependencies    []OutputDependency   `json:"dependencies"`
	Assets          []artifacts.Artifact `json:"assets"`
	RendererVersion string               `json:"rendererVersion"`
	StyleVersion    string               `json:"styleVersion"`
	Purpose         string               `json:"purpose"`
	Omissions       []string             `json:"omissions"`
}
type Output struct {
	Superseded        bool           `json:"superseded"`
	CreatorHumanID    string         `json:"creatorHumanId"`
	RenderHumanID     string         `json:"renderHumanId"`
	ID                string         `json:"id"`
	CaseID            string         `json:"caseId"`
	RevisionID        string         `json:"revisionId"`
	ContentDigest     string         `json:"contentDigest"`
	State             string         `json:"state"`
	RenderSHA256      string         `json:"renderSha256"`
	MediaType         string         `json:"mediaType"`
	Bytes             int64          `json:"bytes"`
	RetainUntil       time.Time      `json:"retainUntil"`
	UnavailableReason string         `json:"unavailableReason"`
	Manifest          OutputManifest `json:"manifest"`
	ObjectKey         string         `json:"-"`
}

func (s Service) output(ctx context.Context, id string) (Output, error) {
	var o Output
	var raw []byte
	e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT id,case_id,revision_id,content_digest,state,render_sha256,media_type,size_bytes,retain_until,unavailable_reason,document,object_key,creator_human_id,render_human_id FROM composite_outputs WHERE id=$1`, id).Scan(&o.ID, &o.CaseID, &o.RevisionID, &o.ContentDigest, &o.State, &o.RenderSHA256, &o.MediaType, &o.Bytes, &o.RetainUntil, &o.UnavailableReason, &raw, &o.ObjectKey, &o.CreatorHumanID, &o.RenderHumanID)
	if errors.Is(e, pgx.ErrNoRows) {
		return o, apperr.NotFound("output", "unavailable")
	}
	if e == nil {
		e = json.Unmarshal(raw, &o.Manifest)
	}
	return o, e
}
func outputAllowed(r publicingest.Rights, use string) bool {
	p := r.OutputPermissions
	if p == nil {
		return false
	}
	yes := func(v *bool) bool { return v != nil && *v }
	if !yes(p.MetadataRetention) {
		return false
	}
	switch use {
	case "reference":
		return yes(p.Citation)
	case "imagery":
		return r.RetainRaw && yes(p.IncludedImagery)
	case "raw":
		return r.RetainRaw && yes(p.RawDistribution)
	case "environment":
		return r.AllowSimulationExport && yes(p.GeographicEnvironment)
	case "internal":
		return r.AllowAnalysis && yes(p.InternalAnalysis)
	}
	return false
}
func (s Service) dependencyPolicy(ctx context.Context, tx pgx.Tx, d OutputDependency) (publicingest.Rights, error) {
	if e := access.RequireSource(ctx, d.SourceID); e != nil {
		return publicingest.Rights{}, e
	}
	if d.AdmissionID != "" {
		var a publicingest.Admission
		var e error
		if tx != nil {
			a, e = publicingest.LockPolicy(ctx, tx, d.SourceID, d.AdmissionID)
		} else {
			a, e = publicingest.ResolvePolicy(ctx, s.Pool, d.SourceID, d.AdmissionID)
		}
		return a.Rights, e
	}
	// Independent reference imports carry their own original source-backed rights;
	// current policy is the newest version of that same dataset, never another feed.
	if tx != nil {
		resource, _ := access.ResourceFrom(ctx)
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+"/reference-import"); e != nil {
			return publicingest.Rights{}, e
		}
		var active bool
		if e := tx.QueryRow(ctx, `SELECT status='active' FROM sources WHERE id=$1 FOR SHARE`, d.SourceID).Scan(&active); e != nil || !active {
			return publicingest.Rights{}, apperr.Conflict("reference source unavailable")
		}
	}
	var original, current []byte
	e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT old.document->'rights',latest.document->'rights' FROM reference_datasets old JOIN sources s ON s.id=old.source_id AND s.scope_id=old.scope_id JOIN LATERAL(SELECT document FROM reference_datasets n WHERE n.source_id=old.source_id AND n.scope_id=old.scope_id AND n.dataset_key=old.dataset_key ORDER BY (document->>'approvedAt')::timestamptz DESC,id DESC LIMIT 1)latest ON true WHERE old.id=$1 AND s.status='active'`, d.ReferenceID).Scan(&original, &current)
	if e != nil {
		return publicingest.Rights{}, apperr.Conflict("reference output policy unavailable; source administrator action required")
	}
	var a, b publicingest.Rights
	if e = json.Unmarshal(original, &a); e != nil {
		return a, e
	}
	if e = json.Unmarshal(current, &b); e != nil {
		return a, e
	}
	a.OutputPermissions = publicingest.IntersectOutputPermissions(a.OutputPermissions, b.OutputPermissions)
	a.RetainRaw = a.RetainRaw && b.RetainRaw
	a.AllowAnalysis = a.AllowAnalysis && b.AllowAnalysis
	a.AllowSimulationExport = a.AllowSimulationExport && b.AllowSimulationExport
	if b.ExpiresAt != nil && (a.ExpiresAt == nil || b.ExpiresAt.Before(*a.ExpiresAt)) {
		a.ExpiresAt = b.ExpiresAt
	}
	return a, nil
}
func (s Service) outputEligibility(ctx context.Context, tx pgx.Tx, o Output, distribution bool) error {
	return s.checkOutput(ctx, tx, o, distribution, true)
}
func (s Service) checkOutput(ctx context.Context, tx pgx.Tx, o Output, distribution, requireHead bool) error {
	if !o.RetainUntil.After(time.Now()) || o.State == "TOMBSTONED" || o.State == "DELETE_PENDING" {
		return apperr.Conflict("output retention unavailable; retain its tombstone instead of regenerating bytes")
	}
	if tx != nil {
		var caseID string
		if e := tx.QueryRow(ctx, `SELECT id FROM investigations WHERE id=$1 FOR SHARE`, o.CaseID).Scan(&caseID); e != nil {
			return apperr.NotFound("output case", "unavailable")
		}
	}
	c, e := s.Get(ctx, o.CaseID, "")
	if e != nil {
		return e
	}
	if requireHead && (c.Revision.ID != o.RevisionID || c.Revision.State == "ARCHIVED") {
		return apperr.Conflict("output is superseded or archived; create a new draft from the intended revision")
	}
	if !requireHead && c.Revision.ID != o.RevisionID {
		c, e = s.Get(ctx, o.CaseID, o.RevisionID)
		if e != nil {
			return e
		}
	}
	if len(c.Notices) > 0 {
		return apperr.Conflict("current support changed; reassessment is required")
	}
	p, r, _ := access.From(ctx)
	if tx != nil {
		ok, e := s.liveHuman(ctx, tx, p.HumanID, r, "outputs.read", o.CaseID)
		if e != nil {
			return e
		}
		if !ok {
			return apperr.Forbidden("live output and all-source access required")
		}
	}
	var totalAssets int64
	for _, a := range o.Manifest.Assets {
		totalAssets += a.Bytes
	}
	if totalAssets > 8<<20 {
		return apperr.Conflict("included asset bytes exceed the bounded renderer profile")
	}
	for _, a := range o.Manifest.Assets {
		current, _, e := artifacts.NewService(s.Pool, s.Store).Read(ctx, a.ID)
		if e != nil {
			return e
		}
		if current.SHA256 != a.SHA256 || current.Bytes != a.Bytes {
			return apperr.Conflict("included asset identity changed")
		}
	}
	for _, d := range o.Manifest.Dependencies {
		rights, e := s.dependencyPolicy(ctx, tx, d)
		if e != nil {
			return e
		}
		if o.Manifest.Purpose == "internal" && !outputAllowed(rights, "internal") {
			return apperr.Forbidden("explicit internal analytical output permission required")
		}
		if o.Manifest.Purpose == "distribution" && (!outputAllowed(rights, "reference") || d.Use == "internal" || d.Use == "environment") {
			return apperr.Forbidden("distribution requires citation permission and permitted declared inclusions")
		}
		if o.Manifest.Environment != nil && !rights.RetainRaw {
			return apperr.Forbidden("portable source retention no longer permitted")
		}
		if o.Manifest.Purpose == "environment" && !outputAllowed(rights, "environment") {
			return apperr.Forbidden("geographic environment export requires its own explicit permission")
		}
		if !outputAllowed(rights, d.Use) || rights.ExpiresAt != nil && (!rights.ExpiresAt.After(time.Now()) || o.RetainUntil.After(*rights.ExpiresAt)) {
			return apperr.Forbidden("output permission unknown, denied or expired; source administrator must review this exact purpose/version")
		}
	}
	if distribution {
		if e := access.Require(ctx, "outputs.distribute"); e != nil {
			return e
		}
		v, e := s.latestReview(ctx, o.CaseID)
		if e != nil {
			return e
		}
		if v == nil || v.Action != "APPROVED" || v.RevisionID != o.RevisionID || v.OutputID != o.ID || v.OutputContentDigest != o.ContentDigest || v.RenderSHA256 != o.RenderSHA256 || v.ContentDigest != revisionDigest(c.Revision) {
			return apperr.Conflict("distribution requires an independent attestation of this exact rendered manifest")
		}
		if tx != nil {
			ok, e := s.liveHuman(ctx, tx, p.HumanID, r, "outputs.distribute", o.CaseID)
			if e != nil {
				return e
			}
			if !ok {
				return apperr.Forbidden("live distribution grant required")
			}
		}
	}
	return nil
}
func (s Service) outputDependencies(ctx context.Context, tx pgx.Tx, c Case, uses map[string]string) ([]OutputDependency, error) {
	rows, e := tx.Query(ctx, `SELECT document FROM investigation_revisions WHERE case_id=$1 ORDER BY version LIMIT 1001`, c.ID)
	if e != nil {
		return nil, e
	}
	refs := []Evidence{}
	n := 0
	for rows.Next() {
		n++
		var raw []byte
		var r Revision
		if e = rows.Scan(&raw); e != nil {
			rows.Close()
			return nil, e
		}
		if e = json.Unmarshal(raw, &r); e != nil {
			rows.Close()
			return nil, e
		}
		refs = append(refs, r.Evidence...)
		for _, f := range r.Findings {
			refs = append(refs, f.Evidence...)
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	if n > 1000 {
		return nil, apperr.Conflict("output history exceeds the supported bound; prepare a separately reviewed release")
	}
	dependencies := []OutputDependency{}
	seen := map[string]bool{}
	covered := map[string]bool{}
	add := func(source, admission, reference string) error {
		key := source + "/" + admission + "/" + reference
		if seen[key] {
			return nil
		}
		seen[key] = true
		covered[source] = true
		use := uses[source]
		if use == "" {
			use = "reference"
		}
		d := OutputDependency{SourceID: source, AdmissionID: admission, ReferenceID: reference, Use: use}
		rights, e := s.dependencyPolicy(ctx, tx, d)
		if e != nil {
			return e
		}
		if !outputAllowed(rights, use) {
			return apperr.Forbidden("explicit source-backed output purpose and metadata retention permission required")
		}
		d.Rights = rights
		d.EvaluatedAt = time.Now().UTC()
		var original []byte
		if admission != "" {
			a, e := publicingest.ResolvePolicy(ctx, s.Pool, source, admission)
			if e != nil {
				return e
			}
			d.CurrentPolicyID = a.CurrentAdmissionID
			if e := tx.QueryRow(ctx, `SELECT document->'rights' FROM source_admissions WHERE id=$1`, admission).Scan(&original); e != nil {
				return e
			}
		} else {
			if e := tx.QueryRow(ctx, `SELECT old.document->'rights',n.id FROM reference_datasets old JOIN LATERAL(SELECT id FROM reference_datasets x WHERE x.source_id=old.source_id AND x.scope_id=old.scope_id AND x.dataset_key=old.dataset_key ORDER BY (document->>'approvedAt')::timestamptz DESC,id DESC LIMIT 1)n ON true WHERE old.id=$1`, reference).Scan(&original, &d.CurrentPolicyID); e != nil {
				return e
			}
		}
		if e := json.Unmarshal(original, &d.OriginalRights); e != nil {
			return e
		}
		dependencies = append(dependencies, d)
		return nil
	}
	for _, ref := range refs {
		if len(dependencies) > 512 {
			return nil, apperr.Conflict("too many output rights dependencies")
		}
		policies, e := s.policies(ctx, tx, ref)
		if e != nil {
			return nil, e
		}
		for _, p := range policies {
			if e := add(p.SourceID, p.AdmissionID, ""); e != nil {
				return nil, e
			}
		}
		switch ref.Kind {
		case "observation", "artifact":
			table := "public_evidence"
			if ref.Kind == "artifact" {
				table = "artifacts"
			}
			var source, admission string
			if e = tx.QueryRow(ctx, `SELECT source_id,admission_id FROM `+table+` WHERE id=$1`, ref.ID).Scan(&source, &admission); e != nil {
				return nil, e
			}
			if e = add(source, admission, ""); e != nil {
				return nil, e
			}
		case "reference":
			if e = add(ref.SourceID, "", ref.ID); e != nil {
				return nil, e
			}
		case "analysis", "capture", "assessment":
			var raw []byte
			query := `SELECT input FROM imagery_jobs WHERE id=$1`
			if ref.Kind == "capture" {
				query = `SELECT input FROM imagery_capture_jobs WHERE id=$1`
			}
			if ref.Kind == "assessment" {
				query = `SELECT document FROM assessment_revisions WHERE id=$1`
			}
			if e = tx.QueryRow(ctx, query, ref.ID).Scan(&raw); e != nil {
				return nil, e
			}
			// Explicitly walk only typed lineage positions, never arbitrary source IDs
			// supplied in prose. Assessments freeze their imagery/report input documents.
			var walk func(any) error
			walk = func(v any) error {
				switch x := v.(type) {
				case map[string]any:
					if src, ok := x["sourceId"].(string); ok {
						if admission, ok := x["admissionId"].(string); ok && admission != "" {
							if e := add(src, admission, ""); e != nil {
								return e
							}
						}
					}
					for _, key := range []string{"scene", "scenes", "artifacts", "input", "evidence", "reportEvidence"} {
						if child, ok := x[key]; ok {
							if e := walk(child); e != nil {
								return e
							}
						}
					}
				case []any:
					for _, child := range x {
						if e := walk(child); e != nil {
							return e
						}
					}
				}
				return nil
			}
			var doc any
			if e = json.Unmarshal(raw, &doc); e != nil {
				return nil, e
			}
			if e = walk(doc); e != nil {
				return nil, e
			}
		default:
			return nil, apperr.Conflict("legacy input-only analysis must be replaced with an exact completed result")
		}
	}
	var required []byte
	if e = tx.QueryRow(ctx, `SELECT required_sources FROM investigations WHERE id=$1`, c.ID).Scan(&required); e != nil {
		return nil, e
	}
	var sources []string
	_ = json.Unmarshal(required, &sources)
	for _, source := range sources {
		if !covered[source] {
			return nil, apperr.Forbidden("a required source has no versioned output-rights lineage; source administrator review is required")
		}
	}
	for source := range uses {
		if !covered[source] {
			return nil, apperr.Validation("output use refers to a source outside the frozen case")
		}
	}
	return dependencies, nil
}
func (s Service) CreateOutput(ctx context.Context, caseID string, in OutputInput) (Output, error) {
	if e := require(ctx, "outputs.create"); e != nil {
		return Output{}, e
	}
	if e := access.Require(ctx, "outputs.read"); e != nil {
		return Output{}, e
	}
	if in.RequestKey == "" || len(in.RequestKey) > 128 || in.ExpectedBytes < 1 || in.ExpectedBytes > 8<<20 || in.RendererVersion == "" || len(in.RendererVersion) > 200 || in.StyleVersion == "" || len(in.StyleVersion) > 200 || !in.RetainUntil.After(time.Now()) || in.RetainUntil.After(time.Now().Add(366*24*time.Hour)) || len(in.AssetIDs) > 100 {
		return Output{}, apperr.Validation("bounded manifest intent, renderer/style and retention required")
	}
	if in.Purpose != "internal" && in.Purpose != "distribution" && in.Purpose != "environment" {
		return Output{}, apperr.Validation("choose internal, distribution or environment purpose")
	}
	in.RetainUntil = in.RetainUntil.UTC().Truncate(time.Microsecond)
	raw, _ := jsoncanon.Marshal(in)
	hash := areas.Hash(raw)
	p, r, _ := access.From(ctx)
	id := "output_" + areas.Hash([]byte(r.ID()+p.HumanID+in.RequestKey))
	e := db.JoinTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, id); e != nil {
			return e
		}
		var old string
		e := tx.QueryRow(ctx, `SELECT input_hash FROM composite_outputs WHERE id=$1`, id).Scan(&old)
		if e == nil {
			if old != hash {
				return apperr.Conflict("output intent reused with different input")
			}
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		var head string
		if e = tx.QueryRow(ctx, `SELECT head_revision_id FROM investigations WHERE id=$1 FOR UPDATE`, caseID).Scan(&head); e != nil {
			return e
		}
		if head != in.ExpectedRevisionID {
			return apperr.Conflict("case changed before output freeze")
		}
		c, e := s.Get(ctx, caseID, "")
		if e != nil {
			return e
		}
		if c.Revision.State == "ARCHIVED" || len(c.Notices) > 0 {
			return apperr.Conflict("case requires reassessment before output freeze")
		}
		deps, e := s.outputDependencies(ctx, tx, c, in.SourceUses)
		if e != nil {
			return e
		}
		manifest := OutputManifest{RetainUntil: in.RetainUntil, Schema: "emad.composite-output.v1", Case: c.Revision, Dependencies: deps, Assets: []artifacts.Artifact{}, RendererVersion: in.RendererVersion, StyleVersion: in.StyleVersion, Purpose: in.Purpose, Omissions: []string{}}
		admitted := map[string]bool{}
		for _, ref := range c.Revision.Evidence {
			if ref.Kind == "artifact" {
				admitted[ref.ID] = true
			}
		}
		for _, f := range c.Revision.Findings {
			for _, ref := range f.Evidence {
				if ref.Kind == "artifact" {
					admitted[ref.ID] = true
				}
			}
		}
		for _, id := range in.AssetIDs {
			if !admitted[id] {
				return apperr.Validation("included assets must be explicitly cited by this case revision")
			}
			a, e := artifacts.NewService(s.Pool, s.Store).Ready(ctx, id)
			if e != nil {
				return e
			}
			policies, e := s.policies(ctx, tx, Evidence{Kind: "artifact", ID: id})
			if e != nil {
				return e
			}
			allowed := len(policies) > 0
			for _, policy := range policies {
				permitted := false
				for _, d := range deps {
					if d.SourceID == policy.SourceID && d.AdmissionID == policy.AdmissionID && (d.Use == "raw" || d.Use == "imagery" && strings.HasPrefix(a.MediaType, "image/") || d.Use == "environment" && in.Purpose == "environment") {
						permitted = true
					}
				}
				allowed = allowed && permitted
			}
			if !allowed || in.RetainUntil.After(a.RetainUntil) {
				return apperr.Forbidden("included asset permission/retention is insufficient")
			}
			manifest.Assets = append(manifest.Assets, a)
		}
		for _, d := range deps {
			if d.Use == "reference" {
				manifest.Omissions = append(manifest.Omissions, "External dependency: "+d.SourceID+" is cited by exact retained identity; raw bytes are not included.")
			}
			if d.Rights.ExpiresAt != nil && in.RetainUntil.After(*d.Rights.ExpiresAt) {
				return apperr.Forbidden("output retention exceeds a dependency's permission")
			}
		}
		if in.DeliveryKind == "brief" {
			snap, e := s.freezeBrief(ctx, tx, c)
			if e != nil {
				return e
			}
			manifest.Brief = &snap
		}
		if in.DeliveryKind == "environment" {
			if in.Environment == nil {
				return apperr.Validation("environment qualification required")
			}
			snap, e := s.freezeEnvironment(ctx, tx, c, *in.Environment)
			if e != nil {
				return e
			}
			var imageryBytes, imageryPixels int64
			for _, a := range manifest.Assets {
				if a.Kind != "imagery_subset" || a.MediaType != "application/json" || a.HorizontalAccuracyM == nil || *a.HorizontalAccuracyM > in.Environment.Qualification.HorizontalToleranceM {
					return apperr.Validation("environment imagery requires a qualified georeferenced multispectral subset with admitted horizontal accuracy")
				}
				imageryBytes += a.Bytes
				estimate := a.Bytes / 6
				if estimate > 65536 {
					estimate = 65536
				}
				imageryPixels += estimate
			}
			if imageryBytes > 4<<20 || snap.EstimatedRAM+imageryBytes*8 > in.Environment.Qualification.MaxRAMBytes || snap.EstimatedBytes+imageryBytes*5 > int64(in.Environment.Qualification.MaxBytes) || imageryPixels+int64(snap.EstimatedPixels) > int64(in.Environment.Qualification.MaxPixels) || int64(snap.GeometryVertices)*(imageryPixels+int64(snap.EstimatedPixels)) > 8000000 {
				return apperr.Validation("imagery estimates exceed environment limits")
			}
			if imageryBytes > 0 {
				snap.EstimatedRAM += imageryBytes * 8
				snap.EstimatedBytes += imageryBytes * 5
				snap.Omissions = append(snap.Omissions[:2], "Only explicitly included multispectral grids are present. Their acquisition dates, surface-reflectance units and source quality classes are preserved; no cloud or radiometric correction is inferred.", "Imagery covers only its retained grid footprint; outside and boundary-crossing pixels claim no coverage.", "Visual GLB omitted; neutral georeferenced files remain authoritative.")
			}
			manifest.Environment = &snap
		}
		if in.DeliveryKind != "" && in.DeliveryKind != "brief" && in.DeliveryKind != "environment" {
			return apperr.Validation("unsupported delivery kind")
		}
		var assetBytes int64
		for _, a := range manifest.Assets {
			assetBytes += a.Bytes
		}
		if assetBytes > 8<<20 {
			return apperr.Validation("included assets exceed 8 MiB")
		}
		provisional := Output{CaseID: caseID, RevisionID: head, State: "FROZEN", RetainUntil: in.RetainUntil, Manifest: manifest}
		allowed, err := s.liveHuman(ctx, tx, p.HumanID, r, "outputs.create", caseID)
		if err != nil {
			return err
		}
		if !allowed {
			return apperr.Forbidden("live output creation grant required")
		}
		if e = s.outputEligibility(ctx, tx, provisional, false); e != nil {
			return e
		}
		doc, _ := jsoncanon.Marshal(manifest)
		digest := areas.Hash(doc)
		var manifestBytes int64
		if e = tx.QueryRow(ctx, `SELECT octet_length($1::jsonb::text)`, doc).Scan(&manifestBytes); e != nil {
			return e
		}
		if e = capacity.Check(ctx, tx, in.ExpectedBytes+manifestBytes, ""); e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO composite_outputs(id,case_id,revision_id,content_digest,creator_human_id,request_key,input_hash,document,size_bytes,retain_until)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, caseID, head, digest, p.HumanID, in.RequestKey, hash, doc, in.ExpectedBytes, in.RetainUntil)
		return e
	})
	if e != nil {
		return Output{}, e
	}
	o, e := s.output(ctx, id)
	if e != nil {
		return o, e
	}
	if e = s.checkOutput(ctx, nil, o, false, false); e != nil {
		return o, e
	}
	return o, nil
}
func (s Service) outputEvent(ctx context.Context, tx pgx.Tx, o Output, action string) error {
	p, _, _ := access.From(ctx)
	raw, _ := json.Marshal(map[string]any{"action": action, "actorHumanId": p.HumanID, "contentDigest": o.ContentDigest, "renderSha256": o.RenderSHA256, "recordedAt": time.Now().UTC()})
	_, e := tx.Exec(ctx, `INSERT INTO composite_output_events(id,case_id,output_id,document)VALUES($1,$2,$3,$4)`, ids.New("output_event"), o.CaseID, o.ID, raw)
	return e
}
func (s Service) StageOutput(ctx context.Context, id, media string, raw []byte) (Output, error) {
	return s.stageOutput(ctx, id, media, raw, false)
}
func (s Service) stageOutput(ctx context.Context, id, media string, raw []byte, delivery bool) (Output, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	if e := require(ctx, "outputs.create"); e != nil {
		return Output{}, e
	}
	o, e := s.output(ctx, id)
	if e != nil {
		return o, e
	}
	if !delivery && (o.Manifest.Brief != nil || o.Manifest.Environment != nil) {
		return o, apperr.Validation("durable outputs can only be staged by their leased renderer")
	}
	if len(raw) == 0 || len(raw) > 8<<20 {
		return o, apperr.Validation("bounded rendered bytes required")
	}
	switch media {
	case "text/html", "application/zip":
		if !delivery {
			return o, apperr.Validation("HTML/packages require a durable delivery job")
		}
	case "application/json":
		if !json.Valid(raw) {
			return o, apperr.Validation("invalid render manifest JSON")
		}
	case "image/png":
		c, e := png.DecodeConfig(bytes.NewReader(raw))
		if e != nil || c.Width > 4096 || c.Height > 4096 || int64(c.Width)*int64(c.Height) > 4<<20 {
			return o, apperr.Validation("invalid bounded PNG figure (4096 per side, 4 megapixels)")
		}
		if _, e := png.Decode(bytes.NewReader(raw)); e != nil {
			return o, apperr.Validation("incomplete or invalid PNG figure")
		}
	default:
		return o, apperr.Validation("foundation accepts JSON render plans or PNG figures; active HTML/archives require the T19 renderer")
	}
	hash := artifacts.Hash(raw)
	if o.State == "READY" {
		if o.RenderSHA256 != hash || o.MediaType != media {
			return o, apperr.Conflict("render identity is immutable; freeze a new output for changed content")
		}
		return o, s.outputEligibility(ctx, nil, o, false)
	}
	if o.State != "FROZEN" || int64(len(raw)) > o.Bytes {
		return o, apperr.Conflict("render exceeds the frozen reservation or output state")
	}
	if e = s.outputEligibility(ctx, nil, o, false); e != nil {
		return o, e
	}
	r, _ := access.ResourceFrom(ctx)
	key := "emad/composite/" + areas.Hash([]byte(r.ID())) + "/" + id + "/" + hash
	if s.Store == nil {
		return o, apperr.Conflict("private output storage unavailable")
	}
	uploadID, e := s.planUpload(ctx, o, key, hash)
	if e != nil {
		return o, e
	}
	if e = s.Store.Put(ctx, key, raw, media); e != nil {
		return o, apperr.Conflict("output storage unavailable; same-byte retry is permitted")
	}
	s.uploadFinished(ctx, o, uploadID)
	ctx, e = access.Refresh(ctx)
	if e != nil {
		return o, e
	}
	e = db.JoinTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if delivery {
			if e := s.guardDelivery(ctx, tx, id); e != nil {
				return e
			}
		}
		if _, e := tx.Exec(ctx, `SELECT id FROM composite_outputs WHERE id=$1 FOR UPDATE`, id); e != nil {
			return e
		}
		live, e := s.output(ctx, id)
		if e != nil {
			return e
		}
		p, r, _ := access.From(ctx)
		allowed, err := s.liveHuman(ctx, tx, p.HumanID, r, "outputs.create", live.CaseID)
		if err != nil {
			return err
		}
		if !allowed {
			return apperr.Forbidden("live output creation grant required")
		}
		if e = s.outputEligibility(ctx, tx, live, false); e != nil {
			return e
		}
		if live.State == "READY" {
			if live.RenderSHA256 != hash || live.MediaType != media {
				return apperr.Conflict("another rendered identity committed")
			}
			return nil
		}
		if live.State != "FROZEN" {
			return apperr.Conflict("output no longer accepts bytes")
		}
		_, e = tx.Exec(ctx, `UPDATE composite_outputs SET state='READY',render_sha256=$2,media_type=$3,size_bytes=$4,object_key=$5,render_human_id=$6 WHERE id=$1`, id, hash, media, len(raw), key, func() string { p, _, _ := access.From(ctx); return p.HumanID }())
		if e != nil {
			return e
		}
		o.RenderSHA256 = hash
		if e := s.outputEvent(ctx, tx, o, "RENDERED"); e != nil {
			return e
		}
		if delivery {
			return s.finishDelivery(ctx, tx)
		}
		return nil
	})
	if e != nil {
		return o, e
	}
	return s.output(ctx, id)
}
func (s Service) ReadOutput(ctx context.Context, id string, distribute bool) (Output, []byte, error) {
	if e := require(ctx, "outputs.read"); e != nil {
		return Output{}, nil, e
	}
	o, e := s.output(ctx, id)
	if e != nil {
		return o, nil, e
	}
	if o.State != "READY" {
		return o, nil, apperr.Conflict("output bytes unavailable")
	}
	if e = s.checkOutput(ctx, nil, o, distribute, distribute); e != nil {
		return o, nil, e
	}
	if s.Store == nil {
		return o, nil, apperr.Conflict("output storage unavailable")
	}
	raw, e := s.Store.Get(ctx, o.ObjectKey, o.Bytes)
	if e != nil {
		return o, nil, apperr.Conflict("output storage temporarily unavailable; no historical bytes were replaced")
	}
	if int64(len(raw)) != o.Bytes || artifacts.Hash(raw) != o.RenderSHA256 {
		return o, nil, apperr.Conflict("output integrity failure; reassessment required")
	}
	ctx, e = access.Refresh(ctx)
	if e != nil {
		return o, nil, e
	}
	e = db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if e := s.checkOutput(ctx, tx, o, distribute, distribute); e != nil {
			return e
		}
		if distribute {
			return s.outputEvent(ctx, tx, o, "DISTRIBUTED")
		}
		return nil
	})
	return o, raw, e
}
func (s Service) ExpireOutput(ctx context.Context, id string) error {
	if e := require(ctx, "outputs.create"); e != nil {
		return e
	}
	o, e := s.output(ctx, id)
	if e != nil {
		return e
	}

	if o.RetainUntil.After(time.Now()) {
		return apperr.Conflict("output is within its frozen retention period")
	}
	if _, e = db.Scoped(s.Pool).Exec(ctx, `UPDATE composite_outputs SET state='DELETE_PENDING',unavailable_reason='retention_expired' WHERE id=$1 AND state IN('FROZEN','READY')`, id); e != nil {
		return e
	}
	keys, e := s.expiredKeys(ctx, o)
	if e != nil {
		return e
	}
	for _, key := range keys {
		if s.Store == nil || s.Store.Delete(ctx, key) != nil {
			return apperr.Conflict("output deletion pending; retry safely")
		}
	}
	if o.State == "TOMBSTONED" {
		return nil
	}

	return db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `UPDATE composite_outputs SET state='TOMBSTONED',document=jsonb_build_object('contentDigest',content_digest,'caseRevisionId',revision_id,'tombstone',true) WHERE id=$1 AND state='DELETE_PENDING'`, id); e != nil {
			return e
		}
		return s.outputEvent(ctx, tx, o, "TOMBSTONED")
	})
}
func (s Service) mountOutputs(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
				defer cancel()
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		r.Post("/investigations/{id}/outputs", func(w http.ResponseWriter, r *http.Request) {
			var in OutputInput
			if !areas.Decode(w, r, &in) {
				return
			}
			if in.DeliveryKind != "" || in.Environment != nil {
				httpx.Error(w, apperr.Validation("use the durable delivery job endpoint"))
				return
			}
			v, e := s.CreateOutput(r.Context(), chi.URLParam(r, "id"), in)
			if e != nil {
				httpx.Error(w, e)
				return
			}
			httpx.JSON(w, 201, v)
		})
		r.Get("/outputs/{id}", func(w http.ResponseWriter, r *http.Request) {
			if e := require(r.Context(), "outputs.read"); e != nil {
				httpx.Error(w, e)
				return
			}
			v, e := s.output(r.Context(), chi.URLParam(r, "id"))
			if e != nil {
				httpx.Error(w, e)
				return
			}
			if e := s.checkOutput(r.Context(), nil, v, false, false); e != nil {
				v.UnavailableReason = "current_eligibility_unavailable"
				v.Manifest = OutputManifest{}
			}
			if !v.RetainUntil.After(time.Now()) {
				v.Manifest = OutputManifest{}
				v.UnavailableReason = "retention_expired"
			}
			if c, err := s.Get(r.Context(), v.CaseID, ""); err == nil {
				v.Superseded = c.Revision.ID != v.RevisionID
			}
			httpx.JSON(w, 200, v)
		})
		r.Put("/outputs/{id}/content", func(w http.ResponseWriter, r *http.Request) {
			raw, e := io.ReadAll(io.LimitReader(r.Body, (8<<20)+1))
			if e != nil {
				httpx.Error(w, e)
				return
			}
			v, e := s.StageOutput(r.Context(), chi.URLParam(r, "id"), strings.Split(r.Header.Get("Content-Type"), ";")[0], raw)
			if e != nil {
				httpx.Error(w, e)
				return
			}
			httpx.JSON(w, 200, v)
		})
		r.Get("/outputs/{id}/content", func(w http.ResponseWriter, r *http.Request) {
			v, raw, e := s.ReadOutput(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("distribution") == "true")
			if e != nil {
				httpx.Error(w, e)
				return
			}
			w.Header().Set("Content-Type", v.MediaType)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src data:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Disposition", `attachment; filename="reviewed-output"`)
			w.Write(raw)
		})
		r.Post("/outputs/{id}/expire", func(w http.ResponseWriter, r *http.Request) {
			if e := s.ExpireOutput(r.Context(), chi.URLParam(r, "id")); e != nil {
				httpx.Error(w, e)
				return
			}
			httpx.JSON(w, 200, map[string]string{"state": "TOMBSTONED"})
		})
	})
}
