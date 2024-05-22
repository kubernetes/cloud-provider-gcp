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
	"time"

	. "github.com/onsi/ginkgo/v2"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	gcePDDetachTimeout  = 10 * time.Minute
	gcePDDetachPollTime = 10 * time.Second
)

// Testing configurations of single a PV/PVC pair attached to a GCE PD
var _ = Describe("[cloud-provider-gcp-e2e] Persistent Volumes", func() {
	var (
		c         clientset.Interface
		diskName  string
		ns        string
		err       error
		pv        *v1.PersistentVolume
		pvc       *v1.PersistentVolumeClaim
		clientPod *v1.Pod
		pvConfig  e2epv.PersistentVolumeConfig
		pvcConfig e2epv.PersistentVolumeClaimConfig
		volLabel  labels.Set
		selector  *metav1.LabelSelector
		node      types.NodeName
	)

	f := framework.NewDefaultFramework("pv")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged
	BeforeEach(func(ctx context.Context) {
		c = f.ClientSet
		ns = f.Namespace.Name

		// Enforce binding only within test space via selector labels
		volLabel = labels.Set{e2epv.VolumeSelectorKey: ns}
		selector = metav1.SetAsLabelSelector(volLabel)

		By("Initializing Test Spec")
		diskName, err = e2epv.CreatePDWithRetry(ctx)
		framework.ExpectNoError(err)

		pvConfig = e2epv.PersistentVolumeConfig{
			NamePrefix: "gce-",
			Labels:     volLabel,
			PVSource: v1.PersistentVolumeSource{
				GCEPersistentDisk: &v1.GCEPersistentDiskVolumeSource{
					PDName:   diskName,
					FSType:   e2epv.GetDefaultFSType(),
					ReadOnly: false,
				},
			},
			Prebind: nil,
		}
		emptyStorageClass := ""
		pvcConfig = e2epv.PersistentVolumeClaimConfig{
			Selector:         selector,
			StorageClassName: &emptyStorageClass,
		}
		clientPod, pv, pvc = initializeGCETestSpec(ctx, c, f.Timeouts, ns, pvConfig, pvcConfig, false)
		node = types.NodeName(clientPod.Spec.NodeName)
	})

	AfterEach(func(ctx context.Context) {
		framework.Logf("AfterEach: Cleaning up test resources")
		if c != nil {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, c, clientPod))
			if errs := e2epv.PVPVCCleanup(ctx, c, ns, pv, pvc); len(errs) > 0 {
				framework.Failf("AfterEach: Failed to delete PVC and/or PV. Errors: %v", utilerrors.NewAggregate(errs))
			}
			clientPod, pv, pvc, node = nil, nil, nil, ""
			if diskName != "" {
				framework.ExpectNoError(e2epv.DeletePDWithRetry(ctx, diskName))
			}
		}
	})

	// Attach a persistent disk to a pod using a PVC.
	// Delete the PVC and then the pod.  Expect the pod to succeed in unmounting and detaching PD on delete.
	f.It("should test that deleting a PVC before the pod does not cause pod deletion to fail on PD detach", func(ctx context.Context) {

		By("Deleting the Claim")
		framework.ExpectNoError(e2epv.DeletePersistentVolumeClaim(ctx, c, pvc.Name, ns), "Unable to delete PVC ", pvc.Name)
		if !verifyGCEDiskAttached(diskName, node) {
			framework.Failf("Disk %s is not attached to node %s", diskName, node)
		}

		By("Deleting the Pod")
		framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, c, clientPod), "Failed to delete pod ", clientPod.Name)

		By("Verifying Persistent Disk detach")
		framework.ExpectNoError(waitForPDDetach(diskName, node), "PD ", diskName, " did not detach")
	})

	// Attach a persistent disk to a pod using a PVC.
	// Delete the PV and then the pod.  Expect the pod to succeed in unmounting and detaching PD on delete.
	f.It("should test that deleting the PV before the pod does not cause pod deletion to fail on PD detach", func(ctx context.Context) {

		By("Deleting the Persistent Volume")
		framework.ExpectNoError(e2epv.DeletePersistentVolume(ctx, c, pv.Name), "Failed to delete PV ", pv.Name)
		if !verifyGCEDiskAttached(diskName, node) {
			framework.Failf("Disk %s is not attached to node %s", diskName, node)
		}

		By("Deleting the client pod")
		framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, c, clientPod), "Failed to delete pod ", clientPod.Name)

		By("Verifying Persistent Disk detaches")
		framework.ExpectNoError(waitForPDDetach(diskName, node), "PD ", diskName, " did not detach")
	})

	// Test that a Pod and PVC attached to a GCEPD successfully unmounts and detaches when the encompassing Namespace is deleted.
	f.It("should test that deleting the Namespace of a PVC and Pod causes the successful detach of Persistent Disk", func(ctx context.Context) {

		By("Deleting the Namespace")
		err := c.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{})
		framework.ExpectNoError(err)

		// issue deletes for the client pod and claim, accelerating namespace controller actions
		e2epod.DeletePodOrFail(ctx, c, clientPod.Namespace, clientPod.Name)
		framework.ExpectNoError(e2epv.DeletePersistentVolumeClaim(ctx, c, pvc.Name, ns), "Unable to delete PVC ", pvc.Name)

		By("Verifying Persistent Disk detaches")
		framework.ExpectNoError(waitForPDDetach(diskName, node), "PD ", diskName, " did not detach")
	})
})

