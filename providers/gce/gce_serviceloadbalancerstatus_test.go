//go:build !providerless
// +build !providerless

/*
Copyright 2024 The Kubernetes Authors.

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

package gce

import (
	"context"
	"fmt"
	"testing"

	svclbstatus "github.com/GoogleCloudPlatform/gke-networking-api/apis/serviceloadbalancerstatus/v1"
	fakesvclbstatusclient "github.com/GoogleCloudPlatform/gke-networking-api/client/serviceloadbalancerstatus/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ktesting "k8s.io/client-go/testing"
)

func TestInitializeServiceLoadBalancerStatusCRD(t *testing.T) {
	t.Parallel()
	gce := &Cloud{}

	t.Run("successful initialization", func(t *testing.T) {
		// A minimal valid config.
		config := &rest.Config{Host: "http://localhost:8080"}
		err := gce.InitializeServiceLoadBalancerStatusCRD(config)
		assert.NoError(t, err)
		assert.NotNil(t, gce.serviceLBStatusClient)
	})

	t.Run("initialization with invalid config", func(t *testing.T) {
		// An invalid config without a host will cause an error.
		err := gce.InitializeServiceLoadBalancerStatusCRD(&rest.Config{})
		assert.Error(t, err)
	})
}

func TestServiceLoadBalancerStatusStatusEqual(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc   string
		a      svclbstatus.ServiceLoadBalancerStatusStatus
		b      svclbstatus.ServiceLoadBalancerStatusStatus
		expect bool
	}{
		{
			desc:   "empty statuses",
			a:      svclbstatus.ServiceLoadBalancerStatusStatus{},
			b:      svclbstatus.ServiceLoadBalancerStatusStatus{},
			expect: true,
		},
		{
			desc: "equal lists, same order",
			a: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res1", "res2"},
			},
			b: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res1", "res2"},
			},
			expect: true,
		},
		{
			desc: "equal lists, different order",
			a: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res1", "res2"},
			},
			b: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res2", "res1"},
			},
			expect: true,
		},
		{
			desc: "equal lists with duplicates",
			a: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res1", "res2", "res1"},
			},
			b: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res2", "res1", "res1"},
			},
			expect: true,
		},
		{
			desc: "different lengths",
			a: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res1"},
			},
			b: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res1", "res2"},
			},
			expect: false,
		},
		{
			desc: "same length, different content",
			a: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res1", "res2"},
			},
			b: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res1", "res3"},
			},
			expect: false,
		},
		{
			desc: "one list is a subset of the other",
			a: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res1", "res2", "res1"},
			},
			b: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res2", "res1", "res3", "res1"},
			},
			expect: false,
		},
		{
			desc: "one list is empty",
			a: svclbstatus.ServiceLoadBalancerStatusStatus{
				GceResources: []string{"res1"},
			},
			b:      svclbstatus.ServiceLoadBalancerStatusStatus{},
			expect: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			gce := &Cloud{}
			result := gce.serviceLoadBalancerStatusStatusEqual(tc.a, tc.b)
			assert.Equal(t, tc.expect, result)
		})
	}
}

func TestGenerateServiceLoadBalancerStatus(t *testing.T) {
	t.Parallel()
	gce := &Cloud{}

	t.Run("valid service and resources", func(t *testing.T) {
		svc := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
				UID:       "test-uid",
			},
		}
		resources := []string{"url1", "url2"}

		cr := gce.generateServiceLoadBalancerStatus(svc, resources)
		require.NotNil(t, cr)
		assert.Equal(t, "test-service-status", cr.Name)
		assert.Equal(t, "default", cr.Namespace)
		require.Len(t, cr.OwnerReferences, 1)
		assert.Equal(t, "Service", cr.OwnerReferences[0].Kind)
		assert.Equal(t, "test-service", cr.OwnerReferences[0].Name)
		assert.Equal(t, svc.UID, cr.OwnerReferences[0].UID)
		assert.ElementsMatch(t, resources, cr.Status.GceResources)
	})

	t.Run("nil service", func(t *testing.T) {
		cr := gce.generateServiceLoadBalancerStatus(nil, []string{"url1"})
		assert.Nil(t, cr)
	})

	t.Run("empty resources", func(t *testing.T) {
		svc := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
			},
		}
		cr := gce.generateServiceLoadBalancerStatus(svc, []string{})
		require.NotNil(t, cr)
		assert.Empty(t, cr.Status.GceResources)
	})
}

func TestEnsureServiceLoadBalancerStatusCR(t *testing.T) {
	t.Parallel()

	resourceURLs := []string{"https://compute.googleapis.com/v1/projects/test-project/global/forwardingRules/a123"}

	t.Run("client not initialized", func(t *testing.T) {
		gce, err := fakeGCECloud(DefaultTestClusterValues())
		require.NoError(t, err)
		gce.serviceLBStatusClient = nil
		svc := fakeLoadbalancerService("")

		err = gce.EnsureServiceLoadBalancerStatusCR(svc, resourceURLs)
		assert.NoError(t, err, "Should not return an error if client is not initialized")
	})

	t.Run("CR does not exist, should create it", func(t *testing.T) {
		gce, err := fakeGCECloud(DefaultTestClusterValues())
		require.NoError(t, err)
		fakeSvcLbClient := fakesvclbstatusclient.NewSimpleClientset()
		gce.serviceLBStatusClient = fakeSvcLbClient

		svc := fakeLoadbalancerService("")
		err = gce.EnsureServiceLoadBalancerStatusCR(svc, resourceURLs)
		require.NoError(t, err)

		crName := svc.Name + "-status"
		cr, err := fakeSvcLbClient.NetworkingV1().ServiceLoadBalancerStatuses(svc.Namespace).Get(context.TODO(), crName, metav1.GetOptions{})
		require.NoError(t, err)
		assert.ElementsMatch(t, resourceURLs, cr.Status.GceResources)
	})

	t.Run("CR exists and is up-to-date, should do nothing", func(t *testing.T) {
		gce, err := fakeGCECloud(DefaultTestClusterValues())
		require.NoError(t, err)
		svc := fakeLoadbalancerService("")
		existingCR := gce.generateServiceLoadBalancerStatus(svc, resourceURLs)
		fakeSvcLbClient := fakesvclbstatusclient.NewSimpleClientset(existingCR)
		gce.serviceLBStatusClient = fakeSvcLbClient

		err = gce.EnsureServiceLoadBalancerStatusCR(svc, resourceURLs)
		require.NoError(t, err)
		assert.Len(t, fakeSvcLbClient.Actions(), 1, "Should only have one 'get' action")
		assert.Equal(t, "get", fakeSvcLbClient.Actions()[0].GetVerb())
	})

	t.Run("CR exists and is outdated, should update it", func(t *testing.T) {
		gce, err := fakeGCECloud(DefaultTestClusterValues())
		require.NoError(t, err)
		svc := fakeLoadbalancerService("")
		existingCR := gce.generateServiceLoadBalancerStatus(svc, []string{"old-resource-url"})
		fakeSvcLbClient := fakesvclbstatusclient.NewSimpleClientset(existingCR)
		gce.serviceLBStatusClient = fakeSvcLbClient

		err = gce.EnsureServiceLoadBalancerStatusCR(svc, resourceURLs)
		require.NoError(t, err)

		crName := svc.Name + "-status"
		updatedCR, err := fakeSvcLbClient.NetworkingV1().ServiceLoadBalancerStatuses(svc.Namespace).Get(context.TODO(), crName, metav1.GetOptions{})
		require.NoError(t, err)
		assert.ElementsMatch(t, resourceURLs, updatedCR.Status.GceResources)

		updateActionFound := false
		for _, action := range fakeSvcLbClient.Actions() {
			if action.GetVerb() == "update" && action.GetSubresource() == "status" {
				updateActionFound = true
				break
			}
		}
		assert.True(t, updateActionFound, "Expected an update-status action")
	})

	t.Run("failure on Get", func(t *testing.T) {
		gce, err := fakeGCECloud(DefaultTestClusterValues())
		require.NoError(t, err)
		fakeSvcLbClient := fakesvclbstatusclient.NewSimpleClientset()
		fakeSvcLbClient.PrependReactor("get", "serviceloadbalancerstatuses", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, errors.NewInternalError(fmt.Errorf("internal api error"))
		})
		gce.serviceLBStatusClient = fakeSvcLbClient
		svc := fakeLoadbalancerService("")

		err = gce.EnsureServiceLoadBalancerStatusCR(svc, resourceURLs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "internal api error")
	})

	t.Run("failure on Create", func(t *testing.T) {
		gce, err := fakeGCECloud(DefaultTestClusterValues())
		require.NoError(t, err)
		fakeSvcLbClient := fakesvclbstatusclient.NewSimpleClientset()
		fakeSvcLbClient.PrependReactor("create", "serviceloadbalancerstatuses", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, errors.NewInternalError(fmt.Errorf("internal api error"))
		})
		gce.serviceLBStatusClient = fakeSvcLbClient
		svc := fakeLoadbalancerService("")

		err = gce.EnsureServiceLoadBalancerStatusCR(svc, resourceURLs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "internal api error")
	})

	t.Run("failure on UpdateStatus", func(t *testing.T) {
		gce, err := fakeGCECloud(DefaultTestClusterValues())
		require.NoError(t, err)
		svc := fakeLoadbalancerService("")
		existingCR := gce.generateServiceLoadBalancerStatus(svc, []string{"old-resource-url"})
		fakeSvcLbClient := fakesvclbstatusclient.NewSimpleClientset(existingCR)
		fakeSvcLbClient.PrependReactor("update", "serviceloadbalancerstatuses", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			if action.GetSubresource() == "status" {
				return true, nil, errors.NewInternalError(fmt.Errorf("internal api error"))
			}
			return false, nil, nil
		})
		gce.serviceLBStatusClient = fakeSvcLbClient

		err = gce.EnsureServiceLoadBalancerStatusCR(svc, resourceURLs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "internal api error")
	})
}
