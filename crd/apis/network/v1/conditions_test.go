package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsReady(t *testing.T) {
	makeNetwork := func(cond []metav1.Condition) *Network {
		return &Network{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Status: NetworkStatus{
				Conditions: cond,
			},
		}
	}

	tests := []struct {
		name        string
		input       interface{}
		want        bool
		expectedErr bool
	}{
		{
			name:  "nil condition",
			input: &Network{},
			want:  false,
		},
		{
			name: "true condition",
			input: makeNetwork([]metav1.Condition{
				{
					Type:   string(NetworkConditionStatusParamsReady),
					Status: metav1.ConditionFalse,
				},
				{
					Type:   string(NetworkConditionStatusReady),
					Status: metav1.ConditionTrue,
				},
			}),
			want: true,
		},
		{
			name: "false condition",
			input: makeNetwork([]metav1.Condition{
				{
					Type:   string(NetworkConditionStatusReady),
					Status: metav1.ConditionFalse,
				},
				{
					Type:   string(NetworkConditionStatusParamsReady),
					Status: metav1.ConditionTrue,
				},
			}),
			want: false,
		},
		{
			name: "missing ready condition",
			input: makeNetwork([]metav1.Condition{
				{
					Type:   string(NetworkConditionStatusParamsReady),
					Status: metav1.ConditionTrue,
				},
			}),
			want: false,
		},
		{
			name:        "unsupported type",
			input:       metav1.APIGroup{},
			want:        false,
			expectedErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := IsReady(tc.input)
			if tc.expectedErr && err == nil {
				t.Fatalf("IsReady(%+v) expected error but got nil", tc.input)
			} else if !tc.expectedErr && err != nil {
				t.Fatalf("IsReady(%+v) unexpected error %v", tc.input, err)
			}
			if tc.expectedErr {
				return
			}
			if got != tc.want {
				t.Fatalf("IsReady(%+v) returns %v but want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestGetCondition(t *testing.T) {
	makeNetwork := func(cond []metav1.Condition) *Network {
		return &Network{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Status: NetworkStatus{
				Conditions: cond,
			},
		}
	}

	tests := []struct {
		name        string
		input       interface{}
		condType    NetworkConditionType
		want        *metav1.Condition
		expectedErr bool
	}{
		{
			name:     "nil condition",
			input:    &Network{},
			condType: NetworkConditionStatusReady,
			want:     nil,
		},
		{
			name: "condition present",
			input: makeNetwork([]metav1.Condition{
				{
					Type:   string(NetworkConditionStatusParamsReady),
					Status: metav1.ConditionFalse,
				},
				{
					Type:   string(NetworkConditionStatusReady),
					Status: metav1.ConditionTrue,
				},
			}),
			condType: NetworkConditionStatusReady,
			want: &metav1.Condition{
				Type:   string(NetworkConditionStatusReady),
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "condition not present",
			input: makeNetwork([]metav1.Condition{
				{
					Type:   string(NetworkConditionStatusParamsReady),
					Status: metav1.ConditionFalse,
				},
			}),
			condType: NetworkConditionStatusReady,
			want:     nil,
		},
		{
			name:        "unsupported type",
			input:       metav1.APIGroup{},
			condType:    NetworkConditionStatusReady,
			want:        nil,
			expectedErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GetCondition(tc.input, string(tc.condType))
			if tc.expectedErr && err == nil {
				t.Fatalf("GetCondition(%+v) expected error but got nil", tc.input)
			} else if !tc.expectedErr && err != nil {
				t.Fatalf("GetCondition(%+v) unexpected error %v", tc.input, err)
			}
			if tc.expectedErr {
				return
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("GetCondition() has diff (-want +got):\n%s", diff)
			}
		})
	}
}
