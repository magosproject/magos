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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Prometheus metrics for the RefWatcher controller. All metrics are registered
// on the controller-runtime metrics registry so they are automatically exposed
// on the manager's /metrics endpoint.
//
// The fleet-level gauges (registrySize and workerQueueDepth) help operators
// tell whether the controller is keeping up with the number of tracked
// Workspaces. A sustained high queue depth means the worker pool is saturated
// and --worker-count should be increased. The per-Workspace counters and
// histograms (pollTotal and pollDuration) are labeled by namespace and name so
// that individual repo owners can alert on a Workspace that is consistently
// failing or abnormally slow.
var (
	// pollTotal counts every git ls-remote attempt. The "result" label is one
	// of "changed" (new SHA detected and annotation patched), "unchanged"
	// (remote matches lastSHA), or "error" (network failure, auth failure, or
	// ref-not-found).
	pollTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "refwatcher_poll_total",
			Help: "Total number of git remote polls partitioned by result.",
		},
		[]string{"namespace", "name", "result"},
	)

	// registrySize tracks how many Workspaces are currently registered for
	// polling. Updated on every registry mutation (upsert or delete).
	registrySize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "refwatcher_registry_size",
			Help: "Current number of workspaces tracked in the refwatcher registry.",
		},
	)

	// workerQueueDepth samples the number of pending items in the buffered
	// work channel each time the scheduler dispatches due entries. A value
	// consistently near --work-queue-size indicates back-pressure from slow
	// remotes or an undersized worker pool.
	workerQueueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "refwatcher_worker_queue_depth",
			Help: "Current number of items in the worker queue.",
		},
	)

	// pollDuration measures the wall-clock time of each go-git remote.List
	// call (the actual network round-trip). This is the primary latency
	// indicator — a jump here usually points to Git host degradation or
	// network issues rather than a problem in RefWatcher itself.
	pollDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "refwatcher_poll_duration_seconds",
			Help:    "Duration of git remote list operations in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace", "name"},
	)
)

func init() {
	metrics.Registry.MustRegister(pollTotal, registrySize, workerQueueDepth, pollDuration)
}
