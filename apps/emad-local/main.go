// emad-local installs explicit local development identities using trusted DB
// authority. It is never mounted as an HTTP endpoint or shipped in the UI.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/recordings"
	"github.com/jackc/pgx/v5"
)

type profile struct {
	ID, Human, Name string
	Grants          []access.Grant
}

func sorted(v []string) []string { out := append([]string{}, v...); sort.Strings(out); return out }
func run() error {
	root := flag.String("credentials-dir", "private/local/credentials", "ignored private directory; credentials never printed")
	productProfile := flag.String("profile", "map", "map or historical-full")
	rotateExpired := flag.Bool("rotate-expired", false, "explicitly rotate expired local development credentials belonging to their expected principal")
	flag.Parse()
	if *productProfile != "map" && *productProfile != "historical-full" {
		return fmt.Errorf("profile must be map or historical-full")
	}
	url, e := config.LoadDatabaseURL()
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, e := db.Open(ctx, url)
	if e != nil {
		return e
	}
	defer pool.Close()
	conn, e := pool.Acquire(ctx)
	if e != nil {
		return e
	}
	defer conn.Release()
	if _, e = conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended('emad-local-setup',0))`); e != nil {
		return e
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended('emad-local-setup',0))`)
	store := access.NewStore(pool)
	admin := access.Bootstrap(ctx)
	p, r, _ := access.From(admin)
	p.Grants[0].Permissions = append(p.Grants[0].Permissions, "sim.truth.grant")
	admin, e = access.Bind(admin, p, r, nil)
	if e != nil {
		return e
	}
	pub := access.Resource{WorkspaceID: "default", EnvironmentID: "local", Partition: access.PublicEvidence}
	sourceID := func(name string) string {
		return access.ResourceID(access.Internal(ctx, pub, "local-identifiers"), "source", "local-"+name)
	}
	input := pub
	input.Partition = access.ExerciseInput
	observation := pub
	observation.Partition = access.ExerciseObservation
	observation.RunID = "local-ready"
	allRuns := observation
	allRuns.RunID = "*"
	truth := allRuns
	truth.Partition = access.ExerciseTruth
	grant := func(r access.Resource, ps ...string) access.Grant {
		return access.Grant{Resource: r, Permissions: append([]string{"operational.read"}, ps...), SourceIDs: []string{}}
	}
	publicPermissions := []string{"outputs.read", "outputs.create", "outputs.distribute", "acquisitions.read", "acquisitions.request", "geography.request", "investigations.read", "investigations.create", "investigations.edit", "investigations.manage", "sources.read", "sources.manage", "observations.read", "tracks.read", "classifications.read", "geofences.read", "alerts.read", "incidents.read", "assessments.read", "assessments.create", "audit.read", "imagery.jobs.submit", "context.export"}
	// Use known permissions only; profile drift is rejected below, never silently
	// applied to an existing identity maintained outside this installer.
	filter := func(ps []string) []string {
		out := []string{}
		for _, p := range ps {
			if access.KnownPermission(p) {
				out = append(out, p)
			}
		}
		return out
	}
	publicPermissions = filter(publicPermissions)
	op := filter(append(append([]string{}, recordings.ViewPermissions...), "operational.read", "commands.issue", "missions.create", "incidents.create", "sim.recordings.read", "sim.recordings.manage", "sim.evaluation.read", "sim.evaluation.release", "scenarios.control"))
	profiles := []profile{
		{"local-analyst", "operator-01", "Development analyst / scene author", []access.Grant{grant(pub, publicPermissions...), grant(input, "sim.scenarios.author", "sim.runtime.control", "sim.recordings.read"), {Resource: allRuns, Permissions: op, SourceIDs: []string{}}}},
		{"local-author-alias", "operator-01", "Development same-human review alias (negative test)", []access.Grant{grant(pub, "sources.read", "assessments.read", "assessments.review", "context.export")}},
		{"local-reviewer", "local-review-human", "Development reviewer (account-level test)", []access.Grant{grant(pub, "sources.read", "assessments.read", "assessments.review", "context.export")}},
		{"local-operator", "local-exercise-human", "Development exercise operator", []access.Grant{{Resource: observation, Permissions: op, SourceIDs: []string{}}, {Resource: allRuns, Permissions: op, SourceIDs: []string{}}}},
		{"local-evaluator", "local-evaluator-human", "Development evaluator", []access.Grant{{Resource: observation, Permissions: op, SourceIDs: []string{}}, {Resource: allRuns, Permissions: op, SourceIDs: []string{}}, grant(truth, "sim.truth.read", "sim.recordings.read", "sim.evaluation.read", "sim.evaluation.release"), grant(input, "sim.scenarios.author", "sim.runtime.control")}},
	}
	analystPermissions := []string{}
	for _, permission := range publicPermissions {
		if permission != "sources.manage" {
			analystPermissions = append(analystPermissions, permission)
		}
	}
	profiles = append(profiles, profile{"local-map-reviewer", "local-map-review-human", "Map review account (requires independent human validation)", []access.Grant{grant(pub, "sources.read", "observations.read", "investigations.read", "investigations.review", "outputs.read", "outputs.distribute")}})
	profiles = append(profiles, profile{"local-map-analyst", "operator-01", "Map analyst", []access.Grant{grant(pub, analystPermissions...)}}, profile{"local-source-admin", "operator-01", "Local source administrator", []access.Grant{grant(pub, publicPermissions...)}})
	for _, connector := range []string{"usgs", "usgs-history", "firms", "opensky", "ais", "metar", "gdacs", "cdse", "earthsearch"} {
		g := grant(pub, "sources.read", "connectors.execute")
		g.SourceIDs = []string{sourceID(connector)}
		profiles = append(profiles, profile{ID: "worker-" + connector, Name: "Local source worker " + connector, Grants: []access.Grant{g}})
	}
	ig := grant(pub, "sources.read", "imagery.jobs.execute")
	ig.SourceIDs = []string{sourceID("cdse"), sourceID("earthsearch")}
	captureGrant := grant(pub, "sources.read", "sources.manage", "imagery.jobs.submit", "imagery.jobs.execute", "imagery.capture.execute")
	captureGrant.SourceIDs = []string{sourceID("earthsearch")}
	profiles = append(profiles, profile{ID: "worker-preparation", Name: "Local real raster preparation", Grants: []access.Grant{captureGrant}})
	profiles = append(profiles, profile{ID: "worker-imagery", Name: "Local imagery worker", Grants: []access.Grant{ig}}, profile{ID: "worker-assets", Name: "Local model worker", Grants: []access.Grant{grant(input, "sim.assets.import")}}, profile{ID: "worker-runtime", Name: "Local runtime worker", Grants: []access.Grant{grant(input, "sim.runtime.execute")}}, profile{ID: "worker-recordings", Name: "Local archive worker", Grants: []access.Grant{grant(input, "sim.recordings.execute")}})
	if e = os.MkdirAll(*root, 0700); e != nil {
		return e
	}
	for _, item := range profiles {
		if *productProfile == "map" && (item.ID == "local-analyst" || item.ID == "local-author-alias" || item.ID == "local-reviewer" || item.ID == "local-operator" || item.ID == "local-evaluator" || item.ID == "worker-assets" || item.ID == "worker-runtime" || item.ID == "worker-recordings") {
			continue
		}
		if item.Human != "" {
			if _, e = pool.Exec(ctx, `INSERT INTO operators(id,name,role)VALUES($1,$2,'supervisor')ON CONFLICT DO NOTHING`, item.Human, item.Name); e != nil {
				return e
			}
		}
		var kind string
		var human *string
		e = pool.QueryRow(ctx, `SELECT kind,operator_id FROM principals WHERE id=$1`, item.ID).Scan(&kind, &human)
		if e == pgx.ErrNoRows {
			if item.Human != "" {
				e = store.CreateHuman(admin, item.ID, item.Human, item.Grants[0])
			} else {
				g := item.Grants[0]
				e = store.CreateService(admin, item.ID, item.Name, g.Resource, g.Permissions, g.SourceIDs)
			}
			if e != nil {
				return fmt.Errorf("create %s: %w", item.ID, e)
			}
		} else if e != nil {
			return e
		} else if (item.Human != "" && (kind != "human" || human == nil || *human != item.Human)) || (item.Human == "" && kind != "service") {
			return fmt.Errorf("identity conflict for %s; refusing to reassign", item.ID)
		}
		for _, g := range item.Grants {
			if g.Resource.RunID != "*" {
				if e = store.EnsureResource(admin, g.Resource); e != nil {
					return e
				}
			}
			var ps, ss []string
			e = pool.QueryRow(ctx, `SELECT permissions,source_ids FROM principal_grants WHERE principal_id=$1 AND workspace_id=$2 AND environment_id=$3 AND partition=$4 AND run_id=$5`, item.ID, g.Resource.WorkspaceID, g.Resource.EnvironmentID, string(g.Resource.Partition), g.Resource.RunID).Scan(&ps, &ss)
			if e == pgx.ErrNoRows {
				e = store.AddGrant(admin, item.ID, g)
			} else if e == nil && (!reflect.DeepEqual(sorted(ps), sorted(g.Permissions)) || !reflect.DeepEqual(sorted(ss), sorted(g.SourceIDs))) {
				// Correct the early local installer's logical/canonical source ID
				// mismatch only when every other grant property matches exactly.
				converted := []string{}
				for _, id := range ss {
					if strings.HasPrefix(id, "local-") {
						converted = append(converted, sourceID(strings.TrimPrefix(id, "local-")))
					}
				}
				if item.Human == "" && len(ss) > 0 && len(converted) == len(ss) && reflect.DeepEqual(sorted(ps), sorted(g.Permissions)) && reflect.DeepEqual(sorted(converted), sorted(g.SourceIDs)) {
					_, e = pool.Exec(ctx, `UPDATE principal_grants SET source_ids=$6 WHERE principal_id=$1 AND workspace_id=$2 AND environment_id=$3 AND partition=$4 AND run_id=$5 AND source_ids=$7`, item.ID, g.Resource.WorkspaceID, g.Resource.EnvironmentID, string(g.Resource.Partition), g.Resource.RunID, g.SourceIDs, ss)
					if e == nil {
						_, e = pool.Exec(ctx, `INSERT INTO authorization_changes(actor_id,subject_id,action)VALUES('internal:bootstrap',$1,'local.source-identifiers.corrected')`, item.ID)
					}
				} else {
					return fmt.Errorf("grant drift for %s at %s; refusing to widen or overwrite", item.ID, g.Resource.ID())
				}
			}
			if e != nil {
				return fmt.Errorf("grant %s: %w", item.ID, e)
			}
		}
		path := filepath.Join(*root, item.ID+".token")
		if raw, readErr := os.ReadFile(path); readErr == nil {
			tokenStr := strings.TrimSpace(string(raw))
			s, authErr := store.Authenticate(ctx, tokenStr, httpx.PermissionsForRole)
			if authErr != nil || s.Principal.ID != item.ID {
				if !*rotateExpired {
					return fmt.Errorf("existing credential for %s is expired or mismatched; rotate explicitly with emad-admin or emad-local -rotate-expired", item.ID)
				}
				sum := sha256.Sum256([]byte(tokenStr))
				var oldID, owner string
				if lookupErr := pool.QueryRow(ctx, `SELECT id,principal_id FROM principal_credentials WHERE token_hash=$1`, sum[:]).Scan(&oldID, &owner); lookupErr != nil || owner != item.ID {
					return fmt.Errorf("existing credential for %s is mismatched or unrecognized; refusing automatic rotation", item.ID)
				}
				newID, secret, issueErr := store.IssueCredential(admin, item.ID, item.Grants[0].Resource, time.Now().Add(30*24*time.Hour))
				if issueErr != nil {
					return issueErr
				}
				tmp := path + ".rotating"
				f, createErr := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
				if createErr != nil {
					_ = store.RevokeCredential(admin, newID)
					return createErr
				}
				_, writeErr := f.WriteString(secret)
				syncErr := f.Sync()
				closeErr := f.Close()
				if writeErr != nil || syncErr != nil || closeErr != nil {
					_ = os.Remove(tmp)
					_ = store.RevokeCredential(admin, newID)
					if writeErr != nil {
						return writeErr
					}
					if syncErr != nil {
						return syncErr
					}
					return closeErr
				}
				if renameErr := os.Rename(tmp, path); renameErr != nil {
					_ = os.Remove(tmp)
					_ = store.RevokeCredential(admin, newID)
					return renameErr
				}
				if revokeErr := store.RevokeCredential(admin, oldID); revokeErr != nil {
					return revokeErr
				}
				fmt.Printf("%s: rotated expired credential -> %s\n", item.ID, path)
				continue
			}
		} else if !os.IsNotExist(readErr) {
			return readErr
		} else {
			_, secret, issueErr := store.IssueCredential(admin, item.ID, item.Grants[0].Resource, time.Now().Add(30*24*time.Hour))
			if issueErr != nil {
				return issueErr
			}
			f, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if createErr != nil {
				return createErr
			}
			_, e = f.WriteString(secret)
			closeErr := f.Close()
			if e != nil {
				return e
			}
			if closeErr != nil {
				return closeErr
			}
		}
		fmt.Printf("%s: %s\n", item.ID, path)
	}
	if e = reconcileGDACS(ctx, pool, pub, sourceID("gdacs")); e != nil {
		return e
	}
	if e = reconcileOpenSkyBudget(ctx, pool, pub, sourceID("opensky")); e != nil {
		return e
	}
	summary := map[string]any{"profile": "emad.local.v2", "productProfile": *productProfile, "accounts": "development identities; no claim of review by two actual people", "workspaceId": "default", "environmentId": "local", "installedAt": time.Now().UTC()}
	raw, _ := json.MarshalIndent(summary, "", "  ")
	return os.WriteFile(filepath.Join(*root, "profile.json"), raw, 0600)
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "local provisioning:", e)
		os.Exit(1)
	}
}
