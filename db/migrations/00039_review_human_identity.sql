-- +goose Up
-- Preserve immutable revision bytes. This append-only projection also resolves
-- historical credentials whose registered humans are disabled today.
CREATE TABLE assessment_review_identities (
 revision_id text PRIMARY KEY REFERENCES assessment_revisions(id),
 scope_id text NOT NULL, source_id text NOT NULL,
 author_human_id text, actor_human_id text, publisher_human_id text,
 resolved_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(source_id,scope_id) REFERENCES sources(id,scope_id)
);
INSERT INTO assessment_review_identities(revision_id,scope_id,source_id,author_human_id,actor_human_id,publisher_human_id)
SELECT r.id,r.scope_id,r.source_id,a.operator_id,c.operator_id,p.operator_id
FROM assessment_revisions r
LEFT JOIN principals a ON a.id=r.document->>'authorId' AND a.kind='human'
LEFT JOIN principals c ON c.id=r.document->>'actorId' AND c.kind='human'
LEFT JOIN principals p ON p.id=r.document->>'publishedBy' AND p.kind='human';
ALTER TABLE assessment_review_identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment_review_identities FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON assessment_review_identities TO emad_data;
CREATE POLICY assessment_identity_scope ON assessment_review_identities TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),''))
 WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE POLICY assessment_identity_source ON assessment_review_identities AS RESTRICTIVE TO emad_data
 USING(COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)))
 WITH CHECK(COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)));
CREATE TRIGGER assessment_identity_immutable BEFORE UPDATE OR DELETE ON assessment_review_identities FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- A credential may be revoked; its registered human may never be reassigned.
-- +goose StatementBegin
CREATE FUNCTION emad_principal_person_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.kind IS DISTINCT FROM OLD.kind OR NEW.operator_id IS DISTINCT FROM OLD.operator_id THEN
  RAISE EXCEPTION 'principal registered identity is immutable';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER principal_person_immutable BEFORE UPDATE ON principals FOR EACH ROW EXECUTE FUNCTION emad_principal_person_immutable();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'review identity history requires forward recovery'; END $$;
-- +goose StatementEnd
