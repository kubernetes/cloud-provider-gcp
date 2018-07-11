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

package app

import (
	"fmt"
	"testing"
)

func TestParseNodeURL(t *testing.T) {
	cs := []struct {
		nodeURL                 string
		project, zone, instance string
		expectErr               bool
	}{
		{
			nodeURL:  "gce://a/b/c",
			project:  "a",
			zone:     "b",
			instance: "c",
		},
		{
			nodeURL:   "gce://a/b/c/d",
			expectErr: true,
		},
		{
			nodeURL:   "aws://a/b/c",
			expectErr: true,
		},
		{
			nodeURL:   "/a/b/c",
			expectErr: true,
		},
		{
			nodeURL:   "a/b/c",
			expectErr: true,
		},
		{
			nodeURL:   "gce://a/b",
			expectErr: true,
		},
	}

	for i, c := range cs {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			project, zone, instance, err := parseNodeURL(c.nodeURL)
			if c.project != project || c.zone != zone || c.instance != instance {
				t.Errorf("got:\t(%q,%q,%q)\nwant:\t(%q,%q,%q)", project, zone, instance, c.project, c.zone, c.instance)
			}
			if (err != nil) != c.expectErr {
				t.Errorf("unexpected value of err: %v", err)
			}
		})
	}
}
