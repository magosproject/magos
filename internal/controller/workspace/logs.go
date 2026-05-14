/*
Copyright 2026. The Magos Authors.
Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/
package workspace

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"io"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

const maxLogBytes = 50 * 1024 * 1024 // 50 MiB
// policyResult mirrors the structured output emitted by the plan job when
// policy validation runs. The workspace controller parses this from pod logs.
type policyResult struct {
	Passed     bool                       `json:"passed"`
	Violations []v1alpha1.PolicyViolation `json:"violations"`
}

// readPolicyViolations reads the pod logs for a completed plan job and extracts
// the MAGOS_RESULT line emitted by the kyverno-json validation step.
func (r *WorkspaceReconciler) readPolicyViolations(ctx context.Context, namespace, jobName string) ([]v1alpha1.PolicyViolation, error) {
	var podList corev1.PodList
	if err := r.List(ctx, &podList,
		client.InNamespace(namespace),
		client.MatchingLabels{"job-name": jobName},
	); err != nil {
		return nil, fmt.Errorf("failed to list pods for job %s: %w", jobName, err)
	}
	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("no pods found for job %s", jobName)
	}
	pod := &podList.Items[0]
	logStream, err := r.Clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{}).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to stream logs for pod %s: %w", pod.Name, err)
	}
	defer func() {
		if err := logStream.Close(); err != nil {
			log.FromContext(ctx).Error(err, "Failed to close pod log stream")
		}
	}()
	scanner := bufio.NewScanner(logStream)
	for scanner.Scan() {
		line := scanner.Text()
		if resultJSON, ok := strings.CutPrefix(line, "MAGOS_RESULT:"); ok {
			var result policyResult
			if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
				return nil, fmt.Errorf("failed to parse MAGOS_RESULT: %w", err)
			}
			return result.Violations, nil
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("error reading pod logs: %w", err)
	}
	return nil, nil
}
func (r *WorkspaceReconciler) getJobPod(ctx context.Context, namespace, jobName string) (*corev1.Pod, error) {
	var podList corev1.PodList
	if err := r.List(ctx, &podList,
		client.InNamespace(namespace),
		client.MatchingLabels{"job-name": jobName},
	); err != nil {
		return nil, fmt.Errorf("failed to list pods for job %s: %w", jobName, err)
	}
	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("no pods found for job %s", jobName)
	}
	return &podList.Items[0], nil
}
func (r *WorkspaceReconciler) readPodLogs(ctx context.Context, namespace, podName string) ([]byte, error) {
	logStream, err := r.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{}).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to stream logs for pod %s: %w", podName, err)
	}
	defer func() {
		if err := logStream.Close(); err != nil {
			log.FromContext(ctx).Error(err, "Failed to close pod log stream")
		}
	}()
	// Cap the read at maxLogBytes. LimitReader stops silently at the limit, so
	// we read one byte beyond to detect truncation and append a clear marker.
	data, err := io.ReadAll(io.LimitReader(logStream, maxLogBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read pod log stream for %s: %w", podName, err)
	}
	if len(data) > maxLogBytes {
		data = append(data[:maxLogBytes], []byte("\n[log truncated: exceeded 50 MiB limit]")...)
	}
	return data, nil
}
func gzipLogContent(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, fmt.Errorf("gzip log content: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize gzip log content: %w", err)
	}
	return buf.Bytes(), nil
}
func terminalJobFinishedAt(job *batchv1.Job) *metav1.Time {
	if job.Status.CompletionTime != nil {
		return job.Status.CompletionTime
	}
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			t := cond.LastTransitionTime
			return &t
		}
	}
	return nil
}

// archiveRunLogs reads the pod logs for the given job, compresses them, writes
// the blob to RustFS, and records run metadata through the API. The API owns
// Postgres-backed run metadata, so the controller never writes the database directly.
func (r *WorkspaceReconciler) archiveRunLogs(
	ctx context.Context,
	workspace *v1alpha1.Workspace,
	job *batchv1.Job,
	phase v1alpha1.RunPhase,
	result v1alpha1.RunLogResult,
) error {
	if r.LogStore == nil || r.Clientset == nil || r.RunRecorder == nil {
		return nil
	}
	// CurrentRunID is set when a plan and apply run starts and shared by both the
	// plan and apply jobs. If it is absent the workspace has not started a run
	// yet and there is nothing to archive.
	runID := workspace.Status.CurrentRunID
	if runID == "" && job.Labels != nil {
		runID = job.Labels[runIDLabelKey]
		workspace.Status.CurrentRunID = runID
	}
	if runID == "" {
		return nil
	}
	trigger := ensureCurrentRunTrigger(workspace)
	pod, err := r.getJobPod(ctx, workspace.Namespace, job.Name)
	if err != nil {
		return err
	}
	rawLogs, err := r.readPodLogs(ctx, workspace.Namespace, pod.Name)
	if err != nil {
		return err
	}
	compressed, err := gzipLogContent(rawLogs)
	if err != nil {
		return err
	}
	logKey, err := r.LogStore.PutRunPhaseLog(ctx, workspace.Namespace, workspace.Name, runID, phase, compressed)
	if err != nil {
		return err
	}
	phaseSummary := &v1alpha1.RunPhaseSummary{
		JobName:      job.Name,
		PodName:      pod.Name,
		StartedAt:    job.Status.StartTime,
		FinishedAt:   terminalJobFinishedAt(job),
		Result:       result,
		LogKey:       logKey,
		LogSizeBytes: int64(len(compressed)),
	}
	run := v1alpha1.Run{
		ID:               runID,
		Trigger:          trigger,
		TargetRevision:   workspace.Spec.Source.TargetRevision,
		ObservedRevision: currentRunObservedRevision(workspace),
	}
	switch phase {
	case v1alpha1.RunPhasePlan:
		run.Plan = phaseSummary
		run.StartedAt = phaseSummary.StartedAt
		if result == v1alpha1.RunLogResultFailed {
			run.FinishedAt = phaseSummary.FinishedAt
		}
	case v1alpha1.RunPhaseApply:
		run.Apply = phaseSummary
		run.FinishedAt = phaseSummary.FinishedAt
	}
	return r.RunRecorder.RecordRunPhase(ctx, workspace.Namespace, workspace.Name, runID, phase, run)
}
