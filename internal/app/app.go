// Package app composes the C4ISR server: repositories, application services,
// event subscriptions, realtime hub, transports, and the HTTP router.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/acquisition"
	"github.com/SalehAlobaylan/c4isr-systems/internal/alerts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/analystscope"
	"github.com/SalehAlobaylan/c4isr-systems/internal/arearule"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/assessments"
	"github.com/SalehAlobaylan/c4isr-systems/internal/assets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/audit"
	"github.com/SalehAlobaylan/c4isr-systems/internal/catalog"
	"github.com/SalehAlobaylan/c4isr-systems/internal/classifications"
	"github.com/SalehAlobaylan/c4isr-systems/internal/commands"
	"github.com/SalehAlobaylan/c4isr-systems/internal/contentpurpose"
	"github.com/SalehAlobaylan/c4isr-systems/internal/contextpackage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/coverage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/eventrules"
	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/exerciseingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/externaltracks"
	"github.com/SalehAlobaylan/c4isr-systems/internal/geoenv"
	"github.com/SalehAlobaylan/c4isr-systems/internal/geospatial"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imagery"
	"github.com/SalehAlobaylan/c4isr-systems/internal/incidents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/investigations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/missions"
	"github.com/SalehAlobaylan/c4isr-systems/internal/modelassets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/nativeexercise"
	"github.com/SalehAlobaylan/c4isr-systems/internal/observations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/operators"
	"github.com/SalehAlobaylan/c4isr-systems/internal/picture"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/observability"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/readiness"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publiccontext"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/realtime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/recordings"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/SalehAlobaylan/c4isr-systems/internal/scenarios"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simgateway"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/sources"
	"github.com/SalehAlobaylan/c4isr-systems/internal/telemetry"
	"github.com/SalehAlobaylan/c4isr-systems/internal/tracks"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// DefaultOperator is the local-development actor used when authentication is
// explicitly disabled (for example, by the opt-in integration harness).
const DefaultOperator = "operator-01"

// App is the assembled server.
type App struct {
	config             config.Config
	logger             *slog.Logger
	pool               *pgxpool.Pool
	handler            http.Handler
	scenarios          *scenarios.Service
	simGateway         *simgateway.Service
	staleMonitorCancel context.CancelFunc
	staleMonitorDone   <-chan struct{}
}

type ingestionOptions struct {
	artifactStore artifacts.Store
	scenariosDir  string
	locations     publicingest.Registry
	fetcher       publicingest.Fetcher
}
type Option func(*ingestionOptions)

func WithArtifactStore(store artifacts.Store) Option {
	return func(o *ingestionOptions) { o.artifactStore = store }
}

// WithPublicIngestion supplies trusted adapters at the composition boundary.
// Environment configuration always uses the restricted HTTPS fetcher.
func WithPublicIngestion(locations publicingest.Registry, fetcher publicingest.Fetcher) Option {
	return func(o *ingestionOptions) { o.locations = locations; o.fetcher = fetcher }
}

// WithScenariosDirectory supplies a trusted local fixture/authoring directory.
func WithScenariosDirectory(dir string) Option {
	return func(o *ingestionOptions) { o.scenariosDir = dir }
}

