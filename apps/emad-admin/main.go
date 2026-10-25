// emad-admin is local control-plane provisioning. It uses migration/operator DB
// credentials and writes issued bearer secrets only to an explicitly named file.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
)

type spec struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Name        string          `json:"name"`
	OperatorID  string          `json:"operatorId"`
	Resource    access.Resource `json:"resource"`
	Permissions []string        `json:"permissions"`
	SourceIDs   []string        `json:"sourceIds"`
}

func run() error {
	mode := flag.String("action", "report", "provision, grant, issue, rotate, revoke, report, or backfill")
	file := flag.String("spec", "", "principal/grant JSON file")
	out := flag.String("out", "", "new credential file; must not exist")
	key := flag.String("credential", "", "credential ID to revoke")
	lifetime := flag.Duration("lifetime", 24*time.Hour, "issued credential lifetime, at most 90 days")
	truthGrants := flag.Bool("allow-truth-grants", false, "explicitly permit local provisioning of restricted truth/evaluator grants")
	flag.Parse()
	databaseURL, err := config.LoadDatabaseURL()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	store := access.NewStore(pool)
	admin := access.Bootstrap(ctx)
	if *truthGrants {
		principal, resource, _ := access.From(admin)
		principal.Grants[0].Permissions = append(principal.Grants[0].Permissions, "sim.truth.grant")
		admin, err = access.Bind(ctx, principal, resource, nil)
		if err != nil {
			return err
		}
	}
	if *mode == "report" || *mode == "backfill" {
		if *mode == "backfill" {
			if _, err = pool.Exec(ctx, `SELECT emad_backfill_ownership()`); err != nil {
				return err
			}
		}
		rows, err := pool.Query(ctx, `SELECT table_name,resolved,quarantined FROM emad_ownership_report ORDER BY table_name`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			var resolved, quarantined int64
			if err = rows.Scan(&name, &resolved, &quarantined); err != nil {
				return err
			}
			fmt.Printf("%s resolved=%d quarantined=%d\n", name, resolved, quarantined)
		}
		return rows.Err()
	}
	if *mode == "revoke" {
		if *key == "" {
			return fmt.Errorf("-credential is required")
		}
		return store.RevokeCredential(admin, *key)
	}
	if *mode != "provision" && *mode != "rotate" && *mode != "grant" && *mode != "issue" {
		return fmt.Errorf("unknown action")
	}
	if *file == "" {
		return fmt.Errorf("-spec is required")
	}
	input, err := os.Open(*file)
	if err != nil {
		return err
	}
	defer input.Close()
	var desired spec
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&desired); err != nil {
		return err
	}
	grant := access.Grant{Resource: desired.Resource, Permissions: desired.Permissions, SourceIDs: desired.SourceIDs}
	if *mode == "grant" {
		return store.AddGrant(admin, desired.ID, grant)
	}
	if *lifetime <= 0 || *lifetime > 90*24*time.Hour {
		return fmt.Errorf("invalid credential lifetime")
	}
	if *mode == "rotate" {
		if *key == "" {
			return fmt.Errorf("rotate requires -credential identifying the old key")
		}
		var owner string
		if err = pool.QueryRow(ctx, `SELECT principal_id FROM principal_credentials WHERE id=$1`, *key).Scan(&owner); err != nil {
			return err
		}
		if owner != desired.ID {
			return fmt.Errorf("credential belongs to another principal")
		}
	}
	if *out == "" {
		return fmt.Errorf("-out is required; bearer secrets are never printed")
	}
	secretFile, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer secretFile.Close()
	written := false
	defer func() {
		if !written {
			_ = secretFile.Close()
			_ = os.Remove(*out)
		}
	}()
	if *mode == "provision" {
		switch desired.Kind {
		case "human":
			err = store.CreateHuman(admin, desired.ID, desired.OperatorID, grant)
		case "service":
			err = store.CreateService(admin, desired.ID, desired.Name, desired.Resource, desired.Permissions, desired.SourceIDs)
		default:
			err = fmt.Errorf("kind must be human or service")
		}
		if err != nil {
			return err
		}
	}
	credentialID, token, err := store.IssueCredential(admin, desired.ID, desired.Resource, time.Now().Add(*lifetime))
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintln(secretFile, token); err != nil {
		_ = store.RevokeCredential(admin, credentialID)
		return err
	}
	if err = secretFile.Sync(); err != nil {
		_ = store.RevokeCredential(admin, credentialID)
		return err
	}
	written = true
	if *mode == "rotate" {
		if err = store.RevokeCredential(admin, *key); err != nil {
			return err
		}
	}
	fmt.Printf("credential=%s principal=%s expires-in=%s\n", credentialID, desired.ID, lifetime.String())
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
