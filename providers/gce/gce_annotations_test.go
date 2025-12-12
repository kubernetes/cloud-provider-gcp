//go:build !providerless
// +build !providerless

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

package gce

import (
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

func TestServiceNetworkTierAnnotationKey(t *testing.T) {
	createTestService := func() *v1.Service {
		return &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "randome-uid",
				Name:      "test-svc",
				Namespace: "test-ns",
			},
		}
	}

	for testName, testCase := range map[string]struct {
		annotations  map[string]string
		expectedTier cloud.NetworkTier
		expectErr    bool
	}{
		"Use the default when the annotation does not exist": {
			annotations:  nil,
			expectedTier: cloud.NetworkTierDefault,
		},
		"Standard tier": {
			annotations:  map[string]string{NetworkTierAnnotationKey: "Standard"},
			expectedTier: cloud.NetworkTierStandard,
		},
		"Premium tier": {
			annotations:  map[string]string{NetworkTierAnnotationKey: "Premium"},
			expectedTier: cloud.NetworkTierPremium,
		},
		"Report an error on invalid network tier value": {
			annotations:  map[string]string{NetworkTierAnnotationKey: "Unknown-tier"},
			expectedTier: cloud.NetworkTierPremium,
			expectErr:    true,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			svc := createTestService()
			svc.Annotations = testCase.annotations
			actualTier, err := GetServiceNetworkTier(svc)
			assert.Equal(t, testCase.expectedTier, actualTier)
			assert.Equal(t, testCase.expectErr, err != nil)
		})
	}
}

func TestMergeMap(t *testing.T) {
	for _, tc := range []struct {
		desc           string
		existing       map[string]string
		updates        map[string]string
		expectedResult map[string]string
	}{
		{
			desc:     "new annotations should be added",
			existing: map[string]string{"key1": "val1"},
			updates:  map[string]string{"key2": "val2"},
			expectedResult: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
		},
		{
			desc:     "existing annotations should be overwritten",
			existing: map[string]string{"key1": "val1"},
			updates:  map[string]string{"key1": "val2"},
			expectedResult: map[string]string{
				"key1": "val2",
			},
		},
		{
			desc:     "empty value in updates should delete the key",
			existing: map[string]string{"key1": "val1", "key2": "val2"},
			updates:  map[string]string{"key1": ""},
			expectedResult: map[string]string{
				"key2": "val2",
			},
		},
		{
			desc:     "mixed updates and deletions",
			existing: map[string]string{"key1": "val1", "key2": "val2"},
			updates:  map[string]string{"key1": "new-val", "key2": ""},
			expectedResult: map[string]string{
				"key1": "new-val",
			},
		},
		{
			desc:           "empty updates should result in no change",
			existing:       map[string]string{"key1": "val1"},
			updates:        map[string]string{},
			expectedResult: map[string]string{"key1": "val1"},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			mergeMap(tc.existing, tc.updates)
			assert.Equal(t, tc.expectedResult, tc.existing)
		})
	}
}
