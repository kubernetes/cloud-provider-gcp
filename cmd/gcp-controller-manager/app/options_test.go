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

// Package app implements a server that runs a stand-alone version of the
// certificates controller.
package app

import (
	"testing"
)

func TestGetRegionFromZone(t *testing.T) {
	var testcases = []struct {
		zone         string
		expectRegion string
		shouldError  bool
	}{
		{
			zone:         "us-central1-f",
			expectRegion: "us-central1",
		},
		{
			zone:         "us-central1-foobar",
			expectRegion: "us-central1",
		},
		{
			zone:        "invalid-input",
			shouldError: true,
		},
	}

	for _, tc := range testcases {
		region, err := getRegionFromZone(tc.zone)

		hasError := err != nil
		if hasError != tc.shouldError {
			t.Errorf("Zone %s: expect error %v, got error %v", tc.zone, tc.shouldError, err)
			continue
		}

		if err == nil && tc.expectRegion != region {
			t.Errorf("Zone %s: expect to get region %s, got region %s", tc.zone, tc.expectRegion, region)
		}
	}
}
