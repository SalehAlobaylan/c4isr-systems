package picture

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler serves reconstructed pictures.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the picture service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers replay routes.
//
// Replay is read-only. It reconstructs from evidence and never writes, so an
// operator scrubbing a timeline cannot alter what the audit trail recorded.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/picture", func(r chi.Router) {
		r.Get("/", h.snapshot)
		r.Get("/window", h.window)
	})
}

type trackResponse struct {
	ID             string     `json:"id"`
	ExternalRef    string     `json:"externalRef"`
	Status         string     `json:"status"`
	Position       *pointJSON `json:"position,omitempty"`
	Speed          *float64   `json:"speed"`
	Heading        *float64   `json:"heading"`
	ObservedAt     time.Time  `json:"observedAt"`
	FirstSeenAt    time.Time  `json:"firstSeenAt"`
	LastSeenAt     time.Time  `json:"lastSeenAt"`
	Freshness      string     `json:"freshness"`
	FreshnessLabel string     `json:"freshnessLabel"`
}

type assetResponse struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	Position        *pointJSON `json:"position,omitempty"`
	Speed           *float64   `json:"speed"`
	Heading         *float64   `json:"heading"`
	ObservedAt      time.Time  `json:"observedAt"`
	ConnectionState string     `json:"connectionState"`
	Health          string     `json:"health"`
	Freshness       string     `json:"freshness"`
	FreshnessLabel  string     `json:"freshnessLabel"`
}

type pointJSON struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type alertResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Severity   string         `json:"severity"`
	State      string         `json:"state"`
	Title      string         `json:"title"`
	Message    string         `json:"message"`
	TrackID    string         `json:"trackId"`
	AssetID    string         `json:"assetId"`
	GeofenceID string         `json:"geofenceId"`
	CreatedAt  time.Time      `json:"createdAt"`
	Reference  map[string]any `json:"sourceReference,omitempty"`
}

type geofenceResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Active   bool   `json:"active"`
	GeoJSON  string `json:"geojson"`
}

type snapshotResponse struct {
	At              time.Time          `json:"at"`
	Live            bool               `json:"live"`
	Tracks          []trackResponse    `json:"tracks"`
	Assets          []assetResponse    `json:"assets"`
	Alerts          []alertResponse    `json:"alerts"`
	Geofences       []geofenceResponse `json:"geofences"`
	Window          windowResponse     `json:"window"`
	TruncatedAlerts bool               `json:"truncatedAlerts"`
}

type windowResponse struct {
	Earliest time.Time `json:"earliest"`
	Latest   time.Time `json:"latest"`
}

func (h *Handler) snapshot(w http.ResponseWriter, r *http.Request) {
	req := Request{
		At:          parseInstant(r.URL.Query().Get("at")),
		ScopePrefix: r.URL.Query().Get("scope"),
		// `live=true` returns the operator to the present explicitly, rather
		// than relying on the instant happening to equal now.
		Live: r.URL.Query().Get("live") == "true",
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if limit, err := strconv.Atoi(raw); err == nil {
			req.Limit = limit
		}
	}

	snapshot, err := h.svc.Snapshot(r.Context(), req)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toSnapshotResponse(snapshot))
}

func (h *Handler) window(w http.ResponseWriter, r *http.Request) {
	window, err := h.svc.Window(r.Context(), r.URL.Query().Get("scope"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, windowResponse{Earliest: window.Earliest, Latest: window.Latest})
}

// parseInstant accepts an RFC3339 timestamp. An unparseable value yields the
// zero time, which the service treats as "now" - the live picture - rather than
// failing the whole request over a display-level mistake.
func parseInstant(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func toSnapshotResponse(snapshot Snapshot) snapshotResponse {
	resp := snapshotResponse{
		At:              snapshot.At,
		Live:            snapshot.Live,
		Tracks:          make([]trackResponse, 0, len(snapshot.Tracks)),
		Assets:          make([]assetResponse, 0, len(snapshot.Assets)),
		Alerts:          make([]alertResponse, 0, len(snapshot.Alerts)),
		Geofences:       make([]geofenceResponse, 0, len(snapshot.Geofences)),
		TruncatedAlerts: snapshot.TruncatedAlerts,
		Window:          windowResponse{Earliest: snapshot.Window.Earliest, Latest: snapshot.Window.Latest},
	}
	for _, track := range snapshot.Tracks {
		resp.Tracks = append(resp.Tracks, trackResponse{
			ID:             track.ID,
			ExternalRef:    track.ExternalRef,
			Status:         track.Status,
			Position:       toPointJSON(track.Position),
			Speed:          track.Speed,
			Heading:        track.Heading,
			ObservedAt:     track.ObservedAt,
			FirstSeenAt:    track.FirstSeenAt,
			LastSeenAt:     track.LastSeenAt,
			Freshness:      string(track.Freshness),
			FreshnessLabel: track.Freshness.Label(),
		})
	}
	for _, asset := range snapshot.Assets {
		resp.Assets = append(resp.Assets, assetResponse{
			ID:              asset.ID,
			Name:            asset.Name,
			Type:            asset.Type,
			Status:          asset.Status,
			Position:        toPointJSON(asset.Position),
			Speed:           asset.Speed,
			Heading:         asset.Heading,
			ObservedAt:      asset.ObservedAt,
			ConnectionState: asset.ConnectionState,
			Health:          asset.Health,
			Freshness:       string(asset.Freshness),
			FreshnessLabel:  asset.Freshness.Label(),
		})
	}
	for _, alert := range snapshot.Alerts {
		resp.Alerts = append(resp.Alerts, alertResponse{
			ID:         alert.ID,
			Type:       alert.Type,
			Severity:   alert.Severity,
			State:      alert.State,
			Title:      alert.Title,
			Message:    alert.Message,
			TrackID:    alert.TrackID,
			AssetID:    alert.AssetID,
			GeofenceID: alert.GeofenceID,
			CreatedAt:  alert.CreatedAt,
			Reference:  alert.SourceReference,
		})
	}
	for _, geofence := range snapshot.Geofences {
		resp.Geofences = append(resp.Geofences, geofenceResponse{
			ID:       geofence.ID,
			Name:     geofence.Name,
			Type:     geofence.Type,
			Severity: geofence.Severity,
			Active:   geofence.Active,
			GeoJSON:  geofence.GeoJSON,
		})
	}
	return resp
}

func toPointJSON(point *geo.Point) *pointJSON {
	if point == nil {
		return nil
	}
	return &pointJSON{Lat: point.Lat, Lng: point.Lng}
}
