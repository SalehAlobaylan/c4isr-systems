-- +goose Up
CREATE TABLE investigations (
 id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 owner_human_id text NOT NULL, members text[] NOT NULL, membership_version integer NOT NULL DEFAULT 1 CHECK(membership_version>0), head_revision_id text NOT NULL,
 required_sources jsonb NOT NULL DEFAULT '[]',
 PRIMARY KEY(id,scope_id), CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE'),
 CHECK(owner_human_id=ANY(members)), CHECK(cardinality(members) BETWEEN 1 AND 32), CHECK(jsonb_typeof(required_sources)='array')
);
CREATE TABLE investigation_revisions (
 id text NOT NULL, case_id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 version integer NOT NULL CHECK(version>0), document jsonb NOT NULL,
 PRIMARY KEY(id,scope_id), UNIQUE(case_id,scope_id,version),
 FOREIGN KEY(case_id,scope_id) REFERENCES investigations(id,scope_id)
);
ALTER TABLE investigations ADD CONSTRAINT investigation_head FOREIGN KEY(head_revision_id,scope_id) REFERENCES investigation_revisions(id,scope_id) DEFERRABLE INITIALLY DEFERRED;
CREATE TABLE investigation_membership_events (
 id text PRIMARY KEY, case_id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''), document jsonb NOT NULL,
 FOREIGN KEY(case_id,scope_id) REFERENCES investigations(id,scope_id)
);
-- Only membership plus live source/domain permission grants visibility. Source
-- dependencies grow over retained history; removing a reference cannot declassify prose.
ALTER TABLE investigations ENABLE ROW LEVEL SECURITY;
ALTER TABLE investigations FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT,UPDATE ON investigations TO emad_data;
CREATE POLICY investigation_members ON investigations TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND NULLIF(current_setting('emad.human_id',true),'')=ANY(members)
 AND current_setting('emad.case_read',true)='true' AND emad_scope_sources_visible(required_sources))
 WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND NULLIF(current_setting('emad.human_id',true),'')=ANY(members)
 AND current_setting('emad.case_read',true)='true' AND emad_scope_sources_visible(required_sources));
-- +goose StatementBegin
CREATE FUNCTION emad_investigation_visible(case_key text, resource_key text) RETURNS boolean
 LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog,public SET row_security=off AS $$
 SELECT resource_key=NULLIF(current_setting('emad.scope_id',true),'') AND current_setting('emad.case_read',true)='true'
 AND EXISTS(SELECT 1 FROM public.investigations c WHERE c.id=case_key AND c.scope_id=resource_key
 AND NULLIF(current_setting('emad.human_id',true),'')=ANY(c.members) AND public.emad_scope_sources_visible(c.required_sources))
$$;
REVOKE ALL ON FUNCTION emad_investigation_visible(text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION emad_investigation_visible(text,text) TO emad_data;
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['investigation_revisions','investigation_membership_events'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format('CREATE POLICY %I ON %I TO emad_data USING(emad_investigation_visible(case_id,scope_id)) WITH CHECK(emad_investigation_visible(case_id,scope_id))',t||'_members',t);
  EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence()',t||'_immutable',t);
 END LOOP;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'investigation history requires forward recovery'; END $$;
-- +goose StatementEnd
