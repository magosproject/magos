-- TODO @ramonvermeulen: Migrate this schema to api/internal/db/schema once the API has 3 or more SQL tables.
CREATE TABLE IF NOT EXISTS runs (
	namespace TEXT NOT NULL,
	workspace TEXT NOT NULL,
	run_id TEXT NOT NULL,
	trigger TEXT NOT NULL DEFAULT 'unknown',
	target_revision TEXT NOT NULL DEFAULT '',
	observed_revision TEXT NOT NULL DEFAULT '',
	started_at TEXT,
	finished_at TEXT,
	scheduled_at TEXT,
	sort_time TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	plan JSONB,
	apply JSONB,
	PRIMARY KEY (namespace, workspace, run_id)
)

CREATE INDEX IF NOT EXISTS idx_runs_workspace_sort
	ON runs (namespace, workspace, sort_time DESC, run_id DESC)
