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
	"maps"
	"reflect"
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
		{
			desc:           "empty updates should result in no change",
			existing:       nil,
			updates:        map[string]string{},
			expectedResult: nil,
		},
		{
			desc:     "nil existing map should be initialized and updated",
			existing: nil,
			updates:  map[string]string{"key": "val"},
			expectedResult: map[string]string{
				"key": "val",
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			originalExisting := maps.Clone(tc.existing)

			result := mergeMap(tc.existing, tc.updates)
			assert.Equal(t, tc.expectedResult, result)

			if tc.existing != nil {
				assert.Equal(t, originalExisting, tc.existing, "Input map should not be modified")
			}
		})
	}
}

func TestComputeNewAnnotationsIfNeeded(t *testing.T) {
	testCases := []struct {
		desc           string
		svc            *v1.Service
		newAnnotations map[string]string
		expectUpdate   bool
		expectedAnns   map[string]string
	}{
		{
			desc: "nil annotations in svc, new annotations added",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-svc",
					Namespace:   "test-ns",
					Annotations: nil,
				},
			},
			newAnnotations: map[string]string{"key1": "val1"},
			expectUpdate:   true,
			expectedAnns:   map[string]string{"key1": "val1"},
		},
		{
			desc: "empty annotations in svc, new annotations added",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-svc",
					Namespace:   "test-ns",
					Annotations: map[string]string{},
				},
			},
			newAnnotations: map[string]string{"key1": "val1"},
			expectUpdate:   true,
			expectedAnns:   map[string]string{"key1": "val1"},
		},
		{
			desc: "existing annotations, new annotations merged",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "test-ns",
					Annotations: map[string]string{
						"existing1": "old1",
						"key1":      "oldVal1",
					},
				},
			},
			newAnnotations: map[string]string{"key1": "val1", "key2": "val2"},
			expectUpdate:   true,
			expectedAnns: map[string]string{
				"existing1": "old1",
				"key1":      "val1",
				"key2":      "val2",
			},
		},
		{
			desc: "existing annotations, key removed",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "test-ns",
					Annotations: map[string]string{
						"key1": "val1",
						"key2": "val2",
					},
				},
			},
			newAnnotations: map[string]string{"key1": ""},
			expectUpdate:   true,
			expectedAnns:   map[string]string{"key2": "val2"},
		},
		{
			desc: "no changes to annotations",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "test-ns",
					Annotations: map[string]string{
						"key1": "val1",
					},
				},
			},
			newAnnotations: map[string]string{"key1": "val1"},
			expectUpdate:   false,
			expectedAnns:   nil,
		},
		{
			desc: "nil newAnnotations",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "test-ns",
					Annotations: map[string]string{
						"key1": "val1",
					},
				},
			},
			newAnnotations: nil,
			expectUpdate:   false,
			expectedAnns:   nil,
		},
		{
			desc: "empty newAnnotations",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "test-ns",
					Annotations: map[string]string{
						"key1": "val1",
					},
				},
			},
			newAnnotations: map[string]string{},
			expectUpdate:   false,
			expectedAnns:   nil,
		},
		{
			desc: "nil annotations in svc, empty new annotations",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-svc",
					Namespace:   "test-ns",
					Annotations: nil,
				},
			},
			newAnnotations: map[string]string{},
			expectUpdate:   false,
			expectedAnns:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Capture original annotations to check for unintended modifications
			originalAnnotations := make(map[string]string)
			if tc.svc.Annotations != nil {
				for k, v := range tc.svc.Annotations {
					originalAnnotations[k] = v
				}
			}

			newMeta, needsUpdate := computeNewAnnotationsIfNeeded(tc.svc, tc.newAnnotations)

			assert.Equal(t, tc.expectUpdate, needsUpdate)

			if tc.expectUpdate {
				assert.NotNil(t, newMeta)
				assert.True(t, reflect.DeepEqual(tc.expectedAnns, newMeta.Annotations), "Expected annotations: %v, got: %v", tc.expectedAnns, newMeta.Annotations)
				// Ensure the map was actually changed from the original
				assert.False(t, reflect.DeepEqual(originalAnnotations, newMeta.Annotations), "Annotations should have changed, but match original")
			} else {
				assert.Nil(t, newMeta)
				// Ensure original service annotations are not modified
				if len(originalAnnotations) == 0 && tc.svc.Annotations == nil {
					// Special case: nil to nil is fine
				} else {
					assert.True(t, reflect.DeepEqual(originalAnnotations, tc.svc.Annotations), "Original svc.Annotations should not be modified on no update")
				}
			}

			// Always check that the original service object's annotations map instance is not a different instance if no update was needed.
			if !needsUpdate {
				if len(originalAnnotations) > 0 || (len(originalAnnotations) == 0 && tc.svc.Annotations != nil) {
					assert.True(t, reflect.DeepEqual(originalAnnotations, tc.svc.Annotations), "Original svc.Annotations instance should not change when no update is needed")
				}
			}
		})
	}
}
