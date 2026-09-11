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
	"strings"
	"time"

	"google.golang.org/grpc/status"
	"k8s.io/metis/pkg/store"
)

// MetricsRecorder defines the domain interface for recording Metis operational telemetry.
type MetricsRecorder interface {
	// RPC & Request metrics
	RecordGRPCRequest(method, network, containerID, podName string, err error, duration time.Duration)
	RecordDynamicAllocation(network, containerID, podName string, duration time.Duration)

	// Monitor & Controller lifecycle metrics
	RecordMonitorAction(action string, network string)
	RecordPendingRequests(network string, count int)
	RecordWatcherCIDROperation(operation string, network string)

	// Storage state & inventory metrics
	RecordStoreUsage(network string, family store.IPFamily, usage store.NetworkIPUsage)
}

type prometheusRecorder struct{}

// NewPrometheusRecorder constructs a MetricsRecorder that records telemetry using Prometheus.
func NewPrometheusRecorder() MetricsRecorder {
	return &prometheusRecorder{}
}

func (r *prometheusRecorder) RecordGRPCRequest(method, network, containerID, podName string, err error, duration time.Duration) {
	code := status.Code(err).String()
	GRPCServerHandledTotal.WithLabelValues(method, code, network, containerID, podName).Inc()
	RPCLatencySeconds.WithLabelValues(method, network, containerID, podName).Observe(duration.Seconds())
}

func (r *prometheusRecorder) RecordDynamicAllocation(network, containerID, podName string, duration time.Duration) {
	OutgoingDynamicIPAllocRequestTotal.WithLabelValues(network, containerID, podName).Inc()
	if duration > 0 {
		DynamicIPAllocRPCLatencySeconds.WithLabelValues(network, containerID, podName).Observe(duration.Seconds())
	}
}

func (r *prometheusRecorder) RecordMonitorAction(action string, network string) {
	MonitorActionTotal.WithLabelValues(action, network).Inc()
}

func (r *prometheusRecorder) RecordPendingRequests(network string, count int) {
	PendingDynamicRequestGauge.WithLabelValues(network).Set(float64(count))
}

func (r *prometheusRecorder) RecordWatcherCIDROperation(operation string, network string) {
	WatcherCIDROperationTotal.WithLabelValues(operation, network).Inc()
}

func (r *prometheusRecorder) RecordStoreUsage(network string, family store.IPFamily, usage store.NetworkIPUsage) {
	familyStr := strings.ToLower(string(family))
	available := max(0, usage.IPs.ActiveTotal-(usage.IPs.Allocated+usage.IPs.Cooldown+usage.IPs.Draining))

	LocalStoreIPTotalGauge.WithLabelValues(network, familyStr, "allocated").Set(float64(usage.IPs.Allocated))
	LocalStoreIPTotalGauge.WithLabelValues(network, familyStr, "cooldown").Set(float64(usage.IPs.Cooldown))
	LocalStoreIPTotalGauge.WithLabelValues(network, familyStr, "draining").Set(float64(usage.IPs.Draining))
	LocalStoreIPTotalGauge.WithLabelValues(network, familyStr, "deleting").Set(float64(usage.IPs.Deleting))
	LocalStoreIPTotalGauge.WithLabelValues(network, familyStr, "available").Set(float64(available))
	LocalStoreIPTotalGauge.WithLabelValues(network, familyStr, "total").Set(float64(usage.IPs.Total))

	LocalStoreCIDRBlockTotalGauge.WithLabelValues(network, familyStr, "ready").Set(float64(usage.CIDRs.Ready))
	LocalStoreCIDRBlockTotalGauge.WithLabelValues(network, familyStr, "draining").Set(float64(usage.CIDRs.Draining))
	LocalStoreCIDRBlockTotalGauge.WithLabelValues(network, familyStr, "deleting").Set(float64(usage.CIDRs.Deleting))
}

type noOpRecorder struct{}

// NewNoOpRecorder constructs a MetricsRecorder that discards all recorded telemetry.
func NewNoOpRecorder() MetricsRecorder {
	return noOpRecorder{}
}

func (noOpRecorder) RecordGRPCRequest(string, string, string, string, error, time.Duration) {}
func (noOpRecorder) RecordDynamicAllocation(string, string, string, time.Duration)          {}
func (noOpRecorder) RecordMonitorAction(string, string)                                     {}
func (noOpRecorder) RecordPendingRequests(string, int)                                      {}
func (noOpRecorder) RecordWatcherCIDROperation(string, string)                              {}
func (noOpRecorder) RecordStoreUsage(string, store.IPFamily, store.NetworkIPUsage)          {}
