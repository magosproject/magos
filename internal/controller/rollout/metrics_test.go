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

package rollout

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestReconcileTotalIncrementsOnSuccess(t *testing.T) {
	before := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "my-rollout", "success"))
	reconcileTotal.WithLabelValues("default", "my-rollout", "success").Inc()
	after := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "my-rollout", "success"))
	assert.Equal(t, before+1, after)
}

func TestReconcileTotalIncrementsOnError(t *testing.T) {
	before := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "my-rollout", "error"))
	reconcileTotal.WithLabelValues("default", "my-rollout", "error").Inc()
	after := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "my-rollout", "error"))
	assert.Equal(t, before+1, after)
}

func TestCurrentLevelMetricSetsValue(t *testing.T) {
	currentLevelMetric.WithLabelValues("default", "my-rollout").Set(2)
	assert.Equal(t, float64(2), testutil.ToFloat64(currentLevelMetric.WithLabelValues("default", "my-rollout")))

	currentLevelMetric.WithLabelValues("default", "my-rollout").Set(0)
	assert.Equal(t, float64(0), testutil.ToFloat64(currentLevelMetric.WithLabelValues("default", "my-rollout")))
}

func TestLevelTransitionsTotalIncrements(t *testing.T) {
	before := testutil.ToFloat64(levelTransitionsTotal.WithLabelValues("default", "my-rollout"))
	levelTransitionsTotal.WithLabelValues("default", "my-rollout").Inc()
	after := testutil.ToFloat64(levelTransitionsTotal.WithLabelValues("default", "my-rollout"))
	assert.Equal(t, before+1, after)
}

func TestActiveCountSetsValue(t *testing.T) {
	activeCount.Set(3)
	assert.Equal(t, float64(3), testutil.ToFloat64(activeCount))

	activeCount.Set(0)
	assert.Equal(t, float64(0), testutil.ToFloat64(activeCount))
}
