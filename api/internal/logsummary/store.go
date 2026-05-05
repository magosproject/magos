package logsummary

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	_ "modernc.org/sqlite"
)

const (
	envLogsEnabled = "MAGOS_LOGS_ENABLED"
	envSQLitePath  = "MAGOS_SQLITE_PATH"

	defaultListLimit  = 30
	defaultSQLitePath = "/tmp/magos.db"
)

var (
	ErrInvalidCursor = errors.New("invalid run list cursor")
	ErrNotFound      = errors.New("run summary not found")
)

type Config struct {
	Enabled    bool
	SQLitePath string
}

type Store struct {
	db *sql.DB
}

type listCursor struct {
	SortTime string `json:"sortTime"`
	RunID    string `json:"runID"`
}

type listedRun struct {
	run      v1alpha1.Run
	sortTime string
}

func LoadConfigFromEnv() Config {
	sqlitePath := os.Getenv(envSQLitePath)
	if sqlitePath == "" {
		sqlitePath = defaultSQLitePath
	}

	return Config{
		Enabled:    parseBoolEnv(envLogsEnabled, false),
		SQLitePath: sqlitePath,
	}
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.SQLitePath == "" {
		return fmt.Errorf("%s must be set when log storage is enabled", envSQLitePath)
	}
	return nil
}

func NewStore(ctx context.Context, cfg Config) (*Store, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(cfg.SQLitePath, "file:") {
		if err := os.MkdirAll(filepath.Dir(cfg.SQLitePath), 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", cfg.SQLitePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite run summary store: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{db: db}
	if err := store.init(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) init(ctx context.Context) error {
	statements := []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
		`CREATE TABLE IF NOT EXISTS runs (
			namespace TEXT NOT NULL,
			workspace TEXT NOT NULL,
			run_id TEXT NOT NULL,
			trigger TEXT NOT NULL DEFAULT '',
			target_revision TEXT NOT NULL DEFAULT '',
			observed_revision TEXT NOT NULL DEFAULT '',
			started_at TEXT,
			finished_at TEXT,
			sort_time TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (namespace, workspace, run_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_workspace_sort
			ON runs (namespace, workspace, sort_time DESC, run_id DESC)`,
		`CREATE TABLE IF NOT EXISTS run_phases (
			namespace TEXT NOT NULL,
			workspace TEXT NOT NULL,
			run_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			job_name TEXT NOT NULL DEFAULT '',
			pod_name TEXT NOT NULL DEFAULT '',
			started_at TEXT,
			finished_at TEXT,
			result TEXT NOT NULL DEFAULT '',
			log_key TEXT NOT NULL DEFAULT '',
			log_size_bytes INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (namespace, workspace, run_id, phase),
			FOREIGN KEY (namespace, workspace, run_id)
				REFERENCES runs(namespace, workspace, run_id)
				ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_run_phases_log_key ON run_phases (log_key)`,
	}

	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("initialize sqlite run summary store: %w", err)
		}
	}
	return nil
}

