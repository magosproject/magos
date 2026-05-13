package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/magosproject/magos/internal/logstore"
	"github.com/magosproject/magos/types/magosproject/v1alpha1"
)

const (
	envLogsAPIURL = "MAGOS_LOGS_API_URL"
)

type RunRecorder interface {
	RecordRunPhase(ctx context.Context, namespace, workspace, runID string, phase v1alpha1.RunPhase, run v1alpha1.Run) error
	RecordRun(ctx context.Context, namespace, workspace string, run v1alpha1.Run) error
}

type HTTPRunRecorder struct {
	baseURL string
	client  *http.Client
}

type recordRunPhaseRequest struct {
	Run v1alpha1.Run `json:"run"`
}

func NewHTTPRunRecorderFromEnv() (RunRecorder, error) {
	if !logstore.LoadConfigFromEnv().Enabled() {
		return nil, nil
	}

	baseURL := strings.TrimRight(os.Getenv(envLogsAPIURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("%s must be set when log storage is enabled", envLogsAPIURL)
	}

	return &HTTPRunRecorder{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

func (r *HTTPRunRecorder) RecordRunPhase(ctx context.Context, namespace, workspace, runID string, phase v1alpha1.RunPhase, run v1alpha1.Run) error {
	endpoint, err := url.JoinPath(
		r.baseURL,
		"internal",
		"apis",
		"magosproject.io",
		"v1alpha1",
		"workspaces",
		namespace,
		workspace,
		"runs",
		runID,
		"phases",
		string(phase),
	)
	if err != nil {
		return fmt.Errorf("build run phase record endpoint: %w", err)
	}

	body, err := json.Marshal(recordRunPhaseRequest{Run: run})
	if err != nil {
		return fmt.Errorf("marshal run phase record request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create run phase record request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("record run phase summary: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
	return fmt.Errorf("record run phase summary: status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
}

func (r *HTTPRunRecorder) RecordRun(ctx context.Context, namespace, workspace string, run v1alpha1.Run) error {
	endpoint, err := url.JoinPath(
		r.baseURL,
		"internal",
		"apis",
		"magosproject.io",
		"v1alpha1",
		"workspaces",
		namespace,
		workspace,
		"runs",
		run.ID,
	)
	if err != nil {
		return fmt.Errorf("build run record endpoint: %w", err)
	}

	body, err := json.Marshal(recordRunPhaseRequest{Run: run})
	if err != nil {
		return fmt.Errorf("marshal run record request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create run record request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("record run: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
	return fmt.Errorf("record run: status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
}
