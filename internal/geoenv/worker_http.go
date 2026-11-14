package geoenv

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
)

const leaseHeader = "X-Environment-Lease"
const artifactDigestHeader = "X-Artifact-SHA256"

func leaseProof(worker, lease string) string {
	return artifacts.Hash([]byte(worker + "\x00" + lease))
}

type workerClaim struct {
	JobID       string    `json:"jobId"`
	Lease       string    `json:"lease"`
	LeaseUntil  time.Time `json:"leaseUntil"`
	InputSHA256 string    `json:"inputSha256"`
	Recipe      Recipe    `json:"recipe"`
}

func (s Service) mountWorker(r chi.Router) {
	r.Route("/environment-worker", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := require(r.Context(), Execute, true); err != nil {
					httpx.Error(w, err)
					return
				}
				if s.ReadOnly || s.Store == nil {
					httpx.Error(w, apperr.Conflict("environment execution is unavailable"))
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		r.Post("/claim", s.claimWorkerHTTP)
		r.Post("/reconcile", func(w http.ResponseWriter, r *http.Request) {
			if err := s.ReconcileOne(r.Context()); err != nil {
				httpx.Error(w, err)
				return
			}
			if err := CleanSpool(s.workerUploadRoot(), time.Now()); err != nil && !errors.Is(err, os.ErrNotExist) {
				httpx.Error(w, apperr.Internal(err))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		r.Get("/builds/{id}/inputs/{asset}", s.workerInput)
		r.Post("/builds/{id}/heartbeat", s.workerHeartbeat)
		r.Post("/builds/{id}/fail", s.workerFail)
		r.Post("/builds/{id}/complete", s.workerComplete)
	})
}

func (s Service) workerUploadRoot() string {
	if s.Spool != "" {
		return filepath.Join(s.Spool, "http-uploads")
	}
	return filepath.Join(os.TempDir(), "emad-environment-http-uploads")
}

func (s Service) claimWorkerHTTP(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Toolchain string `json:"toolchainSha256"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.Error(w, err)
		return
	}
	if !digestPattern.MatchString(in.Toolchain) || in.Toolchain != s.Toolchain && in.Toolchain != s.EnrichmentToolchain && in.Toolchain != s.ObservationToolchain && in.Toolchain != s.CoastalToolchain {
		httpx.Error(w, apperr.Conflict("worker toolchain differs from the configured processor"))
		return
	}
	ctx := r.Context()
	s.Toolchain = in.Toolchain
	j, err := s.claim(ctx, "", "")
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if j.ID == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	human, err := s.actor(ctx, j)
	if err == nil {
		err = s.checkInputs(human, j.recipe, false)
	}
	if err == nil {
		err = s.checkInputs(ctx, j.recipe, false)
	}
	if err != nil {
		if failErr := s.fail(ctx, j, "input_policy_unavailable", false); failErr != nil {
			httpx.Error(w, failErr)
			return
		}
		httpx.Error(w, apperr.Conflict("claimed inputs are no longer available"))
		return
	}
	httpx.JSON(w, http.StatusOK, workerClaim{j.ID, j.lease, j.leaseUntil, j.InputSHA256, j.recipe})
}

func (s Service) leasedJob(r *http.Request) (Job, error) {
	ctx := r.Context()
	lease := r.Header.Get(leaseHeader)
	if !keyPattern.MatchString(lease) || !keyPattern.MatchString(chi.URLParam(r, "id")) {
		return Job{}, apperr.Validation("exact build and lease identities are required")
	}
	p, _, _ := access.From(ctx)
	j, err := scanJob(db.Scoped(s.Pool).QueryRow(ctx, jobSelect+`WHERE id=$1 AND state='RUNNING' AND lease_id=$2 AND worker_id=$3 AND lease_until>clock_timestamp() AND NOT recipe ? 'display'`, chi.URLParam(r, "id"), lease, p.ID))
	if apperr.Is(err, apperr.CodeNotFound) {
		err = apperr.Conflict("build lease is no longer active")
	}
	return j, err
}

func (s Service) workerHeartbeat(w http.ResponseWriter, r *http.Request) {
	j, err := s.leasedJob(r)
	if err == nil {
		err = s.checkInputs(r.Context(), j.recipe, false)
	}
	if err == nil {
		err = s.keepAlive(r.Context(), r.Context(), j)
	}
	if err != nil {
		httpx.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s Service) workerInput(w http.ResponseWriter, r *http.Request) {
	j, err := s.leasedJob(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	for _, expected := range j.recipe.Inputs {
		if expected.ID != chi.URLParam(r, "asset") {
			continue
		}
		a, err := s.asset(r.Context(), expected.ID)
		if err == nil && (a.State != "READY" || a.SHA256 != expected.SHA256 || a.AdmissionID != expected.AdmissionID) {
			err = apperr.Conflict("frozen input bytes are unavailable")
		}
		if err == nil {
			_, err = s.policy(r.Context(), a, true, false)
		}
		if err != nil {
			httpx.Error(w, err)
			return
		}
		s.stream(w, r, a, a.ID)
		return
	}
	httpx.Error(w, apperr.NotFound("build input", "unavailable"))
}

func (s Service) workerFail(w http.ResponseWriter, r *http.Request) {
	j, err := s.leasedJob(r)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.Error(w, err)
		return
	}
	if in.Code != "transient_io" && in.Code != "invalid_input" && in.Code != "invalid_output" && in.Code != "scratch_budget_exceeded" {
		httpx.Error(w, apperr.Validation("unsupported worker failure code"))
		return
	}
	if err := s.fail(r.Context(), j, in.Code, in.Code == "transient_io"); err != nil {
		httpx.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s Service) workerComplete(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), attemptTimeout)
	defer cancel()
	digest := r.Header.Get(artifactDigestHeader)
	if !digestPattern.MatchString(digest) || r.ContentLength < 1 || r.ContentLength > artifacts.MaxGeographicBytes {
		httpx.Error(w, apperr.Validation("prepared ZIP digest and bounded content length are required"))
		return
	}
	j, err := s.leasedJob(r)
	if apperr.Is(err, apperr.CodeConflict) && s.completedByWorker(r, digest) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if s.UploadSlots != nil {
		select {
		case s.UploadSlots <- struct{}{}:
			defer func() { <-s.UploadSlots }()
		default:
			httpx.Error(w, apperr.Conflict("geographic uploads are at capacity"))
			return
		}
	}
	root := s.workerUploadRoot()
	if err := os.MkdirAll(root, 0700); err != nil {
		httpx.Error(w, apperr.Internal(err))
		return
	}
	dir, err := os.MkdirTemp(root, attemptDirectoryPrefix)
	if err != nil {
		httpx.Error(w, apperr.Internal(err))
		return
	}
	defer os.RemoveAll(dir)
	f, err := os.OpenFile(filepath.Join(dir, "prepared.zip"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		httpx.Error(w, apperr.Internal(err))
		return
	}
	defer f.Close()
	controller := http.NewResponseController(w)
	if deadline, ok := ctx.Deadline(); ok && controller.SetReadDeadline(deadline) == nil {
		defer controller.SetReadDeadline(time.Time{})
	}
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, hash), http.MaxBytesReader(w, r.Body, r.ContentLength))
	if err != nil || n != r.ContentLength || hex.EncodeToString(hash.Sum(nil)) != digest {
		httpx.Error(w, apperr.Validation("prepared ZIP did not match its declared length and SHA-256"))
		return
	}
	manifest, err := manifestFromArchive(f, n)
	if err != nil {
		httpx.Error(w, apperr.Validation("prepared ZIP has no valid manifest"))
		return
	}
	if err := s.stageArchive(ctx, j, f, manifest); err != nil {
		if errors.Is(err, ErrInvalidInputs) {
			err = apperr.Validation("prepared ZIP does not match the build recipe")
		} else if errors.Is(err, errPublicationPolicy) {
			err = apperr.Conflict("current source policy prevents publication")
		}
		httpx.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func manifestFromArchive(f *os.File, size int64) ([]byte, error) {
	z, err := zip.NewReader(f, size)
	if err != nil || len(z.File) > 1024 {
		return nil, ErrInvalidInputs
	}
	for _, entry := range z.File {
		if entry.Name != "manifest.json" {
			continue
		}
		if !entry.Mode().IsRegular() || entry.UncompressedSize64 > 8<<20 {
			return nil, ErrInvalidInputs
		}
		reader, err := entry.Open()
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		raw, err := io.ReadAll(io.LimitReader(reader, (8<<20)+1))
		if err != nil || uint64(len(raw)) != entry.UncompressedSize64 {
			return nil, ErrInvalidInputs
		}
		return raw, nil
	}
	return nil, ErrInvalidInputs
}

// A lost success response can be retried by the same service for the exact ZIP.
// This acknowledges existing publication and cannot create a new one.
func (s Service) completedByWorker(r *http.Request, digest string) bool {
	ctx := r.Context()
	p, _, _ := access.From(ctx)
	var matches bool
	err := db.Scoped(s.Pool).QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM geographic_environment_releases r
		JOIN geographic_assets a ON (a.id,a.scope_id)=(r.bundle_id,r.scope_id)
		WHERE r.build_id=$1 AND a.state='READY'
		AND a.retain_until>clock_timestamp() AND a.document->>'sha256'=$2 AND a.size_bytes=$3
		AND r.publication_lease_sha256=$4
	)`, chi.URLParam(r, "id"), digest, r.ContentLength, leaseProof(p.ID, r.Header.Get(leaseHeader))).Scan(&matches)
	return err == nil && matches
}

func workerHTTPStatus(operation string, code int) error {
	return fmt.Errorf("environment worker %s: HTTP %d", operation, code)
}
