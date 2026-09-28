-- Replace the obsolete automatically generated ninth/tenth reserve crews.
-- Retain historical snapshots/plans and any crew that actually received work.
DO $$
DECLARE
    entry record;
    root_plan jsonb;
    current_plan jsonb;
    target jsonb;
    crew jsonb;
    crews jsonb;
    initial_used boolean;
    ever_used boolean;
    next_plan_id text;
BEGIN
    FOR entry IN
        SELECT s.id, s.revision, s.current_plan_id, p.body
        FROM scenarios s JOIN snapshots p ON p.scenario_id=s.id AND p.revision=s.revision
        WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(p.body->'issues') i WHERE i->>'code'='DEMO_ENGINEERS')
        AND EXISTS (SELECT 1 FROM jsonb_array_elements(p.body->'engineers') e
                    WHERE e->>'id' IN ((p.body->>'region_id')||'-eng-09', (p.body->>'region_id')||'-eng-10')
                    AND e->>'reserve'='true' AND e->>'available'='false')
        FOR UPDATE OF s
    LOOP
        current_plan := NULL;
        root_plan := NULL;
        IF entry.current_plan_id IS NOT NULL THEN
            SELECT body INTO current_plan FROM plans WHERE id=entry.current_plan_id;
            root_plan := current_plan;
            WHILE root_plan->>'base_plan_id' IS NOT NULL LOOP
                SELECT body INTO root_plan FROM plans WHERE id=root_plan->>'base_plan_id';
            END LOOP;
        END IF;
        crews := '[]'::jsonb;
        FOR crew IN SELECT value FROM jsonb_array_elements(entry.body->'engineers') LOOP
            SELECT EXISTS (
                SELECT 1 FROM plans p, jsonb_array_elements(p.body->'routes') r
                WHERE p.scenario_id=entry.id AND r->>'engineer_id'=crew->>'id'
                AND jsonb_array_length(r->'visits')>0
            ) INTO ever_used;
            IF crew->>'id' IN ((entry.body->>'region_id')||'-eng-09', (entry.body->>'region_id')||'-eng-10')
                AND crew->>'reserve'='true' AND crew->>'available'='false' AND NOT ever_used THEN
                CONTINUE;
            END IF;
            IF root_plan IS NOT NULL THEN
                SELECT EXISTS (SELECT 1 FROM jsonb_array_elements(root_plan->'routes') r
                    WHERE r->>'engineer_id'=crew->>'id' AND jsonb_array_length(r->'visits')>0) INTO initial_used;
                IF NOT initial_used AND NOT ever_used AND crew->>'available'='true' THEN
                    crew := crew || '{"available":false,"reserve":true}'::jsonb;
                END IF;
            END IF;
            crews := crews || jsonb_build_array(crew);
        END LOOP;
        IF crews = entry.body->'engineers' THEN CONTINUE; END IF;
        target := entry.body || jsonb_build_object('revision',entry.revision+1,'engineers',crews,
            'reserve_initialized',entry.current_plan_id IS NOT NULL);
        INSERT INTO snapshots VALUES(entry.id,entry.revision+1,target);
        next_plan_id := NULL;
        IF current_plan IS NOT NULL THEN
            next_plan_id := entry.current_plan_id || '-reserve-v2';
            INSERT INTO plans VALUES(next_plan_id,entry.id,entry.revision+1,
                current_plan || jsonb_build_object('id',next_plan_id,'snapshot_revision',entry.revision+1));
        END IF;
        UPDATE scenarios SET revision=entry.revision+1,current_plan_id=next_plan_id WHERE id=entry.id;
    END LOOP;
END $$;
