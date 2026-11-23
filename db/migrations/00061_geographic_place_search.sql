-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION emad_geographic_search_ar_v1(value text) RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
 SELECT emad_geographic_search(translate(normalize(value, NFC),
   U&'\0623\0625\0622\0671\0640\064B\064C\064D\064E\064F\0650\0651\0652\0670',
   U&'\0627\0627\0627\0627'))
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION emad_geographic_match_v1(name text, aliases text, query text) RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
 SELECT CASE
   WHEN query = '' THEN NULL
   WHEN normalize(name, NFC) = normalize(query, NFC) THEN 'exact'
   WHEN EXISTS (SELECT 1 FROM regexp_split_to_table(aliases, '\s*\|\s*') AS alias(value)
       WHERE normalize(alias.value, NFC) = normalize(query, NFC)) THEN 'alias'
   WHEN emad_geographic_search_ar_v1(query) <> ''
     AND strpos(emad_geographic_search_ar_v1(name || ' ' || aliases), emad_geographic_search_ar_v1(query)) > 0
     THEN 'folded'
 END
$$;
-- +goose StatementEnd

CREATE POLICY geographic_editions_unexpired ON geographic_editions AS RESTRICTIVE
 FOR SELECT TO emad_data USING (
   COALESCE((document#>>'{rights,expiresAt}')::timestamptz > clock_timestamp(), true)
 );
CREATE POLICY geographic_regions_unexpired ON geographic_regions AS RESTRICTIVE
 FOR SELECT TO emad_data USING (
   EXISTS (SELECT 1 FROM geographic_editions e
           WHERE e.id = geographic_regions.edition_id AND e.scope_id = geographic_regions.scope_id)
 );
CREATE POLICY reference_features_unexpired ON reference_features AS RESTRICTIVE
 FOR SELECT TO emad_data USING (
   EXISTS (SELECT 1 FROM reference_datasets d WHERE d.id = reference_features.dataset_id
           AND d.scope_id = reference_features.scope_id
           AND COALESCE((d.document#>>'{rights,expiresAt}')::timestamptz > clock_timestamp(), true))
 );

-- +goose Down
DROP POLICY reference_features_unexpired ON reference_features;
DROP POLICY geographic_regions_unexpired ON geographic_regions;
DROP POLICY geographic_editions_unexpired ON geographic_editions;
DROP FUNCTION emad_geographic_match_v1(text, text, text);
DROP FUNCTION emad_geographic_search_ar_v1(text);
