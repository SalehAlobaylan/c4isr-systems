package missions

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes missions over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the mission service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers mission routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/missions", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{id}", h.get)
		r.Post("/{id}/status", h.updateStatus)
		r.Post("/{id}/assets", h.assignAsset)
		r.Post("/{id}/tasks", h.addTask)
		r.Post("/{id}/tasks/{taskId}/status", h.updateTaskStatus)
	})
}

type taskRequest struct {
	Type        string     `json:"type"`
	Description string     `json:"description"`
	Target      *geo.Point `json:"target"`
}

type createRequest struct {
	Name       string        `json:"name"`
	Objective  string        `json:"objective"`
	Priority   string        `json:"priority"`
	IncidentID string        `json:"incidentId"`
	Assets     []string      `json:"assets"`
	Tasks      []taskRequest `json:"tasks"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

type assignAssetRequest struct {
	AssetID string `json:"assetId"`
}

type updateTaskStatusRequest struct {
	Status string `json:"status"`
}

type relatedAssetResponse struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	Position        *geo.Point `json:"position,omitempty"`
	ConnectionState string     `json:"connectionState,omitempty"`
}

type taskResponse struct {
	ID          string     `json:"id"`
	MissionID   string     `json:"missionId"`
	Type        string     `json:"type"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Target      *geo.Point `json:"target,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type missionResponse struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Objective  string                 `json:"objective"`
	Priority   string                 `json:"priority"`
	Status     string                 `json:"status"`
	IncidentID string                 `json:"incidentId,omitempty"`
	CreatedAt  time.Time              `json:"createdAt"`
	UpdatedAt  time.Time              `json:"updatedAt"`
	StartedAt  *time.Time             `json:"startedAt,omitempty"`
	EndedAt    *time.Time             `json:"endedAt,omitempty"`
	Assets     []relatedAssetResponse `json:"assets"`
	Tasks      []taskResponse         `json:"tasks"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	tasks := make([]TaskInput, 0, len(req.Tasks))
	for _, task := range req.Tasks {
		tasks = append(tasks, TaskInput{
			Type:        task.Type,
			Description: task.Description,
			Target:      task.Target,
		})
	}
	mission, err := h.svc.Create(r.Context(), CreateInput{
		Name:       req.Name,
		Objective:  req.Objective,
		Priority:   Priority(req.Priority),
		IncidentID: req.IncidentID,
		Assets:     req.Assets,
		Tasks:      tasks,
		Actor:      httpx.GetOperatorID(r.Context()),
	})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toResponse(mission))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	mission, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(mission))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.svc.List(r.Context(), r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	resp := httpx.ListResponse[missionResponse]{
		Items: make([]missionResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	var req updateStatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	mission, err := h.svc.UpdateStatus(r.Context(), chi.URLParam(r, "id"), Status(req.Status), httpx.GetOperatorID(r.Context()))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(mission))
}

func (h *Handler) assignAsset(w http.ResponseWriter, r *http.Request) {
	var req assignAssetRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	mission, err := h.svc.AssignAsset(r.Context(), chi.URLParam(r, "id"), req.AssetID, httpx.GetOperatorID(r.Context()))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(mission))
}

func (h *Handler) addTask(w http.ResponseWriter, r *http.Request) {
	var req taskRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	task, err := h.svc.AddTask(r.Context(), chi.URLParam(r, "id"), TaskInput{
		Type:        req.Type,
		Description: req.Description,
		Target:      req.Target,
	}, httpx.GetOperatorID(r.Context()))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toTaskResponse(task))
}

func (h *Handler) updateTaskStatus(w http.ResponseWriter, r *http.Request) {
	var req updateTaskStatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	task, err := h.svc.UpdateTaskStatus(r.Context(),
		chi.URLParam(r, "id"),
		chi.URLParam(r, "taskId"),
		TaskStatus(req.Status),
		httpx.GetOperatorID(r.Context()),
	)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toTaskResponse(task))
}

func toResponse(mission Mission) missionResponse {
	resp := missionResponse{
		ID:         mission.ID,
		Name:       mission.Name,
		Objective:  mission.Objective,
		Priority:   string(mission.Priority),
		Status:     string(mission.Status),
		IncidentID: mission.IncidentID,
		CreatedAt:  mission.CreatedAt,
		UpdatedAt:  mission.UpdatedAt,
		StartedAt:  mission.StartedAt,
		EndedAt:    mission.EndedAt,
		Assets:     make([]relatedAssetResponse, 0, len(mission.Assets)),
		Tasks:      make([]taskResponse, 0, len(mission.Tasks)),
	}
	for _, asset := range mission.Assets {
		resp.Assets = append(resp.Assets, relatedAssetResponse{
			ID:              asset.ID,
			Name:            asset.Name,
			Type:            asset.Type,
			Status:          asset.Status,
			Position:        asset.Position,
			ConnectionState: asset.ConnectionState,
		})
	}
	for _, task := range mission.Tasks {
		resp.Tasks = append(resp.Tasks, toTaskResponse(task))
	}
	return resp
}

func toTaskResponse(task Task) taskResponse {
	return taskResponse{
		ID:          task.ID,
		MissionID:   task.MissionID,
		Type:        task.Type,
		Description: task.Description,
		Status:      string(task.Status),
		Target:      task.Target,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
	}
}
