package logstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/magosproject/magos/types/magosproject/v1alpha1"
)

const (
	envLogsEnabled           = "MAGOS_LOGS_ENABLED"
	envLogsRetention         = "MAGOS_LOGS_RETENTION"
	envLogsS3Endpoint        = "MAGOS_LOGS_S3_ENDPOINT"
	envLogsS3AccessKeyID     = "MAGOS_LOGS_S3_ACCESS_KEY_ID"
	envLogsS3SecretAccessKey = "MAGOS_LOGS_S3_SECRET_ACCESS_KEY"

	DefaultRetention = 30
	defaultBucket    = "magos-run-logs"

	maxDescendingTS = int64(math.MaxInt64)
)

// Config holds the user-facing log storage settings. The storage backend is
// an internal concern and is not exposed.
type Config struct {
	Enabled         bool
	Retention       int
	endpoint        string
	accessKeyID     string
	secretAccessKey string
}

// Store is the interface for reading and writing run logs and reconcile run
// summaries. The implementation is expected to be backed by an S3-compatible
// object store.
type Store interface {
	// PutRunLog stores the compressed log body for one phase of a reconcile
	// run and returns the object key. The key is deterministic and can be
	// re-derived with RunLogKey when the stored value is unavailable.
	PutRunLog(ctx context.Context, namespace, workspace, runID string, phase v1alpha1.RunPhase, body []byte) (string, error)

	// UpsertReconcileRun writes or merges a reconcile run summary. When a
	// summary for the given RunID already exists the plan and apply fields are
	// merged independently so the controller can call this after each phase
	// without overwriting the other.
	UpsertReconcileRun(ctx context.Context, namespace, workspace string, run v1alpha1.ReconcileRun) error

	// ListReconcileRuns returns up to limit recent reconcile runs for the
	// workspace, ordered newest first. Pass the cursor returned by a previous
	// call to page through older results.
	ListReconcileRuns(ctx context.Context, namespace, workspace string, limit int, cursor string) ([]v1alpha1.ReconcileRun, string, error)

	// GetRunLog returns a reader for the decompressed log identified by key.
	// Callers are responsible for closing the returned reader.
	GetRunLog(ctx context.Context, key string) (io.ReadCloser, error)

	// PruneOldRuns deletes the oldest reconcile run summaries and their
	// associated log objects when the total exceeds the retention count.
	PruneOldRuns(ctx context.Context, namespace, workspace string, retention int) error
}

// RunLogKey returns the deterministic object-store key for a phase log. The
// key is derived entirely from the run identity so it can be reconstructed
// without a summary lookup.
func RunLogKey(namespace, workspace, runID string, phase v1alpha1.RunPhase) string {
	return path.Join("run-logs", namespace, workspace, runID, string(phase)+".log.gz")
}

func LoadConfigFromEnv() Config {
	retention := DefaultRetention
	if raw := os.Getenv(envLogsRetention); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			retention = v
		}
	}
	return Config{
		Enabled:         parseBoolEnv(envLogsEnabled, false),
		Retention:       retention,
		endpoint:        os.Getenv(envLogsS3Endpoint),
		accessKeyID:     os.Getenv(envLogsS3AccessKeyID),
		secretAccessKey: os.Getenv(envLogsS3SecretAccessKey),
	}
}

func (c Config) validate() error {
	if !c.Enabled {
		return nil
	}
	if c.endpoint == "" {
		return fmt.Errorf("%s must be set when log storage is enabled", envLogsS3Endpoint)
	}
	if c.accessKeyID == "" {
		return fmt.Errorf("%s must be set when log storage is enabled", envLogsS3AccessKeyID)
	}
	if c.secretAccessKey == "" {
		return fmt.Errorf("%s must be set when log storage is enabled", envLogsS3SecretAccessKey)
	}
	return nil
}

func NewStore(ctx context.Context, cfg Config) (Store, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return newS3Store(ctx, cfg)
}

type s3Store struct {
	client *s3.Client
	bucket string
}

