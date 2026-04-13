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

package project

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestReconcileTotalIncrementsOnSuccess(t *testing.T) {
	before := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "my-project", "success"))
	reconcileTotal.WithLabelValues("default", "my-project", "success").Inc()
	after := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "my-project", "success"))
	assert.Equal(t, before+1, after)
}

func TestReconcileTotalIncrementsOnError(t *testing.T) {
	before := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "my-project", "error"))
	reconcileTotal.WithLabelValues("default", "my-project", "error").Inc()
	after := testutil.ToFloat64(reconcileTotal.WithLabelValues("default", "my-project", "error"))
	assert.Equal(t, before+1, after)
}

func TestWorkspaceCountSetsValue(t *testing.T) {
	workspaceCount.WithLabelValues("default", "my-project").Set(5)
	assert.Equal(t, float64(5), testutil.ToFloat64(workspaceCount.WithLabelValues("default", "my-project")))

	workspaceCount.WithLabelValues("default", "my-project").Set(0)
	assert.Equal(t, float64(0), testutil.ToFloat64(workspaceCount.WithLabelValues("default", "my-project")))
}

func TestManagedByRolloutToggle(t *testing.T) {
	managedByRollout.WithLabelValues("default", "my-project").Set(1)
	assert.Equal(t, float64(1), testutil.ToFloat64(managedByRollout.WithLabelValues("default", "my-project")))

	managedByRollout.WithLabelValues("default", "my-project").Set(0)
	assert.Equal(t, float64(0), testutil.ToFloat64(managedByRollout.WithLabelValues("default", "my-project")))
}
