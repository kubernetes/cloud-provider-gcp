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
	"reflect"
	"testing"
)

func TestStaticRangeProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("empty input", func(t *testing.T) {
		provider := NewStaticRangeProvider(nil)
		ranges, err := provider.GetCandidateRanges(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ranges) != 0 {
			t.Fatalf("expected empty ranges, got %v", ranges)
		}
	})

	t.Run("comma-separated and multiple slices", func(t *testing.T) {
		input := []string{"range-1, range-2", " range-3 "}
		provider := NewStaticRangeProvider(input)
		ranges, err := provider.GetCandidateRanges(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"range-1", "range-2", "range-3"}
		if !reflect.DeepEqual(ranges, expected) {
			t.Fatalf("expected %v, got %v", expected, ranges)
		}
	})

	t.Run("deduplication and empty entries", func(t *testing.T) {
		input := []string{"range-1", "", "range-2", "range-1", "   ", "range-3"}
		provider := NewStaticRangeProvider(input)
		ranges, err := provider.GetCandidateRanges(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"range-1", "range-2", "range-3"}
		if !reflect.DeepEqual(ranges, expected) {
			t.Fatalf("expected %v, got %v", expected, ranges)
		}
	})

	t.Run("invalidate is no-op", func(t *testing.T) {
		provider := NewStaticRangeProvider([]string{"range-1"})
		provider.Invalidate()
		ranges, err := provider.GetCandidateRanges(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ranges) != 1 || ranges[0] != "range-1" {
			t.Fatalf("expected [range-1], got %v", ranges)
		}
	})

	t.Run("configs returns parsed configurations", func(t *testing.T) {
		input := []string{"subnet1/range1=ACTIVE", "range2=DRAINING"}
		provider := NewStaticRangeProvider(input)
		configs := provider.Configs()
		if len(configs) != 2 {
			t.Fatalf("expected 2 configs, got %d", len(configs))
		}
		if configs[0].Subnetwork != "subnet1" || configs[0].Name != "range1" || configs[0].Status != SecondaryRangeActive {
			t.Errorf("unexpected config 0: %+v", configs[0])
		}
		if configs[1].Subnetwork != "" || configs[1].Name != "range2" || configs[1].Status != SecondaryRangeDraining {
			t.Errorf("unexpected config 1: %+v", configs[1])
		}
	})
}

func TestStaticRangeProvider_LifecycleStatus(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "mixed active and draining via key=value",
			input: []string{
				"range1=ACTIVE",
				"range2=DRAINING",
				"range3=active",
			},
			expected: []string{"range1", "range3"},
		},
		{
			name: "implicit active without status",
			input: []string{
				"range1",
				"range2=DRAINING",
				"range3",
			},
			expected: []string{"range1", "range3"},
		},
		{
			name: "case insensitivity and whitespace trimming",
			input: []string{
				" range1 = active ",
				"range2 = Draining",
				" range3 = ACTIVE ",
			},
			expected: []string{"range1", "range3"},
		},
		{
			name: "all draining ranges excludes all from candidates",
			input: []string{
				"range1=DRAINING",
				"range2=draining",
			},
			expected: nil,
		},
		{
			name: "duplicate range with updated status overrides earlier status",
			input: []string{
				"range1=ACTIVE",
				"range1=DRAINING",
				"range2=DRAINING",
				"range2=ACTIVE",
			},
			expected: []string{"range2"},
		},
		{
			name: "empty status string defaults to active",
			input: []string{
				"range1=",
				"range2",
			},
			expected: []string{"range1", "range2"},
		},
		{
			name: "unknown status defaults to active with warning",
			input: []string{
				"range1=UNKNOWN_STATUS",
			},
			expected: []string{"range1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewStaticRangeProvider(tc.input)
			candidates, err := provider.GetCandidateRanges(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(candidates) == 0 && len(tc.expected) == 0 {
				return
			}
			if !reflect.DeepEqual(candidates, tc.expected) {
				t.Errorf("GetCandidateRanges() = %v, want %v", candidates, tc.expected)
			}
		})
	}
}

func TestStaticRangeProvider_MultiSubnet(t *testing.T) {
	ctx := context.Background()

	input := []string{
		"subnet-1/range-1a=ACTIVE",
		"subnet-1/range-1b=DRAINING",
		"projects/p/regions/r/subnetworks/subnet-2/range-2a=ACTIVE",
		"range-default=ACTIVE",
	}

	provider := NewStaticRangeProviderWithDefaultSubnet(input, "subnet-1")

	// Default subnetwork should include subnet-1 qualified ranges + unqualified
	// ranges, excluding DRAINING
	defaultCandidates, err := provider.GetCandidateRanges(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantDefault := []string{"range-1a", "range-default"}
	if !reflect.DeepEqual(defaultCandidates, wantDefault) {
		t.Errorf("GetCandidateRanges() = %v, want %v", defaultCandidates, wantDefault)
	}

	// Specific query for subnet-2 should return subnet-2 candidates
	subnet2Candidates := provider.GetCandidateRangesForSubnetwork("subnet-2")
	wantSubnet2 := []string{"range-2a"}
	if !reflect.DeepEqual(subnet2Candidates, wantSubnet2) {
		t.Errorf("GetCandidateRangesForSubnetwork(subnet-2) = %v, want %v", subnet2Candidates, wantSubnet2)
	}

	// Full URL query should canonicalize and match subnet-2
	subnet2URLCandidates := provider.GetCandidateRangesForSubnetwork("https://www.googleapis.com/compute/v1/projects/p/regions/r/subnetworks/subnet-2")
	if !reflect.DeepEqual(subnet2URLCandidates, wantSubnet2) {
		t.Errorf("GetCandidateRangesForSubnetwork(URL) = %v, want %v", subnet2URLCandidates, wantSubnet2)
	}

	// Query for non-existent subnet should return nil
	if res := provider.GetCandidateRangesForSubnetwork("subnet-unknown"); res != nil {
		t.Errorf("expected nil for unknown subnet, got %v", res)
	}
}

func TestFilterActiveSecondaryRanges(t *testing.T) {
	configs := []SecondaryRangeConfig{
		{Name: "r1", Status: SecondaryRangeActive},
		{Name: "r2", Status: SecondaryRangeDraining},
		{Name: "r3", Status: SecondaryRangeActive},
	}
	active := FilterActiveSecondaryRanges(configs)
	expected := []string{"r1", "r3"}
	if !reflect.DeepEqual(active, expected) {
		t.Fatalf("expected %v, got %v", expected, active)
	}
}