func newS3Store(ctx context.Context, cfg Config) (Store, error) {
	endpointURL, err := url.Parse(cfg.endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid S3 endpoint %q: %w", cfg.endpoint, err)
	}

	// Region is required by the AWS SDK but not meaningful for most
	// S3-compatible backends; any non-empty value satisfies the SDK.
	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.accessKeyID, cfg.secretAccessKey, "")),
		config.WithBaseEndpoint(endpointURL.String()),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	// Path-style addressing is required for S3-compatible stores that are not
	// AWS S3 native (virtual-hosted-style is an AWS-specific convention).
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	store := &s3Store{client: client, bucket: defaultBucket}
	if err := store.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *s3Store) ensureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "bucketalreadyownedbyyou") && !strings.Contains(strings.ToLower(err.Error()), "bucket already exists") {
		return fmt.Errorf("ensure bucket %q: %w", s.bucket, err)
	}
	return nil
}

func (s *s3Store) PutRunLog(ctx context.Context, namespace, workspace, runID string, phase v1alpha1.RunPhase, body []byte) (string, error) {
	key := RunLogKey(namespace, workspace, runID, phase)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:          aws.String(s.bucket),
		Key:             aws.String(key),
		Body:            bytes.NewReader(body),
		ContentType:     aws.String("text/plain"),
		ContentEncoding: aws.String("gzip"),
	})
	if err != nil {
		return "", fmt.Errorf("put log object %q: %w", key, err)
	}
	return key, nil
}

// UpsertReconcileRun writes or merges a reconcile run summary. If a summary
// already exists for this RunID (identified by its deterministic key) the
// incoming Plan and Apply fields are merged into the existing record so both
// phases can be written independently without clobbering each other.
func (s *s3Store) UpsertReconcileRun(ctx context.Context, namespace, workspace string, run v1alpha1.ReconcileRun) error {
	t, err := parseRunIDTime(run.RunID)
	if err != nil {
		return err
	}
	key := reconcileSummaryKey(namespace, workspace, t, run.RunID)

	// Merge with any existing summary for this run so that archiving the plan
	// and apply phases independently produces a single coherent record.
	if existing, readErr := s.readReconcileRun(ctx, key); readErr == nil {
		if run.Plan != nil {
			existing.Plan = run.Plan
		}
		if run.Apply != nil {
			existing.Apply = run.Apply
		}
		if run.FinishedAt != nil {
			existing.FinishedAt = run.FinishedAt
		}
		run = *existing
	}

	body, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("marshal reconcile run %q: %w", run.RunID, err)
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("put reconcile run summary %q: %w", key, err)
	}
	return nil
}

// ListReconcileRuns returns up to limit reconcile run summaries for the
// workspace, ordered newest first. The cursor is the S3 object key of the
// last item from the previous page; pass an empty string to start from the
// most recent run.
//
// Because summary keys embed a descending timestamp, S3's lexicographic
// listing already returns runs in newest-first order. We fetch at most
// limit+1 keys to detect whether a next page exists, then download only the
// limit summaries we actually need, in parallel.
func (s *s3Store) ListReconcileRuns(ctx context.Context, namespace, workspace string, limit int, cursor string) ([]v1alpha1.ReconcileRun, string, error) {
	if limit <= 0 {
		limit = DefaultRetention
	}

	prefix := reconcileSummaryPrefix(namespace, workspace)
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(int32(limit + 1)),
	}
	if cursor != "" {
		input.StartAfter = aws.String(cursor)
	}

	out, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, "", fmt.Errorf("list reconcile runs with prefix %q: %w", prefix, err)
	}

	hasMore := len(out.Contents) > limit
	objects := out.Contents
	if hasMore {
		objects = out.Contents[:limit]
	}

	// Fetch each summary in parallel to avoid paying per-object round-trip
	// latency sequentially.
	type result struct {
		index int
		run   v1alpha1.ReconcileRun
		err   error
	}
	results := make([]result, len(objects))
	var wg sync.WaitGroup
	for i, obj := range objects {
		wg.Add(1)
		go func(i int, key string) {
			defer wg.Done()
			run, err := s.readReconcileRun(ctx, key)
			if err != nil {
				results[i] = result{index: i, err: err}
				return
			}
			results[i] = result{index: i, run: *run}
		}(i, aws.ToString(obj.Key))
	}
	wg.Wait()

	runs := make([]v1alpha1.ReconcileRun, len(objects))
	for _, r := range results {
		if r.err != nil {
			return nil, "", r.err
		}
		runs[r.index] = r.run
	}

	nextCursor := ""
	if hasMore {
		nextCursor = aws.ToString(objects[len(objects)-1].Key)
	}
	return runs, nextCursor, nil
}

func (s *s3Store) GetRunLog(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get log object %q: %w", key, err)
	}
	return out.Body, nil
}

