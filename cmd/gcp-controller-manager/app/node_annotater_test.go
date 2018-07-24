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
	"reflect"
	"testing"

	compute "google.golang.org/api/compute/v1"
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

func TestExtrackKubeLabels(t *testing.T) {
	var something = "something"
	cs := map[string]struct {
		vm        *compute.Instance
		in        string
		out       map[string]string
		expectErr bool
	}{
		"no metadata": {
			vm:  &compute.Instance{},
			out: map[string]string{},
		},
		"no 'kube-labels' metadata": {
			vm: &compute.Instance{
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key:   something,
							Value: &something,
						},
					},
				},
			},
			out: map[string]string{},
		},
		"no value of 'kube-labels' metadata": {
			vm: &compute.Instance{
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{
							Key: something,
						},
					},
				},
			},
			out: map[string]string{},
		},
		"empty 'kube-labels'": {
			in:  "",
			out: map[string]string{},
		},
		"unformated 'kube-labels'": {
			in:        "hi",
			expectErr: true,
		},
		"valid label 1": {
			in: "hi=",
			out: map[string]string{
				"hi": "",
			},
		},
		"valid label 2": {
			in: "hi=hi",
			out: map[string]string{
				"hi": "hi",
			},
		},
		"valid label 3": {
			in: "google.google/hi=hi",
			out: map[string]string{
				"google.google/hi": "hi",
			},
		},
		"valid labels": {
			in: "a=b,c=d",
			out: map[string]string{
				"a": "b",
				"c": "d",
			},
		},
		"invalid label key": {
			in:        "a.^=5",
			expectErr: true,
		},
		"invalid label value": {
			in:        "a=5^",
			expectErr: true,
		},
		"invalid labels 1": {
			in:        "a=b,c",
			expectErr: true,
		},
		"invalid labels 2": {
			in:        ",,,",
			expectErr: true,
		},
		"invalid labels 3": {
			in:        " ",
			expectErr: true,
		},
	}

	for name, c := range cs {
		t.Run(name, func(t *testing.T) {
			vm := c.vm
			if vm == nil {
				vm = &compute.Instance{
					Metadata: &compute.Metadata{
						Items: []*compute.MetadataItems{
							{
								Key:   "kube-labels",
								Value: &c.in,
							},
						},
					},
				}
			}
			out, err := extractKubeLabels(vm)
			if got, want := out, c.out; !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected labels\n\tgot:\t%v\n\twant:\t%v", got, want)
			}
			if got, want := (err != nil), c.expectErr; got != want {
				t.Errorf("unexpected error value: %v", err)
			}
		})
	}
}
