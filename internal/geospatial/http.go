package geospatial

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes geospatial capabilities over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the geospatial service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers geofence and spatial query routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/geofences", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{id}", h.get)
		r.Post("/{id}/active", h.setActive)
	})
	r.Route("/geospatial", func(r chi.Router) {
		r.Get("/geofences-containing", h.containing)
		r.Get("/assets-within", h.assetsWithin)
		r.Get("/nearest-assets", h.nearest)
	})
}

type createRequest struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Severity string         `json:"severity"`
	Polygon  []geo.Point    `json:"polygon"`
	Active   *bool          `json:"active"`
	Metadata map[string]any `json:"metadata"`
}

type setActiveRequest struct {
	Active *bool `json:"active"`
}

type geofenceResponse struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Severity  string         `json:"severity"`
	Active    bool           `json:"active"`
	GeoJSON   string         `json:"geojson"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type assetDistanceResponse struct {
	AssetID         string     `json:"assetId"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	ConnectionState string     `json:"connectionState"`
	Position        *geo.Point `json:"position"`
	DistanceM       float64    `json:"distanceM"`
	LastSeenAt      *time.Time `json:"lastSeenAt"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	geofence, err := h.svc.Create(r.Context(), CreateInput{
		ID:       req.ID,
		Name:     req.Name,
		Type:     Type(req.Type),
		Severity: Severity(req.Severity),
		Polygon:  req.Polygon,
		Active:   req.Active,
		Metadata: req.Metadata,
	})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toResponse(geofence))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	geofence, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(geofence))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, geofenceList(items, total))
}

func (h *Handler) setActive(w http.ResponseWriter, r *http.Request) {
	var req setActiveRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	if req.Active == nil {
		httpx.Error(w, apperr.Validation("active is required"))
		return
	}
	geofence, err := h.svc.SetActive(r.Context(), chi.URLParam(r, "id"), *req.Active)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(geofence))
}

func (h *Handler) containing(w http.ResponseWriter, r *http.Request) {
	point, err := queryPoint(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	items, err := h.svc.ContainingPoint(r.Context(), point)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, geofenceList(items, len(items)))
}

func (h *Handler) assetsWithin(w http.ResponseWriter, r *http.Request) {
	point, err := queryPoint(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	radiusM, err := strconv.ParseFloat(r.URL.Query().Get("radius_m"), 64)
	if err != nil {
		httpx.Error(w, apperr.Validation("radius_m must be a number"))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	items, err := h.svc.AssetsWithinRadius(r.Context(), point, radiusM, limit)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, assetList(items))
}

func (h *Handler) nearest(w http.ResponseWriter, r *http.Request) {
	point, err := queryPoint(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	items, err := h.svc.NearestAssets(r.Context(), point, limit)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, assetList(items))
}

func queryPoint(r *http.Request) (geo.Point, error) {
	lat, err := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	if err != nil {
		return geo.Point{}, apperr.Validation("lat must be a number")
	}
	lng, err := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
	if err != nil {
		return geo.Point{}, apperr.Validation("lng must be a number")
	}
	point := geo.Point{Lat: lat, Lng: lng}
	if !point.Valid() {
		return geo.Point{}, apperr.Validation("lat/lng must be a valid WGS84 coordinate")
	}
	return point, nil
}

func geofenceList(items []Geofence, total int) httpx.ListResponse[geofenceResponse] {
	resp := httpx.ListResponse[geofenceResponse]{
		Items: make([]geofenceResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item))
	}
	return resp
}

func assetList(items []AssetDistance) httpx.ListResponse[assetDistanceResponse] {
	resp := httpx.ListResponse[assetDistanceResponse]{
		Items: make([]assetDistanceResponse, 0, len(items)),
		Total: len(items),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toAssetResponse(item))
	}
	return resp
}

func toResponse(geofence Geofence) geofenceResponse {
	metadata := geofence.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	return geofenceResponse{
		ID:        geofence.ID,
		Name:      geofence.Name,
		Type:      string(geofence.Type),
		Severity:  string(geofence.Severity),
		Active:    geofence.Active,
		GeoJSON:   geofence.GeoJSON,
		Metadata:  metadata,
		CreatedAt: geofence.CreatedAt,
		UpdatedAt: geofence.UpdatedAt,
	}
}

func toAssetResponse(asset AssetDistance) assetDistanceResponse {
	return assetDistanceResponse{
		AssetID:         asset.AssetID,
		Name:            asset.Name,
		Type:            asset.Type,
		Status:          asset.Status,
		ConnectionState: asset.ConnectionState,
		Position:        asset.Position,
		DistanceM:       asset.DistanceM,
		LastSeenAt:      asset.LastSeenAt,
	}
}
