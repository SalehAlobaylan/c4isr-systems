package incidents

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

// Handler exposes incidents over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the incident service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers incident routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/incidents", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{id}", h.get)
		r.Patch("/{id}", h.update)
		r.Post("/{id}/status", h.updateStatus)
		r.Post("/{id}/relations", h.attachRelation)
	})
}

type createRequest struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Priority       string   `json:"priority"`
	AlertIDs       []string `json:"alertIds"`
	TrackIDs       []string `json:"trackIds"`
	AssetIDs       []string `json:"assetIds"`
	ObservationIDs []string `json:"observationIds"`
	AssessmentIDs  []string `json:"assessmentIds"`
}

type updateRequest struct {
	Title            string `json:"title"`
	Description      string `json:"description"`
	Priority         string `json:"priority"`
	AssignedOperator string `json:"assignedOperator"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

type attachRelationRequest struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type incidentResponse struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Priority         string     `json:"priority"`
	Status           string     `json:"status"`
	AssignedOperator string     `json:"assignedOperator,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	ResolvedAt       *time.Time `json:"resolvedAt,omitempty"`
	ClosedAt         *time.Time `json:"closedAt,omitempty"`
}

type relatedAlertResponse struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	State     string    `json:"state"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
}

type relatedTrackResponse struct {
	ID          string     `json:"id"`
	ExternalRef string     `json:"externalRef,omitempty"`
	Status      string     `json:"status"`
	Position    *geo.Point `json:"position,omitempty"`
	LastSeenAt  time.Time  `json:"lastSeenAt"`
}

type relatedAssetResponse struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	Position        *geo.Point `json:"position,omitempty"`
	ConnectionState string     `json:"connectionState,omitempty"`
}

type relatedObservationResponse struct {
	ID         string     `json:"id"`
	SourceID   string     `json:"sourceId"`
	Type       string     `json:"type"`
	ObservedAt time.Time  `json:"observedAt"`
	Position   *geo.Point `json:"position,omitempty"`
}

type relatedAssessmentResponse struct {
	ID          string    `json:"id"`
	SubjectType string    `json:"subjectType"`
	SubjectID   string    `json:"subjectId"`
	Type        string    `json:"type"`
	Conclusion  string    `json:"conclusion"`
	Method      string    `json:"method"`
	Confidence  *float64  `json:"confidence,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type detailResponse struct {
	incidentResponse
	Alerts       []relatedAlertResponse       `json:"alerts"`
	Tracks       []relatedTrackResponse       `json:"tracks"`
	Assets       []relatedAssetResponse       `json:"assets"`
	Observations []relatedObservationResponse `json:"observations"`
	Assessments  []relatedAssessmentResponse  `json:"assessments"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	incident, err := h.svc.Create(r.Context(), CreateInput{
		Title:          req.Title,
		Description:    req.Description,
		Priority:       Priority(req.Priority),
		AlertIDs:       req.AlertIDs,
		TrackIDs:       req.TrackIDs,
		AssetIDs:       req.AssetIDs,
		ObservationIDs: req.ObservationIDs,
		AssessmentIDs:  req.AssessmentIDs,
		Actor:          httpx.GetOperatorID(r.Context()),
	})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toResponse(incident))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	detail, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toDetailResponse(detail))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.svc.List(r.Context(), r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[incidentResponse]{
		Items: make([]incidentResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req updateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	incident, err := h.svc.Update(r.Context(), chi.URLParam(r, "id"), UpdateInput{
		Title:            req.Title,
		Description:      req.Description,
		Priority:         Priority(req.Priority),
		AssignedOperator: req.AssignedOperator,
	}, httpx.GetOperatorID(r.Context()))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(incident))
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	var req updateStatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	incident, err := h.svc.UpdateStatus(r.Context(), chi.URLParam(r, "id"), Status(req.Status), httpx.GetOperatorID(r.Context()))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(incident))
}

func (h *Handler) attachRelation(w http.ResponseWriter, r *http.Request) {
	var req attachRelationRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	actor := httpx.GetOperatorID(r.Context())

	var (
		incident Incident
		err      error
	)
	switch strings.ToLower(strings.TrimSpace(req.Kind)) {
	case "alert":
		incident, err = h.svc.AttachAlert(r.Context(), id, req.ID, actor)
	case "track":
		incident, err = h.svc.AttachTrack(r.Context(), id, req.ID, actor)
	case "asset":
		incident, err = h.svc.AttachAsset(r.Context(), id, req.ID, actor)
	case "observation":
		incident, err = h.svc.AttachObservation(r.Context(), id, req.ID, actor)
	case "assessment":
		incident, err = h.svc.AttachAssessment(r.Context(), id, req.ID, actor)
	default:
		err = apperr.Validation("relation kind must be one of alert, track, asset, observation, assessment")
	}
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(incident))
}

func toResponse(incident Incident) incidentResponse {
	return incidentResponse{
		ID:               incident.ID,
		Title:            incident.Title,
		Description:      incident.Description,
		Priority:         string(incident.Priority),
		Status:           string(incident.Status),
		AssignedOperator: incident.AssignedOperator,
		CreatedAt:        incident.CreatedAt,
		UpdatedAt:        incident.UpdatedAt,
		ResolvedAt:       incident.ResolvedAt,
		ClosedAt:         incident.ClosedAt,
	}
}

func toDetailResponse(detail Detail) detailResponse {
	resp := detailResponse{
		incidentResponse: toResponse(detail.Incident),
		Alerts:           make([]relatedAlertResponse, 0, len(detail.Alerts)),
		Tracks:           make([]relatedTrackResponse, 0, len(detail.Tracks)),
		Assets:           make([]relatedAssetResponse, 0, len(detail.Assets)),
		Observations:     make([]relatedObservationResponse, 0, len(detail.Observations)),
		Assessments:      make([]relatedAssessmentResponse, 0, len(detail.Assessments)),
	}
	for _, alert := range detail.Alerts {
		resp.Alerts = append(resp.Alerts, relatedAlertResponse{
			ID:        alert.ID,
			Type:      alert.Type,
			Severity:  alert.Severity,
			State:     alert.State,
			Title:     alert.Title,
			CreatedAt: alert.CreatedAt,
		})
	}
	for _, track := range detail.Tracks {
		resp.Tracks = append(resp.Tracks, relatedTrackResponse{
			ID:          track.ID,
			ExternalRef: track.ExternalRef,
			Status:      track.Status,
			Position:    track.Position,
			LastSeenAt:  track.LastSeenAt,
		})
	}
	for _, asset := range detail.Assets {
		resp.Assets = append(resp.Assets, relatedAssetResponse{
			ID:              asset.ID,
			Name:            asset.Name,
			Type:            asset.Type,
			Status:          asset.Status,
			Position:        asset.Position,
			ConnectionState: asset.ConnectionState,
		})
	}
	for _, observation := range detail.Observations {
		resp.Observations = append(resp.Observations, relatedObservationResponse{
			ID:         observation.ID,
			SourceID:   observation.SourceID,
			Type:       observation.Type,
			ObservedAt: observation.ObservedAt,
			Position:   observation.Position,
		})
	}
	for _, assessment := range detail.Assessments {
		resp.Assessments = append(resp.Assessments, relatedAssessmentResponse{
			ID:          assessment.ID,
			SubjectType: assessment.SubjectType,
			SubjectID:   assessment.SubjectID,
			Type:        assessment.Type,
			Conclusion:  assessment.Conclusion,
			Method:      assessment.Method,
			Confidence:  assessment.Confidence,
			CreatedAt:   assessment.CreatedAt,
		})
	}
	return resp
}
