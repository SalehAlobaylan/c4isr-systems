package access

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store owns the platform identity tables. It deliberately uses the privileged
// control-plane pool; domain repositories must use db.Scoped instead.
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool} }

func (s *Store) Audit(ctx context.Context, d Denial) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	_, _ = s.pool.Exec(ctx, `INSERT INTO authorization_denials(principal_id,permission,scope_id,reason) VALUES($1,$2,$3,$4)`, d.PrincipalID, d.Permission, d.ScopeID, d.Reason)
}

func (s *Store) EnsureResource(ctx context.Context, r Resource) error {
	if err := r.Validate(); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO workspaces(id) VALUES($1) ON CONFLICT DO NOTHING`, r.WorkspaceID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO environments(workspace_id,id) VALUES($1,$2) ON CONFLICT DO NOTHING`, r.WorkspaceID, r.EnvironmentID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO data_scopes(id,workspace_id,environment_id,partition,run_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, r.ID(), r.WorkspaceID, r.EnvironmentID, string(r.Partition), r.RunID)
	return err
}

func (s *Store) LegacyCredentials(ctx context.Context, tokens map[string]string) error {
	active := []string{}
	for token, operatorID := range tokens {
		var name, role string
		if err := s.pool.QueryRow(ctx, `SELECT name,role FROM operators WHERE id=$1`, operatorID).Scan(&name, &role); err != nil {
			return fmt.Errorf("legacy token subject is not registered: %w", err)
		}
		id := "human:" + operatorID
		_, err := s.pool.Exec(ctx, `INSERT INTO principals(id,kind,operator_id,name) VALUES($1,'human',$2,$3) ON CONFLICT DO NOTHING`, id, operatorID, name)
		if err != nil {
			return err
		}
		var exists bool
		if err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM principal_grants WHERE principal_id=$1)`, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			_, err = s.pool.Exec(ctx, `INSERT INTO principal_grants(principal_id,workspace_id,environment_id,partition,run_id,permissions) VALUES($1,'default','local','EXERCISE_OBSERVATION','*',ARRAY['legacy.role'])`, id)
			if err != nil {
				return err
			}
		}
		h := sha256.Sum256([]byte(token))
		credentialID := "legacy_" + hex.EncodeToString(h[:16])
		active = append(active, credentialID)
		_, err = s.pool.Exec(ctx, `INSERT INTO principal_credentials(id,principal_id,token_hash,default_scope_id,expires_at) VALUES($1,$2,$3,$4,now()+interval '30 days') ON CONFLICT DO NOTHING`, credentialID, id, h[:], LegacyResource().ID())
		if err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx, `UPDATE principal_credentials SET revoked_at=now() WHERE id LIKE 'legacy\_%' ESCAPE '\' AND NOT(id=ANY($1::text[])) AND revoked_at IS NULL`, active)
	return err
}

type Session struct {
	Principal              Principal
	Resource               Resource
	OperatorID, Name, Role string
	refresh                func(context.Context) (Session, error)
}

func (s *Store) Authenticate(ctx context.Context, token string, legacyPermissions func(string) []string) (Session, error) {
	h := sha256.Sum256([]byte(token))
	return s.authenticate(ctx, "token_hash", h[:], legacyPermissions)
}