// New opens the database, verifies migrations, wires every module, and builds
// the HTTP router.
func New(ctx context.Context, cfg config.Config, logger *slog.Logger, options ...Option) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	metrics := observability.New()
	pool, err := db.Open(ctx, cfg.DatabaseURL, metrics)
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
	for _, topic := range events.AllTopics() {
		bus.Subscribe(topic, func(eventCtx context.Context, ev events.Event) {
			metrics.ObserveDomainEvent(ev)
			logDomainEvent(logger, eventCtx, ev)
		})
	}

	// Infrastructure modules first so audit and realtime observe the full
	// stream and operator actions remain traceable.
	auditSvc := audit.NewService(audit.NewPostgresRepository(pool), bus)
	operatorSvc := operators.NewService(operators.NewPostgresRepository(pool))
	if _, _, err := operatorSvc.Ensure(access.Bootstrap(ctx), operators.CreateInput{
		ID:   DefaultOperator,
		Name: "Operator 01",
		Role: operators.RoleOperator,
	}); err != nil {
		return nil, fmt.Errorf("ensure default operator: %w", err)
	}
	lookupOperator := func(ctx context.Context, id string) (httpx.Identity, error) {
		operator, err := operatorSvc.Get(access.Bootstrap(ctx), id)
		if err != nil {
			return httpx.Identity{}, err
		}
		return httpx.Identity{
			ID:   operator.ID,
			Name: operator.Name,
			Role: string(operator.Role),
		}, nil
	}

	identityStore := access.NewStore(pool)
	if err := identityStore.LegacyCredentials(ctx, cfg.AuthTokens); err != nil {
		return nil, fmt.Errorf("initialize identity credentials: %w", err)
	}

	// Freshness thresholds are resolved once here so the API, the realtime
	// stream, and any future integration all classify trust identically.
	freshnessPolicy := cfg.FreshnessPolicy()

	sourceSvc := sources.NewService(sources.NewPostgresRepository(pool), bus)
	locations, err := publicingest.LoadRegistry(cfg.PublicConnectorsFile)
	if err != nil {
		return nil, err
	}
	ingestion := ingestionOptions{locations: locations, fetcher: publicingest.NewHTTPFetcher()}
	for _, option := range options {
		option(&ingestion)
	}
	ingestionRepo := publicingest.NewRepository(pool)
	var artifactStore artifacts.Store = artifacts.Unavailable{}
	if cfg.ArtifactS3Endpoint != "" {
		artifactStore, err = artifacts.NewS3(artifacts.S3Config{Endpoint: cfg.ArtifactS3Endpoint, Bucket: cfg.ArtifactS3Bucket, Region: cfg.ArtifactS3Region, AccessKey: cfg.ArtifactS3AccessKey, SecretKey: cfg.ArtifactS3SecretKey, AllowHTTP: cfg.ArtifactS3AllowHTTP || cfg.Environment == config.EnvironmentDevelopment || cfg.Environment == config.EnvironmentTest})
		if err != nil {
			return nil, err
		}
	}
	if ingestion.artifactStore != nil {
		artifactStore = ingestion.artifactStore
	}
	artifactSvc := artifacts.NewService(pool, artifactStore)
	imagerySvc := imagery.NewService(pool, artifactSvc)
	reviewSvc := assessments.NewReviews(pool, imagerySvc)
	ingestionRepo.SetProjector(publiccontext.Projector{Next: externaltracks.Projector{Next: imagery.Projector{Next: publicevents.Projector{}}}})
	ingestionSvc := publicingest.NewService(ingestionRepo, ingestion.locations, ingestion.fetcher, sourceSvc, identityStore)
	if cfg.EnvironmentLibraryEnabled || cfg.EnvironmentToolchain != "" {
		ingestionSvc.SetGeographicRetentionReader(geoenv.Service{Pool: pool})
	}
	sourceSvc.SetCatalog(ingestionRepo)
	assetSvc := assets.NewServiceWithFreshness(assets.NewPostgresRepository(pool), bus, freshnessPolicy)
	observationSvc := observations.NewService(observations.NewPostgresRepository(pool), sourceSvc, bus)
	telemetrySvc := telemetry.NewService(telemetry.NewPostgresRepository(pool), assetSvc, bus)
	trackSvc := tracks.NewServiceWithFreshness(tracks.NewPostgresRepository(pool), observationSvc, bus, freshnessPolicy)
	classificationSvc := classifications.NewService(classifications.NewPostgresRepository(pool), bus)
	geofenceSvc := geospatial.NewService(geospatial.NewPostgresRepository(pool), bus)
	geofenceSvc.SetObserver(metrics)
	// Area observations are routed to spatial rules instead of the track
	// pipeline: a wildfire or a weather cell has extent, not a trajectory.
	areaRuleSvc := arearule.NewService(arearule.NewGeospatialAdapter(geofenceSvc), bus)
	areaRuleSvc.SetObserver(metrics)
	areaRuleSvc.SetFailureObserver(metrics)
	eventrules.New(pool, bus)
	// Alerts name their subject in operator language rather than by internal
	// identifier, so the alert centre reads as prose an operator can act on.
	alertSvc := alerts.NewServiceWithClassifier(alerts.NewPostgresRepository(pool), bus, classificationSvc)
	incidentSvc := incidents.NewService(incidents.NewPostgresRepository(pool), bus)
	missionSvc := missions.NewService(missions.NewPostgresRepository(pool), assetSvc, bus)
	commandSvc := commands.NewService(commands.NewPostgresRepository(pool), assetSvc, bus)
	assessmentSvc := assessments.NewService(assessments.NewPostgresRepository(pool), bus)

	// Realtime is registered after domain subscribers so notifications carry
	// the state changes their handlers produced.
	hub := realtime.NewHub(logger, bus, metrics)

	var scenarioSvc *scenarios.Service
	if !cfg.MapOnly() {
		scenarioSvc = scenarios.NewService(
			scenarios.NewPostgresRepository(pool),
			scenarios.NewLoader(func() string {
				if ingestion.scenariosDir != "" {
					return ingestion.scenariosDir
				}
				return cfg.ScenariosDir
			}()),
			scenarios.EngineDeps{
				Ingress:         exerciseingest.NewService(pool, observationSvc, telemetrySvc),
				ContextPackages: contextpackage.NewService(pool, artifactSvc),
				Sources:         sourceSvc,
				Assets:          assetSvc,
				Observations:    observationSvc,
				Telemetry:       telemetrySvc,
				Geofences:       geofenceSvc,
				Classifications: classificationSvc,
				Assessments:     assessmentSvc,
				Incidents:       incidentSvc,
				Commands:        commandSvc,
				Tracks:          trackSvc,
				Logger:          logger,
			},
			bus,
		)

	}
	native := &nativeexercise.Service{Pool: pool, Sources: sourceSvc, Assets: assetSvc, Observations: observationSvc, Telemetry: telemetrySvc, Commands: commandSvc, Missions: missionSvc}
	if scenarioSvc != nil {
		scenarioSvc.NativeRun = native.GetRun
	}
	clock := func(ctx context.Context, wall time.Time) time.Time {
		if cfg.MapOnly() {
			return wall
		}
		n := native.Clock(ctx, wall)
		if n != wall {
			return n
		}
		return scenarioSvc.ClockNow(ctx, wall)
	}
	studio := &simscene.Service{Pool: pool, Assets: modelassets.New(pool, artifactStore), Packages: contextpackage.NewService(pool, artifactSvc)}
	recorder := &recordings.Service{Pool: pool, Store: artifactStore}
	allowLocalDebrief := cfg.Environment == config.EnvironmentDevelopment || cfg.Environment == config.EnvironmentTest
	simGatewaySvc := simgateway.NewService(
		cfg.SimWorkerURL,
		cfg.SimWorkerSecret,
		cfg.SimMaxConcurrentJobs,
		allowLocalDebrief,
		pool,
		artifactStore,
		logger,
	)
	router := buildRouter(cfg, logger, pool, metrics, lookupOperator, identityStore, moduleHandlers{
		blobStore:       artifactStore,
		recordings:      recorder,
		modelAssets:     studio.Assets,
		studio:          studio,
		runtime:         &simruntime.Broker{Pool: pool, Scenes: studio, Exercise: native, Recorder: recorder, AdmittedBuilds: cfg.RuntimeBuilds},
		simGateway:      simGatewaySvc,
		contextRecords:  publiccontext.Service{Pool: pool},
		externalTracks:  externaltracks.Service{Pool: pool},
		artifacts:       artifacts.Handler{Service: artifactSvc},
		imagery:         imagery.Handler{Service: imagerySvc},
		reviews:         reviewSvc,
		sources:         sources.NewHandler(sourceSvc),
		publicevents:    publicevents.NewHandler(pool),
		reference:       reference.NewHandler(reference.NewService(pool)),
		contextPackages: contextpackage.NewHandler(contextpackage.NewService(pool, artifactSvc)),
		acquisition:     acquisition.Service{Pool: pool, Ingestion: ingestionSvc},
		coverage:        coverage.Service{Pool: pool, Ingestion: ingestionSvc, Metrics: metrics},
		publicingest:    publicingest.NewHandler(ingestionSvc),
		observations:    observations.NewHandler(observationSvc),
		assets:          assets.NewHandler(assetSvc),
		telemetry:       telemetry.NewHandlerWithFreshness(telemetrySvc, freshnessPolicy),
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
		clock:           clock,
		picture:         picture.NewHandler(picture.NewService(picture.NewPostgresRepository(pool), freshnessPolicy)),
		hub:             hub,
	})
	freshnessMonitor := telemetry.NewFreshnessMonitor(telemetry.NewPostgresRepository(pool), bus, metrics, freshnessPolicy)
	var staleMonitorCancel context.CancelFunc
	var staleMonitorDone <-chan struct{}
	if !cfg.MapOnly() {
		staleMonitorCancel, staleMonitorDone = startFreshnessMonitor(freshnessMonitor, freshnessPolicy, logger, identityStore, clock)
	}

	poolClosed = true
	return &App{
		config:             cfg,
		logger:             logger,
		pool:               pool,
		handler:            router,
		scenarios:          scenarioSvc,
		simGateway:         simGatewaySvc,
		staleMonitorCancel: staleMonitorCancel,
		staleMonitorDone:   staleMonitorDone,
	}, nil
}

