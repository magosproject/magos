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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Prometheus metrics for the Project controller. All metrics are registered on
// the controller-runtime metrics registry so they are automatically exposed on
// the manager's /metrics endpoint.
//
// The reconcile counter lets operators see how often each Project is reconciled
// and whether errors are clustering around a specific Project. The workspace
// count gauge shows the blast radius of a given Project and makes it easy to
// spot orphaned Projects that have zero Workspaces. The managed-by-rollout
// gauge gives a quick fleet-wide view of which Projects are under rollout
// control versus using default parallel execution.
var (
	// reconcileTotal counts every Project reconciliation. The "result" label
	// is one of "success" (reconcile loop completed without error) or "error"
	// (the reconcile loop returned an error). Operators can alert on a
	// sustained error rate for a specific Project to catch configuration
	// problems early.
	reconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "project_reconcile_total",
			Help: "Total number of Project reconciliations partitioned by result.",
		},
		[]string{"namespace", "name", "result"},
	)

	// workspaceCount tracks the number of Workspaces that reference this
	// Project. Updated on every reconcile. Helps operators understand the
	// blast radius of a Project and spot orphaned Projects with zero
	// Workspaces.
	workspaceCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "project_workspace_count",
			Help: "Current number of Workspaces that reference this Project.",
		},
		[]string{"namespace", "name"},
	)

	// managedByRollout is set to 1 when a matching Rollout exists and the
	// Project defers orchestration to it, or 0 when the Project uses default
	// parallel execution. Lets operators quickly see which Projects are under
	// rollout control across the fleet.
	managedByRollout = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "project_managed_by_rollout",
			Help: "Whether this Project defers orchestration to a Rollout (1) or uses default parallel execution (0).",
		},
		[]string{"namespace", "name"},
	)
)

func init() {
	metrics.Registry.MustRegister(reconcileTotal, workspaceCount, managedByRollout)
}