func (s *Store) UpsertRun(ctx context.Context, namespace, workspace string, run v1alpha1.Run) error {
	if namespace == "" || workspace == "" || run.ID == "" {
		return fmt.Errorf("namespace, workspace, and runID are required")
	}

	now := formatTime(time.Now().UTC())
	startedAt := formatMetaTime(run.StartedAt)
	finishedAt := formatMetaTime(run.FinishedAt)
	sortTime := firstNonEmpty(startedAt, phaseStartedAt(run), phaseFinishedAt(run), runIDSortTime(run.ID), now)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin run summary transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO runs (
			namespace, workspace, run_id, trigger, target_revision, observed_revision,
			started_at, finished_at, sort_time, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(namespace, workspace, run_id) DO UPDATE SET
			trigger = CASE WHEN excluded.trigger <> '' THEN excluded.trigger ELSE runs.trigger END,
			target_revision = CASE WHEN excluded.target_revision <> '' THEN excluded.target_revision ELSE runs.target_revision END,
			observed_revision = CASE WHEN excluded.observed_revision <> '' THEN excluded.observed_revision ELSE runs.observed_revision END,
			started_at = COALESCE(runs.started_at, excluded.started_at),
			finished_at = COALESCE(excluded.finished_at, runs.finished_at),
			sort_time = CASE
				WHEN runs.started_at IS NULL AND excluded.started_at IS NOT NULL THEN excluded.started_at
				ELSE runs.sort_time
			END,
			updated_at = excluded.updated_at`,
		namespace,
		workspace,
		run.ID,
		string(run.Trigger),
		run.TargetRevision,
		run.ObservedRevision,
		nullEmpty(startedAt),
		nullEmpty(finishedAt),
		sortTime,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("upsert run summary %q: %w", run.ID, err)
	}

	if run.Plan != nil {
		err = upsertPhase(ctx, tx, namespace, workspace, run.ID, v1alpha1.RunPhasePlan, run.Plan)
		if err != nil {
			return err
		}
	}
	if run.Apply != nil {
		err = upsertPhase(ctx, tx, namespace, workspace, run.ID, v1alpha1.RunPhaseApply, run.Apply)
		if err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit run summary transaction: %w", err)
	}
	committed = true
	return nil
}

func (s *Store) ListRuns(ctx context.Context, namespace, workspace string, limit int, cursor string) ([]v1alpha1.Run, string, error) {
	if limit <= 0 {
		limit = defaultListLimit
	}

	var cur *listCursor
	if cursor != "" {
		decoded, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		cur = decoded
	}

	query := `
		SELECT run_id, trigger, target_revision, observed_revision, started_at, finished_at, sort_time
		FROM runs
		WHERE namespace = ? AND workspace = ?`
	args := []any{namespace, workspace}
	if cur != nil {
		query += ` AND (sort_time < ? OR (sort_time = ? AND run_id < ?))`
		args = append(args, cur.SortTime, cur.SortTime, cur.RunID)
	}
	query += ` ORDER BY sort_time DESC, run_id DESC LIMIT ?`
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list run summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	listed := make([]listedRun, 0, limit+1)
	for rows.Next() {
		run, sortTime, err := scanRun(rows)
		if err != nil {
			return nil, "", err
		}
		listed = append(listed, listedRun{run: run, sortTime: sortTime})
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate run summaries: %w", err)
	}

	hasMore := len(listed) > limit
	if hasMore {
		listed = listed[:limit]
	}

	runs := make([]v1alpha1.Run, 0, len(listed))
	for _, item := range listed {
		run := item.run
		plan, err := s.getRunPhase(ctx, namespace, workspace, run.ID, v1alpha1.RunPhasePlan)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, "", err
		}
		apply, err := s.getRunPhase(ctx, namespace, workspace, run.ID, v1alpha1.RunPhaseApply)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, "", err
		}
		run.Plan = plan
		run.Apply = apply
		runs = append(runs, run)
	}

	nextCursor := ""
	if hasMore && len(listed) > 0 {
		last := listed[len(listed)-1]
		nextCursor, err = encodeCursor(listCursor{SortTime: last.sortTime, RunID: last.run.ID})
		if err != nil {
			return nil, "", err
		}
	}
	return runs, nextCursor, nil
}

func (s *Store) GetRunPhase(ctx context.Context, namespace, workspace, runID string, phase v1alpha1.RunPhase) (*v1alpha1.RunPhaseSummary, error) {
	return s.getRunPhase(ctx, namespace, workspace, runID, phase)
}


func upsertPhase(ctx context.Context, tx *sql.Tx, namespace, workspace, runID string, phase v1alpha1.RunPhase, summary *v1alpha1.RunPhaseSummary) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO run_phases (
			namespace, workspace, run_id, phase, job_name, pod_name,
			started_at, finished_at, result, log_key, log_size_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(namespace, workspace, run_id, phase) DO UPDATE SET
			job_name = excluded.job_name,
			pod_name = excluded.pod_name,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			result = excluded.result,
			log_key = excluded.log_key,
			log_size_bytes = excluded.log_size_bytes`,
		namespace,
		workspace,
		runID,
		string(phase),
		summary.JobName,
		summary.PodName,
		nullEmpty(formatMetaTime(summary.StartedAt)),
		nullEmpty(formatMetaTime(summary.FinishedAt)),
		string(summary.Result),
		summary.LogKey,
		summary.LogSizeBytes,
	)
	if err != nil {
		return fmt.Errorf("upsert %s phase summary for run %q: %w", phase, runID, err)
	}
	return nil
}

