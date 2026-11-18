-- +goose Up
-- Immutable, release-scoped physical-accuracy assessments. A correction adds a
-- new row that names the record it supersedes; the original stays readable so
-- an earlier suitability decision can still be explained.
CREATE TABLE geographic_environment_assessments (
 id text NOT NULL,
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 release_id text NOT NULL,
 release_manifest_sha256 text NOT NULL CHECK(release_manifest_sha256 ~ '^[0-9a-f]{64}$'),
 member_sha256 text NOT NULL CHECK(member_sha256 ~ '^[0-9a-f]{64}$'),
 layer text NOT NULL CHECK(layer ~ '^[a-z0-9][a-z0-9-]{0,63}$'),
 use_key text NOT NULL CHECK(use_key ~ '^[A-Za-z0-9_.:-]{1,128}$'),
 method text NOT NULL CHECK(length(method) BETWEEN 1 AND 1000),
 control_set_sha256 text NOT NULL CHECK(control_set_sha256 ~ '^[0-9a-f]{64}$'),
 statistic_m double precision NOT NULL CHECK(statistic_m >= 0 AND statistic_m <= 100000),
 uncertainty_m double precision NOT NULL CHECK(uncertainty_m >= 0 AND uncertainty_m <= 100000),
 area jsonb NOT NULL CHECK(jsonb_typeof(area)='object' AND octet_length(area::text)<=131072),
 decision text NOT NULL CHECK(decision IN('ACCEPTED','REJECTED')),
 sequence integer NOT NULL CHECK(sequence BETWEEN 1 AND 1000000),
 supersedes_id text,
 reviewer_human_id text NOT NULL,
 note text NOT NULL CHECK(length(note) BETWEEN 1 AND 2000),
 evidence_sha256 text NOT NULL CHECK(evidence_sha256 ~ '^[0-9a-f]{64}$'),
 required_sources jsonb NOT NULL,
 reviewed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(id,scope_id),
 UNIQUE(scope_id,release_id,layer,use_key,sequence),
 FOREIGN KEY(release_id,scope_id) REFERENCES geographic_environment_releases(id,scope_id),
 FOREIGN KEY(supersedes_id,scope_id) REFERENCES geographic_environment_assessments(id,scope_id)
);
CREATE INDEX geographic_environment_assessment_release
 ON geographic_environment_assessments(scope_id,release_id,reviewed_at,id);
ALTER TABLE geographic_environment_assessments ENABLE ROW LEVEL SECURITY;
ALTER TABLE geographic_environment_assessments FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON geographic_environment_assessments TO emad_data;
CREATE POLICY geographic_scope ON geographic_environment_assessments TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'')
  AND split_part(scope_id,'/',3)='PUBLIC_EVIDENCE' AND emad_scope_sources_visible(required_sources));
CREATE TRIGGER geographic_immutable BEFORE UPDATE OR DELETE ON geographic_environment_assessments
 FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'geographic assessment history requires forward recovery'; END $$;
-- +goose StatementEnd
