// emad-connectors executes bounded admitted public captures as a separate process.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/SalehAlobaylan/c4isr-systems/internal/externaltracks"
	"github.com/SalehAlobaylan/c4isr-systems/internal/imagery"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publiccontext"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicevents"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/sources"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "public connector stopped:", err)
		os.Exit(1)
	}
}
func run() error {
	tokenFile := flag.String("token-file", "", "private exact-source service credential file")
	source := flag.String("source", "", "registered admitted source id")
	once := flag.Bool("once", false, "one bounded capture, without reconnect loop")
	flag.Parse()
	if *tokenFile == "" || *source == "" {
		return fmt.Errorf("-token-file and -source are required")
	}
	raw, err := os.ReadFile(*tokenFile)
	if err != nil || len(raw) > 4096 {
		return fmt.Errorf("cannot read bounded credential file")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := platformdb.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	locations, err := publicingest.LoadRegistry(cfg.PublicConnectorsFile)
	if err != nil {
		return err
	}
	identities := access.NewStore(pool)
	session, err := identities.Authenticate(ctx, strings.TrimSpace(string(raw)), nil)
	if err != nil {
		return err
	}
	bound, err := identities.BindSession(ctx, session, session.Resource)
	if err != nil {
		return err
	}
	repo := publicingest.NewRepository(pool)
	repo.SetProjector(publiccontext.Projector{Next: externaltracks.Projector{Next: imagery.Projector{Next: publicevents.Projector{}}}})
	svc := publicingest.NewService(repo, locations, publicingest.NewHTTPFetcher(), sources.NewPostgresRepository(pool), identities)
	backoff := time.Second * 5
	for {
		fresh, err := access.RefreshWithContext(bound, ctx)
		if err != nil {
			return err
		}
		result, executeErr := svc.Execute(fresh, *source)
		if *once {
			if executeErr != nil {
				return executeErr
			}
			fmt.Printf("capture status=%s; durable evidence=%d\n", result.Status, result.NewEvidence)
			if result.Status == "FAILED" || result.Status == "INTERRUPTED" {
				return fmt.Errorf("capture failed (%s)", result.FailureCode)
			}
			return nil
		}
		if result.FailureCode == "source_storage_budget" {
			return fmt.Errorf("source archive budget exhausted; inspect source health before another capture")
		}
		if ctx.Err() != nil {
			return nil
		}
		wait := time.Second * 10
		if executeErr != nil || result.Status == "FAILED" || result.Status == "INTERRUPTED" {
			wait = backoff
			backoff = min(backoff*2, time.Minute*5)
		} else {
			backoff = 5 * time.Second
		}
		// Persistent database quotas/backoff remain authoritative across processes.
		if summary, err := svc.Health(fresh, *source); err == nil && summary.NextAttemptAt != nil {
			wait = max(wait, time.Until(*summary.NextAttemptAt))
		}
		wait += time.Duration(rand.IntN(1000)) * time.Millisecond
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
