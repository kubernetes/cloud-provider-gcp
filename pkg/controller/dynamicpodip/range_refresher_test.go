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
	clocktesting "k8s.io/utils/clock/testing"
)

func TestStaticRangeProvider(t *testing.T) {
	input := []string{"range-1", "range-2", "range-1", "", "  range-3  "}
	expected := []string{"range-1", "range-2", "range-3"}

	provider := NewStaticRangeProvider(input)

	ranges, err := provider.GetCandidateRanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(ranges, expected) {
		t.Fatalf("got %v, want %v", ranges, expected)
	}

	// Invalidate is a no-op
	provider.Invalidate()
	rangesAfterInvalidate, err := provider.GetCandidateRanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(rangesAfterInvalidate, expected) {
		t.Fatalf("got %v, want %v", rangesAfterInvalidate, expected)
	}

	// Run terminates on stopCh
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	go func() {
		provider.Run(stopCh)
		close(doneCh)
	}()
	close(stopCh)
	select {
	case <-doneCh:
	case <-time.After(1 * time.Second):
		t.Fatal("StaticRangeProvider.Run did not terminate on stopCh")
	}
}

func TestStaticRangeProvider_LifecycleStatus(t *testing.T) {
	testCases := []struct {
		desc          string
		input         []string
		wantConfigs   []SecondaryRangeConfig
		wantCandidate []string
	}{
		{
			desc:  "mixed active and draining via key=value",
			input: []string{"range1=ACTIVE,range2=DRAINING"},
			wantConfigs: []SecondaryRangeConfig{
				{Name: "range1", Status: SecondaryRangeActive},
				{Name: "range2", Status: SecondaryRangeDraining},
			},
			wantCandidate: []string{"range1"},
		},
		{
			desc:  "implicit active without status",
			input: []string{"range1", "range2=DRAINING", "range3"},
			wantConfigs: []SecondaryRangeConfig{
				{Name: "range1", Status: SecondaryRangeActive},
				{Name: "range2", Status: SecondaryRangeDraining},
				{Name: "range3", Status: SecondaryRangeActive},
			},
			wantCandidate: []string{"range1", "range3"},
		},
		{
			desc:  "case insensitivity and whitespace trimming",
			input: []string{"  range1 = active , range2 = draining  "},
			wantConfigs: []SecondaryRangeConfig{
				{Name: "range1", Status: SecondaryRangeActive},
				{Name: "range2", Status: SecondaryRangeDraining},
			},
			wantCandidate: []string{"range1"},
		},
		{
			desc:  "all draining ranges excludes all from candidates",
			input: []string{"range1=DRAINING,range2=DRAINING"},
			wantConfigs: []SecondaryRangeConfig{
				{Name: "range1", Status: SecondaryRangeDraining},
				{Name: "range2", Status: SecondaryRangeDraining},
			},
			wantCandidate: []string{},
		},
		{
			desc:  "duplicate range with updated status overrides earlier status",
			input: []string{"range1=ACTIVE", "range1=DRAINING"},
			wantConfigs: []SecondaryRangeConfig{
				{Name: "range1", Status: SecondaryRangeDraining},
			},
			wantCandidate: []string{},
		},
		{
			desc:  "empty status string defaults to active",
			input: []string{"range1="},
			wantConfigs: []SecondaryRangeConfig{
				{Name: "range1", Status: SecondaryRangeActive},
			},
			wantCandidate: []string{"range1"},
		},
		{
			desc:  "unknown status defaults to active with warning",
			input: []string{"range1=UNKNOWN_STATUS"},
			wantConfigs: []SecondaryRangeConfig{
				{Name: "range1", Status: SecondaryRangeActive},
			},
			wantCandidate: []string{"range1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			provider := NewStaticRangeProvider(tc.input)

			gotConfigs := provider.Configs()
			if !reflect.DeepEqual(gotConfigs, tc.wantConfigs) {
				t.Errorf("Configs() = %v, want %v", gotConfigs, tc.wantConfigs)
			}

			gotCandidate, err := provider.GetCandidateRanges(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Compare empty slices safely
			if len(gotCandidate) == 0 && len(tc.wantCandidate) == 0 {
				return
			}
			if !reflect.DeepEqual(gotCandidate, tc.wantCandidate) {
				t.Errorf("GetCandidateRanges() = %v, want %v", gotCandidate, tc.wantCandidate)
			}
		})
	}
}

func TestStaticRangeProvider_MultiSubnet(t *testing.T) {
	input := []string{
		"subnet-1/range-1a=ACTIVE",
		"subnet-1:range-1b=ACTIVE",
		"subnet-1/range-1c=DRAINING",
		"projects/my-proj/regions/us-central1/subnetworks/subnet-2/range-2a=ACTIVE",
		"subnet-2:range-2b=DRAINING",
		"range-default=ACTIVE",
	}

	provider := NewStaticRangeProviderWithDefaultSubnet(input, "subnet-1")

	// GetCandidateRanges() should return ACTIVE ranges for default subnet
	// ("subnet-1") plus unqualified ranges ("range-default"), excluding
	// draining ranges and other subnets
	gotDefault, err := provider.GetCandidateRanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantDefault := []string{"range-1a", "range-1b", "range-default"}
	if !reflect.DeepEqual(gotDefault, wantDefault) {
		t.Errorf("GetCandidateRanges() = %v, want %v", gotDefault, wantDefault)
	}

	// Query subnet-2 via GetCandidateRangesForSubnetwork: returns active
	// range-2a
	gotSubnet2 := provider.GetCandidateRangesForSubnetwork("subnet-2")
	wantSubnet2 := []string{"range-2a"}
	if !reflect.DeepEqual(gotSubnet2, wantSubnet2) {
		t.Errorf("GetCandidateRangesForSubnetwork(subnet-2) = %v, want %v", gotSubnet2, wantSubnet2)
	}

	// Query unconfigured subnet-3: should return empty/nil
	gotSubnet3 := provider.GetCandidateRangesForSubnetwork("subnet-3")
	if len(gotSubnet3) != 0 {
		t.Errorf("GetCandidateRangesForSubnetwork(subnet-3) = %v, want empty", gotSubnet3)
	}
}

func TestExtractPodSecondaryRanges(t *testing.T) {
	tests := []struct {
		name           string
		cluster        *container.Cluster
		expectedRanges map[string][]string
		expectedSubnet string
	}{
		{
			name:           "nil cluster",
			cluster:        nil,
			expectedRanges: nil,
			expectedSubnet: "",
		},
		{
			name: "nil IpAllocationPolicy",
			cluster: &container.Cluster{
				Name: "test-cluster",
			},
			expectedRanges: nil,
			expectedSubnet: "",
		},
		{
			name: "primary pod secondary range only in default subnet",
			cluster: &container.Cluster{
				Subnetwork: "projects/my-proj/regions/us-central1/subnetworks/default-subnet",
				IpAllocationPolicy: &container.IPAllocationPolicy{
					ClusterSecondaryRangeName:  "gke-pods-1",
					ServicesSecondaryRangeName: "gke-services",
				},
			},
			expectedRanges: map[string][]string{
				"":               {"gke-pods-1"},
				"default-subnet": {"gke-pods-1"},
			},
			expectedSubnet: "default-subnet",
		},
		{
			name: "primary and additional pod secondary ranges in default subnet",
			cluster: &container.Cluster{
				Subnetwork: "default-subnet",
				IpAllocationPolicy: &container.IPAllocationPolicy{
					ClusterSecondaryRangeName:  "gke-pods-1",
					ServicesSecondaryRangeName: "gke-services",
					AdditionalPodRangesConfig: &container.AdditionalPodRangesConfig{
						PodRangeNames: []string{"gke-pods-2", "gke-pods-3"},
					},
				},
			},
			expectedRanges: map[string][]string{
				"":               {"gke-pods-1", "gke-pods-2", "gke-pods-3"},
				"default-subnet": {"gke-pods-1", "gke-pods-2", "gke-pods-3"},
			},
			expectedSubnet: "default-subnet",
		},
		{
			name: "multi-subnet with active and draining additional subnetworks",
			cluster: &container.Cluster{
				Subnetwork: "subnet-1",
				IpAllocationPolicy: &container.IPAllocationPolicy{
					ClusterSecondaryRangeName: "subnet-1-pods",
					AdditionalIpRangesConfigs: []*container.AdditionalIPRangesConfig{
						{
							Subnetwork:        "projects/p/regions/r/subnetworks/subnet-2",
							PodIpv4RangeNames: []string{"subnet-2-pods-1", "subnet-2-pods-2"},
							Status:            "ACTIVE",
						},
						{
							Subnetwork:        "subnet-3",
							PodIpv4RangeNames: []string{"subnet-3-pods"},
							Status:            "DRAINING",
						},
						{
							Subnetwork:        "subnet-4",
							PodIpv4RangeNames: []string{"subnet-4-pods"},
							Status:            "",
						},
					},
				},
			},
			expectedRanges: map[string][]string{
				"":         {"subnet-1-pods"},
				"subnet-1": {"subnet-1-pods"},
				"subnet-2": {"subnet-2-pods-1", "subnet-2-pods-2"},
				"subnet-4": {"subnet-4-pods"},
			},
			expectedSubnet: "subnet-1",
		},
		{
			name: "duplicates and whitespaces and empty names",
			cluster: &container.Cluster{
				IpAllocationPolicy: &container.IPAllocationPolicy{
					ClusterSecondaryRangeName:  "gke-pods-1",
					ServicesSecondaryRangeName: "gke-services",
					AdditionalPodRangesConfig: &container.AdditionalPodRangesConfig{
						PodRangeNames: []string{"gke-pods-1", "", "  gke-pods-2  ", "gke-pods-2"},
					},
				},
			},
			expectedRanges: map[string][]string{
				"": {"gke-pods-1", "gke-pods-2"},
			},
			expectedSubnet: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualRanges, actualSubnet := extractPodSecondaryRanges(tc.cluster)
			if !reflect.DeepEqual(actualRanges, tc.expectedRanges) {
				t.Fatalf("extractPodSecondaryRanges() ranges = %v, want %v", actualRanges, tc.expectedRanges)
			}
			if actualSubnet != tc.expectedSubnet {
				t.Fatalf("extractPodSecondaryRanges() defaultSubnet = %q, want %q", actualSubnet, tc.expectedSubnet)
			}
		})
	}
}

