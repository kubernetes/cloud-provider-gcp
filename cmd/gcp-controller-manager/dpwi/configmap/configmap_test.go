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

package configmap

import (
	"bytes"
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/serviceaccounts"
)

const (
	testNamespace = "test-space"
)

type fakeVerifier struct {
	verifiedKSAs map[serviceaccounts.ServiceAccount]serviceaccounts.GSAEmail
}

func (v *fakeVerifier) AllVerified(ctx context.Context) (map[serviceaccounts.ServiceAccount]serviceaccounts.GSAEmail, error) {
	return v.verifiedKSAs, nil
}

type fakeIndexer struct {
	cache.Indexer
	c clientset.Interface
}

// GetByKey gets the configMap from the fake clientset for the given key.
func (f *fakeIndexer) GetByKey(key string) (interface{}, bool, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, false, err
	}
	cm, err := f.c.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return cm, true, nil
}

func TestPersist(t *testing.T) {
	client := fake.NewSimpleClientset()
	verifiedKSAs := map[serviceaccounts.ServiceAccount]serviceaccounts.GSAEmail{
		{Namespace: testNamespace, Name: "ksa2"}: "gsa2@anything",
		{Namespace: testNamespace, Name: "ksa3"}: "gsa3@anything",
	}
	h := Handler{
		c: client,
		verifier: &fakeVerifier{
			verifiedKSAs: verifiedKSAs,
		},
		cmIndexer: &fakeIndexer{c: client},
	}
	ctx := context.Background()
	t.Logf("1. Create a new config map when the clientset has nothing.")
	err := h.persist(ctx, verifiedSAConfigMapCacheKey)
	if err != nil {
		t.Fatalf("persist failed: %v", err)
	}
	cm := verifyConfigMap(ctx, t, client, verifiedKSAs)

	t.Logf("2. Overwrite any unexpected config map changes.")
	cm.BinaryData = nil
	_, err = h.c.CoreV1().ConfigMaps(verifiedSAConfigMapNamespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed update the config map with nil BinaryData.")
	}
	h.persist(ctx, verifiedSAConfigMapCacheKey)
	verifyConfigMap(ctx, t, client, verifiedKSAs)

	t.Logf("3. Recover the config map if deleted unexpectedly.")
	err = h.c.CoreV1().ConfigMaps(verifiedSAConfigMapNamespace).Delete(ctx, cm.Name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed deleting the config map.")
	}
	h.persist(ctx, verifiedSAConfigMapCacheKey)
	verifyConfigMap(ctx, t, client, verifiedKSAs)
}

// verifyConfigMap gets the configMap from the clientset and compares with the internal expected verified KSAs.
// It fails the test when there's any difference. It returns the configmap if there's no error.
func verifyConfigMap(ctx context.Context, t *testing.T, c clientset.Interface,
	expectedSAs map[serviceaccounts.ServiceAccount]serviceaccounts.GSAEmail) *v1.ConfigMap {
	t.Helper()
	cm, err := c.CoreV1().ConfigMaps(verifiedSAConfigMapNamespace).Get(ctx, verifiedSAConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed getting config maps: %v", err)
	}
	got := cm.BinaryData[verifiedSAConfigMapKey]
	want, err := serialize(expectedSAs)
	if err != nil {
		t.Fatalf("Failed to serialize %v: %v", expectedSAs, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("verifiedSAs=%q, want %q", got, want)
	}
	return cm
}
