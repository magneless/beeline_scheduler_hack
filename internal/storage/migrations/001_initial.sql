CREATE TABLE IF NOT EXISTS scenarios (
 id text PRIMARY KEY, revision bigint NOT NULL CHECK(revision > 0), current_plan_id text
);
CREATE TABLE IF NOT EXISTS snapshots (
 scenario_id text NOT NULL REFERENCES scenarios(id), revision bigint NOT NULL,
 body jsonb NOT NULL, PRIMARY KEY(scenario_id,revision)
);
CREATE TABLE IF NOT EXISTS plans (
 id text PRIMARY KEY, scenario_id text NOT NULL REFERENCES scenarios(id),
 revision bigint NOT NULL, body jsonb NOT NULL,
 FOREIGN KEY(scenario_id,revision) REFERENCES snapshots(scenario_id,revision)
);
CREATE TABLE IF NOT EXISTS runs (
 id text PRIMARY KEY, scenario_id text NOT NULL REFERENCES scenarios(id), request_id text NOT NULL,
 event_id text, command jsonb NOT NULL, status text NOT NULL CHECK(status IN ('queued','running','succeeded','failed')),
 plan_id text REFERENCES plans(id), error jsonb, commit_input jsonb,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(scenario_id,request_id), UNIQUE(scenario_id,event_id),
 CHECK ((status='succeeded') = (plan_id IS NOT NULL)),
 CHECK ((status='failed') = (error IS NOT NULL))
);
CREATE TABLE IF NOT EXISTS events (
 scenario_id text NOT NULL REFERENCES scenarios(id), id text NOT NULL,
 plan_id text NOT NULL REFERENCES plans(id), body jsonb NOT NULL,
 PRIMARY KEY(scenario_id,id)
);
CREATE TABLE IF NOT EXISTS imports (
 scenario_id text PRIMARY KEY REFERENCES scenarios(id), metadata jsonb NOT NULL
);
