package operators

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

const (
	createOperatorSQL = `INSERT INTO operators (id, name, role, created_at)
VALUES ($1, $2, $3, now())
RETURNING id, name, role, created_at`
	getOperatorSQL    = `SELECT id, name, role, created_at FROM operators WHERE id = $1`
	listOperatorsSQL  = `SELECT id, name, role, created_at FROM operators ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2`
	operatorExistsSQL = `SELECT EXISTS (SELECT 1 FROM operators WHERE id = $1)`
)

// PostgresRepository stores operators in PostgreSQL. No sqlc queries were
// generated for operators, so this module owns its parameterized SQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts an operator.
func (r *PostgresRepository) Create(ctx context.Context, operator Operator) (Operator, error) {
	var row dbgen.Operator
	err := r.pool.QueryRow(ctx, createOperatorSQL,
		operator.ID,
		operator.Name,
		string(operator.Role),
	).Scan(&row.ID, &row.Name, &row.Role, &row.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Operator{}, apperr.Conflict("operator already exists")
		}
		return Operator{}, err
	}
	return toDomain(row), nil
}

// Get loads an operator by id.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Operator, error) {
	var row dbgen.Operator
	err := r.pool.QueryRow(ctx, getOperatorSQL, id).Scan(&row.ID, &row.Name, &row.Role, &row.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Operator{}, apperr.NotFound("operator", id)
		}
		return Operator{}, err
	}
	return toDomain(row), nil
}

// List returns operators newest first. The total is the size of the returned
// page because no count query exists in the generated query set.
func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]Operator, int, error) {
	rows, err := r.pool.Query(ctx, listOperatorsSQL, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Operator, 0)
	for rows.Next() {
		var row dbgen.Operator
		if err := rows.Scan(&row.ID, &row.Name, &row.Role, &row.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, toDomain(row))
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, len(out), nil
}

// Exists reports whether an operator id is registered.
func (r *PostgresRepository) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, operatorExistsSQL, id).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func toDomain(row dbgen.Operator) Operator {
	return Operator{
		ID:        row.ID,
		Name:      row.Name,
		Role:      Role(row.Role),
		CreatedAt: pgconv.Time(row.CreatedAt),
	}
}

var _ Repository = (*PostgresRepository)(nil)
