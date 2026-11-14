// emad-environment processes scoped geographic preparation jobs.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/geoenv"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/readiness"
	"github.com/shirou/gopsutil/v4/process"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "environment worker:", err)
		os.Exit(1)
	}
}

func run() error {
	credential := flag.String("token-file", "", "private source-scoped environment service token")
	apiOrigin := flag.String("api-origin", "", "use the scoped worker HTTP API at this origin")
	displayOnly := flag.Bool("display-only", false, "claim only geographic display jobs through direct transport")
	python := flag.String("python", "python", "Python interpreter with the pinned geographic toolchain")
	script := flag.String("processor", "scripts/data/environment_library.py", "pinned preparation script")
	once := flag.Bool("once", false, "process at most one queued build")
	flag.Parse()
	if *displayOnly && *apiOrigin != "" {
		return errors.New("display-only mode requires direct transport")
	}
	var cfg config.Config
	if *apiOrigin == "" {
		loaded, err := config.Load()
		if err != nil {
			return err
		}
		cfg = loaded
		if !cfg.EnvironmentLibraryEnabled {
			return errors.New("environment library is disabled")
		}
	} else {
		cfg.EnvironmentToolchain = strings.TrimSpace(os.Getenv("C4ISR_ENVIRONMENT_TOOLCHAIN"))
		cfg.EnvironmentSpool = strings.TrimSpace(os.Getenv("C4ISR_ENVIRONMENT_SPOOL"))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	interpreter, err := exec.LookPath(*python)
	if err != nil {
		return err
	}
	interpreter, err = filepath.Abs(interpreter)
	if err != nil {
		return err
	}
	processor, err := filepath.Abs(*script)
	if err != nil {
		return err
	}
	probe, cancel := context.WithTimeout(ctx, 20*time.Second)
	command := exec.CommandContext(probe, interpreter, "-I", processor, "toolchain")
	command.Env = processorEnv("")
	digest, err := command.Output()
	cancel()
	if err != nil || strings.TrimSpace(string(digest)) != cfg.EnvironmentToolchain {
		return errors.New("installed processor does not match configured toolchain digest")
	}
	token, err := os.ReadFile(*credential)
	if err != nil || len(token) == 0 || len(token) > 4096 {
		return errors.New("cannot read bounded worker credential file")
	}
	if cfg.EnvironmentSpool == "" {
		cfg.EnvironmentSpool = filepath.Join(os.TempDir(), "emad-environment-attempts")
	}
	cfg.EnvironmentSpool, err = filepath.Abs(cfg.EnvironmentSpool)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.EnvironmentSpool, 0700); err != nil {
		return err
	}
	prepare := func(ctx context.Context, dir string) error { return processAttempt(ctx, interpreter, processor, dir) }
	workerStatus := func(ctx context.Context) readiness.WorkerHeartbeat {
		status := readiness.WorkerHeartbeat{Feature: "environment-library", ToolchainSHA256: &cfg.EnvironmentToolchain}
		if *displayOnly {
			status.ToolchainSHA256 = &cfg.EnvironmentDisplayToolchain
			if cfg.EnvironmentDisplayToolchain == "" {
				status.ToolchainSHA256 = &cfg.EnvironmentLayerToolchain
			}
		}
		if size, err := scratchBytes(ctx, cfg.EnvironmentSpool, 1<<40); err == nil {
			status.ScratchBytes = &size
		}
		return status
	}
	if *apiOrigin != "" {
		worker, err := geoenv.NewRemoteWorker(*apiOrigin, strings.TrimSpace(string(token)), cfg.EnvironmentToolchain, cfg.EnvironmentSpool)
		if err != nil {
			return err
		}
		if !*once {
			go heartbeat(ctx, func(ctx context.Context) error { return worker.Beat(ctx, workerStatus(ctx)) })
		}
		return workerLoop(ctx, cfg.EnvironmentSpool, *once, func() (bool, error) { return worker.WorkOne(ctx, prepare) }, worker.Reconcile)
	}
	pool, err := platformdb.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	store, err := artifacts.NewS3(artifacts.S3Config{Endpoint: cfg.ArtifactS3Endpoint, Bucket: cfg.ArtifactS3Bucket, Region: cfg.ArtifactS3Region, AccessKey: cfg.ArtifactS3AccessKey, SecretKey: cfg.ArtifactS3SecretKey, AllowHTTP: cfg.ArtifactS3AllowHTTP || cfg.Environment == config.EnvironmentDevelopment || cfg.Environment == config.EnvironmentTest})
	if err != nil {
		return err
	}
	identities := access.NewStore(pool)
	session, err := identities.Authenticate(ctx, strings.TrimSpace(string(token)), func(string) []string { return nil })
	if err != nil {
		return err
	}
	bound, err := identities.BindSession(ctx, session, session.Resource)
	if err != nil {
		return err
	}
	svc := geoenv.Service{Pool: pool, Store: store, Toolchain: cfg.EnvironmentToolchain, Parallel: cfg.EnvironmentParallel, Spool: cfg.EnvironmentSpool, DisplayOnly: *displayOnly}
	if *displayOnly && cfg.EnvironmentDisplayToolchain == "" && cfg.EnvironmentLayerToolchain == "" {
		return errors.New("display-only mode requires a configured display processor digest")
	}
	if cfg.EnvironmentLayerToolchain != "" {
		layerScript := filepath.Join(filepath.Dir(processor), "prepare_geographic_display.py")
		args := []string{"-I", "-c", trustedDisplayRunner, layerScript}
		probe, cancel := context.WithTimeout(ctx, 20*time.Second)
		command := exec.CommandContext(probe, interpreter, append(append([]string{}, args...), "--toolchain")...)
		command.Env = processorEnv("")
		digest, err := command.Output()
		cancel()
		if err != nil || strings.TrimSpace(string(digest)) != cfg.EnvironmentLayerToolchain {
			return errors.New("installed geographic layer processor does not match its configured digest")
		}
		svc.LayerToolchain = cfg.EnvironmentLayerToolchain
		svc.LayerProcessor = func(ctx context.Context, dir string, recipe geoenv.DisplayBuildRecipe) error {
			command := append(append([]string{}, args...), "prepared.zip", recipe.Recipe, "display.bin", "display-request.json", "--manifest-sha256", recipe.ManifestSHA256)
			if recipe.SceneMetadata != "" {
				if err := os.WriteFile(filepath.Join(dir, "scene.json"), []byte(recipe.SceneMetadata), 0600); err != nil {
					return err
				}
				command = append(command, "--scene-metadata", "scene.json")
			}
			return processCommand(ctx, interpreter, dir, command...)
		}
	}
	if cfg.EnvironmentDisplayToolchain != "" {
		displayScript := filepath.Join(filepath.Dir(processor), "prepare_display_terrain.py")
		args := []string{"-I", "-c", trustedDisplayRunner, displayScript}
		probe, cancel := context.WithTimeout(ctx, 20*time.Second)
		command := exec.CommandContext(probe, interpreter, append(append([]string{}, args...), "--toolchain")...)
		command.Env = processorEnv("")
		digest, err := command.Output()
		cancel()
		if err != nil || strings.TrimSpace(string(digest)) != cfg.EnvironmentDisplayToolchain {
			return errors.New("installed terrain processor does not match configured display toolchain digest")
		}
		svc.DisplayToolchain = cfg.EnvironmentDisplayToolchain
		svc.DisplayProcessor = func(ctx context.Context, dir, manifest string) error {
			args := append(append([]string{}, args...), "prepared.zip", "terrain.glb", "display-request.json", "--manifest-sha256", manifest)
			return processCommand(ctx, interpreter, dir, args...)
		}
	}
	if !*once {
		go heartbeat(bound, func(ctx context.Context) error {
			fresh, err := access.Refresh(ctx)
			if err != nil {
				return err
			}
			return (readiness.Service{Pool: pool, EnvironmentLibrary: true}).Beat(fresh, workerStatus(ctx))
		})
	}
	return workerLoop(bound, cfg.EnvironmentSpool, *once, func() (bool, error) { return svc.WorkOne(bound, prepare) }, svc.ReconcileOne)
}