func TestPodRangeRefresher_InitialFetchAndCache(t *testing.T) {
	fakeClock := clocktesting.NewFakeClock(time.Now())
	var callCount atomic.Int32

	loader := func(ctx context.Context) (*container.Cluster, error) {
		callCount.Add(1)
		return &container.Cluster{
			IpAllocationPolicy: &container.IPAllocationPolicy{
				ClusterSecondaryRangeName: "pods-primary",
				AdditionalPodRangesConfig: &container.AdditionalPodRangesConfig{
					PodRangeNames: []string{"pods-secondary-1"},
				},
			},
		}, nil
	}

	refresher := NewPodRangeRefresherWithLoader(loader, 5*time.Minute, fakeClock)

	// First call should trigger loader
	ranges, err := refresher.GetCandidateRanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"pods-primary", "pods-secondary-1"}
	if !reflect.DeepEqual(ranges, expected) {
		t.Fatalf("got %v, want %v", ranges, expected)
	}
	if got := callCount.Load(); got != 1 {
		t.Fatalf("call count = %d, want 1", got)
	}

	// Second call should return cached without calling loader again
	ranges2, err := refresher.GetCandidateRanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(ranges2, expected) {
		t.Fatalf("got %v, want %v", ranges2, expected)
	}
	if got := callCount.Load(); got != 1 {
		t.Fatalf("call count after cache hit = %d, want 1", got)
	}
}

