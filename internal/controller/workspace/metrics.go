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
	"github.com/prometheus/client_golang/prometheus"
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
// while the gauge (activeCount) shows how many Plan or Apply Jobs are running
// concurrently across the cluster.
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

	// activeCount tracks the total number of Workspaces currently in an
	// active (non-terminal) phase: Planning or Applying. Tells operators how
	// many Terraform operations are running concurrently across the cluster.
	activeCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "workspace_active_count",
			Help: "Current number of Workspaces in an active phase (Planning or Applying).",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(reconcileTotal, phaseTransitionsTotal, jobDurationSeconds, activeCount)
}
