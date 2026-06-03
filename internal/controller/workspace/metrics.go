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

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Prometheus metrics for the Workspace controller. All metrics are registered
// on the controller-runtime metrics registry so they are automatically exposed
// on the manager's /metrics endpoint.
//
// The per-Workspace counters (reconcileTotal, phaseTransitionsTotal) are
// labeled by namespace and name so that individual repo owners can alert on a
// Workspace that is consistently erroring or cycling through Failed. The
// histogram (jobDurationSeconds) lets operators spot slow Terraform operations,
// while the gauge (workspace_active_count, served by activeWorkspacesCollector)
// shows how many Plan or Apply Jobs are running concurrently across the cluster.
var (
	// reconcileTotal counts every Reconcile invocation. The "result" label is
	// one of "success" (reconcile completed without error), "error" (reconcile
	// returned an error), or "requeue" (reconcile requested a requeue). This
	// tells operators how often each Workspace reconciles and whether errors
	// are clustering on specific Workspaces.
	reconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "workspace_reconcile_total",
			Help: "Total number of Workspace reconciliations partitioned by result.",
		},
		[]string{"namespace", "name", "result"},
	)

	// phaseTransitionsTotal counts every phase transition for a Workspace. The
	// "phase" label is the phase the Workspace transitioned to (Pending,
	// Planning, Planned, Applying, Applied, Failed). Lets operators see
	// throughput and spot Workspaces that cycle through Failed repeatedly.
	phaseTransitionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "workspace_phase_transitions_total",
			Help: "Total number of Workspace phase transitions partitioned by phase.",
		},
		[]string{"namespace", "name", "phase"},
	)

	// jobDurationSeconds measures the wall-clock duration of completed Plan
	// and Apply Jobs. The "type" label is "plan" or "apply". Helps operators
	// identify slow Terraform operations. A jump here usually points to
	// large state, complex plans, or provider API latency rather than a
	// problem in the controller itself.
	jobDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "workspace_job_duration_seconds",
			Help:    "Duration of completed Terraform Plan and Apply Jobs in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace", "name", "type"},
	)
)

// activeWorkspacesDesc describes the workspace_active_count gauge emitted by
// activeWorkspacesCollector. It tells operators how many Terraform operations
// are running concurrently across the cluster.
var activeWorkspacesDesc = prometheus.NewDesc(
	"workspace_active_count",
	"Current number of Workspaces in an active phase (Planning or Applying).",
	nil, nil,
)

func init() {
	metrics.Registry.MustRegister(reconcileTotal, phaseTransitionsTotal, jobDurationSeconds)
}

// activeWorkspacesCollector reports the number of Workspaces in an active
// (Planning or Applying) phase. The count is computed lazily at scrape time by
// listing Workspaces from the controller's cache, so it costs one cached list
// per Prometheus scrape rather than one per reconcile. Computing it on every
// reconcile previously made each reconcile O(total workspaces); moving it here
// keeps the metric off the hot path so reconcile cost stays independent of
// fleet size.
type activeWorkspacesCollector struct {
	client client.Client
}

func newActiveWorkspacesCollector(c client.Client) *activeWorkspacesCollector {
	return &activeWorkspacesCollector{client: c}
}

func (c *activeWorkspacesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- activeWorkspacesDesc
}

func (c *activeWorkspacesCollector) Collect(ch chan<- prometheus.Metric) {
	var workspaces v1alpha1.WorkspaceList
	if err := c.client.List(context.Background(), &workspaces); err != nil {
		// Skip emitting on error rather than reporting a misleading zero; the
		// next scrape refreshes the value.
		return
	}
	var active float64
	for i := range workspaces.Items {
		switch workspaces.Items[i].Status.Phase {
		case v1alpha1.PhasePlanning, v1alpha1.PhaseApplying:
			active++
		}
	}
	ch <- prometheus.MustNewConstMetric(activeWorkspacesDesc, prometheus.GaugeValue, active)
}
