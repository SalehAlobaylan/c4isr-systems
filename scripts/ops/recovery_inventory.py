"""Private, read-only database inventory for paired recovery (PostgreSQL 17).

Hashes cover every ordinary public table, including ACLs, identity, release heads,
leases, intents, upload plans and tombstones. This is not an authorization bypass
for application reads: only the existing maintenance database owner runs it.
"""
import json
import subprocess
from pathlib import Path


def sql(container, database, statement):
    result = subprocess.run(
        ['docker', 'exec', '-i', container, 'psql', '-X', '-qAt', '-U', 'c4isr',
         '-d', database, '-v', 'ON_ERROR_STOP=1'],
        input="SET TIME ZONE 'UTC'; SET extra_float_digits=3;\n" + statement,
        capture_output=True, text=True, encoding='utf-8', check=True)
    return result.stdout.strip()


def inventory(container, database='c4isr'):
    tables = json.loads(sql(container, database, """
      SELECT json_agg(tablename ORDER BY tablename) FROM pg_tables
      WHERE schemaname='public' AND tablename NOT IN ('spatial_ref_sys');
    """) or '[]')
    result = {'schema': 'emad.recovery-inventory.v1', 'tables': {}, 'objects': []}
    # Definition/grant hashes detect a restore that retained rows but lost RLS,
    # immutability triggers or the canonical-human access functions.
    result['securityDefinitions'] = json.loads(sql(container, database, """
      WITH definitions AS (
        SELECT 'policy' kind,to_jsonb(p)::text value FROM pg_policies p WHERE schemaname='public'
        UNION ALL SELECT 'grant',(to_jsonb(g)-'table_catalog')::text FROM information_schema.role_table_grants g WHERE table_schema='public'
        UNION ALL SELECT 'function',pg_get_functiondef(p.oid) FROM pg_proc p
          JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND p.proname LIKE 'emad_%'
        UNION ALL SELECT 'trigger',pg_get_triggerdef(t.oid) FROM pg_trigger t
          JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
          WHERE n.nspname='public' AND NOT t.tgisinternal
      ) SELECT json_build_object('count',count(*),'sha256',
        encode(sha256(convert_to(COALESCE(string_agg(kind||value,'' ORDER BY kind,value),''),'UTF8')),'hex')) FROM definitions;
    """))
    statements = []
    for table in tables:
        quoted = '"' + table.replace('"', '""') + '"'
        literal = "'" + table.replace("'", "''") + "'"
        statements.append(f"""
          SELECT json_build_object('table',{literal},'rows',count(*),'sha256',
            encode(sha256(convert_to(COALESCE(string_agg(h,'' ORDER BY h),''),'UTF8')),'hex'))
          FROM (SELECT encode(sha256(convert_to(to_jsonb(t)::text,'UTF8')),'hex') h
                FROM public.{quoted} t) q;
        """)
    raw = sql(container, database, 'BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;\n' + '\n'.join(statements) + '\nCOMMIT;')
    for line in raw.splitlines():
        row = json.loads(line)
        result['tables'][row.pop('table')] = row
    families = []
    if 'artifacts' in tables:
        families.append("""SELECT object_key key, id entity_id, sha256, size_bytes bytes, 'artifact' family,
          state, retain_until, state='READY' AND retain_until>clock_timestamp() required
          FROM artifacts WHERE object_key<>''""")
    if 'composite_outputs' in tables:
        families.append("""SELECT object_key key, id entity_id, render_sha256 sha256, size_bytes bytes,
          'composite' family, state, retain_until,
          state='READY' AND retain_until>clock_timestamp() required
          FROM composite_outputs WHERE object_key<>''""")
        families.append("""SELECT e.document->>'objectKey' key, o.id entity_id,
          e.document->>'sha256' sha256, NULL::bigint bytes, 'planned-composite' family,
          o.state,o.retain_until,false required
          FROM composite_output_events e JOIN composite_outputs o
          ON (o.id,o.scope_id)=(e.output_id,e.scope_id)
          WHERE e.document->>'action'='UPLOAD_PLANNED'""")
    if 'geographic_assets' in tables:
        display_bindings = (
            "UNION ALL SELECT release_id,scope_id,asset_id FROM geographic_environment_displays"
            if 'geographic_environment_displays' in tables else ""
        )
        families.append("""SELECT object_key key, id entity_id, document->>'sha256' sha256,
          size_bytes bytes, 'geographic' family,
          CASE WHEN restore_allowed THEN state ELSE 'EXPIRED' END state, retain_until,
          restore_allowed AND state='READY' AND retain_until>clock_timestamp() required
          FROM (
            WITH admission_policy AS (
              SELECT old.id,old.scope_id,
                s.status='active' AND old.document->>'enabled'='true' AND live.document->>'enabled'='true'
                  AND old.document#>>'{rights,retainRaw}'='true' AND live.document#>>'{rights,retainRaw}'='true'
                  AND COALESCE((old.document#>>'{rights,expiresAt}')::timestamptz,'infinity')>clock_timestamp()
                  AND COALESCE((live.document#>>'{rights,expiresAt}')::timestamptz,'infinity')>clock_timestamp() can_retain,
                old.document#>>'{rights,allowAnalysis}'='true' AND live.document#>>'{rights,allowAnalysis}'='true'
                  AND old.document#>>'{rights,outputPermissions,geographicEnvironment}'='true'
                  AND live.document#>>'{rights,outputPermissions,geographicEnvironment}'='true' can_prepare
              FROM source_admissions old
              JOIN retrieval_profile_revisions r ON r.admission_id=old.id AND r.scope_id=old.scope_id
              JOIN retrieval_profiles p ON (p.scope_id,p.source_id,p.profile_key)=(r.scope_id,r.source_id,r.profile_key)
              JOIN retrieval_profile_revisions h ON h.id=p.head_revision_id
              JOIN source_admissions live ON live.id=h.admission_id AND live.scope_id=h.scope_id
              JOIN sources s ON (s.id,s.scope_id)=(old.source_id,old.scope_id)
            ), release_bindings AS (
              SELECT id release_id,scope_id,bundle_id asset_id FROM geographic_environment_releases
              /* display bindings */
            )
            SELECT a.*,a.state='READY' AND CASE
              WHEN a.document#>>'{representation,format}' NOT IN ('release-zip','display-glb','display-layer-json','display-pmtiles')
                THEN COALESCE(policy.can_retain,false)
              ELSE EXISTS(
                SELECT 1 FROM release_bindings binding
                JOIN geographic_environment_releases r ON (r.id,r.scope_id)=(binding.release_id,binding.scope_id)
                JOIN geographic_environment_builds b ON (b.id,b.scope_id)=(r.build_id,r.scope_id)
                JOIN geographic_assets bundle ON (bundle.id,bundle.scope_id)=(r.bundle_id,r.scope_id)
                WHERE (binding.asset_id,binding.scope_id)=(a.id,a.scope_id)
                  AND bundle.state='READY' AND bundle.retain_until>clock_timestamp()
                  AND COALESCE((SELECT state FROM geographic_environment_release_events
                    WHERE (release_id,scope_id)=(r.id,r.scope_id) ORDER BY occurred_at DESC,id DESC LIMIT 1),'EXPIRED')
                      NOT IN ('EXPIRED','WITHDRAWN','SUPERSEDED')
                  AND NOT EXISTS(
                    SELECT 1 FROM jsonb_array_elements(b.recipe->'inputs') input
                    LEFT JOIN geographic_assets parent ON parent.id=input->>'id' AND parent.scope_id=b.scope_id
                    LEFT JOIN admission_policy ap ON (ap.id,ap.scope_id)=(parent.admission_id,parent.scope_id)
                    WHERE NOT COALESCE(ap.can_retain AND ap.can_prepare AND parent.state='READY'
                                      AND parent.retain_until>clock_timestamp(),false)
                  )
              ) END AS restore_allowed
            FROM geographic_assets a
            LEFT JOIN admission_policy policy ON (policy.id,policy.scope_id)=(a.admission_id,a.scope_id)
          ) resolved_geographic_objects""".replace("/* display bindings */", display_bindings))
    if families:
        result['objects'] = json.loads(sql(container, database,
            'SELECT COALESCE(json_agg(r ORDER BY key,family),\'[]\') FROM (' +
            ' UNION ALL '.join(families) + ') r;'))
    if 'imagery_preparations' in tables:
        result['preparationArtifacts'] = json.loads(sql(container, database,
            "SELECT COALESCE(json_agg(r ORDER BY job_id),'[]') FROM (SELECT job_id,artifact_id FROM imagery_preparations) r;"))
    return result


