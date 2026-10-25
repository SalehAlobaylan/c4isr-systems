-- +goose Up
-- Ownership is nullable only for quarantined legacy rows. Application inserts
-- always use a validated transaction-local scope; RLS hides unresolved rows.
ALTER TABLE sources ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX sources_scope_idx ON sources(scope_id);
ALTER TABLE observations ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX observations_scope_idx ON observations(scope_id);
ALTER TABLE assets ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX assets_scope_idx ON assets(scope_id);
ALTER TABLE asset_state ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX asset_state_scope_idx ON asset_state(scope_id);
ALTER TABLE asset_telemetry ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX asset_telemetry_scope_idx ON asset_telemetry(scope_id);
ALTER TABLE tracks ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX tracks_scope_idx ON tracks(scope_id);
ALTER TABLE track_state ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX track_state_scope_idx ON track_state(scope_id);
ALTER TABLE track_observations ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX track_observations_scope_idx ON track_observations(scope_id);
ALTER TABLE track_history ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX track_history_scope_idx ON track_history(scope_id);
ALTER TABLE classifications ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX classifications_scope_idx ON classifications(scope_id);
ALTER TABLE geofences ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX geofences_scope_idx ON geofences(scope_id);
ALTER TABLE geofence_states ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX geofence_states_scope_idx ON geofence_states(scope_id);
ALTER TABLE alerts ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX alerts_scope_idx ON alerts(scope_id);
ALTER TABLE incidents ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX incidents_scope_idx ON incidents(scope_id);
ALTER TABLE incident_alerts ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX incident_alerts_scope_idx ON incident_alerts(scope_id);
ALTER TABLE incident_tracks ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX incident_tracks_scope_idx ON incident_tracks(scope_id);
ALTER TABLE incident_assets ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX incident_assets_scope_idx ON incident_assets(scope_id);
ALTER TABLE incident_observations ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX incident_observations_scope_idx ON incident_observations(scope_id);
ALTER TABLE missions ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX missions_scope_idx ON missions(scope_id);
ALTER TABLE mission_assets ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX mission_assets_scope_idx ON mission_assets(scope_id);
ALTER TABLE mission_tasks ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX mission_tasks_scope_idx ON mission_tasks(scope_id);
ALTER TABLE commands ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX commands_scope_idx ON commands(scope_id);
ALTER TABLE assessments ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX assessments_scope_idx ON assessments(scope_id);
ALTER TABLE assessment_evidence ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX assessment_evidence_scope_idx ON assessment_evidence(scope_id);
ALTER TABLE incident_assessments ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX incident_assessments_scope_idx ON incident_assessments(scope_id);
ALTER TABLE audit_events ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX audit_events_scope_idx ON audit_events(scope_id);
ALTER TABLE scenario_runs ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX scenario_runs_scope_idx ON scenario_runs(scope_id);
ALTER TABLE scenario_run_events ADD COLUMN scope_id text REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),'');
CREATE INDEX scenario_run_events_scope_idx ON scenario_run_events(scope_id);

-- +goose StatementBegin
CREATE FUNCTION emad_backfill_ownership() RETURNS void LANGUAGE plpgsql AS $$
DECLARE names text[]:=ARRAY['sources','observations','assets','asset_state','asset_telemetry','tracks','track_state','track_observations','track_history','classifications','geofences','geofence_states','alerts','incidents','incident_alerts','incident_tracks','incident_assets','incident_observations','missions','mission_assets','mission_tasks','commands','assessments','assessment_evidence','incident_assessments','audit_events','scenario_runs','scenario_run_events'];
    rel text; spec text[]; fk record; changed bigint; removed bigint; resource_kind text; target_table text;
