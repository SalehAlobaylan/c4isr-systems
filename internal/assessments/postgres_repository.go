package assessments

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/SalehAlobaylan/c4isr-systems/internal/dbgen"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/pgconv"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository stores assessments in PostgreSQL.
type PostgresRepository struct {
	q    *dbgen.Queries
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a repository over the connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{q: dbgen.New(platformdb.Scoped(pool)), pool: pool}
}

// Create inserts an assessment and its evidence links.
func (r *PostgresRepository) Create(ctx context.Context, assessment Assessment) (Assessment, error) {
	var created Assessment
	err := platformdb.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		row, err := q.CreateAssessment(ctx, dbgen.CreateAssessmentParams{
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
			return err
		}

		created = toDomain(row)
		created.Evidence = make([]Evidence, 0, len(assessment.Evidence))
		for _, e := range assessment.Evidence {
			if err := q.AddAssessmentEvidence(ctx, dbgen.AddAssessmentEvidenceParams{
				AssessmentID: assessment.ID,
				EvidenceType: string(e.Type),
				EvidenceID:   e.ID,
			}); err != nil {
				return err
			}
			created.Evidence = append(created.Evidence, e)
		}
		return nil
	})
	return created, err
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
	if err := r.reviewProjection(ctx, &out); err != nil {
		return Assessment{}, err
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
		item := toDomain(row)
		if err := r.reviewProjection(ctx, &item); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
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

func toEvidence(row dbgen.ListAssessmentEvidenceRow) Evidence {
	return Evidence{
		Type:    EvidenceType(row.EvidenceType),
		ID:      row.EvidenceID,
		AddedAt: pgconv.Time(row.AddedAt),
	}
}

var _ Repository = (*PostgresRepository)(nil)

func (r *PostgresRepository) reviewProjection(ctx context.Context, a *Assessment) error {
	var raw []byte
	var published *string
	err := platformdb.Scoped(r.pool).QueryRow(ctx, `SELECT v.document,h.published_id FROM assessment_review_heads h JOIN assessment_revisions v ON v.id=h.latest_id AND v.scope_id=h.scope_id WHERE h.assessment_id=$1`, a.ID).Scan(&raw, &published)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var v ReviewRevision
	if err = json.Unmarshal(raw, &v); err != nil {
		return err
	}
	a.Conclusion = v.Conclusion
	a.Confidence = v.Confidence
	a.Method = v.Method
	a.ReviewStatus = v.Status
	a.RevisionID = v.ID
	if published != nil {
		a.PublishedRevisionID = *published
	}
	return nil
}