def load_restore_inventory(container, database, directory, manifest):
    actual = inventory(container, database)
    persisted = 'inventory' in manifest
    if not persisted:
        # v1 archives still need current retention enforcement. Lacking a
        # pre-backup fingerprint must never mean lacking a restore policy.
        return actual, False
    expected = json.loads((Path(directory)/manifest['inventory']['file']).read_text(encoding='utf-8'))
    if actual['tables'] != expected['tables'] or actual['securityDefinitions'] != expected['securityDefinitions']:
        raise RuntimeError('Restored table identity/dependency/ACL inventory differs from the paired database')
    return expected, True


def restore_policy(snapshot, now):
    """Never resurrect expired/tombstoned bytes, even from a valid older backup.

    Unknown historical object families remain copied; their original domain
    checks continue. A key shared with a still-permitted required object is kept.
    Pending uploads remain optional and can be reconciled by forward recovery.
    """
    from datetime import datetime
    forbidden, required = set(), {}
    for row in snapshot.get('objects', []):
        until = datetime.fromisoformat(row['retain_until'].replace('Z', '+00:00'))
        if row['state'] in ('TOMBSTONED', 'DELETE_PENDING', 'DELETED', 'EXPIRED') or until <= now:
            forbidden.add(row['key'])
        elif row['required']:
            prior = required.get(row['key'])
            if prior and (prior['sha256'], prior['bytes']) != (row['sha256'], row['bytes']):
                raise RuntimeError('Conflicting required object identities')
            required[row['key']] = row
    return forbidden - required.keys(), required


def verify_required(snapshot, objects, now):
    omitted, required = restore_policy(snapshot, now)
    found = {o['key']: o for o in objects if o['key'] not in omitted}
    for key, row in required.items():
        actual = found.get(key)
        if not actual or actual['sha256'] != row['sha256'] or actual['bytes'] != row['bytes']:
            raise RuntimeError('Required retained object is missing or has changed')
    return omitted


def unavailable_inputs(snapshot, now):
    """Known policy-expired inputs are negative evidence, not missing backups."""
    artifacts = {row['entity_id'] for row in snapshot.get('objects', [])
        if row['family'] == 'artifact' and restore_policy({'objects': [row]}, now)[0]}
    preparations = {row['job_id'] for row in snapshot.get('preparationArtifacts', [])
                    if row['artifact_id'] in artifacts}
    return artifacts, preparations
