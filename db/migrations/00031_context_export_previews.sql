-- +goose Up
CREATE TABLE context_export_previews(
 id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 creator text NOT NULL,sha256 text NOT NULL,document jsonb NOT NULL,bundle bytea NOT NULL CHECK(octet_length(bundle)<=12582912),expires_at timestamptz NOT NULL,
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
ALTER TABLE context_export_previews ENABLE ROW LEVEL SECURITY;
ALTER TABLE context_export_previews FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT,DELETE ON context_export_previews TO emad_data;
CREATE POLICY context_export_preview_scope ON context_export_previews TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE POLICY context_export_preview_sources ON context_export_previews AS RESTRICTIVE TO emad_data USING(
 COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR NOT EXISTS(SELECT 1 FROM jsonb_array_elements(document->'assets') a WHERE NOT(a->>'sourceId' IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb))))
) WITH CHECK(
 COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR NOT EXISTS(SELECT 1 FROM jsonb_array_elements(document->'assets') a WHERE NOT(a->>'sourceId' IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb))))
);
-- +goose Down
DROP TABLE context_export_previews;
