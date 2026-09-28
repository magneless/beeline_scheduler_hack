CREATE TABLE IF NOT EXISTS proposals (
 id text PRIMARY KEY,
 scenario_id text NOT NULL REFERENCES scenarios(id),
 request_id text NOT NULL,
 snapshot_revision bigint NOT NULL,
 expected_current_plan_id text,
 event_id text,
 body jsonb NOT NULL,
 status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted')),
 accepted_option_key text,
 accepted_request_id text,
 accepted_plan_id text REFERENCES plans(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(scenario_id,request_id),
 CHECK ((status='accepted') = (accepted_plan_id IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS proposals_current_idx ON proposals(scenario_id,created_at DESC,id DESC) WHERE status='pending';
