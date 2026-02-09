/*
Copyright 2026 The Kubernetes Authors.
*/

package gketenantcontrollers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

func TestFilteredInformer_AddEventHandler(t *testing.T) {
	tests := []struct {
		name                string
		providerConfigLabel string
		providerConfigName  string
		allowMissing        bool
		obj                 *corev1.Pod
		shouldTrigger       bool
	}{
		{
			name:                "Matching Label",
			providerConfigLabel: providerConfigLabelKey,
			providerConfigName:  "config1",
			allowMissing:        false,
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod1",
					Labels: map[string]string{
						providerConfigLabelKey: "config1",
					},
				},
			},
			shouldTrigger: true,
		},
		{
			name:                "Non-Matching Label",
			providerConfigLabel: providerConfigLabelKey,
			providerConfigName:  "config1",
			allowMissing:        false,
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod2",
					Labels: map[string]string{
						providerConfigLabelKey: "config2",
					},
				},
			},
			shouldTrigger: false,
		},
		{
			name:                "Missing Label - Allowed",
			providerConfigLabel: providerConfigLabelKey,
			providerConfigName:  "config1",
			allowMissing:        true,
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "pod3",
					Labels: map[string]string{},
				},
			},
			shouldTrigger: true,
		},
		{
			name:                "Missing Label - Not Allowed",
			providerConfigLabel: providerConfigLabelKey,
			providerConfigName:  "config1",
			allowMissing:        false,
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "pod4",
					Labels: map[string]string{},
				},
			},
			shouldTrigger: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fake SharedIndexInformer
			lw := &cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return &corev1.PodList{}, nil
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return watch.NewFake(), nil
				},
			}
			parentInformer := cache.NewSharedIndexInformer(lw, &corev1.Pod{}, 0, cache.Indexers{})

			// Create the filtered informer
			filtered := newFilteredInformer(parentInformer, tc.providerConfigLabel, tc.providerConfigName, tc.allowMissing)

			// Channel to signal event handling
			eventCh := make(chan struct{}, 1)

			// Add event handler
			_, err := filtered.AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					eventCh <- struct{}{}
				},
			})
			assert.NoError(t, err)

			// Start the parent informer (must be running for events to process)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go parentInformer.Run(ctx.Done())

			// Wait for sync
			if !cache.WaitForCacheSync(ctx.Done(), parentInformer.HasSynced) {
				t.Fatal("Timed out waiting for caches to sync")
			}

			// Add the object to the parent informer's store (faking an event from deltaFIFO really)
			// But since we are using a real SharedIndexInformer, we should probably inject via the fake watcher or just mess with the DeltaFIFO if possible.
			// Actually simpler: just use the underlying Store if we didn't care about the event handler.
			// BUT we care about the event handler.
			// So we need to trigger an Add event.
			// Since we mocked ListWatch, we can't easily push to it unless we kept the fake watch.

			// Re-create parent with controllable fake watch
			fakeWatch := watch.NewFake()
			lw = &cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return &corev1.PodList{}, nil
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return fakeWatch, nil
				},
			}
			parentInformer = cache.NewSharedIndexInformer(lw, &corev1.Pod{}, 0, cache.Indexers{})
			filtered = newFilteredInformer(parentInformer, tc.providerConfigLabel, tc.providerConfigName, tc.allowMissing)

			// Re-add handler
			_, err = filtered.AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					eventCh <- struct{}{}
				},
			})
			assert.NoError(t, err)

			go parentInformer.Run(ctx.Done())
			if !cache.WaitForCacheSync(ctx.Done(), parentInformer.HasSynced) {
				t.Fatal("Timed out waiting for caches to sync")
			}

			// Trigger Add event
			fakeWatch.Add(tc.obj)

			// Check if we received an event
			select {
			case <-eventCh:
				if !tc.shouldTrigger {
					t.Errorf("Handler triggered unexpectedly for %s", tc.name)
				}
			case <-time.After(100 * time.Millisecond):
				if tc.shouldTrigger {
					t.Errorf("Handler did NOT trigger for %s", tc.name)
				}
			}
		})
	}
}
