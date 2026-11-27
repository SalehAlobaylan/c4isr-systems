package simgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunRecord is the canonical gateway metadata persisted in PostGIS and returned
// to both the 2D Map setup panels and 3D workbenches.
type RunRecord struct {
	JobID                string         `json:"jobId"`
	Engine               string         `json:"engine"`
	EnvironmentID        string         `json:"environmentId"`
	EnvironmentTitle     string         `json:"environmentTitle,omitempty"`
	Title                string         `json:"title"`
	Mode                 string         `json:"mode,omitempty"`
	Seed                 int64          `json:"seed,omitempty"`
	Status               string         `json:"status"`
	Stage                string         `json:"stage"`
	WorkerJobID          string         `json:"workerJobId,omitempty"`
	CreatedBy            string         `json:"createdBy,omitempty"`
	CreatedAt            string         `json:"createdAt"`
	CompletedAt          *string        `json:"completedAt"`
	ViewerURL            string         `json:"viewerUrl"`
	HasGlb               bool           `json:"hasGlb"`
	HasImagery           bool           `json:"hasImagery"`
	Error                *string        `json:"error"`
	Comparison           any            `json:"comparison,omitempty"`
	Summary              any            `json:"summary,omitempty"`
	ResultOverlay        any            `json:"resultOverlay,omitempty"`
	Policies             []string       `json:"policies,omitempty"`
	ElapsedS             float64        `json:"elapsedS,omitempty"`
	SensorCount          int            `json:"sensorCount,omitempty"`
	MapLayerAglM         float64        `json:"mapLayerAglM,omitempty"`
	ThreatCount          int            `json:"threatCount,omitempty"`
	BatteryCount         int            `json:"batteryCount,omitempty"`
	ScenarioSha256       string         `json:"scenarioSha256,omitempty"`
	ManifestSha256       string         `json:"manifestSha256,omitempty"`
	PersistedToS3        bool           `json:"persistedToS3"`
	ReplayObsBlobKey     string         `json:"replayObsBlobKey,omitempty"`
	ReplayObsBytes       int64          `json:"replayObsBytes,omitempty"`
	ReplayDebriefBlobKey string         `json:"replayDebriefBlobKey,omitempty"`
	ReplayDebriefBytes   int64          `json:"replayDebriefBytes,omitempty"`
	Extra                map[string]any `json:"-"`
}

// Store provides hybrid PostGIS + content-addressed S3 persistence with clean
// in-memory fallback when migration 00062 or S3 is unavailable.
const maxCachedReplayBlobs = 24

type Store struct {
	pool   *pgxpool.Pool
	blobs  artifacts.Store
	logger *slog.Logger

	mu       sync.RWMutex
	runs     map[string]RunRecord
	obsMem   map[string][]byte
	obsOrder []string
	debMem   map[string][]byte
	debOrder []string
	dbOnce   sync.Once
	dbReady  bool
}

func NewStore(pool *pgxpool.Pool, blobs artifacts.Store, logger *slog.Logger) *Store {
	if logger == nil {
		logger = slog.Default()
	}
	return &Store{
		pool:   pool,
		blobs:  blobs,
		logger: logger,
		runs:   make(map[string]RunRecord),
		obsMem: make(map[string][]byte),
		debMem: make(map[string][]byte),
	}
}

func (s *Store) hasDB(ctx context.Context) bool {
	if s.pool == nil {
		return false
	}
	s.dbOnce.Do(func() {
		checkCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var reg *string
		err := s.pool.QueryRow(checkCtx, `SELECT to_regclass('public.simulation_platform_runs')::text`).Scan(&reg)
		s.dbReady = err == nil && reg != nil && *reg != ""
	})
	return s.dbReady
}