// verifyGCEDiskAttached performs a sanity check to verify the PD attached to the node
func verifyGCEDiskAttached(diskName string, nodeName types.NodeName) bool {
	gceCloud, err := GetGCECloud()
	framework.ExpectNoError(err)
	isAttached, err := gceCloud.DiskIsAttached(diskName, nodeName)
	framework.ExpectNoError(err)
	return isAttached
}

// initializeGCETestSpec creates a PV, PVC, and ClientPod that will run until killed by test or clean up.
func initializeGCETestSpec(ctx context.Context, c clientset.Interface, t *framework.TimeoutContext, ns string, pvConfig e2epv.PersistentVolumeConfig, pvcConfig e2epv.PersistentVolumeClaimConfig, isPrebound bool) (*v1.Pod, *v1.PersistentVolume, *v1.PersistentVolumeClaim) {
	By("Creating the PV and PVC")
	pv, pvc, err := e2epv.CreatePVPVC(ctx, c, t, pvConfig, pvcConfig, ns, isPrebound)
	framework.ExpectNoError(err)
	framework.ExpectNoError(e2epv.WaitOnPVandPVC(ctx, c, t, ns, pv, pvc))

	By("Creating the Client Pod")
	clientPod, err := e2epod.CreateClientPod(ctx, c, ns, pvc)
	framework.ExpectNoError(err)
	return clientPod, pv, pvc
}

// Waits for specified PD to detach from specified hostName
func waitForPDDetach(diskName string, nodeName types.NodeName) error {
	framework.Logf("Waiting for GCE PD %q to detach from node %q.", diskName, nodeName)
	gceCloud, err := GetGCECloud()
	if err != nil {
		return err
	}
	for start := time.Now(); time.Since(start) < gcePDDetachTimeout; time.Sleep(gcePDDetachPollTime) {
		diskAttached, err := gceCloud.DiskIsAttached(diskName, nodeName)
		if err != nil {
			framework.Logf("Error waiting for PD %q to detach from node %q. 'DiskIsAttached(...)' failed with %v", diskName, nodeName, err)
			return err
		}
		if !diskAttached {
			// Specified disk does not appear to be attached to specified node
			framework.Logf("GCE PD %q appears to have successfully detached from %q.", diskName, nodeName)
			return nil
		}
		framework.Logf("Waiting for GCE PD %q to detach from %q.", diskName, nodeName)
	}
	return fmt.Errorf("Gave up waiting for GCE PD %q to detach from %q after %v", diskName, nodeName, gcePDDetachTimeout)
}
