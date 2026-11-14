-- +goose Up
ALTER TABLE geographic_assets
 ADD COLUMN purge_retry_at timestamptz,
 ADD COLUMN purge_attempts integer NOT NULL DEFAULT 0 CHECK (purge_attempts BETWEEN 0 AND 30);

CREATE INDEX geographic_assets_pending_purge
 ON geographic_assets(scope_id,retain_until,id) WHERE purged_at IS NULL;

GRANT UPDATE(purge_retry_at,purge_attempts) ON geographic_assets TO emad_data;

-- Owner quota applies across every source in the bound scope. Return only a
-- boolean so reservation callers never learn the hidden owners or object sizes.
-- +goose StatementBegin
CREATE FUNCTION emad_geographic_owner_capacity(owner text, additional bigint) RETURNS boolean
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog,public SET row_security=off AS $$
 SELECT COALESCE(split_part(current_setting('emad.scope_id',true),'/',3)='PUBLIC_EVIDENCE'
   AND owner=current_setting('emad.principal_id',true)
   AND additional BETWEEN 1 AND 1073741824
   AND (SELECT COALESCE(sum(size_bytes),0) FROM public.geographic_assets
        WHERE scope_id=current_setting('emad.scope_id',true) AND owner_id=owner AND purged_at IS NULL)
        + additional <= 17179869184,false);
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION emad_geographic_owner_capacity(text,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION emad_geographic_owner_capacity(text,bigint) TO emad_data;

-- +goose Down
DROP FUNCTION emad_geographic_owner_capacity(text,bigint);
DROP INDEX geographic_assets_pending_purge;
ALTER TABLE geographic_assets DROP COLUMN purge_attempts, DROP COLUMN purge_retry_at;
