package geoenv

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/jackc/pgx/v5"
)

// Processor reads recipe.json and immutable inputs from an attempt directory.
// It writes manifest.json and prepared.zip into the same private directory.
type Processor func(context.Context, string) error

var ErrInvalidInputs = errors.New("geographic input validation failed")
var ErrScratchBudget = errors.New("geographic scratch budget exceeded")

const (
	attemptTimeout         = 30 * time.Minute
	attemptDirectoryPrefix = "emad-environment-"
)

func (s Service) claim(ctx context.Context, displayToolchain, layerToolchain string) (Job, error) {
	var j Job
	p, r, _ := access.From(ctx)
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+":environment-claims"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `WITH expired AS (UPDATE geographic_environment_builds SET state='FAILED',failure_code='lease_expired',lease_until=NULL,lease_id='',worker_id='' WHERE state='RUNNING' AND attempt>=3 AND lease_until<=clock_timestamp() RETURNING id,required_sources,attempt) INSERT INTO geographic_environment_build_events(id,build_id,required_sources,attempt,state,code) SELECT 'lease_expired_'||id,id,required_sources,attempt,'FAILED','lease_expired' FROM expired`)
		if err != nil {
			return err
		}
		var available bool
		if err = tx.QueryRow(ctx, `SELECT emad_environment_claim_capacity($1)`, min(2, max(1, s.Parallel))).Scan(&available); err != nil {
			return err
		}
		if !available {
			return nil
		}
		j, err = scanJob(tx.QueryRow(ctx, jobSelect+`WHERE
			(($1<>'' AND recipe->>'toolchainSha256'=$1 AND NOT recipe ? 'display')
			OR (recipe ? 'display' AND (recipe->>'toolchainSha256'=NULLIF($2,'') OR recipe->>'toolchainSha256'=NULLIF($3,''))))
			AND attempt<3 AND ((state='QUEUED' AND next_attempt_at<=clock_timestamp()) OR (state='RUNNING' AND lease_until<=clock_timestamp()))
			ORDER BY next_attempt_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`, s.Toolchain, displayToolchain, layerToolchain))
		if apperr.Is(err, apperr.CodeNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if j.State == "RUNNING" {
			if err = buildEvent(ctx, tx, j, "LEASE_EXPIRED", "lease_expired"); err != nil {
				return err
			}
		}
		j.State, j.Attempt, j.worker, j.lease = "RUNNING", j.Attempt+1, p.ID, ids.New("geolease")
		if err = tx.QueryRow(ctx, `UPDATE geographic_environment_builds SET state='RUNNING',attempt=$2,worker_id=$3,lease_id=$4,lease_until=clock_timestamp()+interval '60 seconds',failure_code='' WHERE id=$1 RETURNING lease_until`, j.ID, j.Attempt, j.worker, j.lease).Scan(&j.leaseUntil); err != nil {
			return err
		}
		return buildEvent(ctx, tx, j, "RUNNING", "")
	})
	return j, err
}

func (s Service) keepAlive(auth, work context.Context, j Job) error {
	ctx, err := access.RefreshWithContext(auth, work)
	if err != nil {
		return err
	}
	if err = require(ctx, Execute, true); err != nil {
		return err
	}
	tag, err := db.Scoped(s.Pool).Exec(ctx, `UPDATE geographic_environment_builds SET lease_until=clock_timestamp()+interval '60 seconds' WHERE id=$1 AND state='RUNNING' AND lease_id=$2 AND worker_id=$3 AND lease_until>clock_timestamp()`, j.ID, j.lease, j.worker)
	if err == nil && tag.RowsAffected() != 1 {
		return apperr.Conflict("build lease is no longer active")
	}
	return err
}

func (s Service) fail(ctx context.Context, j Job, code string, retry bool) error {
	return db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		state, delay := "FAILED", 0
		if retry && j.Attempt < 3 {
			state = "QUEUED"
			delay = 5
			if j.Attempt == 2 {
				delay = 30
			}
			// Stable, bounded jitter avoids simultaneous retries on restart.
			delay += ((int(j.ID[len(j.ID)-1]) % 3) - 1) * delay / 5
		}
		tag, err := tx.Exec(ctx, `UPDATE geographic_environment_builds SET state=$4,lease_id='',worker_id='',lease_until=NULL,failure_code=$5,next_attempt_at=clock_timestamp()+($6::integer*interval '1 second') WHERE id=$1 AND state='RUNNING' AND lease_id=$2 AND worker_id=$3 AND lease_until>clock_timestamp()`, j.ID, j.lease, j.worker, state, code, delay)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		return buildEvent(ctx, tx, j, state, code)
	})
}

