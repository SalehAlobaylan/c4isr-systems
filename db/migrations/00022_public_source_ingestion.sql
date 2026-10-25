-- +goose Up
CREATE UNIQUE INDEX sources_id_scope_unique ON sources(id,scope_id);

CREATE TABLE source_admissions (
 id text PRIMARY KEY, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),'') REFERENCES data_scopes(id),
 source_id text NOT NULL, version integer NOT NULL CHECK(version>0), document jsonb NOT NULL,
 UNIQUE(id,source_id,scope_id), UNIQUE(scope_id,source_id,version),
 FOREIGN KEY(source_id,scope_id) REFERENCES sources(id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE TABLE source_connector_state (
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),'') REFERENCES data_scopes(id), source_id text NOT NULL,
 admission_id text NOT NULL, checkpoint text NOT NULL DEFAULT '', next_attempt_at timestamptz NOT NULL DEFAULT 'epoch',
 last_attempt_at timestamptz, last_success_at timestamptz, last_failure_code text NOT NULL DEFAULT '', failures integer NOT NULL DEFAULT 0,
 lease_id text, lease_until timestamptz, quota_day date, requests_today integer NOT NULL DEFAULT 0,
 coverage_from timestamptz, coverage_to timestamptz, query_from timestamptz, query_to timestamptz,
 PRIMARY KEY(scope_id,source_id), FOREIGN KEY(admission_id,source_id,scope_id) REFERENCES source_admissions(id,source_id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE TABLE connector_runs (
 id text PRIMARY KEY, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),'') REFERENCES data_scopes(id),
 source_id text NOT NULL, admission_id text NOT NULL, document jsonb NOT NULL,
 UNIQUE(id,source_id,scope_id), FOREIGN KEY(admission_id,source_id,scope_id) REFERENCES source_admissions(id,source_id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE INDEX connector_runs_source_idx ON connector_runs(scope_id,source_id,id DESC);
CREATE TABLE public_evidence (
 id text PRIMARY KEY, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),'') REFERENCES data_scopes(id),
 source_id text NOT NULL, admission_id text NOT NULL, connector_run_id text NOT NULL,
 provider_record_id text NOT NULL, provider_revision text NOT NULL, content_hash text NOT NULL,
 document jsonb NOT NULL, original_record bytea,
 UNIQUE(id,source_id,scope_id), UNIQUE(scope_id,source_id,provider_record_id,provider_revision),
 FOREIGN KEY(admission_id,source_id,scope_id) REFERENCES source_admissions(id,source_id,scope_id),
 FOREIGN KEY(connector_run_id,source_id,scope_id) REFERENCES connector_runs(id,source_id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE'), CHECK(octet_length(original_record)<=262144)
);
CREATE TABLE public_evidence_outbox (
 evidence_id text PRIMARY KEY, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''), source_id text NOT NULL,
 attempts integer NOT NULL DEFAULT 0, delivered_at timestamptz, next_attempt_at timestamptz NOT NULL DEFAULT 'epoch',
 FOREIGN KEY(evidence_id,source_id,scope_id) REFERENCES public_evidence(id,source_id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE INDEX public_evidence_outbox_pending_idx ON public_evidence_outbox(scope_id,source_id,next_attempt_at) WHERE delivered_at IS NULL;
-- A searchable revision catalog is a recoverable projection of immutable
-- evidence. It makes no claim that the provider's record is an observed track.
CREATE TABLE public_evidence_catalog (
 evidence_id text PRIMARY KEY, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''), source_id text NOT NULL,
 published_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(evidence_id,source_id,scope_id) REFERENCES public_evidence(id,source_id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE INDEX public_evidence_catalog_source_idx ON public_evidence_catalog(scope_id,source_id,published_at DESC,evidence_id);

-- +goose StatementBegin
CREATE FUNCTION emad_immutable_public_evidence() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'retained admission/evidence is immutable' USING ERRCODE='23514'; END $$;
CREATE TRIGGER source_admissions_immutable BEFORE UPDATE OR DELETE ON source_admissions FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
CREATE TRIGGER public_evidence_immutable BEFORE UPDATE OR DELETE ON public_evidence FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
DO $$
DECLARE t text;
BEGIN
 FOREACH t IN ARRAY ARRAY['source_admissions','source_connector_state','connector_runs','public_evidence','public_evidence_outbox','public_evidence_catalog'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT,UPDATE ON %I TO emad_data',t);
  EXECUTE format('CREATE POLICY %I ON %I TO emad_data USING (scope_id=NULLIF(current_setting(''emad.scope_id'',true),'''')) WITH CHECK (scope_id=NULLIF(current_setting(''emad.scope_id'',true),''''))',t||'_scope',t);
  EXECUTE format('CREATE POLICY %I ON %I AS RESTRICTIVE TO emad_data USING (COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN (SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb))) WITH CHECK (COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN (SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb)))',t||'_source',t);
 END LOOP;
END $$;
-- +goose StatementEnd

REVOKE UPDATE ON source_admissions,public_evidence,public_evidence_catalog FROM emad_data;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'retained public evidence requires an explicit forward recovery plan'; END $$;
-- +goose StatementEnd
