package geoenv

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/readiness"
)

// RemoteWorker uses the same durable queue as WorkOne through a scoped API token.
type RemoteWorker struct {
	base, token, toolchain, spool string
	client                        *http.Client
}

func NewRemoteWorker(origin, token, toolchain, spool string) (RemoteWorker, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return RemoteWorker{}, errors.New("worker requires an HTTP or HTTPS API origin")
	}
	if strings.TrimSpace(token) == "" || !digestPattern.MatchString(toolchain) {
		return RemoteWorker{}, errors.New("worker requires a service token and pinned toolchain digest")
	}
	return RemoteWorker{base: strings.TrimRight(origin, "/") + "/api/v1", token: token, toolchain: toolchain, spool: spool,
		client: &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (w RemoteWorker) request(ctx context.Context, method, path, lease string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, w.base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+w.token)
	if lease != "" {
		req.Header.Set(leaseHeader, lease)
	}
	return req, nil
}

func (w RemoteWorker) post(ctx context.Context, path, lease string, body any) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := w.request(ctx, http.MethodPost, path, lease, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return w.client.Do(req)
}

func (w RemoteWorker) control(ctx context.Context, path, lease string, body any, want int) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := w.post(ctx, path, lease, body)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != want {
		return workerHTTPStatus("control request", res.StatusCode)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	return nil
}

func (w RemoteWorker) Beat(ctx context.Context, status readiness.WorkerHeartbeat) error {
	return w.control(ctx, "/platform/worker-heartbeats", "", status, http.StatusOK)
}

func (w RemoteWorker) Reconcile(ctx context.Context) error {
	return w.control(ctx, "/environment-worker/reconcile", "", nil, http.StatusNoContent)
}

func (w RemoteWorker) claim(ctx context.Context) (workerClaim, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	res, err := w.post(ctx, "/environment-worker/claim", "", map[string]string{"toolchainSha256": w.toolchain})
	if err != nil {
		return workerClaim{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNoContent {
		return workerClaim{}, nil
	}
	if res.StatusCode != http.StatusOK {
		return workerClaim{}, workerHTTPStatus("claim", res.StatusCode)
	}
	var claim workerClaim
	if err := json.NewDecoder(io.LimitReader(res.Body, 9<<20)).Decode(&claim); err != nil {
		return workerClaim{}, err
	}
	if !strings.HasPrefix(claim.JobID, "geobuild_") || !keyPattern.MatchString(claim.JobID) ||
		!keyPattern.MatchString(claim.Lease) || claim.LeaseUntil.IsZero() || !digestPattern.MatchString(claim.InputSHA256) ||
		claim.Recipe.Toolchain != w.toolchain || len(claim.Recipe.Inputs) == 0 || len(claim.Recipe.Inputs) > 64 {
		return workerClaim{}, ErrInvalidInputs
	}
	return claim, nil
}

func (w RemoteWorker) WorkOne(ctx context.Context, process Processor) (bool, error) {
	if process == nil {
		return false, errors.New("environment processor unavailable")
	}
	claim, err := w.claim(ctx)
	if err != nil || claim.JobID == "" {
		return false, err
	}
	work, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()
	path := "/environment-worker/builds/" + claim.JobID
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-work.Done():
				return
			case <-ticker.C:
				if w.control(work, path+"/heartbeat", claim.Lease, nil, http.StatusNoContent) != nil {
					cancel()
					return
				}
			}
		}
	}()
	code, err := w.prepare(work, path, claim, process)
	cancel()
	<-done
	if err == nil {
		return true, nil
	}
	reportErr := w.control(context.WithoutCancel(ctx), path+"/fail", claim.Lease, map[string]string{"code": code}, http.StatusNoContent)
	return true, reportErr
}

