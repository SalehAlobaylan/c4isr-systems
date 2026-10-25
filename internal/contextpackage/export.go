package contextpackage

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/assessments"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imagery"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imageryanalysis"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"github.com/jackc/pgx/v5"
)

type Preview struct {
	ID        string    `json:"id"`
	SHA256    string    `json:"sha256"`
	ExpiresAt time.Time `json:"expiresAt"`
	Manifest  Manifest  `json:"manifest"`
	Bytes     int       `json:"bytes"`
}

func (s *Service) Preview(ctx context.Context, in BuildInput) (Preview, error) {
	in.Approval = "Preview only; approval is required before freezing"
	b, err := s.assemble(ctx, in)
	if err != nil {
		return Preview{}, err
	}
	raw, _ := json.Marshal(b)
	doc, _ := json.Marshal(b.Manifest)
	if len(raw) > 12*1024*1024 {
		return Preview{}, apperr.Validation("preview bundle exceeds 12 MiB")
	}
	fresh, err := refreshExport(ctx)
	if err != nil {
		return Preview{}, err
	}
	ctx = fresh
	if err := require(ctx); err != nil {
		return Preview{}, err
	}
	p, r, _ := access.From(ctx)
	v := Preview{ids.New("cpx"), reference.Hash(raw), time.Now().UTC().Add(10 * time.Minute), b.Manifest, len(raw)}
	err = platformdb.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+"/context-previews"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM context_export_previews WHERE expires_at<now()`); err != nil {
			return err
		}
		var count, total int64
		if err := tx.QueryRow(ctx, `SELECT count(*),COALESCE(sum(octet_length(bundle)),0) FROM context_export_previews`).Scan(&count, &total); err != nil {
			return err
		}
		if count >= 32 || total+int64(len(raw)) > 64*1024*1024 {
			return apperr.Conflict("active context preview budget exhausted")
		}
		_, err := tx.Exec(ctx, `INSERT INTO context_export_previews(id,creator,sha256,document,bundle,expires_at)VALUES($1,$2,$3,$4,$5,$6)`, v.ID, p.ID, v.SHA256, doc, raw, v.ExpiresAt)
		return err
	})
	return v, err
}
func (s *Service) freeze(ctx context.Context, in BuildInput) (Bundle, error) {
	if err := require(ctx); err != nil {
		return Bundle{}, err
	}
	p, _, _ := access.From(ctx)
	if p.Kind != "human" {
		return Bundle{}, apperr.Forbidden("human package approval required")
	}
	if len(strings.TrimSpace(in.Approval)) < 10 || len(in.Approval) > 2000 {
		return Bundle{}, apperr.Validation("human approval statement of 10–2000 characters required")
	}
	var raw []byte
	var hash, creator string
	var expiry time.Time
	err := platformdb.Scoped(s.pool).QueryRow(ctx, `SELECT bundle,sha256,creator,expires_at FROM context_export_previews WHERE id=$1`, in.PreviewID).Scan(&raw, &hash, &creator, &expiry)
	if errors.Is(err, pgx.ErrNoRows) {
		return Bundle{}, apperr.NotFound("context preview", "unavailable")
	}
	if err != nil {
		return Bundle{}, err
	}
	if creator != p.ID || !expiry.After(time.Now()) || hash != in.PreviewSHA256 || reference.Hash(raw) != hash {
		return Bundle{}, apperr.Conflict("preview changed, expired or belongs to another approver")
	}
	var b Bundle
	if err = json.Unmarshal(raw, &b); err != nil {
		return b, err
	}
	if err = b.Validate(time.Now(), false); err != nil {
		return b, apperr.Conflict(err.Error())
	}
	for _, a := range b.Manifest.Assets {
		if a.Lineage == nil {
			if _, err := s.references.Get(ctx, a.DatasetID); err != nil {
				return b, err
			}
			continue
		}
		current, err := s.images.Artifacts.Admission(ctx, a.SourceID)
		if err != nil || current.ID != a.Lineage.AdmissionID || !current.Rights.AllowSimulationExport {
			return b, apperr.Conflict("source admission or export rights changed; create another preview")
		}
		if a.Lineage.ArtifactID != "" {
			actual, err := s.images.Artifacts.Ready(ctx, a.Lineage.ArtifactID)
			if err != nil || actual.SHA256 != a.Lineage.ArtifactSHA256 {
				return b, apperr.Conflict("pinned imagery dependency is no longer available")
			}
		}
	}
	now := time.Now().UTC()
	b.Manifest.CreatedAt = now
	b.Manifest.Approval = Approval{p.ID, now, in.Approval}
	b.ID = ID(b.Manifest)
	if err = b.Validate(now, false); err != nil {
		return b, apperr.Validation(err.Error())
	}
	return b, s.persist(ctx, b)
}
func (s *Service) addImagery(ctx context.Context, b *Bundle, in BuildInput) error {
	if len(in.AssessmentRevisionIDs) > 0 {
		if err := access.Require(ctx, "assessments.read"); err != nil {
			return err
		}
	}
	b.Manifest.ProcessingVersion = "context-snapshot-v2"
	b.Manifest.ImageryQuality = in.AcceptedQuality + "; prepared angular subsets and indicators retain their declared limitations"
	b.Manifest.Omissions = []string{"No original full-scene rasters are fetched or embedded.", "No public aircraft/vessel track is converted into a controllable actor.", "Reference files/subsets retain their declared extent; only derived finding geometry is clipped to the selected AOI.", "Published assessments are interpretations; derived geometry is synthetic context, not simulator truth.", "The current headless runner permits up to 4 MiB of combined frozen scenario inputs; keep context packages within the intended runner budget."}
	preparations := append([]string{}, in.PreparationIDs...)
	type reviewed struct {
		v   assessments.ReviewRevision
		raw []byte
	}
	reviews := []reviewed{}
	for _, id := range in.AssessmentRevisionIDs {
		var raw []byte
		if err := platformdb.Scoped(s.pool).QueryRow(ctx, `SELECT document FROM assessment_revisions WHERE id=$1 AND status='PUBLISHED'`, id).Scan(&raw); err != nil {
			return apperr.NotFound("published assessment revision", "unavailable")
		}
		var v assessments.ReviewRevision
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		if err := assessments.VerifyPublishedIdentity(ctx, platformdb.Scoped(s.pool), id); err != nil {
			return err
		}
		if err := assessments.ResolveReviewIdentity(ctx, platformdb.Scoped(s.pool), &v); err != nil {
			return err
		}
		if v.ReportEvidence != nil || v.Status != "PUBLISHED" || v.PublishedBy == "" || v.Evidence.Result == nil || len(v.Evidence.Input.PreparationIDs) != 2 {
			return apperr.Validation("only a published, independently reviewed revision may be exported")
		}
		preparations = append(preparations, v.Evidence.Input.PreparationIDs...)
		reviews = append(reviews, reviewed{v, raw})
	}
	seen := map[string]bool{}
	for _, a := range b.Manifest.Assets {
		seen[a.DatasetID] = true
	}
	put := func(a Asset, raw []byte) error {
		if seen[a.DatasetID] {
			return nil
		}
		seen[a.DatasetID] = true
		if len(b.Manifest.Assets) >= 16 {
			return apperr.Validation("package permits at most 16 assets; narrow the selection")
		}
		if in.ReferenceOnly {
			a.Retention = "reference_only"
			b.Manifest.Reproduction = "external_dependencies"
		} else {
			a.Retention = "retained"
			b.Assets[a.SHA256] = raw
		}
		b.Manifest.Assets = append(b.Manifest.Assets, a)
		return nil
	}
	makeAsset := func(scene imagery.Scene, id, kind string, raw []byte, extent publicingest.Bounds, lineage ImageryLineage, expiry *time.Time) (Asset, error) {
		current, err := s.images.Artifacts.Admission(ctx, scene.SourceID)
		if err != nil {
			return Asset{}, err
		}
		if !scene.Rights.AllowSimulationExport || !current.Rights.AllowSimulationExport {
			return Asset{}, apperr.Forbidden("original and current source rights must allow simulation export")
		}
		if scene.AcquiredAt.Before(in.TimeWindow.From) || scene.AcquiredAt.After(in.TimeWindow.To) {
			return Asset{}, apperr.Validation("scene acquisition must be within the selected UTC interval")
		}
		if !in.ReferenceOnly && !current.Rights.RetainRaw {
			return Asset{}, apperr.Forbidden("current source rights do not permit retained package copies")
		}
		lineage.AdmissionID = current.ID
		lineage.CurrentRights = current.Rights
		for _, limit := range []*time.Time{current.Rights.ExpiresAt, scene.Rights.ExpiresAt} {
			if limit != nil && (expiry == nil || limit.Before(*expiry)) {
				t := *limit
				expiry = &t
			}
		}
		ref := scene.CatalogueReference
		if ref == "" {
			ref = "https://stac.dataspace.copernicus.eu/v1/collections/" + scene.Collection + "/items/" + scene.ProviderID
		}
		return Asset{Kind: kind, DatasetAOI: extent, DatasetID: id, SourceID: scene.SourceID, DatasetKey: scene.Collection, Version: scene.ContentHash, OriginalProducer: scene.OriginalProducer, Reference: ref, RetrievedAt: scene.RetrievedAt, SHA256: reference.Hash(raw), Bytes: len(raw), CRS: "EPSG:4326", VerticalDatum: "not_applicable", Resolution: reference.Resolution{Value: 1, Unit: "metadata"}, Quality: in.AcceptedQuality, Validity: "acquired " + scene.AcquiredAt.Format(time.RFC3339Nano), Processing: "context-snapshot-v2; " + simulation.TransformVersion, Rights: scene.Rights, Lineage: &lineage, AvailableUntil: expiry}, nil
	}
	prepareSeen := map[string]bool{}
	for _, id := range preparations {
		if prepareSeen[id] {
			continue
		}
		prepareSeen[id] = true
		j, err := s.images.Job(ctx, id)
		if err != nil {
			return err
		}
		if j.Status != "SUCCEEDED" || j.Input.Kind != "PREPARE" || j.Result == nil || j.Result.Tile == nil {
			return apperr.Conflict("imagery subset preparation must be successful")
		}
		a, raw, err := s.images.Artifacts.Read(ctx, j.Result.ArtifactID)
		if err != nil {
			return err
		}
		if !a.Rights.AllowSimulationExport {
			return apperr.Forbidden("retained artifact rights do not permit export")
		}
		scene := j.Input.Scenes[0]
		extent := j.Result.Tile.Grid.Bounds()
		lineage := ImageryLineage{SceneIDs: []string{scene.ID}, EvidenceIDs: []string{scene.EvidenceID}, ArtifactID: a.ID, ArtifactSHA256: a.SHA256, JobID: j.ID, Synthetic: j.Result.Synthetic, Role: "prepared_multispectral_context"}
		asset, err := makeAsset(scene, "subset:"+j.ID, "imagery", raw, extent, lineage, &a.RetainUntil)
		if err != nil {
			return err
		}
		asset.Resolution = reference.Resolution{Value: j.Result.Tile.Grid.StepLng, Unit: "degrees (longitude); latitude step recorded in retained grid"}
		asset.Processing = j.Result.Tile.PreparationMethod
		if err = put(asset, raw); err != nil {
			return err
		}
	}
	for _, review := range reviews {
		v := review.v
		j, err := s.images.Job(ctx, v.Evidence.ID)
		if err != nil {
			return err
		}
		if j.Status != "SUCCEEDED" || j.Result == nil || j.Result.SHA256 != v.Evidence.Result.SHA256 {
			return apperr.Conflict("published comparison dependency changed")
		}
		a, raw, err := s.images.Artifacts.Read(ctx, j.Result.ArtifactID)
		if err != nil {
			return err
		}
		if !a.Rights.AllowSimulationExport {
			return apperr.Forbidden("analysis artifact is not exportable")
		}
		var result imageryanalysis.Result
		if err = json.Unmarshal(raw, &result); err != nil {
			return err
		}
		scene := j.Input.Scenes[0]
		lineage := ImageryLineage{AuthorHumanID: v.AuthorHumanID, ReviewerHumanID: v.PublisherHumanID, SceneIDs: []string{}, EvidenceIDs: []string{}, ArtifactID: a.ID, ArtifactSHA256: a.SHA256, JobID: j.ID, RevisionID: v.ID, Synthetic: j.Result.Synthetic, Role: "reviewed_environmental_indicator"}
		for _, sc := range j.Input.Scenes {
			lineage.SceneIDs = append(lineage.SceneIDs, sc.ID)
			lineage.EvidenceIDs = append(lineage.EvidenceIDs, sc.EvidenceID)
		}
		for _, part := range []struct {
			id, kind string
			bytes    []byte
		}{{"analysis:" + j.ID, "analysis", raw}, {"assessment:" + v.ID, "assessment", review.raw}} {
			asset, err := makeAsset(scene, part.id, part.kind, part.bytes, result.Grid.Bounds(), lineage, &a.RetainUntil)
			if err != nil {
				return err
			}
			if err = put(asset, part.bytes); err != nil {
				return err
			}
		}
		if string(result.FindingGeometry) != "null" && len(result.FindingGeometry) > 0 {
			var clipped []byte
			err = platformdb.Scoped(s.pool).QueryRow(ctx, `SELECT ST_AsGeoJSON(ST_Multi(ST_CollectionExtract(ST_Intersection(ST_SetSRID(ST_GeomFromGeoJSON($1),4326),ST_MakeEnvelope($2,$3,$4,$5,4326)),3)),15)`, string(result.FindingGeometry), in.AOI.West, in.AOI.South, in.AOI.East, in.AOI.North).Scan(&clipped)
			if err != nil {
				return err
			}
			doc, _ := json.Marshal(map[string]any{"type": "Feature", "geometry": json.RawMessage(clipped), "properties": map[string]any{"syntheticDerived": true, "role": "scenario_context_only", "assessmentRevisionId": v.ID, "jobId": j.ID, "sourceResultSha256": a.SHA256}})
			asset, err := makeAsset(scene, "geometry:"+v.ID, "derived_geometry", doc, in.AOI, lineage, &a.RetainUntil)
			if err != nil {
				return err
			}
			asset.SyntheticDerived = true
			asset.ParentArtifactSHA256 = a.SHA256
			asset.Processing = "PostGIS ST_Intersection + ST_CollectionExtract + ST_Multi; EPSG:4326; context-derived-geometry-v1"
			b.Manifest.SyntheticDerived = true
			if err = put(asset, doc); err != nil {
				return err
			}
		}
	}
	sort.Slice(b.Manifest.Assets, func(i, j int) bool { return b.Manifest.Assets[i].DatasetID < b.Manifest.Assets[j].DatasetID })
	return nil
}

// Admission/source and artifact rows are held through the freeze commit. A
// concurrent revocation or artifact deletion cannot race a stale preflight check.
func (s *Service) validateFrozenDependencies(ctx context.Context, tx pgx.Tx, b Bundle) error {
	if err := b.Validate(time.Now(), false); err != nil {
		return apperr.Conflict(err.Error())
	}
	for _, asset := range b.Manifest.Assets {
		if asset.Lineage != nil && asset.Lineage.RevisionID != "" {
			if err := assessments.VerifyPublishedIdentity(ctx, tx, asset.Lineage.RevisionID); err != nil {
				return err
			}
		}
		if asset.Lineage == nil {
			continue
		}
		var doc []byte
		err := tx.QueryRow(ctx, `SELECT a.document FROM source_connector_state c JOIN source_admissions a ON a.id=c.admission_id AND a.scope_id=c.scope_id JOIN sources s ON s.id=c.source_id AND s.scope_id=c.scope_id WHERE c.source_id=$1 AND s.status='active' FOR SHARE OF c,s`, asset.SourceID).Scan(&doc)
		var admission publicingest.Admission
		if err != nil || json.Unmarshal(doc, &admission) != nil || admission.ID != asset.Lineage.AdmissionID || !admission.Enabled || !admission.Rights.AllowSimulationExport || (admission.Rights.ExpiresAt != nil && !admission.Rights.ExpiresAt.After(time.Now())) {
			return apperr.Conflict("source admission or export rights changed; create another preview")
		}
		var state, hash string
		var until time.Time
		if err := tx.QueryRow(ctx, `SELECT state,sha256,retain_until FROM artifacts WHERE id=$1 FOR SHARE`, asset.Lineage.ArtifactID).Scan(&state, &hash, &until); err != nil || state != "READY" || hash != asset.Lineage.ArtifactSHA256 || !until.After(time.Now()) {
			return apperr.Conflict("pinned imagery dependency is no longer available")
		}
	}
	return nil
}
