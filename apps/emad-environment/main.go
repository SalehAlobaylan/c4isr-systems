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
	python := flag.String("python", "python", "Python interpreter with the pinned geographic toolchain")
	script := flag.String("processor", "scripts/data/environment_library.py", "pinned preparation script")
	once := flag.Bool("once", false, "process at most one queued build")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !cfg.EnvironmentLibraryEnabled {
		return errors.New("environment library is disabled")
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
	svc := geoenv.Service{Pool: pool, Store: store, Toolchain: cfg.EnvironmentToolchain, Parallel: cfg.EnvironmentParallel, Spool: cfg.EnvironmentSpool}
	prepare := func(ctx context.Context, dir string) error { return processAttempt(ctx, interpreter, processor, dir) }
	if !*once {
		go heartbeat(bound, readiness.Service{Pool: pool, EnvironmentLibrary: true})
	}
	var nextCleanup time.Time
	for ctx.Err() == nil {
		if now := time.Now(); now.After(nextCleanup) {
			if err := geoenv.CleanSpool(cfg.EnvironmentSpool, now); err != nil {
				fmt.Fprintln(os.Stderr, "environment scratch cleanup requires inspection")
			}
			nextCleanup = now.Add(time.Minute)
		}
		did, err := svc.WorkOne(bound, prepare)
		if *once {
			return err
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "environment queue unavailable; inspect worker readiness and build events")
		}
		if !did || err != nil {
			reconcile, stop := context.WithTimeout(bound, 10*time.Second)
			if err := svc.ReconcileOne(reconcile); err != nil {
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

func heartbeat(ctx context.Context, svc readiness.Service) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		beat, cancel := context.WithTimeout(ctx, 2*time.Second)
		if fresh, err := access.RefreshWithContext(ctx, beat); err == nil {
			_ = svc.Beat(fresh, "environment-library", "")
		}
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

func processAttempt(ctx context.Context, python, script, dir string) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, python, "-I", script, "prepare", dir)
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
			if err = checkScratch(dir); err != nil {
				return geoenv.ErrInvalidInputs
			}
			return nil
		case <-ticker.C:
			child, err := process.NewProcess(int32(command.Process.Pid))
			if err == nil {
				if mem, err := child.MemoryInfo(); err == nil && mem.RSS > 2<<30 {
					_ = command.Process.Kill()
					<-done
					return geoenv.ErrInvalidInputs
				}
			}
			if checkScratch(dir) != nil {
				_ = command.Process.Kill()
				<-done
				return geoenv.ErrInvalidInputs
			}
		}
	}
}

func checkScratch(dir string) error {
	var size int64
	return filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
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
		if size > 8<<30 {
			return errors.New("scratch budget exceeded")
		}
		return nil
	})
}
