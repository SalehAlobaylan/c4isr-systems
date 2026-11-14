// Package readiness reports scoped feature activation, not merely a database
// ping. No credential, provider URL, hidden run, or private blob key is exposed.
package readiness

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	Feature         string    `json:"feature"`
	LastSeenAt      time.Time `json:"lastSeenAt"`
	State           string    `json:"state"`
	RuntimeBuild    string    `json:"runtimeBuild,omitempty"`
	ToolchainSHA256 *string   `json:"toolchainSha256,omitempty"`
}

type WorkerHeartbeat struct {
	Feature         string  `json:"feature"`
	RuntimeBuild    string  `json:"runtimeBuild,omitempty"`
	ToolchainSHA256 *string `json:"toolchainSha256,omitempty"`
	ScratchBytes    *int64  `json:"scratchBytes,omitempty"`
}

var toolchainDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Feature struct {
	CountMeaning      string `json:"countMeaning,omitempty"`
	UnclassifiedCount int    `json:"unclassifiedCount,omitempty"`
	ID                string `json:"id"`
	State             string `json:"state"`
	Count             int    `json:"count"`
	NextAction        string `json:"nextAction"`
}
type Report struct {
	CheckedAt time.Time       `json:"checkedAt"`
	Resource  access.Resource `json:"resource"`
	Features  []Feature       `json:"features"`
	Workers   []Worker        `json:"workers"`
}
type Service struct {
	EnvironmentLibrary bool
	MapOnly            bool
	Pool               *pgxpool.Pool
	Store              artifacts.Store
	RuntimeBuilds      []string
}