func (s *Store) SaveRun(ctx context.Context, rec RunRecord) {
	s.mu.Lock()
	s.runs[rec.JobID] = rec
	s.mu.Unlock()

	if !s.hasDB(ctx) {
		return
	}
	if _, err := access.ResourceFrom(ctx); err != nil {
		return
	}
	summaryObj := map[string]any{
		"summary":          rec.Summary,
		"comparison":       rec.Comparison,
		"policies":         rec.Policies,
		"elapsedS":         rec.ElapsedS,
		"environmentTitle": rec.EnvironmentTitle,
		"mode":             rec.Mode,
		"hasGlb":           rec.HasGlb,
		"hasImagery":       rec.HasImagery,
		"sensorCount":      rec.SensorCount,
		"mapLayerAglM":     rec.MapLayerAglM,
		"threatCount":      rec.ThreatCount,
		"batteryCount":     rec.BatteryCount,
		"viewerUrl":        rec.ViewerURL,
	}
	summaryBytes, _ := json.Marshal(summaryObj)
	var overlayBytes []byte
	if rec.ResultOverlay != nil {
		overlayBytes, _ = json.Marshal(rec.ResultOverlay)
	}
	var completedAt *time.Time
	if rec.CompletedAt != nil && *rec.CompletedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, *rec.CompletedAt); err == nil {
			completedAt = &parsed
		}
	}
	errMsg := ""
	if rec.Error != nil {
		errMsg = *rec.Error
	}
	_, err := platformdb.Scoped(s.pool).Exec(ctx, `
		INSERT INTO simulation_platform_runs (
			id, engine, environment_id, title, manifest_sha256, scenario_sha256,
			seed, status, stage, worker_job_id, created_by, completed_at, error_message,
			summary, result_overlay, replay_obs_blob_key, replay_obs_bytes,
			replay_debrief_blob_key, replay_debrief_bytes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		ON CONFLICT (id, scope_id) DO UPDATE SET
			status = EXCLUDED.status,
			stage = EXCLUDED.stage,
			worker_job_id = EXCLUDED.worker_job_id,
			completed_at = EXCLUDED.completed_at,
			error_message = EXCLUDED.error_message,
			summary = EXCLUDED.summary,
			result_overlay = EXCLUDED.result_overlay,
			replay_obs_blob_key = EXCLUDED.replay_obs_blob_key,
			replay_obs_bytes = EXCLUDED.replay_obs_bytes,
			replay_debrief_blob_key = EXCLUDED.replay_debrief_blob_key,
			replay_debrief_bytes = EXCLUDED.replay_debrief_bytes`,
		rec.JobID, rec.Engine, rec.EnvironmentID, rec.Title, rec.ManifestSha256, rec.ScenarioSha256,
		rec.Seed, rec.Status, rec.Stage, rec.WorkerJobID, rec.CreatedBy, completedAt, errMsg,
		summaryBytes, overlayBytes, rec.ReplayObsBlobKey, rec.ReplayObsBytes,
		rec.ReplayDebriefBlobKey, rec.ReplayDebriefBytes,
	)
	if err != nil {
		s.logger.Warn("persist simulation run metadata failed", "jobId", rec.JobID, "error", err)
	}
}

func (s *Store) GetRun(ctx context.Context, jobID string) (RunRecord, bool) {
	s.mu.RLock()
	rec, ok := s.runs[jobID]
	s.mu.RUnlock()
	if ok {
		return rec, true
	}
	if !s.hasDB(ctx) {
		return RunRecord{}, false
	}
	if _, err := access.ResourceFrom(ctx); err != nil {
		return RunRecord{}, false
	}
	var out RunRecord
	var createdAt time.Time
	var completedAt *time.Time
	var errMsg string
	var summaryRaw, overlayRaw []byte
	err := platformdb.Scoped(s.pool).QueryRow(ctx, `
		SELECT id, engine, environment_id, title, manifest_sha256, scenario_sha256,
		       seed, status, stage, worker_job_id, created_by, created_at, completed_at,
		       error_message, summary, result_overlay, replay_obs_blob_key, replay_obs_bytes,
		       replay_debrief_blob_key, replay_debrief_bytes
		FROM simulation_platform_runs WHERE id = $1`, jobID).Scan(
		&out.JobID, &out.Engine, &out.EnvironmentID, &out.Title, &out.ManifestSha256, &out.ScenarioSha256,
		&out.Seed, &out.Status, &out.Stage, &out.WorkerJobID, &out.CreatedBy, &createdAt, &completedAt,
		&errMsg, &summaryRaw, &overlayRaw, &out.ReplayObsBlobKey, &out.ReplayObsBytes,
		&out.ReplayDebriefBlobKey, &out.ReplayDebriefBytes,
	)
	if err != nil {
		return RunRecord{}, false
	}
	hydrateRunRecord(&out, createdAt, completedAt, errMsg, summaryRaw, overlayRaw)
	return out, true
}

