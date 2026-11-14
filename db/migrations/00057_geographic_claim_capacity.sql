-- +goose Up
-- Claims are serialized for the whole scope. This predicate counts leases
-- across all source grants without returning any job, source or owner identity.
-- +goose StatementBegin
CREATE FUNCTION emad_environment_claim_capacity(capacity integer) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog,public SET row_security=off AS $$
 SELECT COALESCE(
   current_setting('emad.scope_id',true) <> ''
   AND split_part(current_setting('emad.scope_id',true),'/',3)='PUBLIC_EVIDENCE'
   AND capacity BETWEEN 1 AND 2
   AND (SELECT count(*) FROM public.geographic_environment_builds
        WHERE scope_id=current_setting('emad.scope_id',true) AND state='RUNNING' AND lease_until>statement_timestamp()) < capacity,
   false);
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION emad_environment_claim_capacity(integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION emad_environment_claim_capacity(integer) TO emad_data;

-- +goose Down
DROP FUNCTION emad_environment_claim_capacity(integer);
