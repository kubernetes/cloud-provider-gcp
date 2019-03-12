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

package csrmetrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// CSRSigningStatus is a status string of the CSR Signing metric.
type CSRSigningStatus string

// CSRApprovalStatus is a status string of the CSR Approval metric.
type CSRApprovalStatus string

// OutboundRPCStatus is a status string of the Outbound RPC metric.
type OutboundRPCStatus string

// CSRSigningRecord is a function to record the CSR Signing metric with a status.
type CSRSigningRecord func(status CSRSigningStatus)

// CSRApprovalRecord is a function to record the CSR Approval metric with a status.
type CSRApprovalRecord func(status CSRApprovalStatus)

// OutboundRPCRecord is a function to record the Outbound RPC metric with a status.
type OutboundRPCRecord func(status OutboundRPCStatus)

// Status constants for metrics.
const (
	CSRSigningStatusSignError   CSRSigningStatus = "sign_error"
	CSRSigningStatusUpdateError CSRSigningStatus = "update_error"
	CSRSigningStatusSigned      CSRSigningStatus = "signed"

	CSRApprovalStatusNodeDeleted         CSRApprovalStatus = "node_deleted"
	CSRApprovalStatusParseError          CSRApprovalStatus = "parse_error"
	CSRApprovalStatusSARError            CSRApprovalStatus = "sar_error"
	CSRApprovalStatusSARReject           CSRApprovalStatus = "sar_reject"
	CSRApprovalStatusPreApproveHookError CSRApprovalStatus = "pre_approve_hook_error"
	CSRApprovalStatusDeny                CSRApprovalStatus = "deny"
	CSRApprovalStatusApprove             CSRApprovalStatus = "approve"
	CSRApprovalStatusIgnore              CSRApprovalStatus = "ignore"

	OutboundRPCStatusNotFound OutboundRPCStatus = "not_found"
	OutboundRPCStatusError    OutboundRPCStatus = "error"
	OutboundRPCStatusOK       OutboundRPCStatus = "ok"
)

var (
	csrSigningCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "csr_signing_count",
		Help: "Count of signed CSRs",
	}, []string{"status"})
	csrSigningLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "csr_signing_latencies",
		Help: "Latency of CSR signer, in seconds",
	}, []string{"status"})
	csrApprovalCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "csr_approval_count",
		Help: "Count of approved, denied and ignored CSRs",
	}, []string{"status", "kind"})
	csrApprovalLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "csr_approval_latencies",
		Help: "Latency of CSR approver, in seconds",
	}, []string{"status", "kind"})
	outboundRPCLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "outbound_rpc_latency",
		Help: "Latency of outbound RPCs to GCE and GKE, in seconds",
	}, []string{"status", "kind"})
)

func init() {
	prometheus.MustRegister(csrSigningCount)
	prometheus.MustRegister(csrSigningLatency)
	prometheus.MustRegister(csrApprovalCount)
	prometheus.MustRegister(csrApprovalLatency)
	prometheus.MustRegister(outboundRPCLatency)
}

// CSRSigningStartRecorder starts a timer for kind and returns a
// CSR Singing status record function.
func CSRSigningStartRecorder() CSRSigningRecord {
	start := time.Now()
	return func(status CSRSigningStatus) {
		csrSigningCount.WithLabelValues(string(status)).Inc()
		csrSigningLatency.WithLabelValues(string(status)).Observe(time.Since(start).Seconds())
	}
}

// CSRApprovalStartRecorder starts a timer for kind and returns a
// CSR Approval status record function.
func CSRApprovalStartRecorder(kind string) CSRApprovalRecord {
	start := time.Now()
	return func(status CSRApprovalStatus) {
		csrApprovalCount.WithLabelValues(string(status), kind).Inc()
		csrApprovalLatency.WithLabelValues(string(status), kind).Observe(time.Since(start).Seconds())
	}
}

// OutboundRPCStartRecorder starts a timer for kind and returns a
// Outbound RPC status record function.
func OutboundRPCStartRecorder(kind string) OutboundRPCRecord {
	start := time.Now()
	return func(status OutboundRPCStatus) {
		outboundRPCLatency.WithLabelValues(string(status), kind).Observe(time.Since(start).Seconds())
	}
}