BEGIN
    -- The maintenance caller must be the migration/control role, never the
    -- restricted query role. Locks protect the graph while it is classified.
    IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=current_user AND (rolsuper OR rolbypassrls)) THEN
        RAISE EXCEPTION 'ownership backfill requires the privileged migration role';
    END IF;
    FOREACH rel IN ARRAY names LOOP EXECUTE format('LOCK TABLE %I IN SHARE ROW EXCLUSIVE MODE',rel); END LOOP;
    INSERT INTO data_scopes(id,workspace_id,environment_id,partition,run_id)
    SELECT 'default/local/EXERCISE_OBSERVATION/'||id,'default','local','EXERCISE_OBSERVATION',id FROM scenario_runs
    ON CONFLICT DO NOTHING;
    CREATE TEMP TABLE emad_scope_candidates(relname text,row_tid tid,record_id text,scope_id text,PRIMARY KEY(relname,row_tid)) ON COMMIT DROP;
    FOREACH rel IN ARRAY names LOOP
        EXECUTE format('INSERT INTO emad_scope_candidates SELECT $1,ctid,to_jsonb(t)->>''id'',scope_id FROM %I t WHERE scope_id IS NOT NULL',rel) USING rel;
    END LOOP;
    INSERT INTO emad_scope_candidates SELECT 'scenario_runs',ctid,id,'default/local/EXERCISE_OBSERVATION/'||id FROM scenario_runs WHERE scope_id IS NULL;
    FOREACH spec SLICE 1 IN ARRAY ARRAY[['sources','source'],['assets','asset'],['geofences','geofence']] LOOP
        EXECUTE format('INSERT INTO emad_scope_candidates SELECT $1,p.ctid,p.id,c.scope_id FROM %I p
            JOIN scenario_runs r ON r.id=p.metadata->>''scenarioRunId''
            JOIN emad_scope_candidates c ON c.relname=''scenario_runs'' AND c.row_tid=r.ctid
            WHERE p.scope_id IS NULL AND p.metadata->>''resourceNamespace''=r.id||''__''
              AND left(p.id,length(r.id||''__''||$2||''__''))=r.id||''__''||$2||''__''
              AND EXISTS(SELECT 1 FROM audit_events a WHERE a.subject_id=p.id AND a.subject_type=$2 AND a.action=$2||''.created'' AND a.data->>''scenarioRunId''=r.id)',spec[1])
        USING spec[1],spec[2];
    END LOOP;
    INSERT INTO emad_scope_candidates
    SELECT 'observations',o.ctid,o.id,c.scope_id FROM observations o JOIN sources s ON s.id=o.source_id
    JOIN emad_scope_candidates c ON c.relname='sources' AND c.row_tid=s.ctid
    JOIN scenario_runs r ON o.payload->>'scenarioRunId'=r.id
    WHERE o.scope_id IS NULL AND o.payload->>'resourceNamespace'=r.id||'__'
      AND c.scope_id='default/local/EXERCISE_OBSERVATION/'||r.id
      AND left(o.id,length(r.id||'__observation__'))=r.id||'__observation__';
    INSERT INTO emad_scope_candidates
    SELECT 'tracks',t.ctid,t.id,c.scope_id FROM tracks t JOIN scenario_runs r ON t.metadata->>'scenarioRunId'=r.id
    JOIN emad_scope_candidates c ON c.relname='scenario_runs' AND c.row_tid=r.ctid
    WHERE t.scope_id IS NULL AND t.metadata->>'resourceNamespace'=r.id||'__'
      AND left(t.external_ref,length(r.id||'__track__'))=r.id||'__track__'
      AND EXISTS(SELECT 1 FROM track_observations x JOIN observations o ON o.id=x.observation_id JOIN emad_scope_candidates oc ON oc.relname='observations' AND oc.row_tid=o.ctid WHERE x.track_id=t.id AND oc.scope_id=c.scope_id)
      AND NOT EXISTS(SELECT 1 FROM track_observations x JOIN observations o ON o.id=x.observation_id LEFT JOIN emad_scope_candidates oc ON oc.relname='observations' AND oc.row_tid=o.ctid WHERE x.track_id=t.id AND oc.scope_id IS DISTINCT FROM c.scope_id);
    INSERT INTO emad_scope_candidates
    SELECT 'asset_telemetry',t.ctid,t.id,c.scope_id FROM asset_telemetry t JOIN assets a ON a.id=t.asset_id
    JOIN emad_scope_candidates c ON c.relname='assets' AND c.row_tid=a.ctid
    JOIN scenario_runs r ON t.payload->>'scenarioRunId'=r.id
    WHERE t.scope_id IS NULL AND t.payload->>'resourceNamespace'=r.id||'__' AND c.scope_id='default/local/EXERCISE_OBSERVATION/'||r.id;
    INSERT INTO emad_scope_candidates
    SELECT 'audit_events',a.ctid,a.id,c.scope_id FROM audit_events a JOIN scenario_runs r ON a.data->>'scenarioRunId'=r.id
    JOIN emad_scope_candidates c ON c.relname='scenario_runs' AND c.row_tid=r.ctid
    WHERE a.scope_id IS NULL AND a.data->>'resourceNamespace'=r.id||'__';
    INSERT INTO emad_scope_candidates
    SELECT 'alerts',a.ctid,a.id,c.scope_id FROM alerts a JOIN scenario_runs r ON a.source_reference->>'scenarioRunId'=r.id
    JOIN emad_scope_candidates c ON c.relname='scenario_runs' AND c.row_tid=r.ctid WHERE a.scope_id IS NULL;
    FOREACH spec SLICE 1 IN ARRAY ARRAY[['incidents','incident.created'],['missions','mission.created'],['commands','command.issued'],['assessments','assessment.created']] LOOP
        EXECUTE format('INSERT INTO emad_scope_candidates SELECT $1,t.ctid,t.id,min(c.scope_id)
            FROM %I t JOIN audit_events a ON a.subject_id=t.id AND a.action=$2
            LEFT JOIN emad_scope_candidates c ON c.relname=''audit_events'' AND c.row_tid=a.ctid
            WHERE t.scope_id IS NULL GROUP BY t.ctid,t.id
            HAVING count(DISTINCT c.scope_id)=1 AND bool_and(c.scope_id IS NOT NULL)',spec[1]) USING spec[1],spec[2];
    END LOOP;
    FOREACH spec SLICE 1 IN ARRAY ARRAY[
        ['asset_state','asset_id','assets'],['track_state','track_id','tracks'],['track_history','track_id','tracks'],
        ['track_observations','track_id','tracks'],['geofence_states','geofence_id','geofences'],['classifications','track_id','tracks'],
        ['scenario_run_events','run_id','scenario_runs'],['incident_alerts','incident_id','incidents'],['incident_tracks','incident_id','incidents'],
        ['incident_assets','incident_id','incidents'],['incident_observations','incident_id','incidents'],['incident_assessments','incident_id','incidents'],
        ['mission_assets','mission_id','missions'],['mission_tasks','mission_id','missions'],['assessment_evidence','assessment_id','assessments']
    ] LOOP
        EXECUTE format('INSERT INTO emad_scope_candidates SELECT $1,t.ctid,to_jsonb(t)->>''id'',c.scope_id FROM %I t JOIN %I p ON p.id=t.%I
            JOIN emad_scope_candidates c ON c.relname=$2 AND c.row_tid=p.ctid WHERE t.scope_id IS NULL',spec[1],spec[3],spec[2]) USING spec[1],spec[3];
    END LOOP;
    -- Candidate graph validation reaches a fixed point. A missing/conflicting
    -- parent invalidates its children; mixed-evidence tracks/assessments and
    -- incidents cannot be made authoritative by a plausible creator marker.
    LOOP
        removed:=0;
        FOR fk IN SELECT child.relname AS child_table,parent.relname AS parent_table,ca.attname AS child_col,pa.attname AS parent_col
            FROM pg_constraint pc JOIN pg_class child ON child.oid=pc.conrelid JOIN pg_class parent ON parent.oid=pc.confrelid
            JOIN pg_attribute ca ON ca.attrelid=pc.conrelid AND ca.attnum=pc.conkey[1]
            JOIN pg_attribute pa ON pa.attrelid=pc.confrelid AND pa.attnum=pc.confkey[1]
            WHERE pc.contype='f' AND child.relnamespace='public'::regnamespace AND parent.relnamespace='public'::regnamespace
              AND child.relname=ANY(names) AND parent.relname=ANY(names) AND array_length(pc.conkey,1)=1
        LOOP
            EXECUTE format('DELETE FROM emad_scope_candidates c USING %I t LEFT JOIN %I p ON p.%I=t.%I
                LEFT JOIN emad_scope_candidates pc ON pc.relname=$2 AND pc.row_tid=p.ctid
                WHERE c.relname=$1 AND c.row_tid=t.ctid AND t.%I IS NOT NULL AND pc.scope_id IS DISTINCT FROM c.scope_id',fk.child_table,fk.parent_table,fk.parent_col,fk.child_col,fk.child_col)
            USING fk.child_table,fk.parent_table;
            GET DIAGNOSTICS changed=ROW_COUNT; removed:=removed+changed;
        END LOOP;
        FOR resource_kind,target_table IN SELECT * FROM (VALUES('source','sources'),('observation','observations'),('track','tracks'),('asset','assets'),('incident','incidents'),('classification','classifications'),('alert','alerts')) AS x(kind,target) LOOP
            EXECUTE format('DELETE FROM emad_scope_candidates c USING assessment_evidence e WHERE c.relname=''assessment_evidence'' AND c.row_tid=e.ctid AND e.evidence_type=$1
                AND NOT EXISTS(SELECT 1 FROM %I p JOIN emad_scope_candidates pc ON pc.relname=$2 AND pc.row_tid=p.ctid
                    WHERE pc.scope_id=c.scope_id AND (p.id=e.evidence_id %s))',target_table,CASE WHEN target_table='tracks' THEN 'OR p.external_ref=e.evidence_id' ELSE '' END)
            USING resource_kind,target_table;
            GET DIAGNOSTICS changed=ROW_COUNT; removed:=removed+changed;
            EXECUTE format('DELETE FROM emad_scope_candidates c USING assessments e WHERE c.relname=''assessments'' AND c.row_tid=e.ctid AND e.subject_type=$1
                AND NOT EXISTS(SELECT 1 FROM %I p JOIN emad_scope_candidates pc ON pc.relname=$2 AND pc.row_tid=p.ctid
                    WHERE pc.scope_id=c.scope_id AND (p.id=e.subject_id %s))',target_table,CASE WHEN target_table='tracks' THEN 'OR p.external_ref=e.subject_id' ELSE '' END)
            USING resource_kind,target_table;
            GET DIAGNOSTICS changed=ROW_COUNT; removed:=removed+changed;
        END LOOP;
        DELETE FROM emad_scope_candidates c USING assessment_evidence e WHERE c.relname='assessment_evidence' AND c.row_tid=e.ctid AND e.evidence_type NOT IN ('source','observation','track','asset','incident','classification','alert');
        GET DIAGNOSTICS changed=ROW_COUNT; removed:=removed+changed;
        DELETE FROM emad_scope_candidates c USING assessments a WHERE c.relname='assessments' AND c.row_tid=a.ctid
          AND EXISTS(SELECT 1 FROM assessment_evidence e LEFT JOIN emad_scope_candidates ec ON ec.relname='assessment_evidence' AND ec.row_tid=e.ctid WHERE e.assessment_id=a.id AND ec.scope_id IS DISTINCT FROM c.scope_id);
        GET DIAGNOSTICS changed=ROW_COUNT; removed:=removed+changed;
        FOREACH spec SLICE 1 IN ARRAY ARRAY[['incident_alerts','alert_id','alerts'],['incident_tracks','track_id','tracks'],['incident_assets','asset_id','assets'],['incident_observations','observation_id','observations'],['incident_assessments','assessment_id','assessments']] LOOP
            EXECUTE format('DELETE FROM emad_scope_candidates c USING incidents i WHERE c.relname=''incidents'' AND c.row_tid=i.ctid
                AND EXISTS(SELECT 1 FROM %I x LEFT JOIN emad_scope_candidates xc ON xc.relname=$1 AND xc.row_tid=x.ctid WHERE x.incident_id=i.id AND xc.scope_id IS DISTINCT FROM c.scope_id)',spec[1]) USING spec[1];
            GET DIAGNOSTICS changed=ROW_COUNT; removed:=removed+changed;
        END LOOP;
        EXIT WHEN removed=0;
    END LOOP;
    -- Maintenance-only assignment is atomic. On retry the existing relation
    -- guards are briefly disabled under the same exclusive table locks; every
    -- candidate was validated before any ownership changes, including cycles.
    FOREACH rel IN ARRAY names LOOP
        IF EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid=to_regclass(rel) AND tgname=rel||'_scope_relations') THEN EXECUTE format('ALTER TABLE %I DISABLE TRIGGER %I',rel,rel||'_scope_relations'); END IF;
    END LOOP;
    FOREACH rel IN ARRAY names LOOP
        EXECUTE format('UPDATE %I t SET scope_id=c.scope_id FROM emad_scope_candidates c WHERE c.relname=$1 AND c.row_tid=t.ctid AND t.scope_id IS NULL',rel) USING rel;
    END LOOP;
    FOREACH rel IN ARRAY names LOOP
        IF EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid=to_regclass(rel) AND tgname=rel||'_scope_relations') THEN EXECUTE format('ALTER TABLE %I ENABLE TRIGGER %I',rel,rel||'_scope_relations'); END IF;
    END LOOP;
    DROP TABLE emad_scope_candidates;
END $$;
-- +goose StatementEnd
SELECT emad_backfill_ownership();
REVOKE ALL ON FUNCTION emad_backfill_ownership() FROM PUBLIC;

-- +goose StatementBegin
DO $$ BEGIN
    IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='emad_data') THEN
        CREATE ROLE emad_data NOLOGIN NOSUPERUSER NOBYPASSRLS;
    END IF;
END $$;
-- +goose StatementEnd
ALTER ROLE emad_data NOLOGIN NOSUPERUSER NOBYPASSRLS;
GRANT emad_data TO CURRENT_USER;
GRANT USAGE ON SCHEMA public TO emad_data;
GRANT SELECT ON data_scopes TO emad_data;

-- Validate every FK within ownership as well as polymorphic evidence links.
-- Unresolved legacy rows are left in place and hidden by RLS.
-- +goose StatementBegin
CREATE FUNCTION emad_scope_relations() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE j jsonb:=to_jsonb(NEW); oldj jsonb; child_scope text:=j->>'scope_id'; fk record; parent_scope text; ref_id text; target text;
BEGIN
    IF child_scope IS NULL THEN
        RAISE EXCEPTION 'resource scope required' USING ERRCODE='23514',CONSTRAINT='emad_scope_relation';
    END IF;
    IF TG_OP='UPDATE' THEN
        oldj:=to_jsonb(OLD);
        IF oldj->>'scope_id' IS NOT NULL AND oldj->>'scope_id'<>child_scope THEN
            RAISE EXCEPTION 'ownership is immutable' USING ERRCODE='23514',CONSTRAINT='emad_scope_relation';
        END IF;
    END IF;
    FOR fk IN SELECT pc.confrelid::regclass AS parent,ca.attname AS child_col,pa.attname AS parent_col
        FROM pg_constraint pc JOIN pg_attribute ca ON ca.attrelid=pc.conrelid AND ca.attnum=pc.conkey[1]
        JOIN pg_attribute pa ON pa.attrelid=pc.confrelid AND pa.attnum=pc.confkey[1]
        WHERE pc.contype='f' AND pc.conrelid=TG_RELID AND array_length(pc.conkey,1)=1
          AND EXISTS(SELECT 1 FROM pg_attribute a WHERE a.attrelid=pc.confrelid AND a.attname='scope_id' AND NOT a.attisdropped)
    LOOP
        ref_id:=j->>fk.child_col;
        IF ref_id IS NOT NULL THEN
            EXECUTE format('SELECT scope_id FROM %s WHERE %I=$1',fk.parent,fk.parent_col) INTO parent_scope USING ref_id;
            IF parent_scope IS DISTINCT FROM child_scope THEN
                RAISE EXCEPTION 'resource relationship is not authorized' USING ERRCODE='23514',CONSTRAINT='emad_scope_relation';
            END IF;
        END IF;
    END LOOP;
    IF TG_TABLE_NAME IN ('assessments','assessment_evidence') THEN
        IF TG_TABLE_NAME='assessments' THEN target:=j->>'subject_type'; ref_id:=j->>'subject_id';
        ELSE target:=j->>'evidence_type'; ref_id:=j->>'evidence_id'; END IF;
        target:=CASE target WHEN 'source' THEN 'sources' WHEN 'observation' THEN 'observations' WHEN 'track' THEN 'tracks'
            WHEN 'asset' THEN 'assets' WHEN 'incident' THEN 'incidents' WHEN 'classification' THEN 'classifications' WHEN 'alert' THEN 'alerts' END;
        IF target IS NULL THEN RAISE EXCEPTION 'invalid evidence type' USING ERRCODE='23514',CONSTRAINT='emad_scope_relation'; END IF;
        IF target='tracks' THEN SELECT scope_id INTO parent_scope FROM tracks WHERE scope_id=child_scope AND (id=ref_id OR external_ref=ref_id) LIMIT 1;
        ELSE EXECUTE format('SELECT scope_id FROM %I WHERE id=$1',target) INTO parent_scope USING ref_id; END IF;
        IF parent_scope IS DISTINCT FROM child_scope THEN RAISE EXCEPTION 'evidence scope mismatch' USING ERRCODE='23514',CONSTRAINT='emad_scope_relation'; END IF;
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
GRANT SELECT,INSERT,UPDATE,DELETE ON sources TO emad_data;
ALTER TABLE sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE sources FORCE ROW LEVEL SECURITY;
CREATE POLICY sources_resource_scope ON sources TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER sources_scope_relations BEFORE INSERT OR UPDATE ON sources FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON observations TO emad_data;
ALTER TABLE observations ENABLE ROW LEVEL SECURITY;
ALTER TABLE observations FORCE ROW LEVEL SECURITY;
CREATE POLICY observations_resource_scope ON observations TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER observations_scope_relations BEFORE INSERT OR UPDATE ON observations FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON assets TO emad_data;
ALTER TABLE assets ENABLE ROW LEVEL SECURITY;
ALTER TABLE assets FORCE ROW LEVEL SECURITY;
CREATE POLICY assets_resource_scope ON assets TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER assets_scope_relations BEFORE INSERT OR UPDATE ON assets FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON asset_state TO emad_data;
ALTER TABLE asset_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE asset_state FORCE ROW LEVEL SECURITY;
CREATE POLICY asset_state_resource_scope ON asset_state TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER asset_state_scope_relations BEFORE INSERT OR UPDATE ON asset_state FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON asset_telemetry TO emad_data;
ALTER TABLE asset_telemetry ENABLE ROW LEVEL SECURITY;
ALTER TABLE asset_telemetry FORCE ROW LEVEL SECURITY;
CREATE POLICY asset_telemetry_resource_scope ON asset_telemetry TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER asset_telemetry_scope_relations BEFORE INSERT OR UPDATE ON asset_telemetry FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON tracks TO emad_data;
ALTER TABLE tracks ENABLE ROW LEVEL SECURITY;
ALTER TABLE tracks FORCE ROW LEVEL SECURITY;
CREATE POLICY tracks_resource_scope ON tracks TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER tracks_scope_relations BEFORE INSERT OR UPDATE ON tracks FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON track_state TO emad_data;
ALTER TABLE track_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE track_state FORCE ROW LEVEL SECURITY;
CREATE POLICY track_state_resource_scope ON track_state TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER track_state_scope_relations BEFORE INSERT OR UPDATE ON track_state FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON track_observations TO emad_data;
ALTER TABLE track_observations ENABLE ROW LEVEL SECURITY;
ALTER TABLE track_observations FORCE ROW LEVEL SECURITY;
CREATE POLICY track_observations_resource_scope ON track_observations TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER track_observations_scope_relations BEFORE INSERT OR UPDATE ON track_observations FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON track_history TO emad_data;
ALTER TABLE track_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE track_history FORCE ROW LEVEL SECURITY;
CREATE POLICY track_history_resource_scope ON track_history TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER track_history_scope_relations BEFORE INSERT OR UPDATE ON track_history FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON classifications TO emad_data;
ALTER TABLE classifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE classifications FORCE ROW LEVEL SECURITY;
CREATE POLICY classifications_resource_scope ON classifications TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER classifications_scope_relations BEFORE INSERT OR UPDATE ON classifications FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON geofences TO emad_data;
ALTER TABLE geofences ENABLE ROW LEVEL SECURITY;
ALTER TABLE geofences FORCE ROW LEVEL SECURITY;
CREATE POLICY geofences_resource_scope ON geofences TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER geofences_scope_relations BEFORE INSERT OR UPDATE ON geofences FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON geofence_states TO emad_data;
ALTER TABLE geofence_states ENABLE ROW LEVEL SECURITY;
ALTER TABLE geofence_states FORCE ROW LEVEL SECURITY;
CREATE POLICY geofence_states_resource_scope ON geofence_states TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER geofence_states_scope_relations BEFORE INSERT OR UPDATE ON geofence_states FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON alerts TO emad_data;
ALTER TABLE alerts ENABLE ROW LEVEL SECURITY;
ALTER TABLE alerts FORCE ROW LEVEL SECURITY;
CREATE POLICY alerts_resource_scope ON alerts TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER alerts_scope_relations BEFORE INSERT OR UPDATE ON alerts FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON incidents TO emad_data;
ALTER TABLE incidents ENABLE ROW LEVEL SECURITY;
ALTER TABLE incidents FORCE ROW LEVEL SECURITY;
CREATE POLICY incidents_resource_scope ON incidents TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER incidents_scope_relations BEFORE INSERT OR UPDATE ON incidents FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON incident_alerts TO emad_data;
ALTER TABLE incident_alerts ENABLE ROW LEVEL SECURITY;
ALTER TABLE incident_alerts FORCE ROW LEVEL SECURITY;
CREATE POLICY incident_alerts_resource_scope ON incident_alerts TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER incident_alerts_scope_relations BEFORE INSERT OR UPDATE ON incident_alerts FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON incident_tracks TO emad_data;
ALTER TABLE incident_tracks ENABLE ROW LEVEL SECURITY;
ALTER TABLE incident_tracks FORCE ROW LEVEL SECURITY;
CREATE POLICY incident_tracks_resource_scope ON incident_tracks TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER incident_tracks_scope_relations BEFORE INSERT OR UPDATE ON incident_tracks FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON incident_assets TO emad_data;
ALTER TABLE incident_assets ENABLE ROW LEVEL SECURITY;
ALTER TABLE incident_assets FORCE ROW LEVEL SECURITY;
CREATE POLICY incident_assets_resource_scope ON incident_assets TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER incident_assets_scope_relations BEFORE INSERT OR UPDATE ON incident_assets FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON incident_observations TO emad_data;
ALTER TABLE incident_observations ENABLE ROW LEVEL SECURITY;
ALTER TABLE incident_observations FORCE ROW LEVEL SECURITY;
CREATE POLICY incident_observations_resource_scope ON incident_observations TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER incident_observations_scope_relations BEFORE INSERT OR UPDATE ON incident_observations FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON missions TO emad_data;
ALTER TABLE missions ENABLE ROW LEVEL SECURITY;
ALTER TABLE missions FORCE ROW LEVEL SECURITY;
CREATE POLICY missions_resource_scope ON missions TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER missions_scope_relations BEFORE INSERT OR UPDATE ON missions FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON mission_assets TO emad_data;
ALTER TABLE mission_assets ENABLE ROW LEVEL SECURITY;
ALTER TABLE mission_assets FORCE ROW LEVEL SECURITY;
CREATE POLICY mission_assets_resource_scope ON mission_assets TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER mission_assets_scope_relations BEFORE INSERT OR UPDATE ON mission_assets FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON mission_tasks TO emad_data;
ALTER TABLE mission_tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE mission_tasks FORCE ROW LEVEL SECURITY;
CREATE POLICY mission_tasks_resource_scope ON mission_tasks TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER mission_tasks_scope_relations BEFORE INSERT OR UPDATE ON mission_tasks FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON commands TO emad_data;
ALTER TABLE commands ENABLE ROW LEVEL SECURITY;
ALTER TABLE commands FORCE ROW LEVEL SECURITY;
CREATE POLICY commands_resource_scope ON commands TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER commands_scope_relations BEFORE INSERT OR UPDATE ON commands FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON assessments TO emad_data;
ALTER TABLE assessments ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessments FORCE ROW LEVEL SECURITY;
CREATE POLICY assessments_resource_scope ON assessments TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER assessments_scope_relations BEFORE INSERT OR UPDATE ON assessments FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON assessment_evidence TO emad_data;
ALTER TABLE assessment_evidence ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment_evidence FORCE ROW LEVEL SECURITY;
CREATE POLICY assessment_evidence_resource_scope ON assessment_evidence TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER assessment_evidence_scope_relations BEFORE INSERT OR UPDATE ON assessment_evidence FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON incident_assessments TO emad_data;
ALTER TABLE incident_assessments ENABLE ROW LEVEL SECURITY;
ALTER TABLE incident_assessments FORCE ROW LEVEL SECURITY;
CREATE POLICY incident_assessments_resource_scope ON incident_assessments TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER incident_assessments_scope_relations BEFORE INSERT OR UPDATE ON incident_assessments FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON audit_events TO emad_data;
ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_events FORCE ROW LEVEL SECURITY;
CREATE POLICY audit_events_resource_scope ON audit_events TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER audit_events_scope_relations BEFORE INSERT OR UPDATE ON audit_events FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON scenario_runs TO emad_data;
ALTER TABLE scenario_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE scenario_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY scenario_runs_resource_scope ON scenario_runs TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER scenario_runs_scope_relations BEFORE INSERT OR UPDATE ON scenario_runs FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();
GRANT SELECT,INSERT,UPDATE,DELETE ON scenario_run_events TO emad_data;
ALTER TABLE scenario_run_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE scenario_run_events FORCE ROW LEVEL SECURITY;
CREATE POLICY scenario_run_events_resource_scope ON scenario_run_events TO emad_data USING (scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK (scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER scenario_run_events_scope_relations BEFORE INSERT OR UPDATE ON scenario_run_events FOR EACH ROW EXECUTE FUNCTION emad_scope_relations();

-- Scope source-bound service ingestion at the database boundary as well.
CREATE POLICY sources_producer ON sources AS RESTRICTIVE TO emad_data
USING (COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR id IN (SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)))
WITH CHECK (COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR id IN (SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)));
CREATE POLICY observations_producer ON observations AS RESTRICTIVE TO emad_data
USING (COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN (SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)))
WITH CHECK (COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN (SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)));

CREATE POLICY telemetry_producer ON asset_telemetry AS RESTRICTIVE TO emad_data
USING (COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN (SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)))
WITH CHECK (COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN (SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)));

