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

package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	testutils "k8s.io/kubernetes/test/utils"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	// recreateNodeReadyAgainTimeout is how long a node is allowed to become "Ready" after it is recreated before
	// the test is considered failed.
	recreateNodeReadyAgainTimeout = 10 * time.Minute
)

var _ = ginkgo.Describe("[cloud-provider-gcp-e2e] Recreate Nodes", func() {
	f := framework.NewDefaultFramework("recreate-nodes")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged
	var originalNodes []v1.Node
	var originalPodNames []string
	var ps *testutils.PodStore
	systemNamespace := metav1.NamespaceSystem
	ginkgo.BeforeEach(func(ctx context.Context) {
		var err error
		numNodes, err := e2enode.TotalRegistered(ctx, f.ClientSet)
		framework.ExpectNoError(err)
		originalNodes, err = e2enode.CheckReady(ctx, f.ClientSet, numNodes, framework.NodeReadyInitialTimeout)
		framework.ExpectNoError(err)

		framework.Logf("Got the following nodes before recreate %v", nodeNames(originalNodes))

		ps, err = testutils.NewPodStore(f.ClientSet, systemNamespace, labels.Everything(), fields.Everything())
		framework.ExpectNoError(err)
		allPods := ps.List()
		originalPods := e2epod.FilterNonRestartablePods(allPods)
		originalPodNames = make([]string, len(originalPods))
		for i, p := range originalPods {
			originalPodNames[i] = p.ObjectMeta.Name
		}

		if !e2epod.CheckPodsRunningReadyOrSucceeded(ctx, f.ClientSet, systemNamespace, originalPodNames, framework.PodReadyBeforeTimeout) {
			framework.Failf("At least one pod wasn't running and ready or succeeded at test start.")
		}

	})

	ginkgo.AfterEach(func(ctx context.Context) {
		if ginkgo.CurrentSpecReport().Failed() {
			// Make sure that addon/system pods are running, so dump
			// events for the kube-system namespace on failures
			ginkgo.By(fmt.Sprintf("Collecting events from namespace %q.", systemNamespace))
			events, err := f.ClientSet.CoreV1().Events(systemNamespace).List(ctx, metav1.ListOptions{})
			framework.ExpectNoError(err)

			for _, e := range events.Items {
				framework.Logf("event for %v: %v %v: %v", e.InvolvedObject.Name, e.Source, e.Reason, e.Message)
			}
		}
		if ps != nil {
			ps.Stop()
		}
	})

	ginkgo.It("recreate nodes and ensure they function upon restart", func(ctx context.Context) {
		err := RecreateNodes(f.ClientSet, originalNodes)
		if err != nil {
			framework.Failf("Test failed; failed to start the restart instance group command.")
		}

		err = WaitForNodeBootIdsToChange(ctx, f.ClientSet, originalNodes, recreateNodeReadyAgainTimeout)
		if err != nil {
			framework.Failf("Test failed; failed to recreate at least one node in %v.", recreateNodeReadyAgainTimeout)
		}

		nodesAfter, err := e2enode.CheckReady(ctx, f.ClientSet, len(originalNodes), framework.RestartNodeReadyAgainTimeout)
		framework.ExpectNoError(err)
		framework.Logf("Got the following nodes after recreate: %v", nodeNames(nodesAfter))

		if len(originalNodes) != len(nodesAfter) {
			framework.Failf("Had %d nodes before nodes were recreated, but now only have %d",
				len(originalNodes), len(nodesAfter))
		}

		// Make sure the pods from before node recreation are running/completed
		podCheckStart := time.Now()
		podNamesAfter, err := e2epod.WaitForNRestartablePods(ctx, ps, len(originalPodNames), framework.RestartPodReadyAgainTimeout)
		framework.ExpectNoError(err)
		remaining := framework.RestartPodReadyAgainTimeout - time.Since(podCheckStart)
		if !e2epod.CheckPodsRunningReadyOrSucceeded(ctx, f.ClientSet, systemNamespace, podNamesAfter, remaining) {
			framework.Failf("At least one pod wasn't running and ready after the restart.")
		}
	})
})

// RecreateNodes recreates the given nodes in a managed instance group.
func RecreateNodes(c clientset.Interface, nodes []v1.Node) error {
	// Build mapping from zone to nodes in that zone.
	nodeNamesByZone := make(map[string][]string)
	for i := range nodes {
		node := &nodes[i]

		if zone, ok := node.Labels[v1.LabelFailureDomainBetaZone]; ok {
			nodeNamesByZone[zone] = append(nodeNamesByZone[zone], node.Name)
			continue
		}

		if zone, ok := node.Labels[v1.LabelTopologyZone]; ok {
			nodeNamesByZone[zone] = append(nodeNamesByZone[zone], node.Name)
			continue
		}

		defaultZone := framework.TestContext.CloudConfig.Zone
		nodeNamesByZone[defaultZone] = append(nodeNamesByZone[defaultZone], node.Name)
	}

	// Find the sole managed instance group name
	var instanceGroup string
	if strings.Contains(framework.TestContext.CloudConfig.NodeInstanceGroup, ",") {
		return fmt.Errorf("Test does not support cluster setup with more than one managed instance group: %s", framework.TestContext.CloudConfig.NodeInstanceGroup)
	}
	instanceGroup = framework.TestContext.CloudConfig.NodeInstanceGroup

	// Recreate the nodes.
	for zone, nodeNames := range nodeNamesByZone {
		args := []string{
			"compute",
			fmt.Sprintf("--project=%s", framework.TestContext.CloudConfig.ProjectID),
			"instance-groups",
			"managed",
			"recreate-instances",
			instanceGroup,
		}

		args = append(args, fmt.Sprintf("--instances=%s", strings.Join(nodeNames, ",")))
		args = append(args, fmt.Sprintf("--zone=%s", zone))
		framework.Logf("Recreating instance group %s.", instanceGroup)
		stdout, stderr, err := framework.RunCmd("gcloud", args...)
		if err != nil {
			return fmt.Errorf("error recreating nodes: %s\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
	}
	return nil
}

// WaitForNodeBootIdsToChange waits for the boot ids of the given nodes to change in order to verify the node has been recreated.
func WaitForNodeBootIdsToChange(ctx context.Context, c clientset.Interface, nodes []v1.Node, timeout time.Duration) error {
	errMsg := []string{}
	for i := range nodes {
		node := &nodes[i]
		if err := wait.PollWithContext(ctx, 30*time.Second, timeout, func(ctx context.Context) (bool, error) {
			newNode, err := c.CoreV1().Nodes().Get(ctx, node.Name, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Could not get node info: %s. Retrying in %v.", err, 30*time.Second)
				return false, nil
			}
			return node.Status.NodeInfo.BootID != newNode.Status.NodeInfo.BootID, nil
		}); err != nil {
			errMsg = append(errMsg, "Error waiting for node %s boot ID to change: %s", node.Name, err.Error())
		}
	}
	if len(errMsg) > 0 {
		return fmt.Errorf(strings.Join(errMsg, ","))
	}
	return nil
}

func nodeNames(nodes []v1.Node) []string {
	result := make([]string, 0, len(nodes))
	for i := range nodes {
		result = append(result, nodes[i].Name)
	}
	return result
}
