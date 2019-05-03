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

package main

import (
	"testing"
)

func TestGetRegionFromLocation(t *testing.T) {
	testCases := []struct {
		location     string
		expectRegion string
		shouldError  bool
	}{
		{
			location:     "us-central1-f",
			expectRegion: "us-central1",
		},
		{
			location:     "us-central1-foobar",
			expectRegion: "us-central1",
		},
		{
			location:     "us-central1",
			expectRegion: "us-central1",
		},
		{
			location:    "invalid input",
			shouldError: true,
		},
		{
			location:    "",
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		region, err := getRegionFromLocation(tc.location)

		hasError := err != nil
		if hasError != tc.shouldError {
			t.Errorf("getRegionFromLocation(%q): expect error %v, got error %q", tc.location, tc.shouldError, err)
		}

		if tc.expectRegion != region {
			t.Errorf("getRegionFromLocation(%q): expect to get region %q, got region %q", tc.location, tc.expectRegion, region)
		}
	}
}
