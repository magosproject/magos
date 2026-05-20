CREATE INDEX IF NOT EXISTS idx_runs_workspace_sort
	ON runs (namespace, workspace, sort_time DESC, run_id DESC);