func (s *Store) authenticate(ctx context.Context, column string, value any, legacyPermissions func(string) []string) (Session, error) {
	if column != "id" && column != "token_hash" {
		panic("invalid credential lookup")
	}
	var session Session
	var valid bool
	err := s.pool.QueryRow(ctx, `SELECT p.id,p.kind,c.id,c.expires_at,COALESCE(p.operator_id,p.id),p.name,COALESCE(o.role,'service'),
        d.workspace_id,d.environment_id,d.partition,d.run_id,
        (p.disabled_at IS NULL AND c.revoked_at IS NULL AND c.expires_at>now())
        FROM principal_credentials c JOIN principals p ON p.id=c.principal_id
        JOIN data_scopes d ON d.id=c.default_scope_id LEFT JOIN operators o ON o.id=p.operator_id
        WHERE c.`+column+`=$1`, value).Scan(&session.Principal.ID, &session.Principal.Kind, &session.Principal.CredentialID, &session.Principal.ExpiresAt, &session.OperatorID, &session.Name, &session.Role, &session.Resource.WorkspaceID, &session.Resource.EnvironmentID, &session.Resource.Partition, &session.Resource.RunID, &valid)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, apperr.Unauthorized("a valid credential is required")
	}
	if err != nil {
		return Session{}, apperr.Internal(err)
	}
	if !valid {
		s.Audit(ctx, Denial{session.Principal.ID, "authenticate", session.Resource.ID(), "credential inactive"})
		return Session{}, apperr.Unauthorized("a valid credential is required")
	}
	if session.Principal.Kind == "human" {
		session.Principal.HumanID = session.OperatorID
	}
	rows, err := s.pool.Query(ctx, `SELECT workspace_id,environment_id,partition,run_id,permissions,source_ids FROM principal_grants WHERE principal_id=$1 ORDER BY id`, session.Principal.ID)
	if err != nil {
		return Session{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var g Grant
		if err = rows.Scan(&g.Resource.WorkspaceID, &g.Resource.EnvironmentID, &g.Resource.Partition, &g.Resource.RunID, &g.Permissions, &g.SourceIDs); err != nil {
			return Session{}, err
		}
		// Existing configured operator credentials retain the live operator-role
		// ceiling; changing a role cannot leave stale elevated grants in effect.
		if strings.HasPrefix(session.Principal.ID, "human:") {
			allowed := legacyPermissions(session.Role)
			filtered := []string{}
			for _, p := range g.Permissions {
				if p == "legacy.role" {
					filtered = append(filtered, allowed...)
					continue
				}
				for _, a := range allowed {
					if p == a {
						filtered = append(filtered, p)
						break
					}
				}
			}
			g.Permissions = filtered
		}
		session.Principal.Grants = append(session.Principal.Grants, g)
	}
	credentialID := session.Principal.CredentialID
	session.refresh = func(ctx context.Context) (Session, error) {
		return s.authenticate(ctx, "id", credentialID, legacyPermissions)
	}
	return session, rows.Err()
}