func (s Service) WorkOne(auth context.Context, process Processor) (bool, error) {
	ctx, err := access.Refresh(auth)
	if err != nil {
		return false, err
	}
	if err = require(ctx, Execute, true); err != nil {
		return false, err
	}
	if s.Store == nil || !digestPattern.MatchString(s.Toolchain) || process == nil {
		return false, apperr.Conflict("worker storage or processor unavailable")
	}
	displayToolchain, layerToolchain := "", ""
	if s.DisplayProcessor != nil {
		displayToolchain = s.DisplayToolchain
	}
	if s.LayerProcessor != nil {
		layerToolchain = s.LayerToolchain
	}
	if s.DisplayOnly {
		if !digestPattern.MatchString(displayToolchain) && !digestPattern.MatchString(layerToolchain) {
			return false, apperr.Conflict("geographic display processor unavailable")
		}
		s.Toolchain = ""
	}
	j, err := s.claim(ctx, displayToolchain, layerToolchain)
	if err != nil || j.ID == "" {
		return false, err
	}
	work, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()
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
				if s.keepAlive(auth, work, j) != nil {
					cancel()
					return
				}
			}
		}
	}()
	var code string
	if j.recipe.Display != nil {
		code, err = s.prepareDisplay(work, auth, j)
	} else {
		code, err = s.prepare(work, auth, j, process)
	}
	cancel()
	<-done
	if err == nil {
		return true, nil
	}
	final, stop := context.WithTimeout(context.WithoutCancel(auth), 10*time.Second)
	defer stop()
	final, refreshErr := access.Refresh(final)
	if refreshErr != nil {
		return true, refreshErr
	}
	if refreshErr = require(final, Execute, true); refreshErr != nil {
		return true, refreshErr
	}
	return true, s.fail(final, j, code, code == "transient_io")
}

func (s Service) prepare(work, auth context.Context, j Job, process Processor) (string, error) {
	human, err := s.actor(work, j)
	if err != nil {
		return "builder_unavailable", err
	}
	if err = s.checkInputs(human, j.recipe, false); err != nil {
		return "input_policy_unavailable", err
	}
	if err = s.checkInputs(work, j.recipe, false); err != nil {
		return "worker_policy_unavailable", err
	}
	dir, err := os.MkdirTemp(s.Spool, attemptDirectoryPrefix)
	if err != nil {
		return "transient_io", err
	}
	defer os.RemoveAll(dir)
	inputDir := filepath.Join(dir, "inputs")
	if err = os.Mkdir(inputDir, 0700); err != nil {
		return "transient_io", err
	}
	for _, input := range j.recipe.Inputs {
		if !keyPattern.MatchString(input.ID) {
			return "invalid_input", ErrInvalidInputs
		}
		a, err := s.asset(work, input.ID)
		if err != nil {
			return "input_unavailable", err
		}
		if err = s.spoolInput(work, a, filepath.Join(inputDir, input.ID)); err != nil {
			return "transient_io", err
		}
	}
	raw, err := json.Marshal(struct {
		Recipe
		InputSHA256 string `json:"inputSha256"`
	}{j.recipe, j.InputSHA256})
	if err != nil {
		return "invalid_input", err
	}
	if err = os.WriteFile(filepath.Join(dir, "recipe.json"), raw, 0600); err != nil {
		return "transient_io", err
	}
	if err = process(work, dir); err != nil {
		if errors.Is(err, ErrScratchBudget) {
			return "scratch_budget_exceeded", err
		}
		if errors.Is(err, ErrInvalidInputs) {
			return "invalid_input", err
		}
		return "transient_io", err
	}
	ctx, err := access.RefreshWithContext(auth, work)
	if err != nil {
		return "worker_policy_unavailable", err
	}
	if err = require(ctx, Execute, true); err != nil {
		return "worker_policy_unavailable", err
	}
	if err = s.stage(ctx, j, dir); err != nil {
		if errors.Is(err, ErrInvalidInputs) {
			return "invalid_output", err
		}
		if errors.Is(err, errPublicationPolicy) || apperr.Is(err, apperr.CodeForbidden) || apperr.Is(err, apperr.CodeUnauthorized) {
			return "publication_policy_unavailable", err
		}
		return "transient_io", err
	}
	return "", nil
}

func (s Service) spoolInput(ctx context.Context, a Asset, path string) error {
	stream, err := s.Store.OpenGeographic(ctx, a.objectKey, a.Bytes, nil)
	if err != nil {
		return err
	}
	defer stream.Close()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(file, h), io.LimitReader(stream, a.Bytes+1))
	err = file.Close()
	if copyErr != nil {
		return copyErr
	}
	if err != nil {
		return err
	}
	if n != a.Bytes || hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		return errors.New("retained input integrity mismatch")
	}
	return nil
}