func (s *Store) ListRuns(ctx context.Context, engine string) []RunRecord {
	merged := make(map[string]RunRecord)
	if s.hasDB(ctx) {
		if _, err := access.ResourceFrom(ctx); err == nil {
			query := `
				SELECT id, engine, environment_id, title, manifest_sha256, scenario_sha256,
				       seed, status, stage, worker_job_id, created_by, created_at, completed_at,
				       error_message, summary, result_overlay, replay_obs_blob_key, replay_obs_bytes,
				       replay_debrief_blob_key, replay_debrief_bytes
				FROM simulation_platform_runs`
			var args []any
			if engine != "" {
				query += ` WHERE engine = $1`
				args = append(args, engine)
			}
			query += ` ORDER BY created_at DESC LIMIT 40`
			rows, err := platformdb.Scoped(s.pool).Query(ctx, query, args...)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var out RunRecord
					var createdAt time.Time
					var completedAt *time.Time
					var errMsg string
					var summaryRaw, overlayRaw []byte
					if scanErr := rows.Scan(
						&out.JobID, &out.Engine, &out.EnvironmentID, &out.Title, &out.ManifestSha256, &out.ScenarioSha256,
						&out.Seed, &out.Status, &out.Stage, &out.WorkerJobID, &out.CreatedBy, &createdAt, &completedAt,
						&errMsg, &summaryRaw, &overlayRaw, &out.ReplayObsBlobKey, &out.ReplayObsBytes,
						&out.ReplayDebriefBlobKey, &out.ReplayDebriefBytes,
					); scanErr != nil {
						continue
					}
					hydrateRunRecord(&out, createdAt, completedAt, errMsg, summaryRaw, overlayRaw)
					merged[out.JobID] = out
				}
			}
		}
	}

	s.mu.RLock()
	for _, r := range s.runs {
		if engine != "" && r.Engine != engine {
			continue
		}
		merged[r.JobID] = r
	}
	s.mu.RUnlock()

	out := make([]RunRecord, 0, len(merged))
	for _, r := range merged {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func hydrateRunRecord(out *RunRecord, createdAt time.Time, completedAt *time.Time, errMsg string, summaryRaw, overlayRaw []byte) {
	out.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	if completedAt != nil {
		formatted := completedAt.UTC().Format(time.RFC3339)
		out.CompletedAt = &formatted
	}
	if errMsg != "" {
		out.Error = &errMsg
	}
	out.PersistedToS3 = out.ReplayDebriefBlobKey != "" || out.ReplayObsBlobKey != ""
	var meta map[string]any
	if len(summaryRaw) > 0 && json.Unmarshal(summaryRaw, &meta) == nil {
		out.Summary = meta["summary"]
		out.Comparison = meta["comparison"]
		if s, ok := meta["environmentTitle"].(string); ok {
			out.EnvironmentTitle = s
		}
		if s, ok := meta["mode"].(string); ok {
			out.Mode = s
		}
		if b, ok := meta["hasGlb"].(bool); ok {
			out.HasGlb = b
		}
		if b, ok := meta["hasImagery"].(bool); ok {
			out.HasImagery = b
		}
		if v, ok := meta["viewerUrl"].(string); ok {
			out.ViewerURL = v
		}
		if f, ok := meta["elapsedS"].(float64); ok {
			out.ElapsedS = f
		}
		if rawPols, ok := meta["policies"].([]any); ok {
			for _, p := range rawPols {
				if ps, ok := p.(string); ok {
					out.Policies = append(out.Policies, ps)
				}
			}
		}
	}
	if len(overlayRaw) > 0 {
		var ov any
		if json.Unmarshal(overlayRaw, &ov) == nil {
			out.ResultOverlay = ov
		}
	}
}

func putBoundedBlob(m map[string][]byte, order *[]string, key string, data []byte) {
	if _, exists := m[key]; !exists {
		*order = append(*order, key)
		for len(*order) > maxCachedReplayBlobs {
			evict := (*order)[0]
			*order = (*order)[1:]
			delete(m, evict)
		}
	}
	m[key] = data
}

// PersistReplays stores content-addressed observation and debrief replay JSON
// blobs in S3 (when available and within artifacts.MaxBytes) plus bounded in-memory cache.
func (s *Store) PersistReplays(ctx context.Context, rec *RunRecord, obsBytes, debriefBytes []byte) {
	if len(obsBytes) > 0 {
		s.mu.Lock()
		putBoundedBlob(s.obsMem, &s.obsOrder, rec.JobID, obsBytes)
		s.mu.Unlock()
		if s.blobs != nil && len(obsBytes) <= artifacts.MaxBytes {
			key := fmt.Sprintf("sim-runs/%s/obs/%s", rec.JobID, artifacts.Hash(obsBytes))
			if err := s.blobs.Put(ctx, key, obsBytes, "application/json"); err == nil {
				rec.ReplayObsBlobKey = key
				rec.ReplayObsBytes = int64(len(obsBytes))
				rec.PersistedToS3 = true
			}
		}
	}
	if len(debriefBytes) > 0 {
		s.mu.Lock()
		putBoundedBlob(s.debMem, &s.debOrder, rec.JobID, debriefBytes)
		s.mu.Unlock()
		if s.blobs != nil && len(debriefBytes) <= artifacts.MaxBytes {
			key := fmt.Sprintf("sim-runs/%s/debrief/%s", rec.JobID, artifacts.Hash(debriefBytes))
			if err := s.blobs.Put(ctx, key, debriefBytes, "application/json"); err == nil {
				rec.ReplayDebriefBlobKey = key
				rec.ReplayDebriefBytes = int64(len(debriefBytes))
				rec.PersistedToS3 = true
			}
		}
	}
}

// GetReplayBlob reads a previously persisted replay blob from S3 or memory cache.
func (s *Store) GetReplayBlob(ctx context.Context, rec RunRecord, view string) ([]byte, bool) {
	s.mu.RLock()
	if view == "operator" {
		if b, ok := s.obsMem[rec.JobID]; ok && len(b) > 0 {
			s.mu.RUnlock()
			return b, true
		}
	} else {
		if b, ok := s.debMem[rec.JobID]; ok && len(b) > 0 {
			s.mu.RUnlock()
			return b, true
		}
	}
	s.mu.RUnlock()

	if s.blobs == nil {
		return nil, false
	}
	key, size := rec.ReplayDebriefBlobKey, rec.ReplayDebriefBytes
	if view == "operator" {
		key, size = rec.ReplayObsBlobKey, rec.ReplayObsBytes
	}
	if key == "" || size <= 0 || size > artifacts.MaxBytes {
		return nil, false
	}
	raw, err := s.blobs.Get(ctx, key, size)
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	return raw, true
}

// SaveSensorNetwork persists a cross-simulation sensor-network handoff document to Postgres when available.
func (s *Store) SaveSensorNetwork(ctx context.Context, doc map[string]any, actorID string) {
	if !s.hasDB(ctx) {
		return
	}
	if _, err := access.ResourceFrom(ctx); err != nil {
		return
	}
	id, _ := doc["layoutId"].(string)
	name, _ := doc["name"].(string)
	envID, _ := doc["environmentId"].(string)
	manifest, _ := doc["manifestSha256"].(string)
	sourceJob, _ := doc["sourceJobId"].(string)
	sensorCount := 0
	if n, ok := doc["sensorCount"].(float64); ok {
		sensorCount = int(n)
	}
	if id == "" || envID == "" {
		return
	}
	raw, _ := json.Marshal(doc)
	_, err := platformdb.Scoped(s.pool).Exec(ctx, `
		INSERT INTO simulation_sensor_networks (id, name, environment_id, manifest_sha256, source_job_id, sensor_count, document, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id, scope_id) DO UPDATE SET
			name = EXCLUDED.name,
			document = EXCLUDED.document,
			sensor_count = EXCLUDED.sensor_count`,
		id, name, envID, manifest, sourceJob, sensorCount, raw, actorID,
	)
	if err != nil {
		s.logger.Warn("persist simulation sensor network failed", "layoutId", id, "error", err)
	}
}

// RecordSessionCommand writes an audited live operator command to Postgres when available.
func (s *Store) RecordSessionCommand(ctx context.Context, id, sessionID string, tick int, virtualTimeS float64, kind string, payload map[string]any, actorID string) {
	if !s.hasDB(ctx) {
		return
	}
	if _, err := access.ResourceFrom(ctx); err != nil {
		return
	}
	raw, _ := json.Marshal(payload)
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	_, err := platformdb.Scoped(s.pool).Exec(ctx, `
		INSERT INTO simulation_session_commands (id, session_id, tick, virtual_time_s, command_kind, payload, actor_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		id, sessionID, tick, virtualTimeS, strings.TrimSpace(kind), raw, actorID,
	)
	if err != nil {
		s.logger.Warn("persist session command audit failed", "sessionId", sessionID, "error", err)
	}
}
