/*
Copyright 2026 The Kubernetes Authors.

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

package dynamicpodip

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	container "google.golang.org/api/container/v1"
	testingclock "k8s.io/utils/clock/testing"
)

func TestExtractPodSecondaryRanges(t *testing.T) {
	testCases := []struct {
		name     string
		cluster  *container.Cluster
		expected []string
	}{
		{
			name:     "nil cluster",
			cluster:  nil,
			expected: nil,
		},
		{
			name: "nil IpAllocationPolicy",
			cluster: &container.Cluster{
				Name: "test-cluster",
			},
			expected: nil,
		},
		{
			name: "primary pod secondary range only",
			cluster: &container.Cluster{
				IpAllocationPolicy: &container.IPAllocationPolicy{
					ClusterSecondaryRangeName:  "pods-primary",
					ServicesSecondaryRangeName: "services-range",
				},
			},
			expected: []string{"pods-primary"},
		},
		{
			name: "primary and additional pod secondary ranges",
			cluster: &container.Cluster{
				IpAllocationPolicy: &container.IPAllocationPolicy{
					ClusterSecondaryRangeName: "pods-primary",
					AdditionalPodRangesConfig: &container.AdditionalPodRangesConfig{
						PodRangeNames: []string{"pods-additional-1", "pods-additional-2"},
					},
				},
			},
			expected: []string{"pods-primary", "pods-additional-1", "pods-additional-2"},
		},
		{
			name: "duplicates and whitespaces and empty names",
			cluster: &container.Cluster{
				IpAllocationPolicy: &container.IPAllocationPolicy{
					ClusterSecondaryRangeName: "pods-primary",
					AdditionalPodRangesConfig: &container.AdditionalPodRangesConfig{
						PodRangeNames: []string{" pods-primary ", "", "pods-additional-1", "pods-additional-1"},
					},
				},
			},
			expected: []string{"pods-primary", "pods-additional-1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractPodSecondaryRanges(tc.cluster)
			if len(result) == 0 && len(tc.expected) == 0 {
				return
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("extractPodSecondaryRanges() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestPodRangeRefresher_InitialFetchAndCache(t *testing.T) {
	ctx := context.Background()
	fakeClock := testingclock.NewFakeClock(time.Now())

	var calls int32
	loader := func(ctx context.Context) (*container.Cluster, error) {
		atomic.AddInt32(&calls, 1)
		return &container.Cluster{
			IpAllocationPolicy: &container.IPAllocationPolicy{
				ClusterSecondaryRangeName: "pods-primary",
				AdditionalPodRangesConfig: &container.AdditionalPodRangesConfig{
					PodRangeNames: []string{"pods-secondary"},
				},
			},
		}, nil
	}

	refresher := NewPodRangeRefresherWithLoader(loader, 5*time.Minute, fakeClock)

	// First call triggers synchronous fetch
	ranges, err := refresher.GetCandidateRanges(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"pods-primary", "pods-secondary"}
	if !reflect.DeepEqual(ranges, expected) {
		t.Fatalf("expected %v, got %v", expected, ranges)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 call to loader, got %d", atomic.LoadInt32(&calls))
	}

	// Subsequent call returns cached ranges without calling loader
	rangesCached, err := refresher.GetCandidateRanges(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(rangesCached, expected) {
		t.Fatalf("expected %v, got %v", expected, rangesCached)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected loader call count to remain 1, got %d", atomic.LoadInt32(&calls))
	}
}

func TestPodRangeRefresher_PeriodicRefresh(t *testing.T) {
	ctx := context.Background()
	fakeClock := testingclock.NewFakeClock(time.Now())

	var rangeNames atomic.Value
	rangeNames.Store([]string{"pods-1"})

	loader := func(ctx context.Context) (*container.Cluster, error) {
		names := rangeNames.Load().([]string)
		return &container.Cluster{
			IpAllocationPolicy: &container.IPAllocationPolicy{
				ClusterSecondaryRangeName: names[0],
			},
		}, nil
	}

	interval := 5 * time.Minute
	refresher := NewPodRangeRefresherWithLoader(loader, interval, fakeClock)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go refresher.Run(stopCh)

	// Initial fetch
	ranges, err := refresher.GetCandidateRanges(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(ranges, []string{"pods-1"}) {
		t.Fatalf("expected [pods-1], got %v", ranges)
	}

	// Wait until Run() completes initial fetch and registers ticker
	for i := 0; i < 50; i++ {
		if fakeClock.HasWaiters() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !fakeClock.HasWaiters() {
		t.Fatal("ticker was not registered on fake clock")
	}

	// Simulate cluster adding a second range
	rangeNames.Store([]string{"pods-2"})

	// Step clock forward by interval
	fakeClock.Step(interval + time.Second)

	var rangesUpdated []string
	for i := 0; i < 50; i++ {
		rangesUpdated, _ = refresher.GetCandidateRanges(ctx)
		if reflect.DeepEqual(rangesUpdated, []string{"pods-2"}) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !reflect.DeepEqual(rangesUpdated, []string{"pods-2"}) {
		t.Fatalf("expected [pods-2] after periodic refresh, got %v", rangesUpdated)
	}
}

func TestPodRangeRefresher_InvalidateForcesRefresh(t *testing.T) {
	ctx := context.Background()
	fakeClock := testingclock.NewFakeClock(time.Now())

	var rangeNames atomic.Value
	rangeNames.Store([]string{"pods-initial"})

	loader := func(ctx context.Context) (*container.Cluster, error) {
		names := rangeNames.Load().([]string)
		return &container.Cluster{
			IpAllocationPolicy: &container.IPAllocationPolicy{
				ClusterSecondaryRangeName: names[0],
			},
		}, nil
	}

	interval := 1 * time.Hour
	refresher := NewPodRangeRefresherWithLoader(loader, interval, fakeClock)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go refresher.Run(stopCh)

	ranges, err := refresher.GetCandidateRanges(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(ranges, []string{"pods-initial"}) {
		t.Fatalf("expected [pods-initial], got %v", ranges)
	}

	// Cluster changes to new range
	rangeNames.Store([]string{"pods-invalidated"})

	// Invalidate immediately without advancing periodic interval
	fakeClock.Step(15 * time.Second) // Pass rate-limit window
	refresher.Invalidate()

	var rangesUpdated []string
	for i := 0; i < 50; i++ {
		rangesUpdated, _ = refresher.GetCandidateRanges(ctx)
		if reflect.DeepEqual(rangesUpdated, []string{"pods-invalidated"}) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !reflect.DeepEqual(rangesUpdated, []string{"pods-invalidated"}) {
		t.Fatalf("expected [pods-invalidated] after Invalidate(), got %v", rangesUpdated)
	}
}

func TestPodRangeRefresher_RetainsCacheOnAPIError(t *testing.T) {
	ctx := context.Background()
	fakeClock := testingclock.NewFakeClock(time.Now())

	var returnError atomic.Bool
	loader := func(ctx context.Context) (*container.Cluster, error) {
		if returnError.Load() {
			return nil, fmt.Errorf("container api unavailable")
		}
		return &container.Cluster{
			IpAllocationPolicy: &container.IPAllocationPolicy{
				ClusterSecondaryRangeName: "pods-primary",
			},
		}, nil
	}

	refresher := NewPodRangeRefresherWithLoader(loader, 5*time.Minute, fakeClock)

	// First fetch succeeds
	ranges, err := refresher.GetCandidateRanges(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(ranges, []string{"pods-primary"}) {
		t.Fatalf("expected [pods-primary], got %v", ranges)
	}

	// Next refresh fails
	returnError.Store(true)
	fakeClock.Step(15 * time.Second)
	refresher.Invalidate()

	// Should still return previously cached ranges
	rangesAfterError, err := refresher.GetCandidateRanges(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(rangesAfterError, []string{"pods-primary"}) {
		t.Fatalf("expected cached ranges [pods-primary] to be retained on error, got %v", rangesAfterError)
	}
}
