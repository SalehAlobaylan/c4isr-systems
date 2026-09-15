package assessments

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

// Handler exposes assessments over HTTP. Handlers only translate transport
// input into application calls.
type Handler struct {
	svc *Service
}

// NewHandler builds an HTTP transport for the assessment service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Mount registers assessment routes on the router.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/assessments", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{id}", h.get)
	})
}

type createRequest struct {
	SubjectType string         `json:"subjectType"`
	SubjectID   string         `json:"subjectId"`
	Type        string         `json:"type"`
	Conclusion  string         `json:"conclusion"`
	Confidence  *float64       `json:"confidence"`
	Method      string         `json:"method"`
	CreatedBy   string         `json:"createdBy"`
	Evidence    []evidenceBody `json:"evidence"`
}

type evidenceBody struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type evidenceResponse struct {
	Type    string    `json:"type"`
	ID      string    `json:"id"`
	AddedAt time.Time `json:"addedAt"`
}

type assessmentResponse struct {
	ID          string             `json:"id"`
	SubjectType string             `json:"subjectType"`
	SubjectID   string             `json:"subjectId"`
	Type        string             `json:"type"`
	Conclusion  string             `json:"conclusion"`
	Confidence  *float64           `json:"confidence"`
	Method      string             `json:"method"`
	CreatedBy   string             `json:"createdBy"`
	CreatedAt   time.Time          `json:"createdAt"`
	Evidence    []evidenceResponse `json:"evidence"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}

	evidence := make([]Evidence, 0, len(req.Evidence))
	for _, e := range req.Evidence {
		evidence = append(evidence, Evidence{Type: EvidenceType(e.Type), ID: e.ID})
	}

	assessment, err := h.svc.Create(r.Context(), CreateInput{
		SubjectType: SubjectType(req.SubjectType),
		SubjectID:   req.SubjectID,
		Type:        req.Type,
		Conclusion:  req.Conclusion,
		Confidence:  req.Confidence,
		Method:      Method(req.Method),
		CreatedBy:   req.CreatedBy,
		Evidence:    evidence,
	})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toResponse(assessment))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	assessment, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(assessment))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.svc.List(r.Context(), r.URL.Query().Get("subject_type"), r.URL.Query().Get("subject_id"), limit, offset)
	if err != nil {
		httpx.Error(w, err)
		return
	}

	resp := httpx.ListResponse[assessmentResponse]{
		Items: make([]assessmentResponse, 0, len(items)),
		Total: total,
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toResponse(item))
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func toResponse(a Assessment) assessmentResponse {
	evidence := make([]evidenceResponse, 0, len(a.Evidence))
	for _, e := range a.Evidence {
		evidence = append(evidence, evidenceResponse{
			Type:    string(e.Type),
			ID:      e.ID,
			AddedAt: e.AddedAt,
		})
	}
	return assessmentResponse{
		ID:          a.ID,
		SubjectType: string(a.SubjectType),
		SubjectID:   a.SubjectID,
		Type:        a.Type,
		Conclusion:  a.Conclusion,
		Confidence:  a.Confidence,
		Method:      string(a.Method),
		CreatedBy:   a.CreatedBy,
		CreatedAt:   a.CreatedAt,
		Evidence:    evidence,
	}
}
