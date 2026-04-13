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
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
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

func TestActiveCountSetsValue(t *testing.T) {
	activeCount.Set(7)
	assert.Equal(t, float64(7), testutil.ToFloat64(activeCount))

	activeCount.Set(0)
	assert.Equal(t, float64(0), testutil.ToFloat64(activeCount))
}
