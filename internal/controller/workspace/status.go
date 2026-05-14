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
	"context"
	"time"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// updateStatus writes the phase, reason, message, and Ready condition to the
// Workspace status. workspace is the object the reconcile loop has
// been working with; latest is a fresh re-fetch of the same resource used as
// the write target to avoid conflict errors from stale resourceVersions.
// After a successful write, workspace is updated in-place from latest so that
// any subsequent logic in the same reconcile sees the current state.
func (r *WorkspaceReconciler) updateStatus(ctx context.Context, workspace *v1alpha1.Workspace, phase v1alpha1.Phase, reason, message string, status metav1.ConditionStatus) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &v1alpha1.Workspace{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(workspace), latest); err != nil {
			return err
		}

		needsUpdate := false

		if latest.Status.Phase != phase || latest.Status.Reason != reason || latest.Status.Message != message {
			if latest.Status.Phase != phase {
				phaseTransitionsTotal.WithLabelValues(workspace.Namespace, workspace.Name, string(phase)).Inc()
			}
			latest.Status.Phase = phase
			latest.Status.Reason = reason
			latest.Status.Message = message
			needsUpdate = true
		}

		// Preserve observed revision if it was set
		if workspace.Status.ObservedRevision != "" && latest.Status.ObservedRevision != workspace.Status.ObservedRevision {
			latest.Status.ObservedRevision = workspace.Status.ObservedRevision
			needsUpdate = true
		}

		// Carry the run ID and trigger forward when a new plan and apply run has
		// started in the in-memory copy. Both fields are written before the
		// first status update of a run, so they must survive the
		// optimistic-concurrency retry just as phase and message do.
		if workspace.Status.CurrentRunID != "" && latest.Status.CurrentRunID != workspace.Status.CurrentRunID {
			latest.Status.CurrentRunID = workspace.Status.CurrentRunID
			needsUpdate = true
		}

		if workspace.Status.CurrentRunTrigger != "" && latest.Status.CurrentRunTrigger != workspace.Status.CurrentRunTrigger {
			latest.Status.CurrentRunTrigger = workspace.Status.CurrentRunTrigger
			needsUpdate = true
		}

		if workspace.Status.LastRunStartedAt != nil && !workspace.Status.LastRunStartedAt.Equal(latest.Status.LastRunStartedAt) {
			latest.Status.LastRunStartedAt = workspace.Status.LastRunStartedAt
			needsUpdate = true
		}

		// Policy violations belong to the plan run that produced them. On
		// Pending or Planning a new job is starting, so we clear latest to
		// avoid showing stale failures next to a running job. Otherwise,
		// workspace.Status.PolicyViolations is only non-nil when
		// readPolicyViolations just populated it with failures from the plan
		// job that just finished, so we copy those into latest so they get
		// saved to the Kubernetes API and show up in kubectl describe and the
		// UI.
		if phase == v1alpha1.PhasePending || phase == v1alpha1.PhasePlanning {
			if len(latest.Status.PolicyViolations) > 0 {
				latest.Status.PolicyViolations = nil
				needsUpdate = true
			}
		} else if workspace.Status.PolicyViolations != nil {
			latest.Status.PolicyViolations = workspace.Status.PolicyViolations
			needsUpdate = true
		}

		now := metav1.Now()
		condition := metav1.Condition{
			Type:               v1alpha1.ConditionTypeReady,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: now,
		}

		if meta.SetStatusCondition(&latest.Status.Conditions, condition) {
			needsUpdate = true
		}

		if !needsUpdate {
			return nil
		}

		latest.Status.LastReconcileTime = &now

		if err := r.Status().Update(ctx, latest); err != nil {
			return err
		}

		// Update the original object so the caller has the latest state
		workspace.Status = latest.Status
		workspace.ResourceVersion = latest.ResourceVersion
		return nil
	})
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to update workspace status")
	}
}

// updateNextReconcileTime writes the expected next reconciliation time into the
// Workspace status so that the UI can display when the next sync will happen.
func (r *WorkspaceReconciler) updateNextReconcileTime(ctx context.Context, workspace *v1alpha1.Workspace, next metav1.Time, interval time.Duration) {
	// Use a fresh context so this best-effort update isn't constrained by the
	// reconcile context's deadline.
	updateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &v1alpha1.Workspace{}
		if err := r.Get(updateCtx, client.ObjectKeyFromObject(workspace), latest); err != nil {
			return err
		}

		// We persist two related but different values together in status:
		// - nextReconcileTime: the exact next scheduled wake-up time
		// - observedReconcileInterval: the cadence that produced that time
		//
		// We need both. Without the stored interval, a later reconcile cannot
		// tell whether an existing future nextReconcileTime was computed from
		// the current interval or from an older one, so changing
		// magosproject.io/reconcile-interval would not take effect immediately.
		latest.Status.NextReconcileTime = &next
		latest.Status.ObservedReconcileInterval = interval.String()
		if err := r.Status().Update(updateCtx, latest); err != nil {
			return err
		}

		workspace.Status = latest.Status
		workspace.ResourceVersion = latest.ResourceVersion
		return nil
	})
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to update next reconcile time")
	}
}
