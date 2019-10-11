/*
Copyright 2019 The Kubernetes Authors.

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

// Package csrmetrics contains metric definitions to be recorded by the
// certificates controller.
package csrmetrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// SigningStatus is a status string of the CSR signing metric.
type SigningStatus string

// ApprovalStatus is a status string of the CSR approval metric.
type ApprovalStatus string

// OutboundRPCStatus is a status string of the outbound RPC metric.
type OutboundRPCStatus string

// Status constants for metrics.
const (
	SigningStatusSignError   SigningStatus = "sign_error"
	SigningStatusUpdateError SigningStatus = "update_error"
	SigningStatusSigned      SigningStatus = "signed"

	ApprovalStatusNodeDeleted         ApprovalStatus = "node_deleted"
	ApprovalStatusParseError          ApprovalStatus = "parse_error"
	ApprovalStatusSARError            ApprovalStatus = "sar_error"
	ApprovalStatusSARErrorAtStartup   ApprovalStatus = "sar_error_at_startup"
	ApprovalStatusSARReject           ApprovalStatus = "sar_reject"
	ApprovalStatusSARRejectAtStartup  ApprovalStatus = "sar_reject_at_startup"
	ApprovalStatusPreApproveHookError ApprovalStatus = "pre_approve_hook_error"
	ApprovalStatusDeny                ApprovalStatus = "deny"
	ApprovalStatusApprove             ApprovalStatus = "approve"
	ApprovalStatusIgnore              ApprovalStatus = "ignore"

	OutboundRPCStatusNotFound OutboundRPCStatus = "not_found"
	OutboundRPCStatusError    OutboundRPCStatus = "error"
	OutboundRPCStatusOK       OutboundRPCStatus = "ok"
)

var (
	signingCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "csr_signing_count",
		Help: "Count of signed CSRs",
	}, []string{"status"})
	signingLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "csr_signing_latencies",
		Help: "Latency of CSR signer, in seconds",
	}, []string{"status"})
	approvalCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "csr_approval_count",
		Help: "Count of approved, denied and ignored CSRs",
	}, []string{"status", "kind"})
	approvalLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "csr_approval_latencies",
		Help: "Latency of CSR approver, in seconds",
	}, []string{"status", "kind"})
	outboundRPCCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "outbound_rpc_count",
		Help: "Count of outbound RPCs to GCE and GKE.",
	}, []string{"status", "kind"})
	outboundRPCLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "outbound_rpc_latency",
		Help: "Latency of outbound RPCs to GCE and GKE, in seconds",
	}, []string{"status", "kind"})
)

func init() {
	prometheus.MustRegister(
		signingCount,
		signingLatency,
		approvalCount,
		approvalLatency,
		outboundRPCCount,
		outboundRPCLatency,
	)
}

// SigningStartRecorder marks the start of a CSR signing operation. Caller is
// responsible for calling the returned function, which records Prometheus
// metrics for this operation.
func SigningStartRecorder() func(status SigningStatus) {
	start := time.Now()
	return func(status SigningStatus) {
		signingCount.WithLabelValues(string(status)).Inc()
		signingLatency.WithLabelValues(string(status)).Observe(time.Since(start).Seconds())
	}
}

// ApprovalStartRecorder marks the start of a CSR approval operation. Caller is
// responsible for calling the returned function, which records Prometheus
// metrics for this operation.
func ApprovalStartRecorder(kind string) func(status ApprovalStatus) {
	start := time.Now()
	return func(status ApprovalStatus) {
		approvalCount.WithLabelValues(string(status), kind).Inc()
		approvalLatency.WithLabelValues(string(status), kind).Observe(time.Since(start).Seconds())
	}
}

// OutboundRPCStartRecorder marks the start of a outbound RPC operation. Caller is
// responsible for calling the returned function, which records Prometheus
// metrics for this operation.
func OutboundRPCStartRecorder(kind string) func(status OutboundRPCStatus) {
	start := time.Now()
	return func(status OutboundRPCStatus) {
		outboundRPCCount.WithLabelValues(string(status), kind).Inc()
		outboundRPCLatency.WithLabelValues(string(status), kind).Observe(time.Since(start).Seconds())
	}
}
