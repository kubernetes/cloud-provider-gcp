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
	"sort"
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
			nodeURL:  "gce://example.com:legacy-project/b/c",
			project:  "example.com:legacy-project",
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

func TestExtractKubeLabels(t *testing.T) {
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

func TestExtractNodeTaints(t *testing.T) {
	var something = "something"
	cs := map[string]struct {
		vm                  *compute.Instance
		in                  string
		out                 []core.Taint
		expectNoMetadataErr bool
		expectErr           bool
	}{
		"no metadata": {
			vm:                  &compute.Instance{},
			expectNoMetadataErr: true,
		},
		"no 'kube-env' metadata": {
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
		"no value of 'kube-env' metadata": {
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
		"empty 'kube-env'": {
			in:  "",
			out: nil,
		},
		"valid taint without value": {
			in: "node_taints=k1=:NoSchedule",
			out: []core.Taint{
				{Key: "k1", Effect: core.TaintEffectNoSchedule},
			},
		},
		"valid taint with key, value and effect": {
			in: "node_taints=k1=v1:PreferNoSchedule",
			out: []core.Taint{
				{Key: "k1", Value: "v1", Effect: core.TaintEffectPreferNoSchedule},
			},
		},
		"valid taint with a domain name": {
			in: "node_taints=acme.com/taint-key=taint-value:NoExecute",
			out: []core.Taint{
				{Key: "acme.com/taint-key", Value: "taint-value", Effect: core.TaintEffectNoExecute},
			},
		},
		"valid kube-env with no taints": {
			in:  " ",
			out: nil,
		},
		"multiple valid taints": {
			in: "some_other_env=v1;node_taints=k1=v1:NoSchedule,k2=v2:NoExecute;some_other_env_2=v2",
			out: []core.Taint{
				{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule},
				{Key: "k2", Value: "v2", Effect: core.TaintEffectNoExecute},
			},
		},
		"invalid taint key": {
			in:        "node_taints=k1.^=v1:NoSchedule",
			expectErr: true,
		},
		"invalid taint value": {
			in:        "node_taints=k1=v1^:NoSchedule",
			expectErr: true,
		},
		"invalid taint effect": {
			in:        "node_taints=k1=v1:DontSchedule",
			expectErr: true,
		},
		"invalid taints without effect": {
			in:        "node_taints=a=b,c",
			expectErr: true,
		},
		"invalid taints with empty taint string": {
			in:        "node_taints=,,,",
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
								Key:   "kube-env",
								Value: &c.in,
							},
						},
					},
				}
			}
			out, err := extractNodeTaints(vm)
			if got, want := out, c.out; !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected taints\n\tgot:\t%v\n\twant:\t%v", got, want)
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

func TestMergeManagedLabels(t *testing.T) {
	cs := map[string]struct {
		lastAppliedLabels map[string]string
		liveLabels        map[string]string
		desiredLabels     map[string]string
		outLabels         map[string]string
		outAnnotation     map[string]string
		expectErr         bool
	}{
		"empty last applied label": {
			liveLabels:    map[string]string{},
			desiredLabels: map[string]string{"a": "1"},
			outLabels:     map[string]string{"a": "1"},
			outAnnotation: map[string]string{lastAppliedLabelsKey: "a=1"},
		},
		"empty desired label": {
			liveLabels:    map[string]string{"a": "1"},
			desiredLabels: map[string]string{},
			outLabels:     map[string]string{"a": "1"},
			outAnnotation: map[string]string{lastAppliedLabelsKey: ""},
		},
		"valid merge, same managed label set": {
			lastAppliedLabels: map[string]string{lastAppliedLabelsKey: "a=1,b=2"},
			liveLabels:        map[string]string{"a": "1", "b": "2", "c": "3"},
			desiredLabels:     map[string]string{"a": "3", "b": "4"},
			outLabels:         map[string]string{"a": "3", "b": "4", "c": "3"},
			outAnnotation:     map[string]string{lastAppliedLabelsKey: "a=3,b=4"},
		},
		"valid merge, remove managed label": {
			lastAppliedLabels: map[string]string{lastAppliedLabelsKey: "a=1,b=2"},
			liveLabels:        map[string]string{"a": "1", "b": "2", "c": "3"},
			desiredLabels:     map[string]string{"a": "3"},
			outLabels:         map[string]string{"a": "3", "c": "3"},
			outAnnotation:     map[string]string{lastAppliedLabelsKey: "a=3"},
		},
		"valid merge, add managed label": {
			lastAppliedLabels: map[string]string{lastAppliedLabelsKey: "a=1,b=2"},
			liveLabels:        map[string]string{"a": "1", "b": "2", "c": "3"},
			desiredLabels:     map[string]string{"a": "3", "aa": "3"},
			outLabels:         map[string]string{"a": "3", "c": "3", "aa": "3"},
			outAnnotation:     map[string]string{lastAppliedLabelsKey: "a=3,aa=3"},
		},
		"invalid last applied label": {
			lastAppliedLabels: map[string]string{lastAppliedLabelsKey: "a=1,b"},
			liveLabels:        map[string]string{"a": "1", "b": "2", "c": "3"},
			desiredLabels:     map[string]string{"a": "3", "aa": "3"},
			outLabels:         map[string]string{"a": "3", "b": "2", "c": "3", "aa": "3"},
			outAnnotation:     map[string]string{lastAppliedLabelsKey: "a=3,aa=3"},
		},
	}

	for name, c := range cs {
		t.Run(name, func(t *testing.T) {
			node := &core.Node{
				ObjectMeta: v1.ObjectMeta{
					Labels:      c.liveLabels,
					Annotations: c.lastAppliedLabels,
				},
			}
			err := mergeManagedLabels(node, c.desiredLabels)
			if got, want := node.ObjectMeta.Labels, c.outLabels; !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected labels\n\tgot:\t%v\n\twant:\t%v", got, want)
			}
			if got, want := node.ObjectMeta.Annotations, c.outAnnotation; !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected annotations\n\tgot:\t%v\n\twant:\t%v", got, want)
			}
			if got, want := (err != nil), c.expectErr; got != want {
				t.Errorf("unexpected error value: %v", err)
			}
		})
	}
}

