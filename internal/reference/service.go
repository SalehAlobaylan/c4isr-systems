package reference

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool} }
func require(ctx context.Context, permission string) error {
	if err := access.Require(ctx, permission); err != nil {
		return err
	}
	r, err := access.ResourceFrom(ctx)
	if err != nil {
		return err
	}
	if r.Partition != access.PublicEvidence {
		return apperr.Forbidden("reference catalog requires PUBLIC_EVIDENCE")
	}
	return nil
}
func eligible(d Dataset) bool {
	return d.Rights.AllowSimulationExport && (d.Rights.ExpiresAt == nil || d.Rights.ExpiresAt.After(time.Now()))
}
func (s *Service) Import(ctx context.Context, in Input) (Dataset, bool, error) {
	if err := require(ctx, "sources.manage"); err != nil {
		return Dataset{}, false, err
	}
	p, r, _ := access.From(ctx)
	if p.Kind != "human" {
		return Dataset{}, false, apperr.Forbidden("reference import requires a human reviewer")
	}
	if err := access.RequireSource(ctx, in.SourceID); err != nil {
		return Dataset{}, false, err
	}
	collection, err := in.validate()
	if err != nil {
		return Dataset{}, false, err
	}
	spec, _ := json.Marshal(in)
	specHash := Hash(spec)
	raw := in.Artifact
	in.Artifact = nil
	d := Dataset{Input: in, ID: ids.New("ref"), ApprovedBy: p.ID, ApprovedAt: time.Now().UTC(), Bytes: len(raw), FeatureCount: len(collection.Features)}
	duplicate := false
	err = platformdb.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+"/reference-import"); err != nil {
			return err
		}
		var sourceExists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sources WHERE id=$1)`, in.SourceID).Scan(&sourceExists); err != nil {
			return err
		}
		if !sourceExists {
			return apperr.NotFound("source", "unavailable")
		}
		rights, _ := json.Marshal(in.Rights)
		var changedRights bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM reference_datasets WHERE source_id=$1 AND document->'rights'->>'id'=$2 AND document->'rights'<>$3::jsonb)`, in.SourceID, in.Rights.ID, rights).Scan(&changedRights); err != nil {
			return err
		}
		if changedRights {
			return apperr.Conflict("changed reference rights require a new rights ID")
		}
		var oldHash string
		var old []byte
		err := tx.QueryRow(ctx, `SELECT spec_hash,document FROM reference_datasets WHERE source_id=$1 AND dataset_key=$2 AND version=$3`, in.SourceID, in.DatasetKey, in.Version).Scan(&oldHash, &old)
		if err == nil {
			if oldHash != specHash {
				return apperr.Conflict("dataset version is immutable; use a new version")
			}
			duplicate = true
			return json.Unmarshal(old, &d)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var bytes, count int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(octet_length(artifact)),0),count(*) FROM reference_datasets WHERE source_id=$1`, in.SourceID).Scan(&bytes, &count); err != nil {
			return err
		}
		if count >= 64 || bytes+int64(len(raw)) > 32*1024*1024 {
			return apperr.Conflict("reference catalog budget exceeded")
		}
		d.EligibleForSimulation = eligible(d)
		doc, _ := json.Marshal(d)
		if _, err = tx.Exec(ctx, `INSERT INTO reference_datasets(id,source_id,dataset_key,version,kind,document,artifact,spec_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, d.ID, in.SourceID, in.DatasetKey, in.Version, in.Kind, doc, raw, specHash); err != nil {
			return err
		}
		totalPoints := 0
		for _, f := range collection.Features {
			properties, _ := json.Marshal(f.Properties)
			var valid bool
			var points int
			if err = tx.QueryRow(ctx, `WITH g AS(SELECT ST_SetSRID(ST_GeomFromGeoJSON($1),4326) geom) SELECT ST_SRID(ST_GeomFromGeoJSON($1))=4326 AND ST_IsValid(geom) AND ST_NDims(geom)=2 AND GeometryType(geom) IN ('POINT','MULTIPOINT','LINESTRING','MULTILINESTRING','POLYGON','MULTIPOLYGON') AND ST_XMin(Box3D(geom))>=-180 AND ST_XMax(Box3D(geom))<=180 AND ST_YMin(Box3D(geom))>=-90 AND ST_YMax(Box3D(geom))<=90 AND NOT ST_IsEmpty(geom),ST_NPoints(geom) FROM g`, string(f.Geometry)).Scan(&valid, &points); err != nil {
				return apperr.Validation("invalid reference GeoJSON geometry")
			}
			totalPoints += points
			if !valid || totalPoints > 20000 {
				return apperr.Validation("invalid geometry or reference vertex budget exceeded")
			}
			tag, err := tx.Exec(ctx, `INSERT INTO reference_features(dataset_id,source_id,feature_id,geometry,properties) SELECT $1,$2,$3,ST_Intersection(g.geom,ST_MakeEnvelope($5,$6,$7,$8,4326)),$4 FROM (SELECT ST_SetSRID(ST_GeomFromGeoJSON($9),4326) geom) g WHERE ST_Intersects(g.geom,ST_MakeEnvelope($5,$6,$7,$8,4326))`, d.ID, in.SourceID, f.ID, properties, in.AOI.West, in.AOI.South, in.AOI.East, in.AOI.North, string(f.Geometry))
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return apperr.Validation("reference features must intersect the declared region")
			}
		}
		return nil
	})
	d.EligibleForSimulation = eligible(d)
	return d, duplicate, err
}
func (s *Service) List(ctx context.Context, onlyEligible bool) ([]Dataset, error) {
	if err := require(ctx, "sources.read"); err != nil {
		return nil, err
	}
	rows, err := platformdb.Scoped(s.pool).Query(ctx, `SELECT document FROM reference_datasets WHERE emad_content_visible('reference',id) AND COALESCE((document#>>'{rights,expiresAt}')::timestamptz > clock_timestamp(), true) ORDER BY dataset_key,version,id LIMIT 64`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Dataset{}
	for rows.Next() {
		var raw []byte
		var d Dataset
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &d); err != nil {
			return nil, err
		}
		d.EligibleForSimulation = eligible(d)
		if !onlyEligible || d.EligibleForSimulation {
			items = append(items, d)
		}
	}
	return items, rows.Err()
}
func (s *Service) Get(ctx context.Context, id string) (Dataset, error) {
	if err := require(ctx, "sources.read"); err != nil {
		return Dataset{}, err
	}
	var raw []byte
	var d Dataset
	err := platformdb.Scoped(s.pool).QueryRow(ctx, `SELECT document FROM reference_datasets WHERE id=$1 AND COALESCE((document#>>'{rights,expiresAt}')::timestamptz > clock_timestamp(), true)`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		err = apperr.NotFound("reference version", "unavailable")
	}
	if err == nil {
		err = json.Unmarshal(raw, &d)
	}
	d.EligibleForSimulation = eligible(d)
	return d, err
}
func (s *Service) Artifact(ctx context.Context, id string) ([]byte, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	var raw []byte
	err := platformdb.Scoped(s.pool).QueryRow(ctx, `SELECT artifact FROM reference_datasets WHERE id=$1`, id).Scan(&raw)
	return raw, err
}
func (s *Service) Features(ctx context.Context, id string) (Collection, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return Collection{}, err
	}
	rows, err := platformdb.Scoped(s.pool).Query(ctx, `SELECT feature_id,ST_AsGeoJSON(geometry),properties FROM reference_features WHERE dataset_id=$1 ORDER BY feature_id LIMIT 1000`, id)
	if err != nil {
		return Collection{}, err
	}
	defer rows.Close()
	c := Collection{Type: "FeatureCollection", Features: []Feature{}}
	for rows.Next() {
		f := Feature{Type: "Feature"}
		var geom string
		var props []byte
		if err = rows.Scan(&f.ID, &geom, &props); err != nil {
			return c, err
		}
		f.Geometry = json.RawMessage(geom)
		if err = json.Unmarshal(props, &f.Properties); err != nil {
			return c, err
		}
		c.Features = append(c.Features, f)
	}
	return c, rows.Err()
}
