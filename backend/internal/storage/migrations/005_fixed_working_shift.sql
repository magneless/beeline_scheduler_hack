-- QA chat 4: fixed 10:00–22:00 in scenario time. Keep immutable history.
-- A day with recorded execution must not have its factual timeline rewritten.
DO $$
DECLARE
    entry record;
    required_shift jsonb;
    crews jsonb;
    target jsonb;
BEGIN
    FOR entry IN
        SELECT s.id, s.revision, p.body
        FROM scenarios s JOIN snapshots p ON p.scenario_id=s.id AND p.revision=s.revision
        WHERE jsonb_array_length(p.body->'engineers') > 0
        AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(p.body->'orders') o
                        WHERE o->'execution' IS NOT NULL AND o->'execution' <> 'null'::jsonb)
        FOR UPDATE OF s
    LOOP
        required_shift := jsonb_build_object(
            'start', to_char(((entry.body->>'date')::date + time '10:00')
                AT TIME ZONE (entry.body->>'timezone') AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
            'end', to_char(((entry.body->>'date')::date + time '22:00')
                AT TIME ZONE (entry.body->>'timezone') AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'));
        IF NOT EXISTS (SELECT 1 FROM jsonb_array_elements(entry.body->'engineers') e
            WHERE (e->'shift'->>'start')::timestamptz IS DISTINCT FROM (required_shift->>'start')::timestamptz
               OR (e->'shift'->>'end')::timestamptz IS DISTINCT FROM (required_shift->>'end')::timestamptz)
        THEN CONTINUE; END IF;
        SELECT jsonb_agg(e || jsonb_build_object('shift', required_shift) ||
            CASE WHEN e->>'reserve' = 'true' THEN '{"available":true,"reserve":false}'::jsonb
            ELSE '{}'::jsonb END ORDER BY ordinal)
        INTO crews FROM jsonb_array_elements(entry.body->'engineers') WITH ORDINALITY AS crew(e, ordinal);
        target := entry.body || jsonb_build_object('revision', entry.revision+1,
            'engineers', crews, 'reserve_initialized', false,
            'issues', COALESCE(entry.body->'issues', '[]'::jsonb) || jsonb_build_array(jsonb_build_object(
                'code', 'SHIFT_UPDATED', 'message', 'Смена бригад обновлена на 10:00–22:00. Рассчитайте план заново.')));
        INSERT INTO snapshots VALUES(entry.id, entry.revision+1, target);
        UPDATE scenarios SET revision=entry.revision+1, current_plan_id=NULL WHERE id=entry.id;
    END LOOP;
END $$;
