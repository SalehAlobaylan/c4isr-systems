// Package app composes the C4ISR server: repositories, application services,
// event subscriptions, realtime hub, transports, and the HTTP router.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/alerts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/assessments"
	"github.com/SalehAlobaylan/c4isr-systems/internal/assets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/audit"
	"github.com/SalehAlobaylan/c4isr-systems/internal/classifications"
	"github.com/SalehAlobaylan/c4isr-systems/internal/commands"
	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/geospatial"
	"github.com/SalehAlobaylan/c4isr-systems/internal/incidents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/missions"
	"github.com/SalehAlobaylan/c4isr-systems/internal/observations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/operators"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/realtime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/scenarios"
	"github.com/SalehAlobaylan/c4isr-systems/internal/sources"
	"github.com/SalehAlobaylan/c4isr-systems/internal/telemetry"
	"github.com/SalehAlobaylan/c4isr-systems/internal/tracks"
)

// DefaultOperator is the fallback actor until Phase 17 introduces
// authentication and RBAC.
const DefaultOperator = "operator-01"

// App is the assembled server.
type App struct {
	config  config.Config
	logger  *slog.Logger
	pool    *pgxpool.Pool
	handler http.Handler
}

// New opens the database, verifies migrations, wires every module, and builds
// the HTTP router.
func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	poolClosed := false
	defer func() {
		if !poolClosed {
			pool.Close()
		}
	}()

	if err := verifyDatabase(ctx, pool); err != nil {
		return nil, err
	}

	bus := events.NewDispatcher(logger)

	// Infrastructure modules first so audit and realtime observe the full
	// stream and operator actions remain traceable.
	auditSvc := audit.NewService(audit.NewPostgresRepository(pool), bus)
	operatorSvc := operators.NewService(operators.NewPostgresRepository(pool))
	if _, _, err := operatorSvc.Ensure(ctx, operators.CreateInput{
		ID:   DefaultOperator,
		Name: "Operator 01",
		Role: operators.RoleOperator,
	}); err != nil {
		return nil, fmt.Errorf("ensure default operator: %w", err)
	}

	sourceSvc := sources.NewService(sources.NewPostgresRepository(pool), bus)
	assetSvc := assets.NewService(assets.NewPostgresRepository(pool), bus)
	observationSvc := observations.NewService(observations.NewPostgresRepository(pool), sourceSvc, bus)
	telemetrySvc := telemetry.NewService(telemetry.NewPostgresRepository(pool), assetSvc, bus)
	trackSvc := tracks.NewService(tracks.NewPostgresRepository(pool), observationSvc, bus)
	classificationSvc := classifications.NewService(classifications.NewPostgresRepository(pool), bus)
	geofenceSvc := geospatial.NewService(geospatial.NewPostgresRepository(pool), bus)
	alertSvc := alerts.NewService(alerts.NewPostgresRepository(pool), bus)
	incidentSvc := incidents.NewService(incidents.NewPostgresRepository(pool), bus)
	missionSvc := missions.NewService(missions.NewPostgresRepository(pool), assetSvc, bus)
	commandSvc := commands.NewService(commands.NewPostgresRepository(pool), assetSvc, bus)
	assessmentSvc := assessments.NewService(assessments.NewPostgresRepository(pool), bus)

	// Realtime is registered after domain subscribers so notifications carry
	// the state changes their handlers produced.
	hub := realtime.NewHub(logger, bus)

	scenarioSvc := scenarios.NewService(
		scenarios.NewPostgresRepository(pool),
		scenarios.NewLoader(cfg.ScenariosDir),
		scenarios.EngineDeps{
			Sources:         sourceSvc,
			Assets:          assetSvc,
			Observations:    observationSvc,
			Telemetry:       telemetrySvc,
			Geofences:       geofenceSvc,
			Classifications: classificationSvc,
			Commands:        commandSvc,
			Tracks:          trackSvc,
			Logger:          logger,
		},
		bus,
	)

	router := buildRouter(cfg, logger, pool, moduleHandlers{
		sources:         sources.NewHandler(sourceSvc),
		observations:    observations.NewHandler(observationSvc),
		assets:          assets.NewHandler(assetSvc),
		telemetry:       telemetry.NewHandler(telemetrySvc),
		tracks:          tracks.NewHandler(trackSvc),
		classifications: classifications.NewHandler(classificationSvc),
		geofences:       geospatial.NewHandler(geofenceSvc),
		alerts:          alerts.NewHandler(alertSvc),
		incidents:       incidents.NewHandler(incidentSvc),
		missions:        missions.NewHandler(missionSvc),
		commands:        commands.NewHandler(commandSvc),
		assessments:     assessments.NewHandler(assessmentSvc),
		audit:           audit.NewHandler(auditSvc),
		operators:       operators.NewHandler(operatorSvc),
		scenarios:       scenarios.NewHandler(scenarioSvc),
		hub:             hub,
	})

	poolClosed = true
	return &App{
		config:  cfg,
		logger:  logger,
		pool:    pool,
		handler: router,
	}, nil
}