func workerLoop(ctx context.Context, spool string, once bool, work func() (bool, error), reconcile func(context.Context) error) error {
	var nextCleanup time.Time
	for ctx.Err() == nil {
		if now := time.Now(); now.After(nextCleanup) {
			if err := geoenv.CleanSpool(spool, now); err != nil {
				fmt.Fprintln(os.Stderr, "environment scratch cleanup requires inspection")
			}
			nextCleanup = now.Add(time.Minute)
		}
		did, err := work()
		if once {
			return err
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "environment queue unavailable; inspect worker readiness and build events")
		}
		if !did || err != nil {
			reconcileCtx, stop := context.WithTimeout(ctx, 10*time.Second)
			if err := reconcile(reconcileCtx); err != nil {
				fmt.Fprintln(os.Stderr, "environment retention cleanup requires inspection")
			}
			stop()
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
		}
	}
	return nil
}

func heartbeat(ctx context.Context, send func(context.Context) error) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		beat, cancel := context.WithTimeout(ctx, 2*time.Second)
		_ = send(beat)
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func processorEnv(dir string) []string {
	env := []string{"PROJ_NETWORK=OFF", "GDAL_PAM_ENABLED=NO"}
	// Child requires no API, database, object-store or provider credentials.
	for _, key := range []string{"SystemRoot", "WINDIR", "PATH", "LD_LIBRARY_PATH"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	if dir != "" {
		for _, key := range []string{"TMPDIR", "TEMP", "TMP"} {
			env = append(env, key+"="+dir)
		}
	}
	return env
}

// Python isolated mode excludes the trusted sibling modules used by the terrain script.
const trustedDisplayRunner = "import os,runpy,sys; sys.argv.pop(0); sys.path.insert(0,os.path.dirname(sys.argv[0])); runpy.run_path(sys.argv[0],run_name='__main__')"

func processAttempt(ctx context.Context, python, script, dir string) error {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	return processCommand(ctx, python, absolute, "-I", script, "prepare", absolute)
}

func processCommand(ctx context.Context, python, dir string, args ...string) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, python, args...)
	command.Dir = dir
	command.Env = processorEnv(dir)
	if err := command.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 2 {
				return geoenv.ErrInvalidInputs
			}
			if err != nil {
				return err
			}
			_, err = scratchBytes(ctx, dir, 8<<30)
			return err
		case <-ticker.C:
			child, err := process.NewProcess(int32(command.Process.Pid))
			if err == nil {
				if mem, err := child.MemoryInfo(); err == nil && mem.RSS > 2<<30 {
					_ = command.Process.Kill()
					<-done
					return geoenv.ErrInvalidInputs
				}
			}
			if _, err := scratchBytes(ctx, dir, 8<<30); err != nil {
				_ = command.Process.Kill()
				<-done
				return err
			}
		}
	}
}

func scratchBytes(ctx context.Context, dir string, limit int64) (int64, error) {
	var size int64
	err := filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		size += info.Size()
		if size > limit {
			return geoenv.ErrScratchBudget
		}
		return nil
	})
	return size, err
}
