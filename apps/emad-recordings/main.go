// emad-recordings archives native journals and computes versioned metrics outside
// the synchronous API. It uses the existing private S3 artifact store.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/recordings"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "recording worker: operation failed; check scoped queue status")
		os.Exit(1)
	}
}
func run() error {
	file := flag.String("token-file", "", "private environment-bound recording-worker credential file")
	once := flag.Bool("once", false, "process at most one queued archive")
	flag.Parse()
	if *file == "" {
		return fmt.Errorf("credential file required")
	}
	token, e := os.ReadFile(*file)
	if e != nil || len(token) > 4096 {
		return fmt.Errorf("credential file unavailable")
	}
	cfg, e := config.Load()
	if e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, e := db.Open(ctx, cfg.DatabaseURL)
	if e != nil {
		return e
	}
	defer pool.Close()
	store, e := artifacts.NewS3(artifacts.S3Config{Endpoint: cfg.ArtifactS3Endpoint, Bucket: cfg.ArtifactS3Bucket, Region: cfg.ArtifactS3Region, AccessKey: cfg.ArtifactS3AccessKey, SecretKey: cfg.ArtifactS3SecretKey, AllowHTTP: cfg.ArtifactS3AllowHTTP || cfg.Environment == config.EnvironmentDevelopment || cfg.Environment == config.EnvironmentTest})
	if e != nil {
		return e
	}
	identities := access.NewStore(pool)
	session, e := identities.Authenticate(ctx, strings.TrimSpace(string(token)), httpx.PermissionsForRole)
	if e != nil {
		return e
	}
	bound, e := identities.BindSession(ctx, session, session.Resource)
	if e != nil {
		return e
	}
	service := &recordings.Service{Pool: pool, Store: store}
	return workLoop(bound, *once, service.WorkOne, wait, func(message string) {
		// Never print backend errors: they may contain credentials or object keys.
		fmt.Fprintln(os.Stderr, "recording worker: "+message)
	})
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Per-job retry eligibility is durable queue state. Transient queue/database
// failures use process-local backoff; revoked or expired credentials stop work.
func workLoop(ctx context.Context, once bool, work func(context.Context) (bool, error), pause func(context.Context, time.Duration) error, report func(string)) error {
	backoff := time.Second
	for ctx.Err() == nil {
		did, err := work(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if once || apperr.Is(err, apperr.CodeForbidden) || apperr.Is(err, apperr.CodeUnauthorized) {
			return err
		}
		delay := time.Second
		if err != nil {
			if did {
				report("archive attempt failed; checking for other eligible jobs")
			} else {
				report("queue unavailable; retrying with bounded backoff")
			}
			delay = backoff
			backoff = min(2*backoff, 30*time.Second)
		} else {
			backoff = time.Second
			if did {
				report("archive ready")
				continue
			}
		}
		if err := pause(ctx, delay); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}
	return nil
}
