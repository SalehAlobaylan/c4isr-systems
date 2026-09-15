-- name: CreateAssessment :one
INSERT INTO assessments (
    id, subject_type, subject_id, type, conclusion, confidence, method, created_by, created_at
) VALUES (
    @id, @subject_type, @subject_id, @type, @conclusion, @confidence, @method, @created_by, now()
)
RETURNING *;

-- name: GetAssessment :one
SELECT * FROM assessments WHERE id = @id;

-- name: ListAssessments :many
SELECT * FROM assessments
WHERE (sqlc.narg('subject_type')::text IS NULL OR subject_type = sqlc.narg('subject_type'))
  AND (sqlc.narg('subject_id')::text IS NULL OR subject_id = sqlc.narg('subject_id'))
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountAssessments :one
SELECT count(*) FROM assessments
WHERE (sqlc.narg('subject_type')::text IS NULL OR subject_type = sqlc.narg('subject_type'))
  AND (sqlc.narg('subject_id')::text IS NULL OR subject_id = sqlc.narg('subject_id'));

-- name: AddAssessmentEvidence :exec
INSERT INTO assessment_evidence (assessment_id, evidence_type, evidence_id)
VALUES (@assessment_id, @evidence_type, @evidence_id)
ON CONFLICT DO NOTHING;

-- name: ListAssessmentEvidence :many
SELECT assessment_id, evidence_type, evidence_id, added_at
FROM assessment_evidence
WHERE assessment_id = @assessment_id
ORDER BY added_at ASC;
