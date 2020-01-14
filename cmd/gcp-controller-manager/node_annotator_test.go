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

package main

import (
	"fmt"
	"reflect"
	"testing"

	compute "google.golang.org/api/compute/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	v1lister "k8s.io/client-go/listers/core/v1"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"
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
		{
			nodeURL:  "gce://foo.com:a-c12/us-moon3-a/c-c-1",
			project:  "foo.com:a-c12",
			zone:     "us-moon3-a",
			instance: "c-c-1",
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
		vm                  *compute.Instance
		in                  string
		out                 map[string]string
		expectNoMetadataErr bool
		expectErr           bool
	}{
		"no metadata": {
			vm:                  &compute.Instance{},
			expectNoMetadataErr: true,
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
			expectNoMetadataErr: true,
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
			expectNoMetadataErr: true,
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
			if c.expectNoMetadataErr && err != errNoMetadata {
				t.Errorf("got %v, want errNoMetadata", err)
			}
			if got, want := (err != nil), c.expectErr || c.expectNoMetadataErr; got != want {
				t.Errorf("unexpected error value: %v", err)
			}
		})
	}
}

type fakeNodeLister struct {
	v1lister.NodeLister
	node *core.Node
	err  error
}

func (f fakeNodeLister) Get(name string) (*core.Node, error) { return f.node, f.err }

func TestNodeAnnotatorSync(t *testing.T) {
	node := &core.Node{
		TypeMeta: v1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "test-node",
		},
	}
	annUpdate := annotator{
		name:     "foo",
		annotate: func(*core.Node, *compute.Instance) bool { return true },
	}
	annNoUpdate := annotator{
		name:     "bar",
		annotate: func(*core.Node, *compute.Instance) bool { return false },
	}
	tests := []struct {
		desc        string
		node        *core.Node
		getErr      error
		annotators  []annotator
		wantActions []ktesting.Action
		wantRequeue bool
	}{
		{
			desc:       "success and update",
			node:       node,
			annotators: []annotator{annUpdate},
			wantActions: []ktesting.Action{
				ktesting.NewUpdateAction(schema.GroupVersionResource{Version: "v1", Resource: "nodes"}, "", node),
			},
		},
		{
			desc:        "success and no update",
			node:        node,
			annotators:  []annotator{annNoUpdate},
			wantActions: []ktesting.Action{},
		},
		{
			desc:       "success and mixed annotators",
			node:       node,
			annotators: []annotator{annUpdate, annNoUpdate},
			wantActions: []ktesting.Action{
				ktesting.NewUpdateAction(schema.GroupVersionResource{Version: "v1", Resource: "nodes"}, "", node),
			},
		},
		{
			desc:        "get node error, requeue",
			node:        node,
			getErr:      errors.NewInternalError(fmt.Errorf("foo")),
			wantActions: []ktesting.Action{},
			wantRequeue: true,
		},
		{
			desc:        "node not found, don't requeue",
			node:        node,
			getErr:      errors.NewNotFound(schema.GroupResource{Resource: "nodes"}, node.Name),
			wantActions: []ktesting.Action{},
			wantRequeue: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			c := fake.NewSimpleClientset(tt.node)
			na := &nodeAnnotator{
				c:           c,
				ns:          fakeNodeLister{node: tt.node, err: tt.getErr},
				getInstance: func(nodeURL string) (*compute.Instance, error) { return nil, nil },
				annotators:  tt.annotators,
				queue:       workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
			}
			na.sync("test-node")

			if !reflect.DeepEqual(tt.wantActions, c.Actions()) {
				t.Errorf("got actions:\n%+v\nwant actions\n%+v", c.Actions(), tt.wantActions)
			}
			if gotRequeue := na.queue.Len() > 0; gotRequeue != tt.wantRequeue {
				t.Errorf("node requeued: %v, want: %v", gotRequeue, tt.wantRequeue)
			}
		})
	}
}
