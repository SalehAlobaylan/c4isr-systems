// Package simgateway provides Go's concurrency, RBAC, SSE streaming, live
// session coordination, and durable PostGIS/S3 persistence layer in front of
// the unified Python sim_core compute worker.
package simgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const workerSecretHeader = "X-EMAD-Worker-Secret"

// WorkerClient communicates with the unified Python sim_core compute hub over
// loopback HTTP with constant-time shared-secret authentication.
type WorkerClient struct {
	BaseURL      string
	WorkerSecret string
	HTTP         *http.Client
}

func NewWorkerClient(baseURL, secret string) *WorkerClient {
	return &WorkerClient{
		BaseURL:      strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		WorkerSecret: secret,
		HTTP: &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        32,
				MaxIdleConnsPerHost: 16,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (c *WorkerClient) Configured() bool {
	return c != nil && c.BaseURL != ""
}

func (c *WorkerClient) Do(ctx context.Context, method, path string, body []byte) (int, string, []byte, error) {
	if !c.Configured() {
		return 0, "", nil, fmt.Errorf("simulation compute worker URL is not configured")
	}
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return 0, "", nil, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.WorkerSecret != "" {
		req.Header.Set(workerSecretHeader, c.WorkerSecret)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()
	// Bound responses at 32 MiB to accommodate 24-30 km GLBs and full multi-policy replays.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024+1))
	if err != nil {
		return 0, "", nil, err
	}
	if len(raw) > 32*1024*1024 {
		return 0, "", nil, fmt.Errorf("simulation worker response exceeded 32 MiB budget")
	}
	return resp.StatusCode, resp.Header.Get("Content-Type"), raw, nil
}

func (c *WorkerClient) DoJSON(ctx context.Context, method, path string, payload any) (int, []byte, error) {
	var body []byte
	if payload != nil {
		switch v := payload.(type) {
		case []byte:
			body = v
		case json.RawMessage:
			body = v
		default:
			var err error
			body, err = json.Marshal(v)
			if err != nil {
				return 0, nil, err
			}
		}
	}
	status, _, raw, err := c.Do(ctx, method, path, body)
	return status, raw, err
}
