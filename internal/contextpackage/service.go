package contextpackage

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imagery"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool       *pgxpool.Pool
	references *reference.Service
	images     *imagery.Service
}

func NewService(pool *pgxpool.Pool, stores ...*artifacts.Service) *Service {
	blobs := artifacts.NewService(pool, nil)
	if len(stores) > 0 {
		blobs = stores[0]
	}
	return &Service{pool: pool, references: reference.NewService(pool), images: imagery.NewService(pool, blobs)}
}

type BuildInput struct {
	Scope                 *analystscope.Input `json:"scope,omitempty"`
	PreparationIDs        []string            `json:"preparationIds,omitempty"`
	AssessmentRevisionIDs []string            `json:"assessmentRevisionIds,omitempty"`
	PreviewID             string              `json:"previewId,omitempty"`
	PreviewSHA256         string              `json:"previewSha256,omitempty"`
	DatasetIDs            []string            `json:"datasetIds"`
	AOI                   publicingest.Bounds `json:"aoi"`
	TimeWindow            Window              `json:"timeWindow"`
	AcceptedQuality       string              `json:"acceptedQuality"`
	Approval              string              `json:"approval"`
	ReferenceOnly         bool                `json:"referenceOnly"`
}

func require(ctx context.Context) error {
	if err := access.Require(ctx, "context.export"); err != nil {
		return err
	}
	r, _ := access.ResourceFrom(ctx)
	if r.Partition != access.PublicEvidence {
		return apperr.Forbidden("context export requires PUBLIC_EVIDENCE")
	}
	return nil
}