// Handler returns the assembled HTTP handler.
func (a *App) Handler() http.Handler { return a.handler }

// Close releases the connection pool.
func (a *App) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}

type moduleHandlers struct {
	sources         *sources.Handler
	observations    *observations.Handler
	assets          *assets.Handler
	telemetry       *telemetry.Handler
	tracks          *tracks.Handler
	classifications *classifications.Handler
	geofences       *geospatial.Handler
	alerts          *alerts.Handler
	incidents       *incidents.Handler
	missions        *missions.Handler
	commands        *commands.Handler
	assessments     *assessments.Handler
	audit           *audit.Handler
	operators       *operators.Handler
	scenarios       *scenarios.Handler
	hub             *realtime.Hub
}

func buildRouter(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, h moduleHandlers) http.Handler {
	router := chi.NewRouter()
	router.Use(
		httpx.RequestID,
		httpx.Recoverer(logger),
		httpx.Logger(logger),
		httpx.CORS(cfg.AllowedOrigins),
		httpx.OperatorIdentity(DefaultOperator),
	)

	router.Get("/health", healthHandler(pool))

	router.Route("/api/v1", func(r chi.Router) {
		h.sources.Mount(r)
		h.observations.Mount(r)
		h.assets.Mount(r)
		h.telemetry.Mount(r)
		h.tracks.Mount(r)
		h.classifications.Mount(r)
		h.geofences.Mount(r)
		h.alerts.Mount(r)
		h.incidents.Mount(r)
		h.missions.Mount(r)
		h.commands.Mount(r)
		h.assessments.Mount(r)
		h.audit.Mount(r)
		h.operators.Mount(r)
		h.scenarios.Mount(r)
		h.hub.Mount(r)
	})

	return router
}

type healthResponse struct {
	Status   string    `json:"status"`
	Database string    `json:"database"`
	Time     time.Time `json:"time"`
}

func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		response := healthResponse{Status: "ok", Database: "up", Time: time.Now().UTC()}
		status := http.StatusOK
		if err := pool.Ping(ctx); err != nil {
			response.Status = "degraded"
			response.Database = "down"
			status = http.StatusServiceUnavailable
		}
		httpx.JSON(w, status, response)
	}
}

func verifyDatabase(ctx context.Context, pool *pgxpool.Pool) error {
	var postgisVersion string
	if err := pool.QueryRow(ctx, "SELECT postgis_version()").Scan(&postgisVersion); err != nil {
		return fmt.Errorf("PostGIS is not available in the configured database (run `task db:migrate`): %w", err)
	}

	var table *string
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.sources')::text").Scan(&table); err != nil {
		return fmt.Errorf("check schema: %w", err)
	}
	if table == nil || *table == "" {
		return fmt.Errorf("database schema is not migrated (run `task db:migrate`)")
	}
	return nil
}