func TestMergeManagedTaints(t *testing.T) {
	cs := map[string]struct {
		lastAppliedTaints map[string]string
		liveTaints        []core.Taint
		desiredTaints     []core.Taint
		outTaints         []core.Taint
		outAnnotation     map[string]string
		expectErr         bool
	}{
		"empty last applied taint": {
			liveTaints:    []core.Taint{},
			desiredTaints: []core.Taint{{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule}},
			outTaints:     []core.Taint{{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule}},
			outAnnotation: map[string]string{lastAppliedTaintsKey: "k1=v1:NoSchedule"},
		},
		"empty desired taint": {
			liveTaints:    []core.Taint{{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule}},
			desiredTaints: []core.Taint{},
			outTaints:     []core.Taint{{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule}},
			outAnnotation: map[string]string{lastAppliedTaintsKey: ""},
		},
		"valid merge - add taint": {
			lastAppliedTaints: map[string]string{lastAppliedTaintsKey: "k1=v1:NoSchedule,k2=v2:NoExecute"},
			liveTaints: []core.Taint{
				{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule},
				{Key: "k2", Value: "v2", Effect: core.TaintEffectNoExecute},
				{Key: "k3", Value: "v3", Effect: core.TaintEffectPreferNoSchedule},
			},
			desiredTaints: []core.Taint{
				{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule},
				{Key: "k2", Value: "v2", Effect: core.TaintEffectNoExecute},
				{Key: "k4", Value: "v4", Effect: core.TaintEffectPreferNoSchedule},
			},
			outTaints: []core.Taint{
				{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule},
				{Key: "k2", Value: "v2", Effect: core.TaintEffectNoExecute},
				{Key: "k3", Value: "v3", Effect: core.TaintEffectPreferNoSchedule},
				{Key: "k4", Value: "v4", Effect: core.TaintEffectPreferNoSchedule},
			},
			outAnnotation: map[string]string{lastAppliedTaintsKey: "k1=v1:NoSchedule,k2=v2:NoExecute,k4=v4:PreferNoSchedule"},
		},
		"valid merge - remove taint": {
			lastAppliedTaints: map[string]string{lastAppliedTaintsKey: "k1=v1:NoSchedule,k2=v2:NoExecute"},
			liveTaints: []core.Taint{
				{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule},
				{Key: "k2", Value: "v2", Effect: core.TaintEffectNoExecute},
				{Key: "k3", Value: "v3", Effect: core.TaintEffectPreferNoSchedule},
			},
			desiredTaints: []core.Taint{
				{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule},
			},
			outTaints: []core.Taint{
				{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule},
				{Key: "k3", Value: "v3", Effect: core.TaintEffectPreferNoSchedule},
			},
			outAnnotation: map[string]string{lastAppliedTaintsKey: "k1=v1:NoSchedule"},
		},
		"valid merge - update taint": {
			lastAppliedTaints: map[string]string{lastAppliedTaintsKey: "k1=v1:NoSchedule,k2=v2:NoExecute"},
			liveTaints: []core.Taint{
				{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule},
				{Key: "k2", Value: "v2", Effect: core.TaintEffectNoExecute},
				{Key: "k3", Value: "v3", Effect: core.TaintEffectPreferNoSchedule},
			},
			desiredTaints: []core.Taint{
				{Key: "k1", Value: "v4", Effect: core.TaintEffectPreferNoSchedule},
				{Key: "k2", Value: "v5", Effect: core.TaintEffectNoExecute},
			},
			outTaints: []core.Taint{
				{Key: "k1", Value: "v4", Effect: core.TaintEffectPreferNoSchedule},
				{Key: "k2", Value: "v5", Effect: core.TaintEffectNoExecute},
				{Key: "k3", Value: "v3", Effect: core.TaintEffectPreferNoSchedule},
			},
			outAnnotation: map[string]string{lastAppliedTaintsKey: "k1=v4:PreferNoSchedule,k2=v5:NoExecute"},
		},
		"valid merge - update, add and remove taint": {
			lastAppliedTaints: map[string]string{lastAppliedTaintsKey: "k1=v1:NoSchedule,k2=v2:NoExecute"},
			liveTaints: []core.Taint{
				{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule},
				{Key: "k2", Value: "v2", Effect: core.TaintEffectNoExecute},
				{Key: "k3", Value: "v3", Effect: core.TaintEffectPreferNoSchedule},
			},
			desiredTaints: []core.Taint{
				{Key: "k1", Value: "v5", Effect: core.TaintEffectPreferNoSchedule},
				{Key: "k4", Value: "v4", Effect: core.TaintEffectNoExecute},
			},
			outTaints: []core.Taint{
				{Key: "k1", Value: "v5", Effect: core.TaintEffectPreferNoSchedule},
				{Key: "k3", Value: "v3", Effect: core.TaintEffectPreferNoSchedule},
				{Key: "k4", Value: "v4", Effect: core.TaintEffectNoExecute},
			},
			outAnnotation: map[string]string{lastAppliedTaintsKey: "k1=v5:PreferNoSchedule,k4=v4:NoExecute"},
		},
		"invalid last applied taint": {
			lastAppliedTaints: map[string]string{lastAppliedTaintsKey: "k1=v1,k2"},
			liveTaints: []core.Taint{
				{Key: "k1", Value: "v1", Effect: core.TaintEffectNoSchedule},
				{Key: "k2", Value: "v2", Effect: core.TaintEffectNoExecute},
				{Key: "k3", Value: "v3", Effect: core.TaintEffectPreferNoSchedule},
			},
			desiredTaints: []core.Taint{
				{Key: "k1", Value: "v5", Effect: core.TaintEffectNoSchedule},
				{Key: "k4", Value: "v4", Effect: core.TaintEffectNoExecute},
			},
			outTaints: []core.Taint{
				{Key: "k1", Value: "v5", Effect: core.TaintEffectNoSchedule},
				{Key: "k2", Value: "v2", Effect: core.TaintEffectNoExecute},
				{Key: "k3", Value: "v3", Effect: core.TaintEffectPreferNoSchedule},
				{Key: "k4", Value: "v4", Effect: core.TaintEffectNoExecute},
			},
			outAnnotation: map[string]string{lastAppliedTaintsKey: "k1=v5:NoSchedule,k4=v4:NoExecute"},
		},
	}

	for name, c := range cs {
		t.Run(name, func(t *testing.T) {
			node := &core.Node{
				ObjectMeta: v1.ObjectMeta{
					Annotations: c.lastAppliedTaints,
				},
				Spec: core.NodeSpec{
					Taints: c.liveTaints,
				},
			}
			err := mergeManagedTaints(node, c.desiredTaints)
			sort.Slice(node.Spec.Taints, func(i, j int) bool {
				if node.Spec.Taints[i].Key < node.Spec.Taints[j].Key {
					return true
				}
				return false
			})
			if got, want := node.Spec.Taints, c.outTaints; !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected taints\n\tgot:\t%v\n\twant:\t%v", got, want)
			}
			if got, want := node.ObjectMeta.Annotations, c.outAnnotation; !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected annotations\n\tgot:\t%v\n\twant:\t%v", got, want)
			}
			if got, want := (err != nil), c.expectErr; got != want {
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
		wantErr     bool
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
			wantErr:     true,
		},
		{
			desc:        "node not found, don't requeue",
			node:        node,
			getErr:      errors.NewNotFound(schema.GroupResource{Resource: "nodes"}, node.Name),
			wantActions: []ktesting.Action{},
			wantErr:     false,
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
			err := na.sync("test-node")

			if !reflect.DeepEqual(tt.wantActions, c.Actions()) {
				t.Errorf("got actions:\n%+v\nwant actions\n%+v", c.Actions(), tt.wantActions)
			}
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("node sync got err: %v, want: %v", gotErr, tt.wantErr)
			}
		})
	}
}
