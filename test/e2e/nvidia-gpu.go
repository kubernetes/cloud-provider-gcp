/*
Copyright 2018 The Kubernetes Authors.

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
	"os"
	"regexp"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	extensionsinternal "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edebug "k8s.io/kubernetes/test/e2e/framework/debug"
	e2egpu "k8s.io/kubernetes/test/e2e/framework/gpu"
	e2ejob "k8s.io/kubernetes/test/e2e/framework/job"
	e2emanifest "k8s.io/kubernetes/test/e2e/framework/manifest"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eresource "k8s.io/kubernetes/test/e2e/framework/resource"
	e2etestfiles "k8s.io/kubernetes/test/e2e/framework/testfiles"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

const (
	completions       = int32(1)
	testPodNamePrefix = "nvidia-gpu-"
	// Nvidia driver installation can take upwards of 5 minutes.
	driverInstallTimeout = 10 * time.Minute
)

// NvidiaGPUUpgradeTest tests that gpu resource is available before and after
// a cluster upgrade.
type NvidiaGPUUpgradeTest struct {
}

// Name returns the tracking name of the test.
func (NvidiaGPUUpgradeTest) Name() string { return "nvidia-gpu-upgrade [sig-node] [sig-scheduling]" }

// Setup creates a job requesting gpu.
func (t *NvidiaGPUUpgradeTest) Setup(ctx context.Context, f *framework.Framework) {
	SetupNVIDIAGPUNode(ctx, f, false)
	ginkgo.By("Creating a job requesting gpu")
	StartJob(ctx, f, completions)
}

// Test waits for the upgrade to complete, and then verifies that the
// cuda pod started by the gpu job can successfully finish.
func (t *NvidiaGPUUpgradeTest) Test(ctx context.Context, f *framework.Framework, done <-chan struct{}, upgrade UpgradeType) {
	<-done
	ginkgo.By("Verifying gpu job success")
	VerifyJobNCompletions(ctx, f, completions)
	if upgrade == MasterUpgrade || upgrade == ClusterUpgrade {
		// MasterUpgrade should be totally hitless.
		job, err := e2ejob.GetJob(ctx, f.ClientSet, f.Namespace.Name, "cuda-add")
		framework.ExpectNoError(err)
		gomega.Expect(job.Status.Failed).To(gomega.BeZero(), "Job pods failed during master upgrade: %v", job.Status.Failed)
	}
}

// Teardown cleans up any remaining resources.
func (t *NvidiaGPUUpgradeTest) Teardown(ctx context.Context, f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}

var gpuResourceName v1.ResourceName

// SetupNVIDIAGPUNode install Nvidia Drivers and wait for Nvidia GPUs to be available on nodes
func SetupNVIDIAGPUNode(ctx context.Context, f *framework.Framework, setupResourceGatherer bool) *e2edebug.ContainerResourceGatherer {
	logOSImages(ctx, f)

	var err error
	var ds *appsv1.DaemonSet
	dsYamlURLFromEnv := os.Getenv("NVIDIA_DRIVER_INSTALLER_DAEMONSET")
	if dsYamlURLFromEnv != "" {
		// Using DaemonSet from remote URL
		framework.Logf("Using remote nvidia-driver-installer daemonset manifest from %v", dsYamlURLFromEnv)
		ds, err = e2emanifest.DaemonSetFromURL(ctx, dsYamlURLFromEnv)
		framework.ExpectNoError(err, "failed get remote")
	} else {
		// Using default local DaemonSet
		framework.Logf("Using default local nvidia-driver-installer daemonset manifest.")
		data, err := e2etestfiles.Read("test/e2e/testing-manifests/scheduling/nvidia-driver-installer.yaml")
		framework.ExpectNoError(err, "failed to read local manifest for nvidia-driver-installer daemonset")
		ds, err = e2emanifest.DaemonSetFromData(data)
		framework.ExpectNoError(err, "failed to parse local manifest for nvidia-driver-installer daemonset")
	}
	gpuResourceName = e2egpu.NVIDIAGPUResourceName
	ds.Namespace = f.Namespace.Name

	_, err = f.ClientSet.AppsV1().DaemonSets(ds.Namespace).Create(ctx, ds, metav1.CreateOptions{})
	framework.ExpectNoError(err, "failed to create nvidia-driver-installer daemonset")
	framework.Logf("Successfully created daemonset to install Nvidia drivers.")

	pods, err := e2eresource.WaitForControlledPods(ctx, f.ClientSet, ds.Namespace, ds.Name, extensionsinternal.Kind("DaemonSet"))
	framework.ExpectNoError(err, "failed to get pods controlled by the nvidia-driver-installer daemonset")

	devicepluginPods, err := e2eresource.WaitForControlledPods(ctx, f.ClientSet, "kube-system", "nvidia-gpu-device-plugin", extensionsinternal.Kind("DaemonSet"))
	if err == nil {
		framework.Logf("Adding deviceplugin addon pod.")
		pods.Items = append(pods.Items, devicepluginPods.Items...)
	}

	var rsgather *e2edebug.ContainerResourceGatherer
	if setupResourceGatherer {
		framework.Logf("Starting ResourceUsageGather for the created DaemonSet pods.")
		rsgather, err = e2edebug.NewResourceUsageGatherer(ctx, f.ClientSet, e2edebug.ResourceGathererOptions{InKubemark: false, Nodes: e2edebug.AllNodes, ResourceDataGatheringPeriod: 2 * time.Second, ProbeDuration: 2 * time.Second, PrintVerboseLogs: true}, pods)
		framework.ExpectNoError(err, "creating ResourceUsageGather for the daemonset pods")
		go rsgather.StartGatheringData(ctx)
	}

	// Wait for Nvidia GPUs to be available on nodes
	framework.Logf("Waiting for drivers to be installed and GPUs to be available in Node Capacity...")
	gomega.Eventually(ctx, func(ctx context.Context) bool {
		return areGPUsAvailableOnAllSchedulableNodes(ctx, f)
	}, driverInstallTimeout, time.Second).Should(gomega.BeTrue())

	return rsgather
}

func logOSImages(ctx context.Context, f *framework.Framework) {
	nodeList, err := f.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	framework.ExpectNoError(err, "getting node list")
	for _, node := range nodeList.Items {
		framework.Logf("Nodename: %v, OS Image: %v", node.Name, node.Status.NodeInfo.OSImage)
	}
}

func areGPUsAvailableOnAllSchedulableNodes(ctx context.Context, f *framework.Framework) bool {
	framework.Logf("Getting list of Nodes from API server")
	nodeList, err := f.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	framework.ExpectNoError(err, "getting node list")
	for _, node := range nodeList.Items {
		if node.Spec.Unschedulable || isControlPlaneNode(node) {
			continue
		}

		if val, ok := node.Status.Capacity[gpuResourceName]; !ok || val.Value() == 0 {
			framework.Logf("Nvidia GPUs not available on Node: %q", node.Name)
			return false
		}
	}
	framework.Logf("Nvidia GPUs exist on all schedulable worker nodes")
	return true
}

func isControlPlaneNode(node v1.Node) bool {
	_, isControlPlane := node.Labels["node-role.kubernetes.io/control-plane"]
	if isControlPlane {
		framework.Logf("Node: %q is a control-plane node (label)", node.Name)
		return true
	}

	for _, taint := range node.Spec.Taints {
		if taint.Key == "node-role.kubernetes.io/control-plane" {
			framework.Logf("Node: %q is a control-plane node (taint)", node.Name)
			return true
		}
	}
	framework.Logf("Node: %q is NOT a control-plane node", node.Name)
	return false
}

// StartJob starts a simple CUDA job that requests gpu and the specified number of completions
func StartJob(ctx context.Context, f *framework.Framework, completions int32) {
	var activeSeconds int64 = 3600
	testJob := e2ejob.NewTestJob("succeed", "cuda-add", v1.RestartPolicyAlways, 1, completions, &activeSeconds, 6)
	testJob.Spec.Template.Spec = v1.PodSpec{
		RestartPolicy: v1.RestartPolicyOnFailure,
		Containers: []v1.Container{
			{
				Name:    "vector-addition",
				Image:   imageutils.GetE2EImage(imageutils.CudaVectorAdd),
				Command: []string{"/bin/sh", "-c", "./vectorAdd && sleep 60"},
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						gpuResourceName: *resource.NewQuantity(1, resource.DecimalSI),
					},
				},
			},
		},
	}
	ns := f.Namespace.Name
	_, err := e2ejob.CreateJob(ctx, f.ClientSet, ns, testJob)
	framework.ExpectNoError(err)
	framework.Logf("Created job %v", testJob)
}

// VerifyJobNCompletions verifies that the job has completions number of successful pods
func VerifyJobNCompletions(ctx context.Context, f *framework.Framework, completions int32) {
	ns := f.Namespace.Name
	pods, err := e2ejob.GetJobPods(ctx, f.ClientSet, f.Namespace.Name, "cuda-add")
	framework.ExpectNoError(err)
	createdPods := pods.Items
	createdPodNames := podNames(createdPods)
	framework.Logf("Got the following pods for job cuda-add: %v", createdPodNames)

	successes := int32(0)
	regex := regexp.MustCompile("PASSED")
	for _, podName := range createdPodNames {
		e2epod.NewPodClient(f).WaitForFinish(ctx, podName, 5*time.Minute)
		logs, err := e2epod.GetPodLogs(ctx, f.ClientSet, ns, podName, "vector-addition")
		framework.ExpectNoError(err, "Should be able to get logs for pod %v", podName)
		if regex.MatchString(logs) {
			successes++
		}
	}
	if successes != completions {
		framework.Failf("Only got %v completions. Expected %v completions.", successes, completions)
	}
}

func podNames(pods []v1.Pod) []string {
	originalPodNames := make([]string, len(pods))
	for i, p := range pods {
		originalPodNames[i] = p.ObjectMeta.Name
	}
	return originalPodNames
}
