CREATE TABLE IF NOT EXISTS pending_changes (
 scenario_id text PRIMARY KEY REFERENCES scenarios(id),
 revision bigint NOT NULL,
 body jsonb NOT NULL
);
