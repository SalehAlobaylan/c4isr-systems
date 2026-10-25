package simscene

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/modelassets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"net/http"
)

type EnvironmentModel struct {
	UploadID  string `json:"uploadId"`
	SHA256    string `json:"sha256"`
	Bytes     int64  `json:"bytes"`
	MediaType string `json:"mediaType"`
}
type EnvironmentView struct {
	Schema      string                       `json:"schema"`
	RunID       string                       `json:"runId"`
	Environment modelassets.EnvironmentAsset `json:"environment"`
	Model       EnvironmentModel             `json:"model"`
	Meaning     string                       `json:"meaning"`
}

// PublicEnvironment copies geographic inputs explicitly approved for operators.
// It contains no scene input, actor/template list, route, sensor or truth state.
func PublicEnvironment(scene Scene, id string) *EnvironmentView {
	env := scene.Frozen.Environment
	if !env.ReleaseToOperators || env.Prepared == nil {
		return nil
	}
	for _, model := range scene.Frozen.Models {
		if model.ID == env.ModelVersionID {
			return &EnvironmentView{Schema: "emad.exercise-environment.v1", RunID: id, Environment: env, Model: EnvironmentModel{model.Upload.ID, model.Upload.SHA256, model.Upload.Bytes, model.Upload.MediaType}, Meaning: "Frozen geographic environment released by its author. Contacts require separately authorized observations; model geometry is not exercise truth."}
		}
	}
	return nil
}
func (s *Service) OperatorEnvironment(ctx context.Context, id string) (EnvironmentView, error) {
	if e := access.Require(ctx, "observations.read"); e != nil {
		return EnvironmentView{}, e
	}
	r, e := access.ResourceFrom(ctx)
	if e != nil {
		return EnvironmentView{}, e
	}
	if r.Partition != access.ExerciseObservation || r.RunID != id {
		return EnvironmentView{}, apperr.Forbidden("explicit exercise observation resource required")
	}
	var out EnvironmentView
	var raw []byte
	e = db.Scoped(s.Pool).QueryRow(ctx, `SELECT document FROM native_environment_views`).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, apperr.NotFound("released environment", "this run has no operator-approved geographic environment")
	}
	if e == nil {
		e = json.Unmarshal(raw, &out)
	}
	return out, e
}
func (s *Service) mountEnvironmentView(r chi.Router) {
	r.Get("/exercise-view/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.OperatorEnvironment(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, 200, v)
	})
	r.Get("/exercise-view/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.OperatorEnvironment(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		resource, _ := access.ResourceFrom(r.Context())
		resource.Partition = access.ExerciseInput
		resource.RunID = ""
		c := access.Internal(r.Context(), resource, "released-environment-read")
		_, raw, e := s.Assets.Content(c, v.Model.UploadID)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		fresh, e := access.Refresh(r.Context())
		if e == nil {
			_, e = s.OperatorEnvironment(fresh, chi.URLParam(r, "id"))
		}
		if e != nil {
			httpx.Error(w, e)
			return
		}
		w.Header().Set("Content-Type", v.Model.MediaType)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(200)
		_, _ = w.Write(raw)
	})
}
