package runs

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	envDatabaseURL      = "MAGOS_DATABASE_URL"
	envPostgresHost     = "MAGOS_POSTGRES_HOST"
	envPostgresPort     = "MAGOS_POSTGRES_PORT"
	envPostgresDatabase = "MAGOS_POSTGRES_DATABASE"
	envPostgresUser     = "MAGOS_POSTGRES_USER"
	envPostgresPassword = "MAGOS_POSTGRES_PASSWORD"
	envPostgresSSLMode  = "MAGOS_POSTGRES_SSLMODE"

	defaultListLimit        = 30
	defaultPostgresHost     = "127.0.0.1"
	defaultPostgresPort     = "5432"
	defaultPostgresDatabase = "magos"
	defaultPostgresUser     = "magos"
	defaultPostgresSSLMode  = "disable"
)

var (
	ErrInvalidCursor = errors.New("invalid run list cursor")
	ErrNotFound      = errors.New("run not found")
)

//go:embed schema/001_create_runs_table.sql
var runSchema string

type Config struct {
	DatabaseURL string
}

type Store struct {
	db *sql.DB
}

type listCursor struct {
	SortTime string `json:"sortTime"`
	RunID    string `json:"runID"`
}

func LoadConfigFromEnv() Config {
	databaseURL := os.Getenv(envDatabaseURL)
	if databaseURL == "" {
		databaseURL = postgresURLFromEnv()
	}
	return Config{DatabaseURL: databaseURL}
}

func NewStore(ctx context.Context, cfg Config) (*Store, error) {
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres run store: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect postgres run store: %w", err)
	}

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
	if _, err := s.db.ExecContext(ctx, runSchema); err != nil {
		return fmt.Errorf("initialize postgres run store schema: %w", err)
	}
	return nil
}

func (s *Store) UpsertRun(ctx context.Context, namespace, workspace string, run v1alpha1.Run) error {
	if namespace == "" || workspace == "" || run.ID == "" {
		return fmt.Errorf("namespace, workspace, and runID are required")
	}

	now := time.Now().UTC()
	startedAt := runStartedAt(run)
	finishedAt := runFinishedAt(run)
	scheduledAt := formatMetaTime(run.ScheduledAt)
	sortTime := firstNonEmpty(startedAt, phaseStartedAt(run), phaseFinishedAt(run), runIDSortTime(run.ID), formatTime(now))

	plan, err := marshalPhaseSummary(run.Plan)
	if err != nil {
		return err
	}
	apply, err := marshalPhaseSummary(run.Apply)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO runs (
			namespace, workspace, run_id, trigger, target_revision, observed_revision,
			started_at, finished_at, scheduled_at, sort_time, created_at, updated_at, plan, apply
		) VALUES ($1, $2, $3, COALESCE(NULLIF($4, ''), 'unknown'), $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14::jsonb)
		ON CONFLICT(namespace, workspace, run_id) DO UPDATE SET
			trigger = CASE
				WHEN $4 <> '' THEN excluded.trigger
				WHEN runs.trigger = '' THEN 'unknown'
				ELSE runs.trigger
			END,
			target_revision = CASE WHEN excluded.target_revision <> '' THEN excluded.target_revision ELSE runs.target_revision END,
			observed_revision = CASE WHEN excluded.observed_revision <> '' THEN excluded.observed_revision ELSE runs.observed_revision END,
			started_at = COALESCE(excluded.started_at, runs.started_at),
			finished_at = COALESCE(excluded.finished_at, runs.finished_at),
			scheduled_at = COALESCE(runs.scheduled_at, excluded.scheduled_at),
			plan = COALESCE(excluded.plan, runs.plan),
			apply = COALESCE(excluded.apply, runs.apply),
			sort_time = CASE
				WHEN excluded.started_at IS NOT NULL THEN excluded.started_at
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
		nullEmpty(scheduledAt),
		sortTime,
		formatTime(now),
		formatTime(now),
		plan,
		apply,
	)
	if err != nil {
		return fmt.Errorf("upsert run %q: %w", run.ID, err)
	}
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
		SELECT run_id, trigger, target_revision, observed_revision, started_at, finished_at, scheduled_at, sort_time, plan, apply
		FROM runs
		WHERE namespace = $1 AND workspace = $2`
	args := []any{namespace, workspace}
	if cur != nil {
		query += ` AND (sort_time < $3 OR (sort_time = $4 AND run_id < $5))`
		args = append(args, cur.SortTime, cur.SortTime, cur.RunID)
	}
	query += fmt.Sprintf(` ORDER BY sort_time DESC, run_id DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	runs := make([]v1alpha1.Run, 0, limit)
	lastSortTime := ""
	lastRunID := ""
	hasMore := false
	for rows.Next() {
		run, sortTime, err := scanRun(rows)
		if err != nil {
			return nil, "", err
		}
		if len(runs) == limit {
			hasMore = true
			continue
		}
		runs = append(runs, run)
		lastSortTime = sortTime
		lastRunID = run.ID
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate runs: %w", err)
	}

	nextCursor := ""
	if hasMore && lastSortTime != "" && lastRunID != "" {
		nextCursor, err = encodeCursor(listCursor{SortTime: lastSortTime, RunID: lastRunID})
		if err != nil {
			return nil, "", err
		}
	}
	return runs, nextCursor, nil
}

