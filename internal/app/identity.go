package app

import (
	"context"
	"net/http"
	"strings"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/presentation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func legacyPermissions(role string) []string {
	permissions := httpx.PermissionsForRole(role)
	if role == "operator" || role == "supervisor" || role == "administrator" {
		permissions = append(permissions, "sim.scenarios.author")
	}
	return permissions
}

func identityResolver(cfg config.Config, store *access.Store, lookup httpx.OperatorLookup, pool *pgxpool.Pool) func(context.Context, *http.Request) (context.Context, httpx.Identity, error) {
	return func(ctx context.Context, r *http.Request) (context.Context, httpx.Identity, error) {
		var session access.Session
		raw := strings.TrimSpace(r.Header.Get("Authorization"))
		fields := strings.Fields(raw)
		token := ""
		if len(fields) == 2 && strings.EqualFold(fields[0], "bearer") {
			token = fields[1]
		}
		if raw == "" && strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			token = httpx.RealtimeCredential(r)
		}
		var err error
		if token != "" {
			session, err = store.Authenticate(ctx, token, legacyPermissions)
		} else if !cfg.AuthRequired {
			id := strings.TrimSpace(r.Header.Get(httpx.OperatorHeader))
			if id == "" {
				id = DefaultOperator
			}
			var identity httpx.Identity
			identity, err = lookup(ctx, id)
			if err == nil {
				session = access.Session{OperatorID: identity.ID, Name: identity.Name, Role: identity.Role, Resource: access.LegacyResource(), Principal: access.Principal{ID: "human:" + identity.ID, Kind: "human", Grants: []access.Grant{{Resource: access.Resource{WorkspaceID: "default", EnvironmentID: "local", Partition: access.ExerciseObservation, RunID: "*"}, Permissions: legacyPermissions(identity.Role)}}}}
			}
		} else {
			err = apperr.Unauthorized("a valid bearer token is required")
		}
		if err != nil {
			return nil, httpx.Identity{}, err
		}
		selected := session.Resource
		if v := resourceSelection(r, "X-EMAD-Workspace-ID", "workspaceId"); v != "" {
			selected.WorkspaceID = v
		}
		if v := resourceSelection(r, "X-EMAD-Environment-ID", "environmentId"); v != "" {
			selected.EnvironmentID = v
		}
		if v := resourceSelection(r, "X-EMAD-Partition", "partition"); v != "" {
			selected.Partition = access.Partition(v)
			if selected.Partition == access.PublicEvidence || selected.Partition == access.ExerciseInput {
				selected.RunID = ""
			}
		}
		if v := resourceSelection(r, "X-EMAD-Run-ID", "runId"); v != "" {
			selected.RunID = v
		}
		segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(segments) >= 5 && segments[2] == "scenarios" && segments[3] == "runs" {
			selected.RunID = segments[4]
		}
		bound, err := store.BindSession(ctx, session, selected, resourceSelection(r, "X-EMAD-Workspace-ID", "workspaceId") == "" && resourceSelection(r, "X-EMAD-Environment-ID", "environmentId") == "" && resourceSelection(r, "X-EMAD-Partition", "partition") == "" && resourceSelection(r, "X-EMAD-Run-ID", "runId") == "")
		if err != nil {
			return nil, httpx.Identity{}, err
		}
		bound = presentation.WithResolver(bound, func(ctx context.Context, id string) (string, error) {
			var actual string
			err := platformdb.Scoped(pool).QueryRow(ctx, `SELECT id FROM observations WHERE id=$1 OR public_id=$1`, id).Scan(&actual)
			if err == pgx.ErrNoRows {
				return "", apperr.NotFound("observation", "unavailable")
			}
			return actual, err
		})
		bound = presentation.WithTrackResolver(bound, func(ctx context.Context, id string) (string, error) {
			var canonical string
			err := platformdb.Scoped(pool).QueryRow(ctx, `SELECT id FROM tracks WHERE id=$1 OR external_ref=$1 ORDER BY (id=$1) DESC LIMIT 1`, id).Scan(&canonical)
			if err == pgx.ErrNoRows {
				return "", apperr.NotFound("track", "unavailable")
			}
			return canonical, err
		})
		bound = presentation.WithViewResolver(bound, func(ctx context.Context, ids []string) (map[string]string, error) {
			rows, err := platformdb.Scoped(pool).Query(ctx, `SELECT id,public_id FROM observations WHERE id=ANY($1::text[]) OR public_id=ANY($1::text[])`, ids)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			aliases := map[string]string{}
			for rows.Next() {
				var internalID, publicID string
				if err := rows.Scan(&internalID, &publicID); err != nil {
					return nil, err
				}
				aliases[internalID] = publicID
				aliases[publicID] = publicID
			}
			return aliases, rows.Err()
		})
		return bound, httpx.Identity{ID: session.OperatorID, Name: session.Name, Role: session.Role}, nil
	}
}

func resourceSelection(r *http.Request, header, param string) string {
	if v := r.Header.Get(header); v != "" {
		return v
	}
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return r.URL.Query().Get(param)
	}
	return ""
}
