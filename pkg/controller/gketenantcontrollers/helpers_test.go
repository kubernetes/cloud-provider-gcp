/*
Copyright 2026 The Kubernetes Authors.
*/

package gketenantcontrollers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsObjectInProviderConfig(t *testing.T) {
	tests := []struct {
		name               string
		obj                interface{}
		providerConfigName string
		allowMissing       bool
		want               bool
	}{
		{
			name: "Matching Label",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						providerConfigLabelKey: "config1",
					},
				},
			},
			providerConfigName: "config1",
			allowMissing:       false,
			want:               true,
		},
		{
			name: "Non-Matching Label",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						providerConfigLabelKey: "config2",
					},
				},
			},
			providerConfigName: "config1",
			allowMissing:       false,
			want:               false,
		},
		{
			name: "Missing Label Allowed",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			providerConfigName: "config1",
			allowMissing:       true,
			want:               true,
		},
		{
			name: "Missing Label Not Allowed",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			providerConfigName: "config1",
			allowMissing:       false,
			want:               false,
		},
		{
			name:               "Invalid Object",
			obj:                "not-an-object",
			providerConfigName: "config1",
			allowMissing:       false,
			want:               false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isObjectInProviderConfig(tc.obj, tc.providerConfigName, tc.allowMissing)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestProviderConfigFilteredList(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod1",
			Labels: map[string]string{
				providerConfigLabelKey: "config1",
			},
		},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod2",
			Labels: map[string]string{
				providerConfigLabelKey: "config2",
			},
		},
	}
	pod3 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "pod3",
			Labels: map[string]string{},
		},
	}

	tests := []struct {
		name               string
		items              []interface{}
		providerConfigName string
		allowMissing       bool
		want               []interface{}
	}{
		{
			name:               "Filter Matching Only",
			items:              []interface{}{pod1, pod2, pod3},
			providerConfigName: "config1",
			allowMissing:       false,
			want:               []interface{}{pod1},
		},
		{
			name:               "Filter Matching and Missing",
			items:              []interface{}{pod1, pod2, pod3},
			providerConfigName: "config1",
			allowMissing:       true,
			want:               []interface{}{pod1, pod3},
		},
		{
			name:               "Filter None",
			items:              []interface{}{pod2},
			providerConfigName: "config1",
			allowMissing:       false,
			want:               nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := providerConfigFilteredList(tc.items, tc.providerConfigName, tc.allowMissing)
			assert.Equal(t, len(tc.want), len(got))
			for i, item := range got {
				assert.Equal(t, tc.want[i], item)
			}
		})
	}
}