func (s *Store) getRunPhase(ctx context.Context, namespace, workspace, runID string, phase v1alpha1.RunPhase) (*v1alpha1.RunPhaseSummary, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT job_name, pod_name, started_at, finished_at, result, log_key, log_size_bytes
		FROM run_phases
		WHERE namespace = ? AND workspace = ? AND run_id = ? AND phase = ?`,
		namespace,
		workspace,
		runID,
		string(phase),
	)

	var summary v1alpha1.RunPhaseSummary
	var startedAt, finishedAt sql.NullString
	var result string
	if err := row.Scan(
		&summary.JobName,
		&summary.PodName,
		&startedAt,
		&finishedAt,
		&result,
		&summary.LogKey,
		&summary.LogSizeBytes,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get %s phase summary for run %q: %w", phase, runID, err)
	}

	parsedStartedAt, err := parseMetaTime(startedAt)
	if err != nil {
		return nil, err
	}
	parsedFinishedAt, err := parseMetaTime(finishedAt)
	if err != nil {
		return nil, err
	}
	summary.StartedAt = parsedStartedAt
	summary.FinishedAt = parsedFinishedAt
	summary.Result = v1alpha1.RunLogResult(result)
	return &summary, nil
}

func scanRun(scanner interface {
	Scan(dest ...any) error
}) (v1alpha1.Run, string, error) {
	var run v1alpha1.Run
	var trigger string
	var startedAt, finishedAt sql.NullString
	var sortTime string
	if err := scanner.Scan(
		&run.ID,
		&trigger,
		&run.TargetRevision,
		&run.ObservedRevision,
		&startedAt,
		&finishedAt,
		&sortTime,
	); err != nil {
		return run, "", fmt.Errorf("scan run summary: %w", err)
	}

	parsedStartedAt, err := parseMetaTime(startedAt)
	if err != nil {
		return run, "", err
	}
	parsedFinishedAt, err := parseMetaTime(finishedAt)
	if err != nil {
		return run, "", err
	}
	run.Trigger = v1alpha1.RunTrigger(trigger)
	run.StartedAt = parsedStartedAt
	run.FinishedAt = parsedFinishedAt
	return run, sortTime, nil
}

func encodeCursor(cursor listCursor) (string, error) {
	body, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("marshal cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(body), nil
}

func decodeCursor(raw string) (*listCursor, error) {
	body, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var cursor listCursor
	if err := json.Unmarshal(body, &cursor); err != nil {
		return nil, ErrInvalidCursor
	}
	if cursor.SortTime == "" || cursor.RunID == "" {
		return nil, ErrInvalidCursor
	}
	return &cursor, nil
}

func phaseStartedAt(run v1alpha1.Run) string {
	if run.Plan != nil {
		return formatMetaTime(run.Plan.StartedAt)
	}
	if run.Apply != nil {
		return formatMetaTime(run.Apply.StartedAt)
	}
	return ""
}

func phaseFinishedAt(run v1alpha1.Run) string {
	if run.Plan != nil {
		return formatMetaTime(run.Plan.FinishedAt)
	}
	if run.Apply != nil {
		return formatMetaTime(run.Apply.FinishedAt)
	}
	return ""
}

func runIDSortTime(runID string) string {
	prefix, _, ok := strings.Cut(runID, "-")
	if !ok || prefix == "" {
		return ""
	}
	t, err := time.ParseInLocation("20060102T150405", prefix, time.UTC)
	if err != nil {
		return ""
	}
	return formatTime(t)
}

func formatMetaTime(t *metav1.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return formatTime(t.Time)
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseMetaTime(value sql.NullString) (*metav1.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil, fmt.Errorf("parse stored timestamp %q: %w", value.String, err)
	}
	metav1Time := metav1.NewTime(t)
	return &metav1Time, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func nullEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func parseBoolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