// PruneOldRuns deletes reconcile run summaries and their log objects when the
// total exceeds retention. In steady state the workspace holds at most
// retention+1 summaries (the retention kept runs plus the one just written),
// so the first ListObjectsV2 call returns at most retention+1 keys and the
// function exits without scanning further. A full paginated scan only occurs
// on the first prune after a large backlog has accumulated.
func (s *s3Store) PruneOldRuns(ctx context.Context, namespace, workspace string, retention int) error {
	prefix := reconcileSummaryPrefix(namespace, workspace)

	// Fetch one more than the retention limit. If we get retention or fewer
	// keys back there is nothing to prune and we are done in one API call.
	firstPage, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(int32(retention + 1)),
	})
	if err != nil {
		return fmt.Errorf("list summaries for pruning with prefix %q: %w", prefix, err)
	}
	if len(firstPage.Contents) <= retention {
		return nil
	}

	// The key at index retention is the oldest run we want to keep. Everything
	// lexicographically after it is a candidate for deletion.
	cutoffKey := aws.ToString(firstPage.Contents[retention].Key)

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket:     aws.String(s.bucket),
		Prefix:     aws.String(prefix),
		StartAfter: aws.String(cutoffKey),
	})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("list old summaries for deletion with prefix %q: %w", prefix, err)
		}
		for _, obj := range out.Contents {
			summaryKey := aws.ToString(obj.Key)
			runID := extractRunIDFromSummaryKey(summaryKey)
			for _, phase := range []v1alpha1.RunPhase{v1alpha1.RunPhasePlan, v1alpha1.RunPhaseApply} {
				// Best-effort: the log may not exist if only one phase ran.
				_ = s.deleteObject(ctx, RunLogKey(namespace, workspace, runID, phase))
			}
			if err := s.deleteObject(ctx, summaryKey); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *s3Store) readReconcileRun(ctx context.Context, key string) (*v1alpha1.ReconcileRun, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get reconcile run summary %q: %w", key, err)
	}
	defer func() { _ = out.Body.Close() }()

	var run v1alpha1.ReconcileRun
	if err := json.NewDecoder(out.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("decode reconcile run summary %q: %w", key, err)
	}
	return &run, nil
}

func (s *s3Store) deleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object %q: %w", key, err)
	}
	return nil
}

// reconcileSummaryPrefix returns the common S3 prefix for all reconcile run
// summaries belonging to a workspace.
func reconcileSummaryPrefix(namespace, workspace string) string {
	return path.Join("run-summaries", namespace, workspace) + "/"
}

// reconcileSummaryKey builds the full S3 key for a reconcile run summary. The
// descending timestamp prefix ensures that S3's lexicographic listing returns
// the most recent runs first without any client-side sorting.
func reconcileSummaryKey(namespace, workspace string, t time.Time, runID string) string {
	return path.Join(reconcileSummaryPrefix(namespace, workspace), fmt.Sprintf("%s_%s.json", descendingTimestamp(t), runID))
}

// parseRunIDTime extracts the UTC timestamp encoded in the leading segment of
// a runID. RunIDs have the form "20060102T150405-{hex}", so parsing just the
// prefix is sufficient to reconstruct the summary key.
func parseRunIDTime(runID string) (time.Time, error) {
	parts := strings.SplitN(runID, "-", 2)
	if len(parts) == 0 || parts[0] == "" {
		return time.Time{}, fmt.Errorf("invalid runID %q: missing timestamp prefix", runID)
	}
	t, err := time.ParseInLocation("20060102T150405", parts[0], time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid runID %q: %w", runID, err)
	}
	return t, nil
}

// extractRunIDFromSummaryKey parses the runID out of a summary object key.
// Summary keys have the form "run-summaries/{ns}/{ws}/{desc_ts}_{runID}.json".
func extractRunIDFromSummaryKey(key string) string {
	base := path.Base(key)
	base = strings.TrimSuffix(base, ".json")
	// Key segment is "{desc_ts}_{runID}"; the runID starts after the first "_".
	idx := strings.Index(base, "_")
	if idx < 0 {
		return base
	}
	return base[idx+1:]
}

func descendingTimestamp(t time.Time) string {
	value := maxDescendingTS - t.UTC().UnixNano()
	if value < 0 {
		value = 0
	}
	return fmt.Sprintf("%019d", value)
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
