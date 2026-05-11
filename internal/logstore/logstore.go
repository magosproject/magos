package logstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/magosproject/magos/types/magosproject/v1alpha1"
)

const (
	envLogsS3Endpoint        = "MAGOS_LOGS_S3_ENDPOINT"
	envLogsS3AccessKeyID     = "MAGOS_LOGS_S3_ACCESS_KEY_ID"
	envLogsS3SecretAccessKey = "MAGOS_LOGS_S3_SECRET_ACCESS_KEY"

	defaultBucket = "magos-run-logs"
)

type Config struct {
	endpoint        string
	accessKeyID     string
	secretAccessKey string
}

func (c Config) Enabled() bool {
	return c.endpoint != ""
}

// Store provides persistence for compressed run log blobs.
// The default implementation uses RustFS through the S3-compatible API.
type Store interface {
	// PutRunPhaseLog stores the compressed log body for a single phase
	// (plan or apply) of a run and returns its object key.
	// The returned key must be deterministic and re-derivable via RunLogKey.
	PutRunPhaseLog(ctx context.Context, namespace, workspace, runID string, phase v1alpha1.RunPhase, body []byte) (string, error)

	// GetRunPhaseLog returns a reader for the compressed log identified by key.
	// The caller must close the returned reader.
	GetRunPhaseLog(ctx context.Context, key string) (io.ReadCloser, error)

	// DeleteRunPhaseLog deletes the compressed log identified by key.
	DeleteRunPhaseLog(ctx context.Context, key string) error
}

// RunLogKey returns the deterministic object-store key for a phase log. The key
// is derived entirely from the run identity so it can be reconstructed without
// a summary lookup.
func RunLogKey(namespace, workspace, runID string, phase v1alpha1.RunPhase) string {
	return path.Join("run-logs", namespace, workspace, runID, string(phase)+".log.gz")
}

func LoadConfigFromEnv() Config {
	return Config{
		endpoint:        os.Getenv(envLogsS3Endpoint),
		accessKeyID:     os.Getenv(envLogsS3AccessKeyID),
		secretAccessKey: os.Getenv(envLogsS3SecretAccessKey),
	}
}

func (c Config) validate() error {
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
	if !cfg.Enabled() {
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

	// Region is required by the AWS SDK but ignored by most S3-compatible
	// backends; any non-empty value is sufficient.
	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.accessKeyID, cfg.secretAccessKey, "")),
		config.WithBaseEndpoint(endpointURL.String()),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	// Path-style is required for S3-compatible stores other than AWS S3.
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

// withBucketEnsure calls fn and, if it fails with NoSuchBucket, recreates the
// bucket and retries once. This recovers from a storage backend restart that
// drops bucket state without requiring a controller restart.
func (s *s3Store) withBucketEnsure(ctx context.Context, fn func() error) error {
	err := fn()
	if err != nil && strings.Contains(err.Error(), "NoSuchBucket") {
		if ensureErr := s.ensureBucket(ctx); ensureErr == nil {
			err = fn()
		}
	}
	return err
}

func (s *s3Store) PutRunPhaseLog(ctx context.Context, namespace, workspace, runID string, phase v1alpha1.RunPhase, body []byte) (string, error) {
	key := RunLogKey(namespace, workspace, runID, phase)
	err := s.withBucketEnsure(ctx, func() error {
		_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:          aws.String(s.bucket),
			Key:             aws.String(key),
			Body:            bytes.NewReader(body),
			ContentType:     aws.String("text/plain"),
			ContentEncoding: aws.String("gzip"),
		})
		return err
	})
	if err != nil {
		return "", fmt.Errorf("put log object %q: %w", key, err)
	}
	return key, nil
}

func (s *s3Store) GetRunPhaseLog(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get log object %q: %w", key, err)
	}
	return out.Body, nil
}

func (s *s3Store) DeleteRunPhaseLog(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object %q: %w", key, err)
	}
	return nil
}
