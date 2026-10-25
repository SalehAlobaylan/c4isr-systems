package simruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func (c *Client) controlRequest(ctx context.Context, method, path string, in, out any) error {
	raw, e := json.Marshal(in)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, method, c.BaseURL+"/api/v1/"+path, bytes.NewReader(raw))
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	res, e := c.HTTP.Do(req)
	if e != nil {
		return fmt.Errorf("worker control transport unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("worker control rejected (HTTP %d)", res.StatusCode)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 65536))
		return nil
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, 1024*1024+1))
	if e != nil || len(data) > 1024*1024 {
		return fmt.Errorf("worker control response exceeds budget")
	}
	return json.Unmarshal(data, out)
}

// Consume advertises this exact executable and attaches through the existing
// atomic run lease. It never selects or executes a browser-supplied program.
func (c *Client) Consume(ctx context.Context, build string) error {
	beat := func() error {
		return c.controlRequest(ctx, http.MethodPost, "platform/worker-heartbeats", map[string]string{"feature": "runtime", "runtimeBuild": build}, nil)
	}
	if e := beat(); e != nil {
		return e
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-child.Done():
				return
			case <-ticker.C:
				_ = beat()
			}
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			var queue struct {
				Items []Run `json:"items"`
			}
			if e := c.controlRequest(ctx, http.MethodGet, "simulation-runtime/queue", nil, &queue); e != nil {
				continue
			}
			for _, run := range queue.Items {
				if run.Configuration.RuntimeBuild == build {
					_ = c.Execute(ctx, run.ID, build)
					break
				}
			}
		}
	}
}
