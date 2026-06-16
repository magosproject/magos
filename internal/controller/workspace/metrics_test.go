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
	"strings"
	"testing"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileTotalIncrementsOnSuccess(t *testing.T) {
	before := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "my-workspace", "success"))
	reconcileTotal.WithLabelValues("default", "my-workspace", "success").Inc()
	after := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "my-workspace", "success"))
	assert.Equal(t, before+1, after)
}

func TestReconcileTotalIncrementsOnError(t *testing.T) {
	before := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "my-workspace", "error"))
	reconcileTotal.WithLabelValues("default", "my-workspace", "error").Inc()
	after := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "my-workspace", "error"))
	assert.Equal(t, before+1, after)
}

func TestPhaseTransitionsTotalIncrements(t *testing.T) {
	before := testutil.ToFloat64(phaseTransitionsTotal.WithLabelValues("default", "my-workspace", "Planning"))
	phaseTransitionsTotal.WithLabelValues("default", "my-workspace", "Planning").Inc()
	after := testutil.ToFloat64(phaseTransitionsTotal.WithLabelValues("default", "my-workspace", "Planning"))
	assert.Equal(t, before+1, after)
}

func TestJobDurationSecondsObserves(t *testing.T) {
	before := testutil.CollectAndCount(jobDurationSeconds)
	jobDurationSeconds.WithLabelValues("default", "my-workspace", "plan").Observe(12.5)
	after := testutil.CollectAndCount(jobDurationSeconds)
	assert.Greater(t, after, before)
}

func TestJobDurationSecondsDistinguishesPlanAndApply(t *testing.T) {
	jobDurationSeconds.WithLabelValues("default", "ws-types", "plan").Observe(5.0)
	jobDurationSeconds.WithLabelValues("default", "ws-types", "apply").Observe(30.0)

	planCount := testutil.CollectAndCount(jobDurationSeconds)
	assert.GreaterOrEqual(t, planCount, 2)
}

func TestActiveWorkspacesCollectorCountsActivePhases(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, v1alpha1.AddToScheme(scheme))

	wsInPhase := func(name string, phase v1alpha1.Phase) *v1alpha1.Workspace {
		return &v1alpha1.Workspace{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Status:     v1alpha1.WorkspaceStatus{Phase: phase},
		}
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		wsInPhase("planning", v1alpha1.PhasePlanning),
		wsInPhase("applying", v1alpha1.PhaseApplying),
		wsInPhase("applied", v1alpha1.PhaseApplied),
		wsInPhase("failed", v1alpha1.PhaseFailed),
	).Build()

	collector := newActiveWorkspacesCollector(c)

	const expected = `
# HELP workspace_active_count Current number of Workspaces in an active phase (Planning or Applying).
# TYPE workspace_active_count gauge
workspace_active_count 2
`
	assert.NoError(t, testutil.CollectAndCompare(collector, strings.NewReader(expected)))
}
