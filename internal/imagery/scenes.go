// Package imagery owns a bounded public catalogue and asynchronous analysis jobs.
package imagery

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Scene struct {
	publicingest.SceneMetadata
	ID               string              `json:"id"`
	SourceID         string              `json:"sourceId"`
	EvidenceID       string              `json:"evidenceId"`
	AdmissionID      string              `json:"admissionId"`
	ContentHash      string              `json:"contentHash"`
	RetrievedAt      time.Time           `json:"retrievedAt"`
	OriginalProducer string              `json:"originalProducer"`
	Provider         string              `json:"provider"`
	Rights           publicingest.Rights `json:"rights"`
}
type Projector struct{ Next publicingest.Projector }

func (p Projector) Project(ctx context.Context, tx pgx.Tx, e publicingest.Evidence) error {
	if p.Next != nil {
		if err := p.Next.Project(ctx, tx, e); err != nil {
			return err
		}
	}
	if e.NormalizerVersion != publicingest.STACAdapter && e.NormalizerVersion != publicingest.EarthSearchAdapter {
		return nil
	}
	var m publicingest.SceneMetadata
	if json.Unmarshal(e.Scene, &m) != nil || m.Schema != "emad.scene.v1" {
		return apperr.Validation("invalid normalized scene")
	}
	var admissionRaw []byte
	if err := tx.QueryRow(ctx, `SELECT document FROM source_admissions WHERE id=$1`, e.AdmissionID).Scan(&admissionRaw); err != nil {
		return err
	}
	var admission publicingest.Admission
	if err := json.Unmarshal(admissionRaw, &admission); err != nil {
		return err
	}
	s := Scene{SceneMetadata: m, ID: "scene_" + e.ID, SourceID: e.SourceID, EvidenceID: e.ID, AdmissionID: e.AdmissionID, ContentHash: e.ContentHash, RetrievedAt: e.RetrievedAt, OriginalProducer: e.OriginalProducer, Provider: e.UpstreamProvider, Rights: admission.Rights}
	raw, _ := json.Marshal(s)
	_, err := tx.Exec(ctx, `INSERT INTO imagery_scenes(id,source_id,evidence_id,admission_id,provider_id,acquired_at,geometry,document)VALUES($1,$2,$3,$4,$5,$6,ST_SetSRID(ST_GeomFromGeoJSON($7),4326),$8) ON CONFLICT(evidence_id)DO NOTHING`, s.ID, s.SourceID, s.EvidenceID, s.AdmissionID, s.ProviderID, s.AcquiredAt, string(s.Geometry), raw)
	return err
}

type Service struct {
	Pool      *pgxpool.Pool
	Artifacts *artifacts.Service
}

func NewService(p *pgxpool.Pool, a *artifacts.Service) *Service { return &Service{p, a} }
func (s *Service) Scene(ctx context.Context, id string) (Scene, error) {
	var out Scene
	if e := artifacts.Public(ctx, "sources.read"); e != nil {
		return out, e
	}
	var raw []byte
	e := platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT document FROM imagery_scenes WHERE id=$1`, id).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, apperr.NotFound("imagery scene", "unavailable")
	}
	if e != nil {
		return out, e
	}
	e = json.Unmarshal(raw, &out)
	return out, e
}
func (s *Service) Scenes(ctx context.Context, bounds *publicingest.Bounds, regions ...string) ([]Scene, error) {
	if e := artifacts.Public(ctx, "sources.read"); e != nil {
		return nil, e
	}
	region := ""
	if len(regions) > 0 {
		region = regions[0]
	}
	if len(region) > 256 {
		return nil, apperr.Validation("invalid region")
	}
	geometry := ""
	if len(regions) > 1 {
		geometry = regions[1]
	}
	if geometry != "" {
		if e := publicevents.ValidateStudyGeometry(geometry, false); e != nil {
			return nil, e
		}
	}
	b := publicingest.Bounds{}
	if bounds != nil {
		b = *bounds
		if e := b.Validate(); e != nil {
			return nil, e
		}
	}
	rows, e := platformdb.Scoped(s.Pool).Query(ctx, `SELECT document FROM imagery_scenes WHERE ($1::boolean OR ST_Intersects(geometry,ST_MakeEnvelope($2,$3,$4,$5,4326))) AND ($6::text='' OR EXISTS(SELECT 1 FROM geographic_regions g WHERE g.id=$6 AND ST_Intersects(g.geometry,imagery_scenes.geometry))) AND ($7::text='' OR ST_Intersects(imagery_scenes.geometry,ST_SetSRID(ST_GeomFromGeoJSON(NULLIF($7,'')),4326))) ORDER BY acquired_at DESC,id LIMIT 101`, bounds == nil, b.West, b.South, b.East, b.North, region, geometry)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Scene{}
	for rows.Next() {
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		var scene Scene
		if e = json.Unmarshal(raw, &scene); e != nil {
			return nil, e
		}
		out = append(out, scene)
	}
	return out, rows.Err()
}
