package simruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
)

type Client struct {
	BaseURL, Token string
	HTTP           *http.Client
}

func NewClient(base, token string) (*Client, error) {
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("runtime requires an origin URL")
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "http" || u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return nil, fmt.Errorf("remote runtime transport requires HTTPS")
		}
	}
	return &Client{BaseURL: strings.TrimSuffix(base, "/"), Token: token, HTTP: &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) request(ctx context.Context, path string, in, out any) error {
	raw, e := json.Marshal(in)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/api/v1/simulation-runtime/runs/"+path, bytes.NewReader(raw))
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	res, e := c.HTTP.Do(req)
	if e != nil {
		return fmt.Errorf("runtime transport unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("runtime request rejected (HTTP %d)", res.StatusCode)
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, simscene.MaxBytes+65537))
	if e != nil || len(data) > simscene.MaxBytes+65536 {
		return fmt.Errorf("runtime response exceeds contract")
	}
	return json.Unmarshal(data, out)
}
func (c *Client) Attach(ctx context.Context, id, build string) (Lease, error) {
	var v Lease
	e := c.request(ctx, id+"/attach", Attach{simscene.PerceptionRuntime, build}, &v)
	return v, e
}
func (c *Client) Heartbeat(ctx context.Context, id string, in Beat) (Run, error) {
	var v Run
	e := c.request(ctx, id+"/heartbeat", in, &v)
	return v, e
}

// Execute owns the private world and clock in the separate worker executable.
// Network uncertainty stops advancement; retries reuse the identical sequence.
// A lost/expired lease ends this process; there is no silent takeover or restart.
func (c *Client) Execute(ctx context.Context, id, build string) error {
	lease, e := c.Attach(ctx, id, build)
	if e != nil {
		var old Lease
		if oldError := c.request(ctx, id+"/attach", Attach{simscene.RuntimeVersion, build}, &old); oldError != nil {
			return e
		}
		lease = old
	}

	if lease.Scene.ID != lease.Run.Configuration.SceneID || !simscene.CompatibleRuntime(lease.Run.Configuration.RuntimeVersion) || lease.Run.Configuration.RuntimeBuild != build {
		return fmt.Errorf("runtime attach identity mismatch")
	}
	scene, _, e := simscene.Freeze(lease.Scene.Frozen)
	if e != nil || scene.ID != lease.Scene.ID {
		return fmt.Errorf("runtime frozen scene verification failed")
	}
	world, e := NewWorld(scene.Frozen)
	if e != nil {
		return e
	}
	defer func() { world.paths = nil }()
	sampler := NewSampler(scene.Frozen, world, lease.Run.Configuration.Seed, lease.Token, lease.Bindings)
	tasks := NewTaskRunner(world, lease.Bindings)
	pendingOutcomes := tasks.Advance(lease.Run.Tasks, 0)
	elapsed := int64(0)
	fraction := float64(0)
	seq := 0
	desired := lease.Run.Desired
	last := time.Now()
	began := last
	lastPublished := last.Add(-time.Second)
	ticks := time.NewTicker(50 * time.Millisecond)
	defer ticks.Stop()
	for {
		now := time.Now()
		delta := now.Sub(last)
		last = now
		interrupted := ctx.Err() != nil
		if desired == "RUNNING" && !interrupted {
			fraction += delta.Seconds() * 1000 * lease.Run.Configuration.Scale
			add := int64(fraction)
			fraction -= float64(add)
			elapsed += add
			if elapsed > lease.Run.Configuration.DurationMs {
				elapsed = lease.Run.Configuration.DurationMs
			}
		}
		status, code := "RUNNING", ""
		switch desired {
		case "PAUSED":
			status = "PAUSED"
		case "STOPPED":
			status = "STOPPED"
		}
		if elapsed == lease.Run.Configuration.DurationMs && desired != "STOPPED" {
			status = "COMPLETED"
		}
		if interrupted || now.Sub(began) > time.Hour {
			status, code = "FAILED", "INTERRUPTED"
		}
		terminal := status == "STOPPED" || status == "COMPLETED" || status == "FAILED"
		if desired == "RUNNING" && !interrupted {
			pendingOutcomes = append(pendingOutcomes, tasks.Advance(nil, elapsed)...)
		}
		if terminal || now.Sub(lastPublished) >= 500*time.Millisecond {
			outcomes := pendingOutcomes
			if len(outcomes) > 64 {
				outcomes = outcomes[:64]
			}
			readings, e := sampler.Advance(elapsed)
			if e != nil {
				return e
			}
			actors, e := world.At(elapsed)
			if e != nil {
				return e
			}
			seq++
			beat := Beat{Token: lease.Token, Status: status, FailureCode: code, Snapshot: Snapshot{RunID: id, SceneID: scene.ID, RuntimeBuild: build, Sequence: seq, ElapsedMs: elapsed, Actors: actors}, Batch: simulation.PerceptionBatch{Readings: readings, Outcomes: outcomes}}
			work, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			var result Run
			var firstError error
			for {
				result, e = c.Heartbeat(work, id, beat)
				if e != nil && firstError == nil {
					firstError = e
				}
				if e == nil || work.Err() != nil {
					break
				}
				select {
				case <-work.Done():
				case <-time.After(150 * time.Millisecond):
				}
			}
			cancel()
			if e != nil {
				return fmt.Errorf("runtime heartbeat failed; world discarded: %w", firstError)
			}
			desired = result.Desired
			pendingOutcomes = pendingOutcomes[len(outcomes):]
			// Register task receipt at this virtual instant; execution follows the
			// 20 Hz loop rather than waiting for the next publication window.
			if (result.Status == "RUNNING" || result.Status == "PAUSED") && result.Desired != "STOPPED" {
				pendingOutcomes = append(pendingOutcomes, tasks.Sync(result.Tasks, elapsed)...)
			}
			lastPublished = time.Now()
			// Time spent awaiting network acceptance is deliberately not simulated.
			last = lastPublished
			if terminal {
				return nil
			}
		}
		select {
		case <-ctx.Done():
		case <-ticks.C:
		}
	}
}
