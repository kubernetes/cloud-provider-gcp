/*
Copyright 2023 The Kubernetes Authors.

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
package eventhandler

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// recorder records how many times a key has been processed.
// It returns an error when wantErr is true.
type recorder struct {
	sync.Mutex
	m       map[string]int
	wantErr bool
}

func (r *recorder) process(ctx context.Context, key string) error {
	r.Lock()
	defer r.Unlock()
	r.m[key]++
	if r.wantErr {
		return fmt.Errorf("process error")
	}
	return nil
}

func (r *recorder) count(key string) int {
	r.Lock()
	defer r.Unlock()
	return r.m[key]
}

func TestHandler(t *testing.T) {
	tests := []struct {
		desc    string
		wantErr bool
		// When process returns an error, the handler will retry the event
		// with exponential backoff. We don't assert exact count, but a range.
		processCountLow  int
		processCountHigh int
	}{
		{
			desc:             "expected err",
			wantErr:          true,
			processCountHigh: 10,
			processCountLow:  3,
		},
		{
			desc:             "normal handle",
			processCountHigh: 2,
			processCountLow:  2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			r := recorder{
				m:       make(map[string]int),
				wantErr: tc.wantErr,
			}
			h := &EventHandler{}
			h.InitEventHandler("test", r.process)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go h.Run(ctx, 20)
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "event",
					Name:      "testPod",
				},
			}
			key, err := cache.MetaNamespaceKeyFunc(pod)
			if err != nil {
				t.Fatalf("Failed getting key for pod %v: %v", pod, err)
			}
			h.Enqueue(pod)
			// Make sure the key gets processed twice.
			time.Sleep(time.Millisecond * 10)
			h.EnqueueKey(key)
			// Sleep a second for retry.
			time.Sleep(time.Second)
			c := r.count(key)
			if c < tc.processCountLow || c > tc.processCountHigh {
				t.Errorf("process count=%d, want between %d and %d", c, tc.processCountLow, tc.processCountHigh)
			}
		})
	}
}
