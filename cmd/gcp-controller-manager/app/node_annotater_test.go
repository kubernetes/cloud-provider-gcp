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
	"errors"
	"fmt"
	"testing"

	compute "google.golang.org/api/compute/v1"
)

func TestGetExternalID(t *testing.T) {
	cs := []struct {
		nodeUrl                 string
		project, zone, instance string
	}{
		{
			nodeUrl:  "gce://a/b/c",
			project:  "a",
			zone:     "b",
			instance: "c",
		},
		{
			nodeUrl: "gce://a/b/c/d",
		},
		{
			nodeUrl: "aws://a/b/c",
		},
		{
			nodeUrl: "/a/b/c",
		},
		{
			nodeUrl: "a/b/c",
		},
		{
			nodeUrl: "gce://a/b",
		},
	}

	for i, c := range cs {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			var project, zone, instance string
			na := &nodeAnnotater{
				getInstance: func(p, z, i string) (*compute.Instance, error) {
					project = p
					zone = z
					instance = i
					return nil, errors.New("err")
				},
			}
			na.getExternalID(c.nodeUrl)
			if c.project != project || c.zone != zone || c.instance != instance {
				t.Errorf("got:\t(%q,%q,%q)\nwant:\t(%q,%q,%q)", project, zone, instance, c.project, c.zone, c.instance)
			}
		})
	}
}
