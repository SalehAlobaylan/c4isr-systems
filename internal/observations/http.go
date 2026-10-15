package observations

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes observations over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the observation service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers observation routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/observations", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.ingest)
		r.Get("/{id}", h.get)
	})
}

type ingestRequest struct {
	ID         string     `json:"id"`
	SourceID   string     `json:"sourceId"`
	Type       string     `json:"type"`
	ObservedAt *time.Time `json:"observedAt"`
	ReceivedAt *time.Time `json:"receivedAt"`
	Position   *geo.Point `json:"position"`
	// Area reports an event with extent - a wildfire, an earthquake, a weather
	// cell - instead of a moving object. It carries a footprint and never
	// becomes a track.
	Area      *areaRequest   `json:"area"`
	Payload   map[string]any `json:"payload"`
	Quality   map[string]any `json:"quality"`
	TrackHint string         `json:"trackHint"`
}

type areaRequest struct {
	Kind    string      `json:"kind"`
	Polygon []geo.Point `json:"polygon"`
}

type observationResponse struct {
	ID          string         `json:"id"`
	SourceID    string         `json:"sourceId"`
	Type        string         `json:"type"`
	ObservedAt  time.Time      `json:"observedAt"`
	ReceivedAt  time.Time      `json:"receivedAt"`
	ProcessedAt *time.Time     `json:"processedAt,omitempty"`
	Position    *geo.Point     `json:"position,omitempty"`
	Area        *areaResponse  `json:"area,omitempty"`
	Payload     map[string]any `json:"payload"`
	Quality     map[string]any `json:"quality,omitempty"`
	TrackHint   string         `json:"trackHint,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	Duplicate   bool           `json:"duplicate,omitempty"`
}

// areaResponse is the GeoJSON the browser renders. The polygon is returned in
// the same closed-ring encoding as a geofence so one map layer path draws both.
type areaResponse struct {
	Kind    string      `json:"kind"`
	GeoJSON string      `json:"geojson"`
	Center  geo.Point   `json:"center"`
	AreaM2  float64     `json:"areaM2"`
	Bounds  *areaBounds `json:"bounds,omitempty"`
}

type areaBounds struct {
	MinLat float64 `json:"minLat"`
	MinLng float64 `json:"minLng"`
	MaxLat float64 `json:"maxLat"`
	MaxLng float64 `json:"maxLng"`
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	in := CreateInput{
		ID:        req.ID,
		SourceID:  req.SourceID,
		Type:      req.Type,
		Position:  req.Position,
		Payload:   req.Payload,
		Quality:   req.Quality,
		TrackHint: req.TrackHint,
	}
	if req.ObservedAt != nil {
		in.ObservedAt = *req.ObservedAt
	}
	if req.ReceivedAt != nil {
		in.ReceivedAt = *req.ReceivedAt
	}
	if req.Area != nil {
		in.Area = &Area{Kind: AreaKind(req.Area.Kind), Polygon: req.Area.Polygon}
	}

	result, err := h.svc.Ingest(r.Context(), in)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	status := http.StatusCreated
	if result.Duplicate {
		status = http.StatusOK
	}
	httpx.JSON(w, status, toResponse(result.Observation, result.Duplicate))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	obs, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(obs, false))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	filter := ListFilter{
		TrackID:  r.URL.Query().Get("track_id"),
		AreaOnly: r.URL.Query().Get("area") == "true",
	}
	if kind := strings.TrimSpace(r.URL.Query().Get("area_kind")); filter.AreaOnly && kind != "" {
		filter.AreaKind = AreaKind(kind)
	}
	// An area_kind that is not a real phenomenon would silently return nothing,
	// which reads as "no hazards" rather than "you asked for something invalid".
	if filter.AreaOnly && filter.AreaKind != "" && !validAreaKind(filter.AreaKind) {
		httpx.Error(w, apperr.Validation("areaKind must be one of hazard, weather, environmental, infrastructure, crowd"))
		return
	}
	items, total, err := h.svc.List(r.Context(), filter, limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[observationResponse]{
		Items: make([]observationResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item, false))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func toResponse(obs Observation, duplicate bool) observationResponse {
	payload := obs.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	return observationResponse{
		ID:          obs.ID,
		SourceID:    obs.SourceID,
		Type:        obs.Type,
		ObservedAt:  obs.ObservedAt,
		ReceivedAt:  obs.ReceivedAt,
		ProcessedAt: obs.ProcessedAt,
		Position:    obs.Position,
		Area:        toAreaResponse(obs.Area),
		Payload:     payload,
		Quality:     obs.Quality,
		TrackHint:   obs.TrackHint,
		CreatedAt:   obs.CreatedAt,
		Duplicate:   duplicate,
	}
}

// toAreaResponse renders a footprint for the interface. Bounds are included so
// the browser can frame a large hazard without walking the GeoJSON.
func toAreaResponse(area *Area) *areaResponse {
	if area == nil {
		return nil
	}
	return &areaResponse{
		Kind:    string(area.Kind),
		GeoJSON: area.GeoJSON(),
		Center:  area.Centroid(),
		AreaM2:  area.AreaMeters2(),
		Bounds:  boundsOf(area.Polygon),
	}
}

func boundsOf(polygon []geo.Point) *areaBounds {
	if len(polygon) == 0 {
		return nil
	}
	bounds := &areaBounds{
		MinLat: polygon[0].Lat,
		MaxLat: polygon[0].Lat,
		MinLng: polygon[0].Lng,
		MaxLng: polygon[0].Lng,
	}
	for _, point := range polygon[1:] {
		if point.Lat < bounds.MinLat {
			bounds.MinLat = point.Lat
		}
		if point.Lat > bounds.MaxLat {
			bounds.MaxLat = point.Lat
		}
		if point.Lng < bounds.MinLng {
			bounds.MinLng = point.Lng
		}
		if point.Lng > bounds.MaxLng {
			bounds.MaxLng = point.Lng
		}
	}
	return bounds
}
