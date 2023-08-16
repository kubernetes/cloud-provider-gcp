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

package serviceaccounts

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/auth"
)

const (
	testNamespace = "test-ns"
	gsa1          = "gsa1@something.iam.gserviceaccount.com"
	gsa2          = "gsa2@something.iam.gserviceaccount.com"
)

var (
	ksa1  = ServiceAccount{Namespace: testNamespace, Name: "ksa1"}
	ksa2  = ServiceAccount{Namespace: testNamespace, Name: "ksa2"}
	ksa3  = ServiceAccount{Namespace: testNamespace, Name: "ksa3"}
	v1SA1 = newV1SA(ksa1, gsa1)
	v1SA2 = newV1SA(ksa2, gsa2)
	v1SA3 = newV1SA(ksa3, "")
)

type fakeIndexer struct {
	cache.Indexer
	m map[string]*v1.ServiceAccount
}

func (f *fakeIndexer) List() []interface{} {
	var res []interface{}
	for _, v := range f.m {
		res = append(res, v)
	}
	return res
}

func (f *fakeIndexer) GetByKey(key string) (interface{}, bool, error) {
	val, ok := f.m[key]
	return val, ok, nil
}

func TestForceVerify(t *testing.T) {
	tests := []struct {
		desc           string
		ksa            ServiceAccount
		permittedPairs map[string]string
		wantCallCount  int
		wantDenied     bool
		wantGSA        GSAEmail
	}{
		{
			desc: "no annotated GSA",
			ksa:  ksa3,
		},
		{
			desc:          "not permitted GSA",
			wantCallCount: 2,
			wantDenied:    true,
			ksa:           ksa2,
		},
		{
			desc:          "permitted GSA",
			wantCallCount: 2,
			ksa:           ksa1,
			permittedPairs: map[string]string{
				ksa1.Key(): gsa1,
			},
			wantGSA: gsa1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			v, s := setUpVerifierForTest(t, map[string]*v1.ServiceAccount{
				ksa1.Key(): v1SA1,
				ksa2.Key(): v1SA2,
				ksa3.Key(): v1SA3,
			}, tc.permittedPairs)
			ctx := context.Background()
			res, err := v.ForceVerify(ctx, tc.ksa)
			if err != nil {
				t.Fatalf("ForceVerify() failed: %v", err)
			}
			if res.preVerifiedGSA != "" {
				t.Errorf("preVerifiedGSA=%q, want empty", res.preVerifiedGSA)
			}
			if got, want := res.curGSA, tc.wantGSA; got != want {
				t.Errorf("curGSA=%q, want %q", got, want)
			}
			if got, want := res.denied, tc.wantDenied; got != want {
				t.Errorf("denied=%v, want %v", got, want)
			}
			// ForceVerify again.
			res, err = v.ForceVerify(ctx, tc.ksa)
			if err != nil {
				t.Fatalf("ForceVerify() failed: %v", err)
			}
			if res.preVerifiedGSA != res.curGSA {
				t.Errorf("ForceVerify() again, preVerifiedGSA %q and curGSA %q should be the same", res.preVerifiedGSA, res.curGSA)
			}
			if got, want := s.AuthorizeCount[tc.ksa.Key()], tc.wantCallCount; got != want {
				t.Errorf("ForceVerify() call count=%d, want %d", got, want)
			}
		})
	}
}
func TestVerifiedGSA(t *testing.T) {
	tests := []struct {
		desc           string
		ksa            ServiceAccount
		permittedPairs map[string]string
		wantCallCount  int
		wantGSA        GSAEmail
	}{
		{
			desc: "no annotated GSA",
			ksa:  ksa3,
		},
		{
			desc:          "not permitted GSA",
			wantCallCount: 2,
			ksa:           ksa2,
		},
		{
			desc:          "permitted GSA",
			wantCallCount: 1,
			ksa:           ksa1,
			permittedPairs: map[string]string{
				ksa1.Key(): gsa1,
			},
			wantGSA: gsa1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			v, s := setUpVerifierForTest(t, map[string]*v1.ServiceAccount{
				ksa1.Key(): v1SA1,
				ksa2.Key(): v1SA2,
				ksa3.Key(): v1SA3,
			}, tc.permittedPairs)
			ctx := context.Background()
			gsa, err := v.VerifiedGSA(ctx, tc.ksa)
			if err != nil {
				t.Fatalf("VerifiedGSA() failed: %v", err)
			}
			if got, want := gsa, tc.wantGSA; got != want {
				t.Errorf("VerifiedGSA=%q, want %q", got, want)
			}
			// VerifiedGSA again.
			gsa, err = v.VerifiedGSA(ctx, tc.ksa)
			if err != nil {
				t.Fatalf("VerifiedGSA() failed: %v", err)
			}
			if got, want := s.AuthorizeCount[tc.ksa.Key()], tc.wantCallCount; got != want {
				t.Errorf("VerifiedGSA() call count=%d, want %d", got, want)
			}
		})
	}
}
func TestAllVerified(t *testing.T) {
	tests := []struct {
		desc            string
		ksaMap          map[string]*v1.ServiceAccount
		wantAllVerified map[ServiceAccount]GSAEmail
	}{
		{
			desc:            "no KSA",
			wantAllVerified: map[ServiceAccount]GSAEmail{},
		},
		{
			desc: "no GSA",
			ksaMap: map[string]*v1.ServiceAccount{
				ksa3.Key(): v1SA3,
			},
			wantAllVerified: map[ServiceAccount]GSAEmail{},
		},
		{
			desc: "one KSA with permitted GSA",
			ksaMap: map[string]*v1.ServiceAccount{
				ksa1.Key(): v1SA1,
			},
			wantAllVerified: map[ServiceAccount]GSAEmail{
				ksa1: gsa1,
			},
		},
		{
			desc: "multiple KSAs with permitted GSA",
			ksaMap: map[string]*v1.ServiceAccount{
				ksa1.Key(): v1SA1,
				ksa2.Key(): v1SA2,
				ksa3.Key(): v1SA3,
			},
			wantAllVerified: map[ServiceAccount]GSAEmail{
				ksa1: gsa1,
				ksa2: gsa2,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			v, _ := setUpVerifierForTest(t, tc.ksaMap, map[string]string{
				ksa1.Key(): gsa1,
				ksa2.Key(): gsa2,
			})
			ctx := context.Background()
			all, err := v.AllVerified(ctx)
			if err != nil {
				t.Errorf("AllVerified() failed: %v", err)
			}
			if got, want := all, tc.wantAllVerified; !cmp.Equal(got, want) {
				t.Errorf("AllVerified()=%v, want %v", got, want)
			}
		})
	}
}
func newV1SA(ksa ServiceAccount, gsa string) *v1.ServiceAccount {
	sa := &v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        ksa.Name,
			Namespace:   ksa.Namespace,
			Annotations: make(map[string]string),
		},
	}
	if gsa != "" {
		sa.ObjectMeta.Annotations["iam.gke.io/gcp-service-account"] = gsa
	}
	return sa
}

func setUpDefaultVerifierForTest(t *testing.T) (*Verifier, *auth.FakeServer) {
	return setUpVerifierForTest(t, map[string]*v1.ServiceAccount{
		ksa1.Key(): v1SA1,
		ksa2.Key(): v1SA2,
		ksa3.Key(): v1SA3,
	},
		map[string]string{
			ksa1.Key(): gsa1,
			ksa2.Key(): gsa2,
		},
	)
}
func setUpVerifierForTest(t *testing.T, ksas map[string]*v1.ServiceAccount, permittedPairs map[string]string) (*Verifier, *auth.FakeServer) {
	t.Helper()
	server := auth.NewFakeServer(permittedPairs)
	auth, err := auth.NewClient(server.Server.URL, "", nil)
	if err != nil {
		t.Fatalf("Failed to create Auth service client")
	}
	indexer := &fakeIndexer{
		m: ksas,
	}
	return &Verifier{
		auth:        auth,
		verifiedSAs: newSAMap(),
		saIndexer:   indexer,
	}, server
}
