/*
Copyright 2025 The Kubernetes Authors.

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

package ipam

import (
	"errors"
	"testing"
	"reflect"
	"sync"
	"time"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTaskQueue(t *testing.T) {
	t.Parallel()
	synced := map[*v1.Node]bool{}
	doneCh := make(chan struct{}, 1)

	// var tq TaskQueue[*v1.Node]
	sync := func(key *v1.Node) error {
		synced[key] = true
		switch key.Name {
		case "err":
			return errors.New("injected error")
		case "stop":
			doneCh <- struct{}{}
		case "more":
			t.Error("synced after TaskQueue.Shutdown()")
		}
		return nil
	}

	tq := NewTaskQueue("", "test", 2, sync)

	go tq.Run()
	nodeA := makeNode("a")
	nodeB := makeNode("b")
	nodeErr := makeNode("err")
	nodeStop := makeNode("stop")
	tq.Enqueue(nodeA)
	tq.Enqueue(nodeB)
	tq.Enqueue(nodeErr)
	tq.Enqueue(nodeStop)

	<-doneCh
	tq.Shutdown()

	// Enqueue after Shutdown isn't going to be synced.
	tq.Enqueue(makeNode("more"))

	expected := map[*v1.Node]bool{
		nodeA:    true,
		nodeB:    true,
		nodeErr:  true,
		nodeStop: true,
	}

	if !reflect.DeepEqual(synced, expected) {
		t.Errorf("task queue synced %+v, want %+v", synced, expected)
	}
}

func makeNode(name string) *v1.Node{
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				testNodePoolSubnetLabelPrefix: "subnet1",
			},
		},
	}
}

func TestQueueWithNodeObject(t *testing.T) {
	synced := sync.Map{}
	sync := func(key *v1.Node) error {
		synced.Store(key, true)
		switch key.Name {
		case "err":
			return errors.New("injected error")
		}
		return nil
	}
	errObj := makeNode("err")
	inputObjsWithErr := []*v1.Node{makeNode("a"), makeNode("b"), makeNode("c"), makeNode("d"), makeNode("e"), makeNode("f"), errObj, makeNode("g")}

	tq := NewTaskQueue("multiple-workers", "test", 5, sync)
	// Spawn off worker routines in parallel.
	tq.Run()

	for _, obj := range inputObjsWithErr {
		tq.Enqueue(obj)
	}

	for tq.queue.Len() > 0 {
		time.Sleep(1 * time.Second)
	}

	if tq.queue.NumRequeues(errObj) == 0 {
		t.Errorf("Got 0 requeues for %q, expected non-zero requeue on error.", "err")
	}
	tq.Shutdown()

	// Enqueue after Shutdown isn't going to be synced.
	tq.Enqueue(makeNode("more"))

	syncedLen := 0
	synced.Range(func(_, _ interface{}) bool {
		syncedLen++
		return true
	})

	if syncedLen != len(inputObjsWithErr) {
		t.Errorf("Synced %d keys, but %d input keys were provided.", syncedLen, len(inputObjsWithErr))
	}
	for _, key := range inputObjsWithErr {
		if _, ok := synced.Load(key); !ok {
			t.Errorf("Did not sync input key - %s.", key)
		}
	}
}

func TestQueueWithMultipleWorkers(t *testing.T) {
	t.Parallel()
	// Use a sync map since multiple goroutines will write to disjoint keys in parallel.
	synced := sync.Map{}
	sync := func(key string) error {
		synced.Store(key, true)
		switch key {
		case "err":
			return errors.New("injected error")
		}
		return nil
	}
	validInputObjs := []string{"a", "b", "c", "d", "e", "f", "g"}
	inputObjsWithErr := []string{"a", "b", "c", "d", "e", "f", "err", "g"}
	testCases := []struct {
		desc                string
		numWorkers          int
		expectRequeueForKey string
		inputObjs           []string
		expectNil           bool
	}{
		{"queue with 0 workers should fail", 0, "", nil, true},
		{"queue with 1 worker should work", 1, "", validInputObjs, false},
		{"queue with multiple workers should work", 5, "", validInputObjs, false},
		{"queue with multiple workers should requeue errors", 5, "err", inputObjsWithErr, false},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			tq := NewTaskQueue("multiple-workers", "test", tc.numWorkers, sync)
			gotNil := tq == nil
			if gotNil != tc.expectNil {
				t.Errorf("gotNilQueue - %v, expectNilQueue - %v.", gotNil, tc.expectNil)
			}
			if tq == nil {
				return
			}
			// Spawn off worker routines in parallel.
			tq.Run()

			for _, obj := range tc.inputObjs {
				tq.Enqueue(obj)
			}

			for tq.queue.Len() > 0 {
				time.Sleep(1 * time.Second)
			}

			if tc.expectRequeueForKey != "" {
				if tq.queue.NumRequeues(tc.expectRequeueForKey) == 0 {
					t.Errorf("Got 0 requeues for %q, expected non-zero requeue on error.", tc.expectRequeueForKey)
				}
			}
			tq.Shutdown()

			// Enqueue after Shutdown isn't going to be synced.
			tq.Enqueue("more")

			syncedLen := 0
			synced.Range(func(_, _ interface{}) bool {
				syncedLen++
				return true
			})

			if syncedLen != len(tc.inputObjs) {
				t.Errorf("Synced %d keys, but %d input keys were provided.", syncedLen, len(tc.inputObjs))
			}
			for _, key := range tc.inputObjs {
				if _, ok := synced.Load(key); !ok {
					t.Errorf("Did not sync input key - %s.", key)
				}
			}
		})
	}
}