func (s Service) Mount(r chi.Router) {
	r.Get("/platform/readiness", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Report(r.Context())
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, 200, v)
	})
	r.Post("/platform/worker-heartbeats", func(w http.ResponseWriter, r *http.Request) {
		var in WorkerHeartbeat
		if e := httpx.DecodeJSON(r, &in); e != nil {
			httpx.Error(w, e)
			return
		}
		if e := s.Beat(r.Context(), in); e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]string{"state": "READY"})
	})
}
func (s Service) Beat(ctx context.Context, heartbeat WorkerHeartbeat) error {
	p, r, _ := access.From(ctx)
	feature, build := heartbeat.Feature, heartbeat.RuntimeBuild
	if p.Kind != "service" {
		return apperr.Forbidden("worker service identity required")
	}
	permissions := map[string]string{"connectors": "connectors.execute", "imagery": "imagery.jobs.execute", "assets": "sim.assets.import", "runtime": "sim.runtime.execute", "recordings": "sim.recordings.execute", "preparation": "imagery.capture.execute", "environment-library": "environments.execute"}
	permission, ok := permissions[feature]
	if !ok {
		return apperr.Validation("unknown worker feature")
	}
	if e := access.Require(ctx, permission); e != nil {
		return e
	}
	if feature == "environment-library" && (!s.EnvironmentLibrary || r.Partition != access.PublicEvidence) {
		return apperr.Forbidden("environment worker feature is unavailable")
	}
	if heartbeat.ToolchainSHA256 != nil || heartbeat.ScratchBytes != nil {
		if feature != "environment-library" ||
			heartbeat.ToolchainSHA256 != nil && !toolchainDigest.MatchString(*heartbeat.ToolchainSHA256) ||
			heartbeat.ScratchBytes != nil && (*heartbeat.ScratchBytes < 0 || *heartbeat.ScratchBytes > 1<<40) {
			return apperr.Validation("invalid environment worker telemetry")
		}
	}
	if feature == "runtime" {
		admitted := false
		for _, v := range s.RuntimeBuilds {
			if v == build {
				admitted = true
			}
		}
		if !admitted {
			return apperr.Forbidden("runtime build is not admitted by deployment")
		}
	} else {
		build = ""
	}
	return db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO platform_worker_status(scope_id,principal_id,feature,runtime_build,toolchain_sha256,scratch_bytes)
		 VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(scope_id,principal_id,feature) DO UPDATE SET last_seen_at=now(),
		 runtime_build=excluded.runtime_build,toolchain_sha256=excluded.toolchain_sha256,scratch_bytes=excluded.scratch_bytes`,
			r.ID(), p.ID, feature, build, heartbeat.ToolchainSHA256, heartbeat.ScratchBytes)
		return e
	})
}
func (s Service) Report(ctx context.Context) (Report, error) {
	if e := access.Require(ctx, "operational.read"); e != nil {
		return Report{}, e
	}
	_, r, _ := access.From(ctx)
	out := Report{CheckedAt: time.Now().UTC(), Resource: r, Features: []Feature{}, Workers: []Worker{}}
	queries := [][3]string{}
	switch r.Partition {
	case access.PublicEvidence:
		if access.Allows(ctx, "sources.read") {
			queries = append(queries, [3]string{"reference", "reference_datasets", "Install reviewed geographic layers"}, [3]string{"imagery", "imagery_scenes", "Retrieve admitted scene metadata"}, [3]string{"imagery-captures", "imagery_capture_jobs", "Capture and prepare real band windows"}, [3]string{"evidence", "public_evidence", "Retrieve an admitted public source"})
		}
	case access.ExerciseInput:
		if access.Allows(ctx, "sim.scenarios.author") {
			queries = append(queries, [3]string{"models", "model_asset_versions", "Install validated starter models"}, [3]string{"scenes", "simulation_scenes", "Prepare and freeze a scene"})
		}
	case access.ExerciseObservation:
		if access.Allows(ctx, "observations.read") {
			queries = append(queries, [3]string{"observations", "observations", "Start an authorized exercise"})
		}
	}
	// Table identifiers are fixed above, never request input.
	for _, q := range queries {
		f := Feature{ID: q[0], State: "NOT_INSTALLED", NextAction: q[2]}
		query := fmt.Sprintf("SELECT count(*),0 FROM %s", q[1])
		kind := map[string]string{"reference": "reference", "imagery": "scene", "imagery-captures": "capture"}[q[0]]
		if kind != "" {
			query = fmt.Sprintf("SELECT count(*),count(*) FILTER(WHERE emad_content_purpose('%s',id)='unclassified') FROM %s WHERE emad_content_visible('%s',id)", kind, q[1], kind)
		}
		if q[0] == "imagery-captures" {
			query += " AND status='SUCCEEDED'"
		}
		f.CountMeaning = map[string]string{"reference": "Retained reference versions", "imagery": "Catalogue revisions; not independent acquisitions", "imagery-captures": "Successful capture records; retries and dates counted separately", "evidence": "Retained source evidence revisions"}[q[0]]
		e := db.Scoped(s.Pool).QueryRow(ctx, query).Scan(&f.Count, &f.UnclassifiedCount)
		if e != nil {
			return out, e
		}
		if f.Count > 0 {
			f.State = "AVAILABLE"
			f.NextAction = ""
		}
		if f.UnclassifiedCount > 0 {
			f.State = "UNCLASSIFIED_CONTENT"
			f.NextAction = "Source administrators can review content purpose; counts include unclassified records"
		}
		out.Features = append(out.Features, f)
	}
	if r.Partition == access.PublicEvidence && access.Allows(ctx, "sources.read") {
		extra := [][3]string{
			{"imagery-acquisition-dates", "SELECT count(DISTINCT input#>>'{scene,acquiredAt}') FROM imagery_capture_jobs WHERE status='SUCCEEDED' AND emad_content_visible('capture',id)", "Distinct retained acquisition timestamps; not capture attempts"},
			{"imagery-studies", "SELECT count(DISTINCT (source_id,input->'preparationIds',input->'parameters',input->>'algorithm')) FROM imagery_jobs WHERE status='SUCCEEDED' AND input->>'kind'='NDVI_CHANGE' AND emad_content_visible('job',id)", "Distinct completed comparisons by frozen inputs/method; not reviewed conclusions"},
		}
		if access.Allows(ctx, "assessments.read") {
			extra = append(extra, [3]string{"working-reviewed-outputs", "SELECT count(*) FROM assessment_review_heads h JOIN assessment_revisions r ON r.id=h.latest_id AND r.scope_id=h.scope_id JOIN assessment_review_identities i ON i.revision_id=r.id AND i.scope_id=r.scope_id WHERE r.status='PUBLISHED' AND emad_content_purpose('assessment',h.assessment_id)='working' AND i.author_human_id IS NOT NULL AND i.publisher_human_id IS NOT NULL AND i.actor_human_id=i.publisher_human_id AND i.author_human_id<>i.publisher_human_id", "Published working assessments with canonical human separation; independent usefulness is separately qualified"})
		}
		for _, q := range extra {
			f := Feature{ID: q[0], State: "NOT_INSTALLED", CountMeaning: q[2], NextAction: "Inspect retained inputs and their classification"}
			if e := db.Scoped(s.Pool).QueryRow(ctx, q[1]).Scan(&f.Count); e != nil {
				return out, e
			}
			if f.Count > 0 {
				f.State = "AVAILABLE"
				f.NextAction = ""
			}
			out.Features = append(out.Features, f)
		}
	}
	probe, cancel := context.WithTimeout(ctx, 2*time.Second)
	raw, e := s.Store.Get(probe, "platform/readiness", int64(len("emad-ready")))
	cancel()
	f := Feature{ID: "artifact-storage", State: "UNAVAILABLE", NextAction: "Run local setup or check private object storage"}
	if e == nil && string(raw) == "emad-ready" {
		f.State = "AVAILABLE"
		f.NextAction = ""
	}
	out.Features = append(out.Features, f)
	rows, e := db.Scoped(s.Pool).Query(ctx, `SELECT feature,last_seen_at,runtime_build,toolchain_sha256 FROM platform_worker_status ORDER BY feature,principal_id`)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v Worker
		if e = rows.Scan(&v.Feature, &v.LastSeenAt, &v.RuntimeBuild, &v.ToolchainSHA256); e != nil {
			return out, e
		}
		v.State = "STALE"
		if time.Since(v.LastSeenAt) < 15*time.Second {
			v.State = "READY"
		}
		if v.Feature == "environment-library" {
			if s.EnvironmentLibrary && access.Allows(ctx, "environments.read") {
				out.Workers = append(out.Workers, v)
			}
		} else if !s.MapOnly || v.Feature == "imagery" || v.Feature == "preparation" || v.Feature == "connectors" {
			out.Workers = append(out.Workers, v)
		}
	}
	return out, rows.Err()
}