func (w RemoteWorker) prepare(ctx context.Context, path string, claim workerClaim, process Processor) (string, error) {
	dir, err := os.MkdirTemp(w.spool, attemptDirectoryPrefix)
	if err != nil {
		return "transient_io", err
	}
	defer os.RemoveAll(dir)
	inputDir := filepath.Join(dir, "inputs")
	if err := os.Mkdir(inputDir, 0700); err != nil {
		return "transient_io", err
	}
	for _, input := range claim.Recipe.Inputs {
		id, ok := strings.CutPrefix(input.ID, "geoasset_")
		if !ok || !digestPattern.MatchString(id) || !digestPattern.MatchString(input.SHA256) || input.Bytes < 1 || input.Bytes > artifacts.MaxGeographicBytes {
			return "invalid_input", ErrInvalidInputs
		}
		if err := w.downloadInput(ctx, path+"/inputs/"+input.ID, claim.Lease, filepath.Join(inputDir, input.ID), input); err != nil {
			return "transient_io", err
		}
	}
	raw, err := json.Marshal(struct {
		Recipe
		InputSHA256 string `json:"inputSha256"`
	}{claim.Recipe, claim.InputSHA256})
	if err != nil {
		return "invalid_input", err
	}
	if err := os.WriteFile(filepath.Join(dir, "recipe.json"), raw, 0600); err != nil {
		return "transient_io", err
	}
	if err := process(ctx, dir); err != nil {
		if errors.Is(err, ErrScratchBudget) {
			return "scratch_budget_exceeded", err
		}
		if errors.Is(err, ErrInvalidInputs) {
			return "invalid_input", err
		}
		return "transient_io", err
	}
	return w.complete(ctx, path+"/complete", claim.Lease, filepath.Join(dir, "prepared.zip"))
}

func (w RemoteWorker) downloadInput(ctx context.Context, path, lease, output string, input Asset) error {
	req, err := w.request(ctx, http.MethodGet, path, lease, nil)
	if err != nil {
		return err
	}
	res, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.ContentLength != input.Bytes || res.Header.Get("ETag") != `"`+input.SHA256+`"` {
		return errors.New("remote input identity or length differs from the claimed recipe")
	}
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(res.Body, input.Bytes+1))
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return err
	}
	if n != input.Bytes || hex.EncodeToString(hash.Sum(nil)) != input.SHA256 {
		return errors.New("remote input bytes do not match the claimed recipe")
	}
	return nil
}

func (w RemoteWorker) complete(ctx context.Context, path, lease, output string) (string, error) {
	file, err := os.Open(output)
	if err != nil {
		return "transient_io", err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return "transient_io", err
	}
	if !stat.Mode().IsRegular() || stat.Size() < 1 || stat.Size() > artifacts.MaxGeographicBytes {
		return "invalid_output", ErrInvalidInputs
	}
	hash := sha256.New()
	if n, err := io.Copy(hash, io.LimitReader(file, stat.Size()+1)); err != nil || n != stat.Size() {
		return "transient_io", errors.New("prepared ZIP could not be hashed")
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	for attempt := 0; attempt < 2; attempt++ {
		// A fresh section reader avoids sharing the position with a failed HTTP request.
		req, err := w.request(ctx, http.MethodPost, path, lease, io.NewSectionReader(file, 0, stat.Size()))
		if err != nil {
			return "transient_io", err
		}
		req.ContentLength = stat.Size()
		req.Header.Set("Content-Type", "application/zip")
		req.Header.Set(artifactDigestHeader, digest)
		res, err := w.client.Do(req)
		if err != nil {
			continue
		}
		res.Body.Close()
		switch res.StatusCode {
		case http.StatusNoContent:
			return "", nil
		case http.StatusBadRequest, http.StatusUnprocessableEntity:
			return "invalid_output", workerHTTPStatus("completion", res.StatusCode)
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			// A reverse proxy can lose the response after publication. Reuse
			// the same lease, digest and body to reconcile the exact outcome.
			continue
		default:
			return "transient_io", workerHTTPStatus("completion", res.StatusCode)
		}
	}
	return "transient_io", errors.New("prepared ZIP upload outcome could not be confirmed")
}