// Handler returns the assembled HTTP handler.
func (a *App) Handler() http.Handler { return a.handler }

// Close releases the connection pool.
func (a *App) Close() {
	if a.simGateway != nil {
		a.simGateway.Close()
	}
	if a.staleMonitorCancel != nil {
		a.staleMonitorCancel()
		if a.staleMonitorDone != nil {
			select {
			case <-a.staleMonitorDone:
			case <-time.After(5 * time.Second):
				if a.logger != nil {
					a.logger.Warn("stale telemetry monitor did not stop before timeout")
				}
			}
		}
		a.staleMonitorCancel = nil
	}
	if a.scenarios != nil {
		// Drain scenario engines before closing the pool. The scenario service
		// bounds its terminal persistence operations; using a cancelable
		// shutdown context here could otherwise close the pool while a runner
		// still has an in-flight action that needs to persist its final state.
		if err := a.scenarios.Close(access.Bootstrap(context.Background())); err != nil && a.logger != nil {
			a.logger.Error("close scenario service", "error", err)
		}
	}
	if a.pool != nil {
		a.pool.Close()
	}
}

// startFreshnessMonitor runs the periodic reclassification of every asset.
//
// The tick interval follows the tightest configured freshness band, so an
// entity never sits visibly live for long after it should have been marked
// aging. Freshness is still re-derived on every read; the monitor exists to
// push transitions to clients that already hold a rendered picture.
func startFreshnessMonitor(monitor *telemetry.FreshnessMonitor, policy freshness.Policy, logger *slog.Logger, store *access.Store, clock freshness.Clock) (context.CancelFunc, <-chan struct{}) {
	interval := policy.DelayedAfter / 2
	if interval > time.Minute {
		interval = time.Minute
	}
	if interval < time.Second {
		interval = time.Second
	}
	ctx, cancel := context.WithCancel(freshness.WithClock(context.Background(), clock))
	done := make(chan struct{})
	go func() {
		defer close(done)
		reconcile := func() {
			resources, err := store.ObservationResources(access.Bootstrap(ctx))
			if err != nil {
				logger.Error("list freshness resources", "error", err)
				return
			}
			for _, resource := range resources {
				if err := monitor.Reconcile(access.Internal(ctx, resource, "freshness")); err != nil && ctx.Err() == nil {
					logger.Error("reconcile entity freshness", "error", err)
				}
			}
		}
		reconcile()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reconcile()
			}
		}
	}()
	return cancel, done
}

