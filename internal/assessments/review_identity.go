package assessments

import (
	"context"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/jackc/pgx/v5"
)

type identityQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func ResolveReviewIdentity(ctx context.Context, q identityQuery, v *ReviewRevision) error {
	err := q.QueryRow(ctx, `SELECT COALESCE(author_human_id,''),COALESCE(actor_human_id,''),COALESCE(publisher_human_id,'') FROM assessment_review_identities WHERE revision_id=$1`, v.ID).Scan(&v.AuthorHumanID, &v.ActorHumanID, &v.PublisherHumanID)
	v.ReviewIdentityStatus = "UNRESOLVED"
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}
	if v.AuthorHumanID == "" || v.ActorHumanID == "" {
		return nil
	}
	v.ReviewIdentityStatus = "PENDING"
	if v.Status == "IN_REVIEW" || v.Status == "REJECTED" || v.Status == "PUBLISHED" {
		v.ReviewIdentityStatus = "SAME_HUMAN"
		if v.ActorHumanID != v.AuthorHumanID {
			v.ReviewIdentityStatus = "DISTINCT_REGISTERED_HUMANS"
		}
		if v.Status == "PUBLISHED" && (v.PublisherHumanID == "" || v.PublisherHumanID != v.ActorHumanID) {
			v.ReviewIdentityStatus = "UNRESOLVED"
		}
	}
	return nil
}

// VerifyPublishedIdentity also covers immutable historical publications and
// already-created export previews; a new Act check alone is insufficient.
func VerifyPublishedIdentity(ctx context.Context, q identityQuery, revisionID string) error {
	var valid bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM assessment_revisions r JOIN assessment_review_identities i ON i.revision_id=r.id AND i.scope_id=r.scope_id WHERE r.id=$1 AND r.status='PUBLISHED' AND i.author_human_id IS NOT NULL AND i.actor_human_id=i.publisher_human_id AND i.author_human_id<>i.publisher_human_id)`, revisionID).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return apperr.Conflict("published review has unresolved or non-independent registered human identities; create an independently reviewed revision")
	}
	return nil
}