// Build is an explicit human approval action. Metadata/rights come from reviewed
// immutable catalogue versions, never untrusted client-supplied copies.
func (s *Service) Build(ctx context.Context, in BuildInput) (Bundle, error) {
	if in.PreviewID != "" {
		return s.freeze(ctx, in)
	}
	if len(in.PreparationIDs) > 0 || len(in.AssessmentRevisionIDs) > 0 {
		return Bundle{}, apperr.Validation("preview and approve the pinned selection before imagery export")
	}
	b, err := s.assemble(ctx, in)
	if err != nil {
		return b, err
	}
	err = s.persist(ctx, &b)
	return b, err
}
func (s *Service) assemble(ctx context.Context, in BuildInput) (Bundle, error) {
	if err := require(ctx); err != nil {
		return Bundle{}, err
	}
	p, _, _ := access.From(ctx)
	if p.Kind != "human" {
		return Bundle{}, apperr.Forbidden("human package approval required")
	}
	if len(in.DatasetIDs)+len(in.PreparationIDs)+len(in.AssessmentRevisionIDs) < 1 || len(in.DatasetIDs)+len(in.PreparationIDs)+len(in.AssessmentRevisionIDs) > 16 || in.AOI.Validate() != nil || in.AcceptedQuality == "" || len(in.AcceptedQuality) > 2000 || in.Approval == "" || len(in.Approval) > 2000 {
		return Bundle{}, apperr.Validation("bounded dataset selection, quality acceptance and approval required")
	}
	now := time.Now().UTC()
	m := Manifest{Schema: Schema, AOI: in.AOI, TimeWindow: in.TimeWindow, CreatedAt: now, Creator: p.ID, Approval: Approval{p.ID, now, in.Approval}, TransformVersion: simulation.TransformVersion, ProcessingVersion: "reference-snapshot-v1", GeometryQuality: in.AcceptedQuality, ImageryQuality: "not_included", Reproduction: "retained_offline", Assets: []Asset{}}
	b := Bundle{Manifest: m, Assets: map[string][]byte{}}
	if in.Scope != nil {
		v, e := (analystscope.Service{Pool: s.pool}).Resolve(ctx, *in.Scope)
		if e != nil {
			return b, e
		}
		if v.Capabilities["imagery"].State != "supported" || !v.Temporal.Interval.From.Equal(in.TimeWindow.From) || !v.Temporal.Interval.To.Equal(in.TimeWindow.To) {
			return b, apperr.Validation("export interval must equal its explicitly pinned scope interval")
		}
		if e = analystscope.ValidateBounds(ctx, s.pool, v, in.AOI); e != nil {
			return b, e
		}
		b.Manifest.Scope = &v
	}
	ids := append([]string(nil), in.DatasetIDs...)
	sort.Strings(ids)
	for _, id := range ids {
		d, err := s.references.Get(ctx, id)
		if err != nil {
			return Bundle{}, err
		}
		if !d.EligibleForSimulation {
			return Bundle{}, apperr.Validation("reference rights do not permit simulation export")
		}
		if in.AOI.West < d.AOI.West || in.AOI.East > d.AOI.East || in.AOI.South < d.AOI.South || in.AOI.North > d.AOI.North {
			return Bundle{}, apperr.Validation("package AOI must be within every selected dataset; artifacts are retained without implicit clipping")
		}
		a := Asset{Kind: d.Kind, DatasetAOI: d.AOI, ParentArtifactSHA256: d.ParentArtifactSHA256, DatasetID: d.ID, SourceID: d.SourceID, DatasetKey: d.DatasetKey, Version: d.Version, OriginalProducer: d.OriginalProducer, Reference: d.RetrievalReference, RetrievedAt: d.RetrievedAt, SHA256: d.SHA256, Bytes: d.Bytes, CRS: d.CRS, VerticalDatum: d.VerticalDatum, Resolution: d.Resolution, Quality: d.Quality, Validity: d.Validity, Processing: d.Transformation, Rights: d.Rights, Retention: "retained"}
		if in.ReferenceOnly || !d.Rights.RetainRaw {
			a.Retention = "reference_only"
			b.Manifest.Reproduction = "external_dependencies"
		} else {
			raw, err := s.references.Artifact(ctx, id)
			if err != nil {
				return Bundle{}, err
			}
			b.Assets[a.SHA256] = raw
		}
		b.Manifest.Assets = append(b.Manifest.Assets, a)
	}
	if len(in.PreparationIDs) > 0 || len(in.AssessmentRevisionIDs) > 0 {
		if err := s.addImagery(ctx, &b, in); err != nil {
			return Bundle{}, err
		}
	}
	b.ID = ID(b.Manifest)
	if err := b.Validate(now, false); err != nil {
		return Bundle{}, apperr.Validation(err.Error())
	}
	return b, nil
}
func (s *Service) persist(ctx context.Context, b *Bundle) error {
	fresh, err := refreshExport(ctx)
	if err != nil {
		return err
	}
	ctx = fresh
	_, r, _ := access.From(ctx)
	err = platformdb.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if b.Manifest.Scope != nil {
			saved, e := (analystscope.Service{Pool: s.pool}).Freeze(ctx, tx, analystscope.FromSnapshot(*b.Manifest.Scope))
			if e != nil {
				return e
			}
			if saved.ScopeHash != b.Manifest.Scope.ScopeHash {
				return apperr.Conflict("export scope changed; preview again")
			}
			b.Manifest.Scope = &saved
			b.ID = ID(b.Manifest)
		}
		raw, _ := json.Marshal(b)
		doc, _ := json.Marshal(b.Manifest)
		if err := s.validateFrozenDependencies(ctx, tx, *b); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+"/context-packages"); err != nil {
			return err
		}
		var count, total int64
		if err := tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(octet_length(bundle)),0) FROM context_packages`).Scan(&count, &total); err != nil {
			return err
		}
		if count >= 64 || total+int64(len(raw)) > 64*1024*1024 {
			return apperr.Conflict("context package budget exceeded")
		}
		_, err := tx.Exec(ctx, `INSERT INTO context_packages(id,document,bundle) VALUES($1,$2,$3)`, b.ID, doc, raw)
		return err
	})
	return err
}
func (s *Service) Load(ctx context.Context, id string) (Bundle, error) {
	if err := require(ctx); err != nil {
		return Bundle{}, err
	}
	var raw []byte
	var b Bundle
	err := platformdb.Scoped(s.pool).QueryRow(ctx, `SELECT bundle FROM context_packages WHERE id=$1 AND document->>'schema' IS DISTINCT FROM $2`, id, geographicSchema).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, apperr.NotFound("context package", "unavailable")
	}
	if err != nil {
		return b, err
	}
	if err = json.Unmarshal(raw, &b); err != nil {
		return b, err
	}
	if err = b.Validate(time.Now(), false); err != nil {
		return b, apperr.Conflict(err.Error())
	}
	return b, nil
}
func (s *Service) List(ctx context.Context) ([]map[string]any, error) {
	if err := require(ctx); err != nil {
		return nil, err
	}
	rows, err := platformdb.Scoped(s.pool).Query(ctx, `SELECT id,document FROM context_packages WHERE document->>'schema' IS DISTINCT FROM $1 AND emad_content_visible('package',id) ORDER BY id LIMIT 64`, geographicSchema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id string
		var raw []byte
		if err = rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		var m Manifest
		if err = json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "manifest": m})
	}
	return out, rows.Err()
}

func refreshExport(ctx context.Context) (context.Context, error) {
	expected, err := access.ResourceFrom(ctx)
	if err != nil {
		return nil, err
	}
	fresh, err := access.RefreshWithContext(ctx, ctx)
	if err != nil {
		return nil, err
	}
	current, err := access.ResourceFrom(fresh)
	if err != nil {
		return nil, err
	}
	if current != expected {
		return nil, apperr.Conflict("resource changed; select and preview the context again")
	}
	if err := require(fresh); err != nil {
		return nil, err
	}
	p, _, _ := access.From(fresh)
	if p.Kind != "human" {
		return nil, apperr.Forbidden("human package approval required")
	}
	return fresh, nil
}