func TestPodRangeRefresher_PeriodicRefresh(t *testing.T) {
	fakeClock := clocktesting.NewFakeClock(time.Now())
	var callCount atomic.Int32

	loader := func(ctx context.Context) (*container.Cluster, error) {
		count := callCount.Add(1)
		var additional []string
		if count > 1 {
			additional = []string{fmt.Sprintf("pods-secondary-%d", count)}
		}
		return &container.Cluster{
			IpAllocationPolicy: &container.IPAllocationPolicy{
				ClusterSecondaryRangeName: "pods-primary",
				AdditionalPodRangesConfig: &container.AdditionalPodRangesConfig{
					PodRangeNames: additional,
				},
			},
		}, nil
	}

	interval := 5 * time.Minute
	refresher := NewPodRangeRefresherWithLoader(loader, interval, fakeClock)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go refresher.Run(stopCh)

	// Wait for initial fetch in Run()
	for i := 0; i < 50; i++ {
		if callCount.Load() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if callCount.Load() < 1 {
		t.Fatal("refresher did not run initial fetch")
	}

	ranges, err := refresher.GetCandidateRanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(ranges, []string{"pods-primary"}) {
		t.Fatalf("initial ranges = %v, want [pods-primary]", ranges)
	}

	// Advance clock past interval
	fakeClock.Step(interval + 1*time.Second)

	for i := 0; i < 50; i++ {
		if callCount.Load() >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if callCount.Load() < 2 {
		t.Fatal("refresher did not trigger periodic refresh")
	}

	rangesAfter, err := refresher.GetCandidateRanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedAfter := []string{"pods-primary", "pods-secondary-2"}
	if !reflect.DeepEqual(rangesAfter, expectedAfter) {
		t.Fatalf("ranges after refresh = %v, want %v", rangesAfter, expectedAfter)
	}
}

func TestPodRangeRefresher_InvalidateForcesRefresh(t *testing.T) {
	fakeClock := clocktesting.NewFakeClock(time.Now())
	var callCount atomic.Int32

	loader := func(ctx context.Context) (*container.Cluster, error) {
		count := callCount.Add(1)
		return &container.Cluster{
			IpAllocationPolicy: &container.IPAllocationPolicy{
				ClusterSecondaryRangeName: fmt.Sprintf("pods-%d", count),
			},
		}, nil
	}

	refresher := NewPodRangeRefresherWithLoader(loader, 1*time.Hour, fakeClock)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go refresher.Run(stopCh)

	// Wait for initial fetch
	for i := 0; i < 50; i++ {
		if callCount.Load() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	ranges, err := refresher.GetCandidateRanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(ranges, []string{"pods-1"}) {
		t.Fatalf("got %v, want [pods-1]", ranges)
	}

	// Advance clock beyond rate limit
	fakeClock.Step(minInvalidateInterval + 1*time.Second)

	// Invalidate
	refresher.Invalidate()

	for i := 0; i < 50; i++ {
		if callCount.Load() >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if callCount.Load() < 2 {
		t.Fatal("refresher did not reload after Invalidate()")
	}

	rangesAfter, err := refresher.GetCandidateRanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(rangesAfter, []string{"pods-2"}) {
		t.Fatalf("got %v, want [pods-2]", rangesAfter)
	}
}

func TestPodRangeRefresher_RetainsCacheOnAPIError(t *testing.T) {
	fakeClock := clocktesting.NewFakeClock(time.Now())
	var callCount atomic.Int32
	var shouldFail atomic.Bool

	loader := func(ctx context.Context) (*container.Cluster, error) {
		callCount.Add(1)
		if shouldFail.Load() {
			return nil, fmt.Errorf("container api unavailable")
		}
		return &container.Cluster{
			IpAllocationPolicy: &container.IPAllocationPolicy{
				ClusterSecondaryRangeName: "pods-primary",
			},
		}, nil
	}

	refresher := NewPodRangeRefresherWithLoader(loader, 5*time.Minute, fakeClock)

	// First call succeeds
	ranges, err := refresher.GetCandidateRanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(ranges, []string{"pods-primary"}) {
		t.Fatalf("got %v, want [pods-primary]", ranges)
	}

	// Now make loader fail and force refresh
	shouldFail.Store(true)
	fakeClock.Step(minInvalidateInterval + 1*time.Second)
	refresher.Invalidate()

	// refresh should return the cached ranges despite the API failure
	rangesRetained, _, err := refresher.refresh(context.Background())
	if err != nil {
		t.Fatalf("expected retained cache without error, got error: %v", err)
	}
	if !reflect.DeepEqual(rangesRetained[""], []string{"pods-primary"}) {
		t.Fatalf("got %v, want [pods-primary]", rangesRetained[""])
	}
}

func TestPodRangeRefresher_MultiSubnet(t *testing.T) {
	fakeClock := clocktesting.NewFakeClock(time.Now())

	loader := func(ctx context.Context) (*container.Cluster, error) {
		return &container.Cluster{
			Subnetwork: "subnet-1",
			IpAllocationPolicy: &container.IPAllocationPolicy{
				ClusterSecondaryRangeName: "subnet-1-primary",
				AdditionalIpRangesConfigs: []*container.AdditionalIPRangesConfig{
					{
						Subnetwork:        "projects/p/regions/r/subnetworks/subnet-2",
						PodIpv4RangeNames: []string{"subnet-2-range-a", "subnet-2-range-b"},
						Status:            "ACTIVE",
					},
					{
						Subnetwork:        "subnet-3",
						PodIpv4RangeNames: []string{"subnet-3-range"},
						Status:            "DRAINING",
					},
				},
			},
		}, nil
	}

	refresher := NewPodRangeRefresherWithLoader(loader, 5*time.Minute, fakeClock)

	// Query default subnet
	gotDefault, err := refresher.GetCandidateRanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(gotDefault, []string{"subnet-1-primary"}) {
		t.Errorf("GetCandidateRanges() = %v, want [subnet-1-primary]", gotDefault)
	}

	// Query subnet-1 explicitly via GetCandidateRangesForSubnetwork
	gotSubnet1, err := refresher.GetCandidateRangesForSubnetwork(context.Background(), "subnet-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(gotSubnet1, []string{"subnet-1-primary"}) {
		t.Errorf("GetCandidateRangesForSubnetwork(subnet-1) = %v, want [subnet-1-primary]", gotSubnet1)
	}

	// Query subnet-2 explicitly via GetCandidateRangesForSubnetwork
	gotSubnet2, err := refresher.GetCandidateRangesForSubnetwork(context.Background(), "subnet-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSubnet2 := []string{"subnet-2-range-a", "subnet-2-range-b"}
	if !reflect.DeepEqual(gotSubnet2, wantSubnet2) {
		t.Errorf("GetCandidateRangesForSubnetwork(subnet-2) = %v, want %v", gotSubnet2, wantSubnet2)
	}

	// Query draining subnet-3: should return empty/nil
	gotSubnet3, err := refresher.GetCandidateRangesForSubnetwork(context.Background(), "subnet-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotSubnet3) != 0 {
		t.Errorf("GetCandidateRangesForSubnetwork(subnet-3) = %v, want empty", gotSubnet3)
	}
}
