/*
Copyright 2017 The Kubernetes Authors.

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

package e2e

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/onsi/ginkgo/v2"
	"golang.org/x/oauth2/google"
	gcm "google.golang.org/api/monitoring/v3"
	"google.golang.org/api/option"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	e2egpu "k8s.io/kubernetes/test/e2e/framework/gpu"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"
)

// Stackdriver container accelerator metrics, as described here:
// https://cloud.google.com/monitoring/api/metrics_gcp#gcp-container
var acceleratorMetrics = []string{
	"accelerator/duty_cycle",
	"accelerator/memory_total",
	"accelerator/memory_used",
}

var (
	pollFrequency = time.Second * 5
	pollTimeout   = time.Minute * 7

	rcName = "resource-consumer"
)

// Migrated from k/k in-tree test:
//
// https://github.com/kubernetes/kubernetes/blob/release-1.30/test/e2e/instrumentation/monitoring/accelerator.go
var _ = ginkgo.Describe("[cloud-provider-gcp-e2e] Stackdriver Monitoring", func() {
	f := framework.NewDefaultFramework("stackdriver-monitoring")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	f.It("should have accelerator metrics", func(ctx context.Context) {
		projectID := framework.TestContext.CloudConfig.ProjectID

		client, err := google.DefaultClient(ctx, gcm.CloudPlatformScope)
		framework.ExpectNoError(err)

		gcmService, err := gcm.NewService(ctx, option.WithHTTPClient(client))

		framework.ExpectNoError(err)

		// set this env var if accessing Stackdriver test endpoint (default is prod):
		// $ export STACKDRIVER_API_ENDPOINT_OVERRIDE=https://test-monitoring.sandbox.googleapis.com/
		basePathOverride := os.Getenv("STACKDRIVER_API_ENDPOINT_OVERRIDE")
		if basePathOverride != "" {
			gcmService.BasePath = basePathOverride
		}

		SetupNVIDIAGPUNode(ctx, f, false)

		e2epod.NewPodClient(f).Create(ctx, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: rcName,
			},
			Spec: v1.PodSpec{
				RestartPolicy: v1.RestartPolicyNever,
				Containers: []v1.Container{
					{
						Name:    rcName,
						Image:   image.GetE2EImage(image.CudaVectorAdd),
						Command: []string{"/bin/sh", "-c"},
						Args:    []string{"nvidia-smi && sleep infinity"},
						Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								e2egpu.NVIDIAGPUResourceName: *resource.NewQuantity(1, resource.DecimalSI),
							},
						},
					},
				},
			},
		})

		metricsMap := map[string]bool{}
		pollingFunction := checkForAcceleratorMetrics(projectID, gcmService, time.Now(), metricsMap)
		err = wait.Poll(pollFrequency, pollTimeout, pollingFunction)
		if err != nil {
			framework.Logf("Missing metrics: %+v", metricsMap)
		}
		framework.ExpectNoError(err)
	})
})

func checkForAcceleratorMetrics(projectID string, gcmService *gcm.Service, start time.Time, metricsMap map[string]bool) func() (bool, error) {
	return func() (bool, error) {
		counter := 0
		for _, metric := range acceleratorMetrics {
			metricsMap[metric] = false
		}
		for _, metric := range acceleratorMetrics {
			// TODO: check only for metrics from this cluster
			ts, err := fetchTimeSeries(projectID, gcmService, metric, start, time.Now())
			framework.ExpectNoError(err)
			if len(ts) > 0 {
				counter = counter + 1
				metricsMap[metric] = true
				framework.Logf("Received %v timeseries for metric %v", len(ts), metric)
			} else {
				framework.Logf("No timeseries for metric %v", metric)
			}
		}
		if counter < 3 {
			return false, nil
		}
		return true, nil
	}
}

func createMetricFilter(metric string, containerName string) string {
	return fmt.Sprintf(`metric.type="container.googleapis.com/container/%s" AND
				resource.label.container_name="%s"`, metric, containerName)
}

func fetchTimeSeries(projectID string, gcmService *gcm.Service, metric string, start time.Time, end time.Time) ([]*gcm.TimeSeries, error) {
	response, err := gcmService.Projects.TimeSeries.
		List(fullProjectName(projectID)).
		Filter(createMetricFilter(metric, rcName)).
		IntervalStartTime(start.Format(time.RFC3339)).
		IntervalEndTime(end.Format(time.RFC3339)).
		Do()
	if err != nil {
		return nil, err
	}
	return response.TimeSeries, nil
}

func fullProjectName(name string) string {
	return fmt.Sprintf("projects/%s", name)
}
