/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
without WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics_test

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/metis/pkg/daemon"
	"k8s.io/metis/pkg/metrics"
)

func TestMetrics_Version(t *testing.T) {
	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	foundVersion := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "metis_version" {
			foundVersion = true
			if len(mf.GetMetric()) == 0 {
				t.Errorf("Expected metrics for metis_version, got 0")
			} else {
				m := mf.GetMetric()[0]
				labels := map[string]string{}
				for _, l := range m.GetLabel() {
					labels[l.GetName()] = l.GetValue()
				}
				for _, expectedLabel := range []string{"git_version", "git_commit", "build_date"} {
					if _, ok := labels[expectedLabel]; !ok {
						t.Errorf("Expected label %q in metis_version metric", expectedLabel)
					}
				}
			}
		}
	}
	if !foundVersion {
		t.Errorf("metis_version metric family not found in Prometheus gatherer")
	}
}

func TestMetrics_AllMetricsRegistered(t *testing.T) {
	metrics.MetisVersionGauge.WithLabelValues("v1.0.0", "abc", "2026-08-27T00:00:00Z").Set(1)
	metrics.LocalStoreIPTotalGauge.WithLabelValues("default", "ipv4", "available").Set(10)
	metrics.LocalStoreCIDRBlockTotalGauge.WithLabelValues("default", "ipv4", "ready").Set(1)
	metrics.PendingDynamicRequestGauge.WithLabelValues("default").Set(0)
	metrics.MonitorActionCount.WithLabelValues("scale_up", "default").Inc()
	metrics.GRPCServerHandledTotal.WithLabelValues("AllocatePodIP", "OK", "default", "c1", "p1").Inc()
	metrics.OutgoingDynamicIPAllocRequestTotal.WithLabelValues("default", "c1", "p1").Inc()
	metrics.RPCLatencySeconds.WithLabelValues("AllocatePodIP", "default", "c1", "p1").Observe(0.1)
	metrics.DynamicIPAllocRPCLatencySeconds.WithLabelValues("default", "c1", "p1").Observe(0.5)
	metrics.WatcherCIDROperationCount.WithLabelValues("add", "default").Inc()
	metrics.CNIRequestLatencySeconds.WithLabelValues("CmdAdd", "default", "c1", "p1").Observe(0.05)
	metrics.CNIRequestErrorTotal.WithLabelValues("CmdAdd", "Unknown", "default", "c1", "p1").Inc()

	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	expectedMetrics := []string{
		"metis_version",
		"metis_local_store_ip_total",
		"metis_local_store_cidr_block_total",
		"metis_daemon_pending_dynamic_request_total",
		"metis_daemon_monitor_action_count",
		"metis_daemon_grpc_server_handled_total",
		"metis_daemon_outgoing_dynamic_ip_alloc_request_total",
		"metis_daemon_rpc_latency_seconds",
		"metis_daemon_dynamic_ip_alloc_rpc_latency_seconds",
		"metis_daemon_watcher_cidr_operation_count",
		"metis_cni_request_latency_seconds",
		"metis_cni_request_error_total",
	}

	foundMetrics := map[string]bool{}
	for _, mf := range metricFamilies {
		name := mf.GetName()
		for _, exp := range expectedMetrics {
			if name == exp {
				foundMetrics[exp] = true
			}
		}
	}

	for _, exp := range expectedMetrics {
		if !foundMetrics[exp] {
			t.Errorf("Expected metric %s not found in Prometheus gatherer", exp)
		}
	}
}

func TestDaemon_MetricsHTTPServer(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_metrics_http.sqlite")
	sockPath := filepath.Join(tempDir, "metis_metrics_http.sock")

	cfg := daemon.Config{
		MonitorInterval: 5 * time.Second,
		ReleaseCooldown: 1 * time.Minute,
		DBPath:          dbPath,
		SocketPath:      sockPath,
		MetricsPort:     9997, // Use non-default port for test
	}

	t.Setenv("NODE_NAME", "test-node")

	d := daemon.NewDaemon(cfg)
	d.NNCClient = nncfake.NewSimpleClientset(&nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	})
	d.KubeClient = kubefake.NewSimpleClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = d.Run(ctx)
	}()

	var bodyStr string
	err := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 3*time.Minute, true, func(_ context.Context) (bool, error) {
		resp, err := http.Get("http://localhost:9997/metrics")
		if err != nil {
			return false, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return false, nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, nil
		}

		bodyStr = string(body)
		if !strings.Contains(bodyStr, "metis_version") {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		t.Fatalf("Failed to fetch /metrics with expected content within 3m: %v", err)
	}
}