func (s *Store) GetRunPhase(ctx context.Context, namespace, workspace, runID string, phase v1alpha1.RunPhase) (*v1alpha1.RunPhaseSummary, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT plan, apply
		FROM runs
		WHERE namespace = $1 AND workspace = $2 AND run_id = $3`,
		namespace,
		workspace,
		runID,
	)

	var plan, apply sql.NullString
	if err := row.Scan(&plan, &apply); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get %s phase summary for run %q: %w", phase, runID, err)
	}

	switch phase {
	case v1alpha1.RunPhasePlan:
		return requiredPhaseSummary(plan)
	case v1alpha1.RunPhaseApply:
		return requiredPhaseSummary(apply)
	default:
		return nil, fmt.Errorf("phase must be plan or apply")
	}
}

func scanRun(scanner interface {
	Scan(dest ...any) error
}) (v1alpha1.Run, string, error) {
	var run v1alpha1.Run
	var trigger string
	var startedAt, finishedAt, scheduledAt sql.NullString
	var plan, apply sql.NullString
	var sortTime string
	if err := scanner.Scan(
		&run.ID,
		&trigger,
		&run.TargetRevision,
		&run.ObservedRevision,
		&startedAt,
		&finishedAt,
		&scheduledAt,
		&sortTime,
		&plan,
		&apply,
	); err != nil {
		return run, "", fmt.Errorf("scan run: %w", err)
	}

	parsedStartedAt, err := parseMetaTime(startedAt)
	if err != nil {
		return run, "", err
	}
	parsedFinishedAt, err := parseMetaTime(finishedAt)
	if err != nil {
		return run, "", err
	}
	parsedScheduledAt, err := parseMetaTime(scheduledAt)
	if err != nil {
		return run, "", err
	}
	run.Trigger = v1alpha1.RunTrigger(trigger)
	run.StartedAt = parsedStartedAt
	run.FinishedAt = parsedFinishedAt
	run.ScheduledAt = parsedScheduledAt
	run.Plan, err = unmarshalPhaseSummary(plan)
	if err != nil {
		return run, "", err
	}
	run.Apply, err = unmarshalPhaseSummary(apply)
	if err != nil {
		return run, "", err
	}
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

func runStartedAt(run v1alpha1.Run) string {
	if value := formatMetaTime(run.StartedAt); value != "" {
		return value
	}
	if run.Plan != nil {
		return formatMetaTime(run.Plan.StartedAt)
	}
	return ""
}

func runFinishedAt(run v1alpha1.Run) string {
	if value := formatMetaTime(run.FinishedAt); value != "" {
		return value
	}
	if run.Apply != nil {
		return formatMetaTime(run.Apply.FinishedAt)
	}
	if run.Plan != nil && run.Plan.Result == v1alpha1.RunLogResultFailed {
		return formatMetaTime(run.Plan.FinishedAt)
	}
	return ""
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

func marshalPhaseSummary(summary *v1alpha1.RunPhaseSummary) (any, error) {
	if summary == nil {
		return nil, nil
	}
	body, err := json.Marshal(summary)
	if err != nil {
		return nil, fmt.Errorf("marshal phase summary: %w", err)
	}
	return string(body), nil
}

func unmarshalPhaseSummary(raw sql.NullString) (*v1alpha1.RunPhaseSummary, error) {
	if !raw.Valid || raw.String == "" {
		return nil, nil
	}
	var summary v1alpha1.RunPhaseSummary
	if err := json.Unmarshal([]byte(raw.String), &summary); err != nil {
		return nil, fmt.Errorf("unmarshal phase summary: %w", err)
	}
	return &summary, nil
}

func requiredPhaseSummary(raw sql.NullString) (*v1alpha1.RunPhaseSummary, error) {
	summary, err := unmarshalPhaseSummary(raw)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		return nil, ErrNotFound
	}
	return summary, nil
}

func postgresURLFromEnv() string {
	host := envOrDefault(envPostgresHost, defaultPostgresHost)
	port := envOrDefault(envPostgresPort, defaultPostgresPort)
	database := envOrDefault(envPostgresDatabase, defaultPostgresDatabase)
	user := envOrDefault(envPostgresUser, defaultPostgresUser)
	password := os.Getenv(envPostgresPassword)
	sslMode := envOrDefault(envPostgresSSLMode, defaultPostgresSSLMode)

	hostPort := host
	if port != "" {
		hostPort = net.JoinHostPort(host, port)
	}

	u := url.URL{
		Scheme: "postgres",
		Host:   hostPort,
		Path:   "/" + strings.TrimPrefix(database, "/"),
	}
	if password != "" {
		u.User = url.UserPassword(user, password)
	} else if user != "" {
		u.User = url.User(user)
	}
	if sslMode != "" {
		query := u.Query()
		query.Set("sslmode", sslMode)
		u.RawQuery = query.Encode()
	}
	return u.String()
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
