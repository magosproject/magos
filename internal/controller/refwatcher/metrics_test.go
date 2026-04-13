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

package refwatcher

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestPollTotalIncrementsChanged(t *testing.T) {
	before := testutil.ToFloat64(pollTotal.WithLabelValues("default", "my-workspace", "changed"))
	pollTotal.WithLabelValues("default", "my-workspace", "changed").Inc()
	after := testutil.ToFloat64(pollTotal.WithLabelValues("default", "my-workspace", "changed"))
	assert.Equal(t, before+1, after)
}

func TestPollTotalIncrementsUnchanged(t *testing.T) {
	before := testutil.ToFloat64(pollTotal.WithLabelValues("default", "my-workspace", "unchanged"))
	pollTotal.WithLabelValues("default", "my-workspace", "unchanged").Inc()
	after := testutil.ToFloat64(pollTotal.WithLabelValues("default", "my-workspace", "unchanged"))
	assert.Equal(t, before+1, after)
}

func TestPollTotalIncrementsError(t *testing.T) {
	before := testutil.ToFloat64(pollTotal.WithLabelValues("default", "my-workspace", "error"))
	pollTotal.WithLabelValues("default", "my-workspace", "error").Inc()
	after := testutil.ToFloat64(pollTotal.WithLabelValues("default", "my-workspace", "error"))
	assert.Equal(t, before+1, after)
}

func TestRegistrySizeSetsValue(t *testing.T) {
	registrySize.Set(42)
	assert.Equal(t, float64(42), testutil.ToFloat64(registrySize))

	registrySize.Set(0)
	assert.Equal(t, float64(0), testutil.ToFloat64(registrySize))
}

func TestWorkerQueueDepthSetsValue(t *testing.T) {
	workerQueueDepth.Set(15)
	assert.Equal(t, float64(15), testutil.ToFloat64(workerQueueDepth))

	workerQueueDepth.Set(0)
	assert.Equal(t, float64(0), testutil.ToFloat64(workerQueueDepth))
}

func TestPollDurationObserves(t *testing.T) {
	before := testutil.CollectAndCount(pollDuration)
	pollDuration.WithLabelValues("default", "my-workspace").Observe(0.25)
	after := testutil.CollectAndCount(pollDuration)
	assert.Greater(t, after, before)
}
