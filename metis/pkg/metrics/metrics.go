/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"k8s.io/component-base/version"
)

var (
	// MetisVersionGauge exposes build metadata for the running daemon binary (git version, git commit, build date).
	MetisVersionGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "metis_version",
			Help: "Static metric with constant value 1 containing daemon binary build metadata (git version, git commit, build date).",
		},
		[]string{"git_version", "git_commit", "build_date"},
	)

	// LocalStoreIPTotalGauge tracks IP address counts in local daemon stores by state type.
	LocalStoreIPTotalGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "metis_local_store_ip_total",
			Help: "Count of IP addresses in local daemon stores categorized by type (available, allocated, cooldown, draining, deleting, total).",
		},
		[]string{"network", "ip_family", "type"},
	)

	// LocalStoreCIDRBlockTotalGauge tracks CIDR block counts in local daemon stores by status.
	LocalStoreCIDRBlockTotalGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "metis_local_store_cidr_block_total",
			Help: "Total count of CIDR blocks in local daemon stores categorized by status (ready, draining, deleting).",
		},
		[]string{"network", "ip_family", "status"},
	)

	// PendingDynamicRequestGauge tracks the number of pending IP allocation requests waiting for CIDR expansion.
	PendingDynamicRequestGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "metis_daemon_pending_dynamic_request_total",
			Help: "Number of pending IP allocation requests currently blocked and awaiting GCE allocation.",
		},
		[]string{"network"},
	)

	// MonitorActionCount tracks actions executed by the daemon monitor loop (scale_up, drain_excessive, release, delete).
	MonitorActionCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "metis_daemon_monitor_action_count",
			Help: "Total count of actions initiated by the daemon monitor loop (actions: scale_up, drain_excessive, release, delete).",
		},
		[]string{"action", "network"},
	)

	// GRPCServerHandledTotal tracks completed gRPC requests handled by the daemon server.
	GRPCServerHandledTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "metis_daemon_grpc_server_handled_total",
			Help: "Total number of grpc requests completed by the daemon server, regardless of success or failures.",
		},
		[]string{"method", "status", "network", "container_id", "pod_name"},
	)

	// OutgoingDynamicIPAllocRequestTotal tracks outgoing requests for dynamic IP allocation.
	OutgoingDynamicIPAllocRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "metis_daemon_outgoing_dynamic_ip_alloc_request_total",
			Help: "Total count of external requests initiated to GCE/CCM for dynamic IP allocation.",
		},
		[]string{"network", "container_id", "pod_name"},
	)

	// RPCLatencySeconds measures total latency of gRPC server RPC requests in the daemon.
	RPCLatencySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "metis_daemon_rpc_latency_seconds",
			Help:    "Measured latency across all daemon grpc server RPC requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "network", "container_id", "pod_name"},
	)

	// DynamicIPAllocRPCLatencySeconds measures latency spent waiting for dynamic IP allocation.
	DynamicIPAllocRPCLatencySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "metis_daemon_dynamic_ip_alloc_rpc_latency_seconds",
			Help:    "Specific latency associated with dynamic IP allocation (GCE/CCM path).",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"network", "container_id", "pod_name"},
	)

	// WatcherCIDROperationCount tracks successful CIDR watcher sync operations (add, delete).
	WatcherCIDROperationCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "metis_daemon_watcher_cidr_operation_count",
			Help: "Total count of CIDR watcher operations, categorized by operation type (add, delete).",
		},
		[]string{"operation", "network"},
	)

	// CNIRequestLatencySeconds measures end-to-end latency of CNI plugin client requests.
	CNIRequestLatencySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "metis_cni_request_latency_seconds",
			Help:    "Latency of CNI client calls to the daemon socket.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "network", "container_id", "pod_name"},
	)

	// CNIRequestErrorTotal tracks failures encountered during CNI plugin client requests.
	CNIRequestErrorTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "metis_cni_request_error_total",
			Help: "Total count of CNI gRPC request failures.",
		},
		[]string{"method", "error_code", "network", "container_id", "pod_name"},
	)
)

func init() {
	info := version.Get()
	MetisVersionGauge.WithLabelValues(info.GitVersion, info.GitCommit, info.BuildDate).Set(1)
}
