package logstore

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/magosproject/magos/types/magosproject/v1alpha1"
)

const (
	EnvLogsEnabled           = "MAGOS_LOGS_ENABLED"
	EnvLogsS3Bucket          = "MAGOS_LOGS_S3_BUCKET"
	EnvLogsS3Region          = "MAGOS_LOGS_S3_REGION"
	EnvLogsS3Endpoint        = "MAGOS_LOGS_S3_ENDPOINT"
	EnvLogsS3AccessKeyID     = "MAGOS_LOGS_S3_ACCESS_KEY_ID"
	EnvLogsS3SecretAccessKey = "MAGOS_LOGS_S3_SECRET_ACCESS_KEY"
	EnvLogsS3ForcePathStyle  = "MAGOS_LOGS_S3_FORCE_PATH_STYLE"
	EnvLogsS3InsecureSkipTLS = "MAGOS_LOGS_S3_INSECURE_SKIP_TLS_VERIFY"

	defaultListLimit = 5
	maxDescendingTS  = int64(math.MaxInt64)
)

type Config struct {
	Enabled bool
	S3      S3Config
}

type S3Config struct {
	Bucket                string
	Region                string
	Endpoint              string
	AccessKeyID           string
	SecretAccessKey       string
	ForcePathStyle        bool
	InsecureSkipTLSVerify bool
}

type PutRunLogInput struct {
	Namespace string
	Workspace string
	RunID     string
	Phase     v1alpha1.RunPhase
	StartedAt time.Time
}

type PutRunLogResult struct {
	Key       string
	SizeBytes int64
}

type ObjectMeta struct {
	Key       string
	SizeBytes int64
}

type ListRunSummariesInput struct {
	Namespace string
	Workspace string
	Phase     v1alpha1.RunPhase
	Limit     int
	Cursor    string
}

type ListRunSummariesResult struct {
	Items      []v1alpha1.RunLogSummary
	NextCursor string
}

type Store interface {
	PutRunLog(ctx context.Context, in PutRunLogInput, body []byte) (PutRunLogResult, error)
	PutRunSummary(ctx context.Context, namespace, workspace string, summary v1alpha1.RunLogSummary) error
	ListRunSummaries(ctx context.Context, in ListRunSummariesInput) (ListRunSummariesResult, error)
	FindRunSummary(ctx context.Context, namespace, workspace string, phase v1alpha1.RunPhase, runID string) (*v1alpha1.RunLogSummary, error)
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectMeta, error)
	Delete(ctx context.Context, key string) error
}

func LoadConfigFromEnv() Config {
	return Config{
		Enabled: parseBoolEnv(EnvLogsEnabled, false),
		S3: S3Config{
			Bucket:                os.Getenv(EnvLogsS3Bucket),
			Region:                envOrDefault(EnvLogsS3Region, "us-east-1"),
			Endpoint:              os.Getenv(EnvLogsS3Endpoint),
			AccessKeyID:           os.Getenv(EnvLogsS3AccessKeyID),
			SecretAccessKey:       os.Getenv(EnvLogsS3SecretAccessKey),
			ForcePathStyle:        parseBoolEnv(EnvLogsS3ForcePathStyle, true),
			InsecureSkipTLSVerify: parseBoolEnv(EnvLogsS3InsecureSkipTLS, false),
		},
	}
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.S3.Bucket == "" {
		return fmt.Errorf("%s must be set when log storage is enabled", EnvLogsS3Bucket)
	}
	if c.S3.Endpoint == "" {
		return fmt.Errorf("%s must be set when log storage is enabled", EnvLogsS3Endpoint)
	}
	if c.S3.AccessKeyID == "" {
		return fmt.Errorf("%s must be set when log storage is enabled", EnvLogsS3AccessKeyID)
	}
	if c.S3.SecretAccessKey == "" {
		return fmt.Errorf("%s must be set when log storage is enabled", EnvLogsS3SecretAccessKey)
	}
	return nil
}

func NewStore(ctx context.Context, cfg Config) (Store, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return NewS3Store(ctx, cfg.S3)
}

type s3Store struct {
	client *s3.Client
	bucket string
}

func NewS3Store(ctx context.Context, cfg S3Config) (Store, error) {
	endpointURL, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid S3 endpoint %q: %w", cfg.Endpoint, err)
	}

	httpClient := http.DefaultClient
	if cfg.InsecureSkipTLSVerify {
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}

	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(cfg.Region),
		config.WithHTTPClient(httpClient),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		config.WithBaseEndpoint(endpointURL.String()),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.ForcePathStyle
	})

	store := &s3Store{client: client, bucket: cfg.Bucket}
	if err := store.ensureBucket(ctx, cfg.Region); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *s3Store) ensureBucket(ctx context.Context, region string) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket:                    aws.String(s.bucket),
		CreateBucketConfiguration: createBucketConfiguration(region),
	})
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "bucketalreadyownedbyyou") && !strings.Contains(strings.ToLower(err.Error()), "bucket already exists") {
		return fmt.Errorf("ensure bucket %q: %w", s.bucket, err)
	}
	return nil
}

func (s *s3Store) PutRunLog(ctx context.Context, in PutRunLogInput, body []byte) (PutRunLogResult, error) {
	key := buildRunLogKey(in.Namespace, in.Workspace, in.Phase, in.StartedAt, in.RunID)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:          aws.String(s.bucket),
		Key:             aws.String(key),
		Body:            bytes.NewReader(body),
		ContentType:     aws.String("text/plain"),
		ContentEncoding: aws.String("gzip"),
	})
	if err != nil {
		return PutRunLogResult{}, fmt.Errorf("put object %q: %w", key, err)
	}
	return PutRunLogResult{Key: key, SizeBytes: int64(len(body))}, nil
}

