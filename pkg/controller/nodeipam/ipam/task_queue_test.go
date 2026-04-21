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
	"reflect"
	"sync"
	"testing"
	"time"
)

func simpleKeyFun(obj interface{}) (string, error) {
	key, _ := obj.(string)
	return key, nil
}

func TestTaskQueue(t *testing.T) {
	t.Parallel()
	synced := map[string]bool{}
	var syncedMutex sync.Mutex
	doneCh := make(chan struct{}, 1)

	sync := func(key string) error {
		syncedMutex.Lock()
		synced[key] = true
		syncedMutex.Unlock()
		switch key {
		case "err":
			return errors.New("injected error")
		case "stop":
			doneCh <- struct{}{}
		case "more":
			t.Error("synced after TaskQueue.Shutdown()")
		}
		return nil
	}

	tq := NewTaskQueue("", "test", 2, simpleKeyFun, sync)

	go tq.Run()
	tq.Enqueue("a")
	tq.Enqueue("b")
	tq.Enqueue("err")
	tq.Enqueue("stop")

	<-doneCh
	tq.Shutdown()

	// Enqueue after Shutdown isn't going to be synced.
	tq.Enqueue("more")

	expected := map[string]bool{
		"a":    true,
		"b":    true,
		"err":  true,
		"stop": true,
	}

	if !reflect.DeepEqual(synced, expected) {
		t.Errorf("task queue synced %+v, want %+v", synced, expected)
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
			tq := NewTaskQueue("multiple-workers", "test", tc.numWorkers, simpleKeyFun, sync)
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
