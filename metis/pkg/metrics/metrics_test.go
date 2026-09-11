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
	"github.com/prometheus/client_golang/prometheus/testutil"
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

func TestMetrics_PrometheusLint(t *testing.T) {
	lintProblems, err := testutil.GatherAndLint(prometheus.DefaultGatherer)
	if err != nil {
		t.Fatalf("Failed to gather metrics for linting: %v", err)
	}
	for _, problem := range lintProblems {
		t.Errorf("Prometheus metric lint error: %v", problem)
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
		BindAddress:     "127.0.0.1",
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

func TestPrometheusRecorder(_ *testing.T) {
	recorder := metrics.NewPrometheusRecorder()
	recorder.RecordGRPCRequest("AllocatePodIP", "default", "c1", "p1", nil, 100*time.Millisecond)
	recorder.RecordDynamicAllocation("default", "c1", "p1", 200*time.Millisecond)
	recorder.RecordMonitorAction("scale_up", "default")
	recorder.RecordPendingRequests("default", 2)
	recorder.RecordWatcherCIDROperation("add", "default")
}

func TestNoOpRecorder(_ *testing.T) {
	recorder := metrics.NewNoOpRecorder()
	recorder.RecordGRPCRequest("AllocatePodIP", "default", "c1", "p1", nil, 100*time.Millisecond)
	recorder.RecordDynamicAllocation("default", "c1", "p1", 200*time.Millisecond)
	recorder.RecordMonitorAction("scale_up", "default")
	recorder.RecordPendingRequests("default", 2)
	recorder.RecordWatcherCIDROperation("add", "default")
}