func (s *s3Store) PutRunSummary(ctx context.Context, namespace, workspace string, summary v1alpha1.RunLogSummary) error {
	if summary.StartedAt == nil {
		return fmt.Errorf("run summary missing startedAt")
	}
	key := buildRunSummaryKey(namespace, workspace, summary.Phase, summary.StartedAt.Time, summary.RunID)
	body, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal run summary: %w", err)
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("put summary object %q: %w", key, err)
	}
	return nil
}

func (s *s3Store) ListRunSummaries(ctx context.Context, in ListRunSummariesInput) (ListRunSummariesResult, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	prefix := summaryPrefix(in.Namespace, in.Workspace, in.Phase)

	type summaryWithKey struct {
		key     string
		summary v1alpha1.RunLogSummary
	}

	summaries := make([]summaryWithKey, 0)
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return ListRunSummariesResult{}, fmt.Errorf("list summaries with prefix %q: %w", prefix, err)
		}

		for _, obj := range out.Contents {
			key := aws.ToString(obj.Key)
			if !strings.HasSuffix(key, ".json") {
				continue
			}
			summary, err := s.readSummary(ctx, key)
			if err != nil {
				return ListRunSummariesResult{}, err
			}
			summaries = append(summaries, summaryWithKey{key: key, summary: *summary})
		}
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		left := runSummarySortTime(summaries[i].summary)
		right := runSummarySortTime(summaries[j].summary)
		if !left.Equal(right) {
			return left.After(right)
		}
		return summaries[i].key > summaries[j].key
	})

	startIndex := 0
	if in.Cursor != "" {
		startIndex = len(summaries)
		for i, item := range summaries {
			if item.key == in.Cursor {
				startIndex = i + 1
				break
			}
		}
	}

	if startIndex > len(summaries) {
		startIndex = len(summaries)
	}

	endIndex := startIndex + limit
	if endIndex > len(summaries) {
		endIndex = len(summaries)
	}

	items := make([]v1alpha1.RunLogSummary, 0, endIndex-startIndex)
	for _, item := range summaries[startIndex:endIndex] {
		items = append(items, item.summary)
	}

	result := ListRunSummariesResult{Items: items}
	if endIndex < len(summaries) && endIndex > startIndex {
		result.NextCursor = summaries[endIndex-1].key
	}
	return result, nil
}

func (s *s3Store) FindRunSummary(ctx context.Context, namespace, workspace string, phase v1alpha1.RunPhase, runID string) (*v1alpha1.RunLogSummary, error) {
	cursor := ""
	for {
		page, err := s.ListRunSummaries(ctx, ListRunSummariesInput{
			Namespace: namespace,
			Workspace: workspace,
			Phase:     phase,
			Limit:     100,
			Cursor:    cursor,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range page.Items {
			if item.RunID == runID {
				found := item
				return &found, nil
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return nil, fmt.Errorf("run summary %q not found", runID)
}

func (s *s3Store) Get(ctx context.Context, key string) (io.ReadCloser, ObjectMeta, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, ObjectMeta{}, fmt.Errorf("get object %q: %w", key, err)
	}
	size := int64(0)
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return out.Body, ObjectMeta{Key: key, SizeBytes: size}, nil
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object %q: %w", key, err)
	}
	return nil
}

func (s *s3Store) readSummary(ctx context.Context, key string) (*v1alpha1.RunLogSummary, error) {
	body, _, err := s.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()

	var summary v1alpha1.RunLogSummary
	if err := json.NewDecoder(body).Decode(&summary); err != nil {
		return nil, fmt.Errorf("decode run summary %q: %w", key, err)
	}
	return &summary, nil
}

func logPrefix(namespace, workspace string, phase v1alpha1.RunPhase) string {
	return path.Join("run-logs", namespace, workspace, string(phase)) + "/"
}

func summaryPrefix(namespace, workspace string, phase v1alpha1.RunPhase) string {
	return path.Join("run-summaries", namespace, workspace, string(phase)) + "/"
}

func buildRunLogKey(namespace, workspace string, phase v1alpha1.RunPhase, startedAt time.Time, runID string) string {
	return path.Join(logPrefix(namespace, workspace, phase), fmt.Sprintf("%s_%s.log.gz", descendingTimestamp(startedAt), runID))
}

func buildRunSummaryKey(namespace, workspace string, phase v1alpha1.RunPhase, startedAt time.Time, runID string) string {
	return path.Join(summaryPrefix(namespace, workspace, phase), fmt.Sprintf("%s_%s.json", descendingTimestamp(startedAt), runID))
}

func descendingTimestamp(t time.Time) string {
	value := maxDescendingTS - t.UTC().UnixNano()
	if value < 0 {
		value = 0
	}
	return fmt.Sprintf("%019d", value)
}

func runSummarySortTime(summary v1alpha1.RunLogSummary) time.Time {
	if summary.FinishedAt != nil {
		return summary.FinishedAt.Time.UTC()
	}
	if summary.StartedAt != nil {
		return summary.StartedAt.Time.UTC()
	}
	return time.Time{}
}

func createBucketConfiguration(region string) *s3types.CreateBucketConfiguration {
	if region == "" || region == "us-east-1" {
		return nil
	}
	constraint := s3types.BucketLocationConstraint(region)
	return &s3types.CreateBucketConfiguration{LocationConstraint: constraint}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
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
