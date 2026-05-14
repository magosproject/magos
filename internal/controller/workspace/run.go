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
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// DefaultReconciliationInterval is the fallback duration between scheduled
	// reconciliations when no magosproject.io/reconcile-interval annotation is set.
	DefaultReconciliationInterval = 3 * time.Minute

	// scheduledReconcileReason is the reset reason used when a periodic reconcile
	// fires. It surfaces in Workspace status and run records as the trigger.
	scheduledReconcileReason = "ScheduledReconcile"
)

// computeNextReconcileTime is the single source of truth for the Workspace
// reconcile interval and schedule cadence. It resolves the effective
// interval from the magosproject.io/reconcile-interval annotation when
// present and valid, or falls back to DefaultReconciliationInterval
// otherwise. It also remaps a persisted NextReconcileTime onto the new
// cadence when the configured interval changes, so annotation updates take
// effect immediately instead of waiting for the previously scheduled time.
func computeNextReconcileTime(ws *v1alpha1.Workspace, existing *metav1.Time) (metav1.Time, time.Duration, bool) {
	interval := DefaultReconciliationInterval
	if ws.Annotations != nil {
		if val, ok := ws.Annotations[v1alpha1.WorkspaceReconcileIntervalAnnotation]; ok {
			if d, err := time.ParseDuration(val); err == nil && d > 0 {
				interval = d
			}
		}
	}
	now := time.Now()

	// this condition triggers on the first reconcile when there is
	// no existing nextReconcileTime set yet
	if existing == nil || existing.IsZero() {
		return metav1.NewTime(now.Add(interval)), interval, false
	}

	next := existing.Time

	// this condition triggers when the reconcileInterval annotation on
	// the workspace was updated between reconciles for example when a user
	// changes the annotation to lower the reconcile interval. this check
	// ensures the updated interval takes effect immediately instead of waiting
	// for the previously scheduled time to elapse
	if ws.Status.ObservedReconcileInterval != "" && ws.Status.ObservedReconcileInterval != interval.String() {
		previousInterval, err := time.ParseDuration(ws.Status.ObservedReconcileInterval)
		if err == nil && previousInterval > 0 {
			for next.After(now) {
				next = next.Add(-previousInterval)
			}
			next = next.Add(interval)
		}
	}

	// this is the usual condition that triggers when the scheduled time arrives.
	// we update the nextReconcileTime by adding the interval with the original
	// nextReconcileTime as the base, so that we maintain a consistent cadence
	// even if individual reconciles are delayed for some reason (e.g. cluster load,
	// long terraform execution).
	due := !next.After(now)
	for !next.After(now) {
		next = next.Add(interval)
	}

	return metav1.NewTime(next), interval, due
}

// withScheduledRequeue keeps any deliberately shorter requeue while preventing
// stale schedule durations from drifting after status update latency.
func withScheduledRequeue(res ctrl.Result, next time.Time) ctrl.Result {
	scheduled := time.Until(next)
	if scheduled <= 0 {
		scheduled = time.Nanosecond
	}
	if res.RequeueAfter == 0 || res.RequeueAfter > scheduled {
		res.RequeueAfter = scheduled
	}
	return res
}

func newRunID() string {
	now := time.Now().UTC()
	var suffix [4]byte
	if _, err := crand.Read(suffix[:]); err != nil {
		// Fall back to a timestamp-derived suffix so the ID is always in the
		// form "20060102T150405-{hex}" that parseRunIDTime expects.
		return fmt.Sprintf("%s-%08x", now.Format("20060102T150405"), now.UnixNano()&0xffffffff)
	}
	return fmt.Sprintf("%s-%s", now.Format("20060102T150405"), hex.EncodeToString(suffix[:]))
}

// ensureRunMetadata returns the workspace's current run ID, minting one when a
// run starts. It also stamps the run trigger. Initial runs do not pass through
// the reset path, so job creation is the first point where both values must be
// made durable for log archival.
func ensureRunMetadata(workspace *v1alpha1.Workspace) string {
	if workspace.Status.CurrentRunID == "" {
		workspace.Status.CurrentRunID = newRunID()
		now := metav1.Now()
		workspace.Status.LastRunStartedAt = &now
	}
	ensureCurrentRunTrigger(workspace)
	return workspace.Status.CurrentRunID
}

func ensureCurrentRunTrigger(workspace *v1alpha1.Workspace) v1alpha1.RunTrigger {
	if workspace.Status.CurrentRunTrigger == "" {
		workspace.Status.CurrentRunTrigger = inferMissingRunTrigger(workspace)
	}
	return workspace.Status.CurrentRunTrigger
}

func inferMissingRunTrigger(workspace *v1alpha1.Workspace) v1alpha1.RunTrigger {
	if workspace.Annotations != nil {
		if detected := workspace.Annotations[v1alpha1.WorkspaceDetectedRevisionAnnotation]; detected != "" && detected != workspace.Status.ObservedRevision {
			return v1alpha1.RunTriggerRevision
		}
		if workspace.Annotations[v1alpha1.WorkspaceReconcileRequestAnnotation] != "" {
			return v1alpha1.RunTriggerManual
		}
	}
	return v1alpha1.RunTriggerConfig
}

func currentRunObservedRevision(workspace *v1alpha1.Workspace) string {
	if workspace.Annotations != nil {
		if sha := workspace.Annotations[v1alpha1.WorkspaceDetectedRevisionAnnotation]; sha != "" {
			return sha
		}
	}
	return workspace.Spec.Source.TargetRevision
}

func runTriggerFromResetReason(reason string) v1alpha1.RunTrigger {
	switch reason {
	case "ConfigurationChanged":
		return v1alpha1.RunTriggerConfig
	case "ManualReconcileRequested":
		return v1alpha1.RunTriggerManual
	case scheduledReconcileReason:
		return v1alpha1.RunTriggerScheduled
	case "NewRevisionDetected":
		return v1alpha1.RunTriggerRevision
	case "RetryApply", "RetryPlan":
		return v1alpha1.RunTriggerRetry
	default:
		return v1alpha1.RunTriggerUnknown
	}
}

func terminalWorkspaceRunRecorded(phase v1alpha1.Phase) bool {
	switch phase {
	case v1alpha1.PhaseApplied, v1alpha1.PhaseFailed, v1alpha1.PhaseValidationFailed:
		return true
	default:
		return false
	}
}