type moduleHandlers struct {
	blobStore       artifacts.Store
	studio          *simscene.Service
	runtime         *simruntime.Broker
	simGateway      *simgateway.Service
	recordings      *recordings.Service
	modelAssets     *modelassets.Service
	contextRecords  publiccontext.Service
	externalTracks  externaltracks.Service
	artifacts       artifacts.Handler
	imagery         imagery.Handler
	reviews         *assessments.Reviews
	contextPackages *contextpackage.Handler
	clock           freshness.Clock
	reference       *reference.Handler
	publicevents    *publicevents.Handler
	acquisition     acquisition.Service
	coverage        coverage.Service
	publicingest    *publicingest.Handler
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
	picture         *picture.Handler
	hub             *realtime.Hub
}

func buildRouter(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, metrics *observability.Metrics, lookupOperator httpx.OperatorLookup, store *access.Store, h moduleHandlers) http.Handler {
	router := chi.NewRouter()
	router.Use(
		httpx.RequestID,
		httpx.Recoverer(logger),
		httpx.LoggerWithMetrics(logger, metrics),
		httpx.CORS(cfg.AllowedOrigins),
		httpx.Authenticate(httpx.AuthOptions{
			Resolve:         identityResolver(cfg, store, lookupOperator, pool),
			Required:        cfg.AuthRequired,
			Tokens:          cfg.AuthTokens,
			DefaultOperator: DefaultOperator,
			Lookup:          lookupOperator,
			OnFailure:       metrics.ObserveAuthFailure,
		}),
		httpx.Authorization,
		httpx.ResponseDisclosure,
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(freshness.WithClock(r.Context(), h.clock)))
			})
		},
	)

	router.Get("/health", healthHandler(pool, metrics))
	router.Get("/metrics", metrics.Handler())

	router.Route("/api/v1", func(r chi.Router) {
		r.Use(contentpurpose.CatalogView)
		r.Use(analystscope.Service{Pool: pool}.CatalogQueries)
		contentpurpose.Service{Pool: pool}.Mount(r)
		areas.Service{Pool: pool}.Mount(r)
		analystscope.Service{Pool: pool}.Mount(r)
		catalog.Service{Pool: pool}.Mount(r)
		if cfg.EnvironmentLibraryEnabled || cfg.EnvironmentToolchain != "" {
			objects, _ := h.blobStore.(artifacts.GeographicStore)
			geoenv.Service{Pool: pool, Store: objects, Toolchain: cfg.EnvironmentToolchain, EnrichmentToolchain: cfg.EnvironmentEnrichmentToolchain, ObservationToolchain: cfg.EnvironmentObservationToolchain, CoastalToolchain: cfg.EnvironmentCoastalToolchain, DisplayToolchain: cfg.EnvironmentDisplayToolchain, LayerToolchain: cfg.EnvironmentLayerToolchain, Spool: cfg.EnvironmentSpool, Parallel: cfg.EnvironmentParallel, ReadOnly: !cfg.EnvironmentLibraryEnabled, UploadSlots: make(chan struct{}, 2)}.Mount(r)
		}
		investigations.Service{Pool: pool, Store: h.blobStore}.Mount(r)
		blobStore := h.blobStore
		if blobStore == nil {
			blobStore = artifacts.Unavailable{}
		}
		readiness.Service{Pool: pool, Store: blobStore, RuntimeBuilds: cfg.RuntimeBuilds, MapOnly: cfg.MapOnly(), EnvironmentLibrary: cfg.EnvironmentLibraryEnabled}.Mount(r)
		r.Get("/auth/me", func(w http.ResponseWriter, req *http.Request) {
			resources, err := store.SelectableResources(req.Context())
			if err != nil {
				httpx.Error(w, err)
				return
			}
			if cfg.MapOnly() {
				visible := resources[:0]
				for _, resource := range resources {
					if resource.Partition == access.PublicEvidence {
						visible = append(visible, resource)
					}
				}
				resources = visible
			}
			details, err := store.ResourceDetails(req.Context(), resources)
			if err != nil {
				httpx.Error(w, err)
				return
			}
			identity := sessionOperator{
				ID:   httpx.GetOperatorID(req.Context()),
				Name: httpx.GetOperatorName(req.Context()),
				Role: httpx.GetOperatorRole(req.Context()),
			}
			httpx.JSON(w, http.StatusOK, map[string]any{
				"operator":        identity,
				"productProfile":  cfg.Profile(),
				"permissions":     access.EffectivePermissions(req.Context()),
				"resources":       resources,
				"resourceDetails": details,
				"accessRevision":  access.AccessRevision(req.Context()),
				"principal":       func() access.Principal { principal, _, _ := access.From(req.Context()); return principal }(),
				"resource":        func() access.Resource { _, resource, _ := access.From(req.Context()); return resource }(),
			})
		})
		if !cfg.MapOnly() && h.studio != nil {
			h.studio.Mount(r)
		}
		if !cfg.MapOnly() && h.runtime != nil {
			h.runtime.Mount(r)
			if h.recordings != nil {
				h.recordings.Mount(r)
			}
		}
		if !cfg.MapOnly() && h.modelAssets != nil {
			h.modelAssets.Mount(r)
		}
		if h.simGateway != nil {
			h.simGateway.Mount(r)
		}
		h.contextRecords.Mount(r)
		h.externalTracks.Mount(r)
		h.artifacts.Mount(r)
		h.imagery.Mount(r)
		if h.reviews != nil {
			h.reviews.Mount(r)
		}
		h.sources.Mount(r)
		h.publicingest.Mount(r)
		h.acquisition.Mount(r)
		h.coverage.Mount(r)
		h.publicevents.Mount(r)
		h.reference.Mount(r)
		h.contextPackages.Mount(r)
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
		if !cfg.MapOnly() && h.scenarios != nil {
			h.scenarios.Mount(r)
		}
		h.picture.Mount(r)
		h.hub.Mount(r)
	})

	// otelhttp uses the process-wide OpenTelemetry provider. With no exporter
	// configured it is a cheap no-op; deployments can install a provider and
	// exporter without changing the router or domain modules.
	return httpx.ProtectRealtimeCredential(otelhttp.NewHandler(router, "c4isr.http"))
}

