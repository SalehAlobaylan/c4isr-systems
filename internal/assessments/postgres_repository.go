package assessments

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
)

// PostgresRepository stores assessments in PostgreSQL.
type PostgresRepository struct {
	q *dbgen.Queries
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{q: dbgen.New(pool)}
}

// Create inserts an assessment and its evidence links.
func (r *PostgresRepository) Create(ctx context.Context, assessment Assessment) (Assessment, error) {
	row, err := r.q.CreateAssessment(ctx, dbgen.CreateAssessmentParams{
		ID:          assessment.ID,
		SubjectType: string(assessment.SubjectType),
		SubjectID:   assessment.SubjectID,
		Type:        assessment.Type,
		Conclusion:  assessment.Conclusion,
		Confidence:  assessment.Confidence,
		Method:      string(assessment.Method),
		CreatedBy:   pgconv.TextPtr(assessment.CreatedBy),
	})
	if err != nil {
		return Assessment{}, err
	}

	created := toDomain(row)
	created.Evidence = make([]Evidence, 0, len(assessment.Evidence))
	for _, e := range assessment.Evidence {
		if err := r.q.AddAssessmentEvidence(ctx, dbgen.AddAssessmentEvidenceParams{
			AssessmentID: assessment.ID,
			EvidenceType: string(e.Type),
			EvidenceID:   e.ID,
		}); err != nil {
			return Assessment{}, err
		}
		created.Evidence = append(created.Evidence, e)
	}
	return created, nil
}

// Get loads an assessment and its evidence links by id.
func (r *PostgresRepository) Get(ctx context.Context, id string) (Assessment, error) {
	row, err := r.q.GetAssessment(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Assessment{}, apperr.NotFound("assessment", id)
		}
		return Assessment{}, err
	}

	evidence, err := r.q.ListAssessmentEvidence(ctx, id)
	if err != nil {
		return Assessment{}, err
	}
	out := toDomain(row)
	out.Evidence = make([]Evidence, 0, len(evidence))
	for _, e := range evidence {
		out.Evidence = append(out.Evidence, toEvidence(e))
	}
	return out, nil
}

// List returns assessments matching an optional subject, newest first, with a
// total count. Evidence links are not loaded for list results.
func (r *PostgresRepository) List(ctx context.Context, subjectType, subjectID string, limit, offset int) ([]Assessment, int, error) {
	rows, err := r.q.ListAssessments(ctx, dbgen.ListAssessmentsParams{
		SubjectType: pgconv.TextPtr(subjectType),
		SubjectID:   pgconv.TextPtr(subjectID),
		OffsetCount: int32(offset),
		LimitCount:  int32(limit),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := r.q.CountAssessments(ctx, dbgen.CountAssessmentsParams{
		SubjectType: pgconv.TextPtr(subjectType),
		SubjectID:   pgconv.TextPtr(subjectID),
	})
	if err != nil {
		return nil, 0, err
	}

	out := make([]Assessment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, int(total), nil
}

func toDomain(row dbgen.Assessment) Assessment {
	return Assessment{
		ID:          row.ID,
		SubjectType: SubjectType(row.SubjectType),
		SubjectID:   row.SubjectID,
		Type:        row.Type,
		Conclusion:  row.Conclusion,
		Confidence:  row.Confidence,
		Method:      Method(row.Method),
		CreatedBy:   pgconv.Deref(row.CreatedBy),
		CreatedAt:   pgconv.Time(row.CreatedAt),
		Evidence:    []Evidence{},
	}
}

func toEvidence(row dbgen.AssessmentEvidence) Evidence {
	return Evidence{
		Type:    EvidenceType(row.EvidenceType),
		ID:      row.EvidenceID,
		AddedAt: pgconv.Time(row.AddedAt),
	}
}

var _ Repository = (*PostgresRepository)(nil)
