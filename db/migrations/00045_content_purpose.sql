-- +goose Up
CREATE TABLE content_purpose_revisions (
 id text PRIMARY KEY,
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 entity_kind text NOT NULL CHECK(entity_kind IN ('reference','scene','capture','job','assessment','package')),
 entity_id text NOT NULL, source_id text NOT NULL, version integer NOT NULL CHECK(version>0),
 purpose text NOT NULL CHECK(purpose IN ('working','development','acceptance','unclassified')),
 reason text NOT NULL CHECK(length(reason) BETWEEN 10 AND 2000),
 receipt_reference text NOT NULL CHECK(length(receipt_reference) BETWEEN 1 AND 1024),
 actor_id text NOT NULL, recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(scope_id,entity_kind,entity_id,version),
 FOREIGN KEY(source_id,scope_id) REFERENCES sources(id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
ALTER TABLE content_purpose_revisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE content_purpose_revisions FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON content_purpose_revisions TO emad_data;
CREATE POLICY content_purpose_scope ON content_purpose_revisions TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),''))
 WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE POLICY content_purpose_source ON content_purpose_revisions AS RESTRICTIVE TO emad_data
 USING(COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)))
 WITH CHECK(COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)));
CREATE TRIGGER content_purpose_immutable BEFORE UPDATE OR DELETE ON content_purpose_revisions
 FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose StatementBegin
CREATE FUNCTION emad_content_purpose(kind text, entity text) RETURNS text
 LANGUAGE sql STABLE SECURITY INVOKER AS $$
 SELECT COALESCE((SELECT purpose FROM content_purpose_revisions
 WHERE scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND entity_kind=kind AND entity_id=entity
 ORDER BY version DESC LIMIT 1),'unclassified')
$$;
CREATE FUNCTION emad_content_visible(kind text, entity text) RETURNS boolean
 LANGUAGE sql STABLE SECURITY INVOKER AS $$
 SELECT CASE COALESCE(NULLIF(current_setting('emad.content_view',true),''),'ordinary')
 WHEN 'all' THEN true
 WHEN 'unclassified' THEN emad_content_purpose(kind,entity)='unclassified'
 WHEN 'working' THEN emad_content_purpose(kind,entity)='working'
 ELSE emad_content_purpose(kind,entity) IN ('working','unclassified') END
$$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'content-purpose history requires forward recovery'; END $$;
-- +goose StatementEnd