// BindSession checks the requested resource against live grants and existing
// scope records before installing any application authority.
func (s *Store) BindSession(ctx context.Context, session Session, r Resource, followDefault ...bool) (context.Context, error) {
	bound, err := Bind(ctx, session.Principal, r, s.Audit)
	if err != nil {
		return nil, err
	}
	var exists bool
	if err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM data_scopes WHERE id=$1)`, r.ID()).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		s.Audit(ctx, Denial{session.Principal.ID, "scope.select", r.ID(), "resource does not exist"})
		return nil, apperr.Forbidden("resource is not authorized")
	}
	bound = withLifecycle(bound, s.EnsureResource, func(ctx context.Context, r Resource) error {
		if session.Principal.CredentialID == "" {
			return nil
		}
		_, err := s.pool.Exec(ctx, `UPDATE principal_credentials SET default_scope_id=$1 WHERE id=$2 AND revoked_at IS NULL AND expires_at>now()`, r.ID(), session.Principal.CredentialID)
		return err
	})
	if session.refresh != nil {
		bound = WithRefresh(bound, func(ctx context.Context) (context.Context, error) {
			fresh, err := session.refresh(ctx)
			if err != nil {
				return nil, err
			}
			selected := r
			if len(followDefault) > 0 && followDefault[0] {
				selected = fresh.Resource
			}
			refreshed, err := Bind(ctx, fresh.Principal, selected, s.Audit)
			if err != nil {
				return nil, err
			}
			if err = Require(refreshed, "operational.read"); err != nil {
				return nil, err
			}
			return refreshed, nil
		})
	}
	return bound, nil
}

// CreateService is a deliberately small provisioning API for trusted platform
// tooling. Truth grants require an additional explicit permission.
func (s *Store) CreateService(ctx context.Context, id, name string, resource Resource, permissions, sourceIDs []string) error {
	if sourceIDs == nil {
		sourceIDs = []string{}
	}
	if err := Require(ctx, "admin.manage"); err != nil {
		return err
	}
	if resource.Partition == ExerciseTruth {
		if err := Require(ctx, "sim.truth.grant"); err != nil {
			return err
		}
	}
	if !identifier.MatchString(id) || strings.TrimSpace(name) == "" || strings.HasPrefix(id, "human:") || strings.HasPrefix(id, "internal:") {
		return apperr.Validation("invalid service principal")
	}
	for _, permission := range permissions {
		if !delegable(permission) || permission == "admin.manage" {
			return apperr.Validation("invalid service permission")
		}
		if strings.HasPrefix(permission, "sim.truth.") {
			if err := Require(ctx, "sim.truth.grant"); err != nil {
				return err
			}
		}
	}
	if err := s.EnsureResource(ctx, resource); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if _, err = tx.Exec(ctx, `INSERT INTO principals(id,kind,name) VALUES($1,'service',$2)`, id, name); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO principal_grants(principal_id,workspace_id,environment_id,partition,run_id,permissions,source_ids) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, resource.WorkspaceID, resource.EnvironmentID, string(resource.Partition), resource.RunID, permissions, sourceIDs); err != nil {
		return err
	}
	actor, _, _ := From(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO authorization_changes(actor_id,subject_id,action) VALUES($1,$2,'principal.created')`, actor.ID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) IssueCredential(ctx context.Context, principalID string, r Resource, expires time.Time) (string, string, error) {
	if err := Require(ctx, "admin.manage"); err != nil {
		return "", "", err
	}
	if !expires.After(time.Now()) || expires.After(time.Now().Add(90*24*time.Hour)) {
		return "", "", apperr.Validation("credential lifetime must be positive and at most 90 days")
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(bytes)
	h := sha256.Sum256([]byte(token))
	id := "key_" + hex.EncodeToString(h[:12])
	var granted bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM principal_grants WHERE principal_id=$1 AND workspace_id=$2 AND environment_id=$3 AND partition=$4 AND run_id IN ($5,'*'))`, principalID, r.WorkspaceID, r.EnvironmentID, string(r.Partition), r.RunID).Scan(&granted)
	if err != nil {
		return "", "", err
	}
	if !granted {
		return "", "", apperr.Forbidden("credential default resource is not granted")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if _, err = tx.Exec(ctx, `INSERT INTO principal_credentials(id,principal_id,token_hash,default_scope_id,expires_at) VALUES($1,$2,$3,$4,$5)`, id, principalID, h[:], r.ID(), expires); err != nil {
		return "", "", err
	}
	actor, _, _ := From(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO authorization_changes(actor_id,subject_id,action) VALUES($1,$2,'credential.issued')`, actor.ID, id); err != nil {
		return "", "", err
	}
	return id, token, tx.Commit(ctx)
}

func (s *Store) RevokeCredential(ctx context.Context, id string) error {
	if err := Require(ctx, "admin.manage"); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if _, err = tx.Exec(ctx, `UPDATE principal_credentials SET revoked_at=COALESCE(revoked_at,now()) WHERE id=$1`, id); err != nil {
		return err
	}
	actor, _, _ := From(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO authorization_changes(actor_id,subject_id,action) VALUES($1,$2,'credential.revoked')`, actor.ID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ObservationResources(ctx context.Context) ([]Resource, error) {
	if err := Require(ctx, "platform.maintenance"); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT workspace_id,environment_id,partition,run_id FROM data_scopes WHERE partition IN ('PUBLIC_EVIDENCE','EXERCISE_OBSERVATION') ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Resource
	for rows.Next() {
		var r Resource
		if err = rows.Scan(&r.WorkspaceID, &r.EnvironmentID, &r.Partition, &r.RunID); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
