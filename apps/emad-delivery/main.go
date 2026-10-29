// emad-delivery processes case-authorized brief/environment jobs outside HTTP.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/investigations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "delivery worker:", e)
		os.Exit(1)
	}
}
func run() error {
	tokenFile := flag.String("token-file", "", "private original case author credential; automation does not grant review authority")
	once := flag.Bool("once", false, "process at most one queued job")
	flag.Parse()
	if *tokenFile == "" {
		return fmt.Errorf("-token-file required")
	}
	token, e := os.ReadFile(*tokenFile)
	if e != nil || len(token) > 4096 {
		return fmt.Errorf("cannot read bounded credential file")
	}
	cfg, e := config.Load()
	if e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, e := platformdb.Open(ctx, cfg.DatabaseURL)
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
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	raw, e := os.ReadFile(exe)
	if e != nil {
		return e
	}
	build := artifacts.Hash(raw)
	svc := investigations.Service{Pool: pool, Store: store}
	for {
		did, e := svc.WorkDelivery(bound, build)
		if e != nil {
			fmt.Fprintln(os.Stderr, "delivery attempt failed; inspect job status:", e)
			if *once {
				return e
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(2 * time.Second):
			}
		}
		if *once {
			return nil
		}
		if !did {
			if _, err := svc.ReconcileDeliveryOutput(bound); err != nil {
				fmt.Fprintln(os.Stderr, "delivery expiry reconciliation needs inspection:", err)
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}
