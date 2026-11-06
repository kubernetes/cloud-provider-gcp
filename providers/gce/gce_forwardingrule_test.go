//go:build !providerless
// +build !providerless

/*
Copyright 2026 The Kubernetes Authors.

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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	compute "google.golang.org/api/compute/v1"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
)

func TestSetRegionForwardingRuleLabelsSerializesLabels(t *testing.T) {
	testCases := []struct {
		name     string
		existing map[string]string
		desired  map[string]string
		wantJSON string
	}{
		{
			name:     "clears labels with an explicit empty object",
			existing: map[string]string{"stale": "label"},
			desired:  map[string]string{},
			wantJSON: `{"labelFingerprint":"fingerprint","labels":{}}`,
		},
		{
			name:     "sets labels",
			existing: map[string]string{"stale": "label"},
			desired:  map[string]string{"environment": "test"},
			wantJSON: `{"labelFingerprint":"fingerprint","labels":{"environment":"test"}}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gce, err := fakeGCECloud(DefaultTestClusterValues())
			require.NoError(t, err)

			var request *compute.RegionSetLabelsRequest
			mockGCE := gce.c.(*cloud.MockGCE)
			mockGCE.MockForwardingRules.SetLabelsHook = func(_ context.Context, _ *meta.Key, got *compute.RegionSetLabelsRequest, _ *cloud.MockForwardingRules, _ ...cloud.Option) error {
				request = got
				return nil
			}

			err = gce.SetRegionForwardingRuleLabels(&compute.ForwardingRule{
				Name:             "rule",
				Labels:           tc.existing,
				LabelFingerprint: "fingerprint",
			}, gce.region, tc.desired)
			require.NoError(t, err)
			require.NotNil(t, request)
			assert.Equal(t, []string{"Labels"}, request.ForceSendFields)

			body, err := json.Marshal(request)
			require.NoError(t, err)
			assert.JSONEq(t, tc.wantJSON, string(body))
		})
	}
}
