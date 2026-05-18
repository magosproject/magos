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

package variableset

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Prometheus metrics for the VariableSet controller. The shape mirrors the
// other Magos controllers so dashboards can be assembled uniformly across
// controllers.
var (
	// reconcileTotal counts every VariableSet reconciliation. The "result"
	// label is "success" or "error". Operators can alert on a sustained
	// error rate on a single VariableSet to catch RBAC drift or referenced
	// Secrets going missing in production.
	reconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "variableset_reconcile_total",
			Help: "Total number of VariableSet reconciliations partitioned by result.",
		},
		[]string{"namespace", "name", "result"},
	)

	// unresolvedReferences exposes the size of
	// status.unresolvedReferences. A non-zero value means at least one
	// required reference is missing or unreadable, which is the signal we
	// want to page on. Gauge rather than counter because the value is the
	// current state, not an accumulation.
	unresolvedReferences = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "variableset_unresolved_references",
			Help: "Current number of unresolved references on this VariableSet.",
		},
		[]string{"namespace", "name"},
	)
)

func init() {
	metrics.Registry.MustRegister(reconcileTotal, unresolvedReferences)
}
