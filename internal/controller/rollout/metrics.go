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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Prometheus metrics for the Rollout controller. All metrics are registered on
// the controller-runtime metrics registry so they are automatically exposed on
// the manager's /metrics endpoint.
//
// The per-Rollout counters and gauges (reconcileTotal, currentLevel, and
// levelTransitionsTotal) are labeled by namespace and name so that individual
// rollout owners can alert on a Rollout that is stalling or consistently
// erroring. The fleet-level gauge (activeCount) helps operators tell at a
// glance how many Rollouts are actively orchestrating Workspaces.
var (
	// reconcileTotal counts every reconcile attempt for a Rollout. The
	// "result" label is one of "success" (reconcile completed without error)
	// or "error" (reconcile returned an error). Tells operators how often
	// each Rollout reconciles and whether errors are clustering.
	reconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rollout_reconcile_total",
			Help: "Total number of Rollout reconciliations partitioned by result.",
		},
		[]string{"namespace", "name", "result"},
	)

	// currentLevel is set to the index of the currently executing level for
	// a given Rollout. Lets operators see rollout progression at a glance
	// without inspecting the status subresource.
	currentLevelMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rollout_current_level",
			Help: "Index of the currently executing level for each Rollout.",
		},
		[]string{"namespace", "name"},
	)

	// levelTransitionsTotal is incremented each time a Rollout advances to
	// the next level. Helps operators track rollout velocity and spot
	// stalls by comparing the rate of transitions across Rollouts.
	levelTransitionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rollout_level_transitions_total",
			Help: "Total number of level transitions for each Rollout.",
		},
		[]string{"namespace", "name"},
	)

	// activeCount tracks the total number of Rollouts currently in a
	// non-terminal phase (Reconciling). Tells operators how many rollouts
	// are actively orchestrating Workspaces at any given moment.
	activeCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rollout_active_count",
			Help: "Current number of Rollouts in a non-terminal phase.",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(reconcileTotal, currentLevelMetric, levelTransitionsTotal, activeCount)
}