DROP INDEX tracks_external_ref_key;
CREATE UNIQUE INDEX tracks_external_ref_key ON tracks(scope_id,external_ref) WHERE external_ref IS NOT NULL;
DROP INDEX asset_telemetry_message_id_key;
CREATE UNIQUE INDEX asset_telemetry_message_id_key ON asset_telemetry(scope_id,message_id) WHERE message_id IS NOT NULL;

CREATE VIEW emad_ownership_report AS
SELECT 'sources'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM sources
UNION ALL
SELECT 'observations'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM observations
UNION ALL
SELECT 'assets'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM assets
UNION ALL
SELECT 'asset_state'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM asset_state
UNION ALL
SELECT 'asset_telemetry'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM asset_telemetry
UNION ALL
SELECT 'tracks'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM tracks
UNION ALL
SELECT 'track_state'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM track_state
UNION ALL
SELECT 'track_observations'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM track_observations
UNION ALL
SELECT 'track_history'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM track_history
UNION ALL
SELECT 'classifications'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM classifications
UNION ALL
SELECT 'geofences'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM geofences
UNION ALL
SELECT 'geofence_states'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM geofence_states
UNION ALL
SELECT 'alerts'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM alerts
UNION ALL
SELECT 'incidents'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM incidents
UNION ALL
SELECT 'incident_alerts'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM incident_alerts
UNION ALL
SELECT 'incident_tracks'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM incident_tracks
UNION ALL
SELECT 'incident_assets'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM incident_assets
UNION ALL
SELECT 'incident_observations'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM incident_observations
UNION ALL
SELECT 'missions'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM missions
UNION ALL
SELECT 'mission_assets'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM mission_assets
UNION ALL
SELECT 'mission_tasks'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM mission_tasks
UNION ALL
SELECT 'commands'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM commands
UNION ALL
SELECT 'assessments'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM assessments
UNION ALL
SELECT 'assessment_evidence'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM assessment_evidence
UNION ALL
SELECT 'incident_assessments'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM incident_assessments
UNION ALL
SELECT 'audit_events'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM audit_events
UNION ALL
SELECT 'scenario_runs'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM scenario_runs
UNION ALL
SELECT 'scenario_run_events'::text AS table_name,count(*) FILTER(WHERE scope_id IS NOT NULL)::bigint AS resolved,count(*) FILTER(WHERE scope_id IS NULL)::bigint AS quarantined FROM scenario_run_events;

-- +goose Down
-- Retained scoped data must be migrated explicitly before removing isolation.
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'ownership migration requires an explicit forward recovery plan'; END $$;
-- +goose StatementEnd
