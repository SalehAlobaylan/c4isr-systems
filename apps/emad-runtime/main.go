// emad-runtime is an independent native EMAD executable. The API administers
// runs; this process owns virtual time and private abstract kinematic actors.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "runtime:", e)
		os.Exit(1)
	}
}
func readBounded(path string, max int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, max+1))
	if e != nil || int64(len(b)) > max {
		return nil, fmt.Errorf("file exceeds runtime budget")
	}
	return b, nil
}
func run() error {
	describe := flag.Bool("describe", false, "print runtime contract and executable SHA-256")
	base := flag.String("url", "http://127.0.0.1:8080", "EMAD API origin; HTTPS required off loopback")
	tokenFile := flag.String("token-file", "", "private service credential file")
	id := flag.String("run", "", "one queued run ID to attach")
	consume := flag.Bool("worker", false, "consume the scoped queue, one run at a time")
	offline := flag.String("offline-scene", "", "protected scene JSON for deterministic offline execution")
	elapsed := flag.Int64("elapsed-ms", 1000, "offline virtual instant, at most one hour")
	output := flag.String("out", "", "private offline snapshot output path")
	flag.Parse()
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	binary, e := os.ReadFile(exe)
	if e != nil {
		return e
	}
	build := artifacts.Hash(binary)
	if *describe {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"runtimeVersion": simscene.PerceptionRuntime, "compatibleVersions": []string{simscene.RuntimeVersion, simscene.PerceptionRuntime}, "runtimeBuild": build, "behavior": "abstract ENU waypoint kinematics; seeded uniform-XY synthetic sensing; exercise-only HOLD/RESUME_ROUTE; no validated physical sensor model", "tickHz": 20, "snapshotHz": 2})
	}
	if *offline != "" {
		if *output == "" {
			return fmt.Errorf("offline truth requires an explicit private -out file")
		}
		raw, e := readBounded(*offline, simscene.MaxBytes+1024)
		if e != nil {
			return e
		}
		var scene simscene.Scene
		if e = json.Unmarshal(raw, &scene); e != nil {
			return e
		}
		checked, _, e := simscene.Freeze(scene.Frozen)
		if e != nil || checked.ID != scene.ID {
			return fmt.Errorf("offline scene integrity failure")
		}
		world, e := simruntime.NewWorld(scene.Frozen)
		if e != nil {
			return e
		}
		actors, e := world.At(*elapsed)
		if e != nil {
			return e
		}
		data, e := json.MarshalIndent(simruntime.Snapshot{RunID: "offline", SceneID: scene.ID, RuntimeBuild: build, Sequence: 1, ElapsedMs: *elapsed, Actors: actors}, "", "  ")
		if e != nil {
			return e
		}
		return os.WriteFile(*output, data, 0600)
	}
	if (!*consume && !regexp.MustCompile(`^rt_[a-f0-9]{64}$`).MatchString(*id)) || *tokenFile == "" {
		return fmt.Errorf("-run and -token-file required; use -describe before starting a pinned run")
	}
	raw, e := readBounded(*tokenFile, 4096)
	if e != nil {
		return fmt.Errorf("cannot read private credential file")
	}
	client, e := simruntime.NewClient(*base, strings.TrimSpace(string(raw)))
	if e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *consume {
		return client.Consume(ctx, build)
	}
	return client.Execute(ctx, *id, build)
}
