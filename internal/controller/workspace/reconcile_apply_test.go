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
	"testing"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newApprovalScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	assert.NoError(t, v1alpha1.AddToScheme(scheme))
	assert.NoError(t, batchv1.AddToScheme(scheme))
	assert.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

func newParkedWorkspace(decision string) *v1alpha1.Workspace {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "ws",
			Namespace:   "default",
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.WorkspaceSpec{AutoApply: false},
		Status: v1alpha1.WorkspaceStatus{
			Phase:        v1alpha1.PhasePlanned,
			CurrentRunID: "r1",
		},
	}
	if decision != "" {
		ws.Annotations[v1alpha1.WorkspaceApprovalDecisionAnnotation] = decision
	}
	return ws
}

// TestCreateApplyJobIfApproved_DecisionRejected verifies that a rejected plan
// moves the workspace to PhaseRejected, consumes the decision annotation, and
// does not create an apply job.
func TestCreateApplyJobIfApproved_DecisionRejected(t *testing.T) {
	scheme := newApprovalScheme(t)
	ws := newParkedWorkspace(v1alpha1.ApprovalDecisionRejected)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ws).WithStatusSubresource(ws).Build()
	r := WorkspaceReconciler{Client: cli, Scheme: scheme}

	err := r.createApplyJobIfApproved(context.Background(), ws, runContext{planJob: &batchv1.Job{}})
	assert.NoError(t, err)

	got := &v1alpha1.Workspace{}
	assert.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(ws), got))
	assert.Equal(t, v1alpha1.PhaseRejected, got.Status.Phase)
	_, hasDecision := got.Annotations[v1alpha1.WorkspaceApprovalDecisionAnnotation]
	assert.False(t, hasDecision, "controller should consume the decision annotation")
	var jobs batchv1.JobList
	assert.NoError(t, cli.List(context.Background(), &jobs))
	for _, j := range jobs.Items {
		assert.NotContains(t, j.Name, "apply")
	}
}

// TestCreateApplyJobIfApproved_DecisionRejected_ConsumesDetectedRevision
// verifies that rejecting a plan that was triggered by a RefWatcher commit also
// consumes the detected-revision annotation. If it survives, checkCycleNeeded
// sees detected-revision != observedRevision while the phase is the terminal
// Rejected and immediately restarts a fresh plan, so the rejection never sticks.
func TestCreateApplyJobIfApproved_DecisionRejected_ConsumesDetectedRevision(t *testing.T) {
	scheme := newApprovalScheme(t)
	ws := newParkedWorkspace(v1alpha1.ApprovalDecisionRejected)
	ws.Annotations[v1alpha1.WorkspaceDetectedRevisionAnnotation] = "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2"
	ws.Status.ObservedRevision = "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1"
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ws).WithStatusSubresource(ws).Build()
	r := WorkspaceReconciler{Client: cli, Scheme: scheme}

	err := r.createApplyJobIfApproved(context.Background(), ws, runContext{planJob: &batchv1.Job{}})
	assert.NoError(t, err)

	got := &v1alpha1.Workspace{}
	assert.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(ws), got))
	assert.Equal(t, v1alpha1.PhaseRejected, got.Status.Phase)
	_, hasDetected := got.Annotations[v1alpha1.WorkspaceDetectedRevisionAnnotation]
	assert.False(t, hasDetected, "rejection must consume detected-revision so it does not immediately re-plan")
}

// TestCreateApplyJobIfApproved_DecisionApproved verifies that an approved plan
// creates an apply job, transitions the workspace to PhaseApplying, and
// consumes the decision annotation.
func TestCreateApplyJobIfApproved_DecisionApproved(t *testing.T) {
	scheme := newApprovalScheme(t)
	ws := newParkedWorkspace(v1alpha1.ApprovalDecisionApproved)
	// Ensure the workspace has the fields constructJobForWorkspace needs.
	ws.Spec.Source.RepoURL = "https://github.com/example/infra"
	ws.Spec.Source.TargetRevision = "main"
	ws.Spec.Terraform.Version = "1.9.0"
	ws.Spec.ProjectRef.Name = "platform"
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ws).WithStatusSubresource(ws).Build()
	r := WorkspaceReconciler{Client: cli, Scheme: scheme, JobImage: "test"}

	rc := runContext{
		planJob:      &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "plan-job", Namespace: "default"}},
		applyJobName: "apply-job",
	}
	err := r.createApplyJobIfApproved(context.Background(), ws, rc)
	assert.NoError(t, err)

	got := &v1alpha1.Workspace{}
	assert.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(ws), got))
	assert.Equal(t, v1alpha1.PhaseApplying, got.Status.Phase)
	_, hasDecision := got.Annotations[v1alpha1.WorkspaceApprovalDecisionAnnotation]
	assert.False(t, hasDecision, "approve path should also consume the annotation")
}

// TestCreateApplyJobIfApproved_NoDecisionParks verifies that a workspace with
// no decision annotation and autoApply=false is parked in PhasePlanned.
func TestCreateApplyJobIfApproved_NoDecisionParks(t *testing.T) {
	scheme := newApprovalScheme(t)
	ws := newParkedWorkspace("")
	// Start from an empty phase so the log archive path is exercised.
	ws.Status.Phase = ""
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ws).WithStatusSubresource(ws).Build()
	r := WorkspaceReconciler{Client: cli, Scheme: scheme}

	assert.NoError(t, r.createApplyJobIfApproved(context.Background(), ws, runContext{planJob: &batchv1.Job{}}))

	got := &v1alpha1.Workspace{}
	assert.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(ws), got))
	assert.Equal(t, v1alpha1.PhasePlanned, got.Status.Phase)
}