type sessionOperator struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func permissionsForRole(role string) []string {
	return httpx.PermissionsForRole(role)
}

type healthResponse struct {
	Status   string    `json:"status"`
	Database string    `json:"database"`
	Time     time.Time `json:"time"`
}

func healthHandler(pool *pgxpool.Pool, metrics *observability.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		response := healthResponse{Status: "ok", Database: "up", Time: time.Now().UTC()}
		status := http.StatusOK
		if err := pool.Ping(ctx); err != nil {
			response.Status = "degraded"
			response.Database = "down"
			status = http.StatusServiceUnavailable
			metrics.ObserveDatabaseHealth(false)
		} else {
			metrics.ObserveDatabaseHealth(true)
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
	if err := pool.QueryRow(ctx, "SELECT CASE WHEN to_regclass('public.analyst_read_cursors') IS NOT NULL AND to_regclass('public.acquisition_requests') IS NOT NULL AND to_regclass('public.retrieval_profiles') IS NOT NULL AND to_regclass('public.investigations') IS NOT NULL AND to_regclass('public.geography_reference_requests') IS NOT NULL AND to_regclass('public.analyst_read_snapshots') IS NOT NULL AND to_regclass('public.content_purpose_revisions') IS NOT NULL AND to_regclass('public.simulation_sensor_presets') IS NOT NULL AND to_regclass('public.simulation_recording_events') IS NOT NULL AND to_regclass('public.native_exercises') IS NOT NULL AND to_regclass('public.native_task_dispatch') IS NOT NULL AND to_regclass('public.native_projection_batches') IS NOT NULL AND to_regclass('public.simulation_scenes') IS NOT NULL AND to_regclass('public.simulation_runtime_runs') IS NOT NULL AND to_regclass('public.simulation_runtime_events') IS NOT NULL AND to_regclass('public.simulation_runtime_truth') IS NOT NULL AND to_regclass('public.model_asset_versions') IS NOT NULL AND to_regclass('public.model_asset_uploads') IS NOT NULL AND to_regclass('public.public_context_records') IS NOT NULL AND to_regclass('public.external_track_segments') IS NOT NULL AND to_regclass('public.external_track_associations') IS NOT NULL AND to_regclass('public.context_export_previews') IS NOT NULL AND to_regclass('public.assessment_revisions') IS NOT NULL AND to_regclass('public.imagery_jobs') IS NOT NULL AND to_regclass('public.imagery_scenes') IS NOT NULL AND to_regclass('public.artifacts') IS NOT NULL AND to_regclass('public.exercise_messages') IS NOT NULL AND to_regclass('public.simulation_actor_mappings') IS NOT NULL AND to_regclass('public.simulation_versions') IS NOT NULL AND to_regclass('public.context_packages') IS NOT NULL AND EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='observations' AND column_name='semantic_kind') THEN 'reference_datasets' END").Scan(&table); err != nil {
		return fmt.Errorf("check schema: %w", err)
	}
	if table == nil || *table == "" {
		return fmt.Errorf("database schema is not migrated (run `task db:migrate`)")
	}
	return nil
}
