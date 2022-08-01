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
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/klog/v2"
)

func generateFileFromStringList(fileName string, list []string) error {
	file, err := os.Create(fileName)
	if err != nil {
		panic(fmt.Sprintf("Problem opening %s, error %v\n", fileName, err))
	}
	defer file.Close()
	for _, item := range list {
		bytes, err := file.WriteString(item + "\n")
		klog.Infof("write %v bytes to file %v", bytes, fileName)
		if err != nil {
			return err
		}
	}
	return nil
}

func clearFile(fileName string) error {
	return os.Remove(fileName)
}

func TestLoadCSRAllowList(t *testing.T) {
	testCases := []struct {
		testName         string
		allowList        []string
		csrAllowListPath string
		expectErr        error
	}{
		{
			testName:         "success get allow list from file",
			allowList:        []string{"allow1", "allow2"},
			csrAllowListPath: "/tmp/dataTestLoadCSRAllowList",
		},
		{
			testName:  "fail get allow list from file when file name is nil",
			allowList: []string{"allow1", "allow2"},
			expectErr: fmt.Errorf("csrAllowListPath is nil"),
		},
	}

	for _, tc := range testCases {
		klog.Infof("running %v test", tc.testName)
		if len(tc.csrAllowListPath) != 0 {
			if err := generateFileFromStringList(tc.csrAllowListPath, tc.allowList); err != nil {
				t.Fatalf("fail to create file: %v, err: %v", tc.csrAllowListPath, err)
			}
		}
		gotList, err := loadCSRAllowList(tc.csrAllowListPath)
		if err != nil && (tc.expectErr == nil || err.Error() != tc.expectErr.Error()) {
			t.Fatalf("loadCSRAllowList(%v): expect error %v, got error %v", tc.csrAllowListPath, tc.expectErr, err)
		}

		if diff := cmp.Diff(gotList, tc.allowList); diff != "" {
			t.Fatalf("allowList don't match, diff -want +got\n%s", diff)
		}
	}
}

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
