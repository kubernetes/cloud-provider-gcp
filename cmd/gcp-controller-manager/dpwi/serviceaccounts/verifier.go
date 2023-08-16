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

// Package serviceaccounts provides a common verifier to verify if a Kubernetes Service Account (KSA)
// can act as a GCP Service Account (GSA). It also listens to and process KSA events.
// If a KSA's permission changes, it notifies the configmap handler to update configmap
// and the node syncer to sync related nodes.
package serviceaccounts

import (
	"context"
	"fmt"

	"golang.org/x/sync/singleflight"
	core "k8s.io/api/core/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/auth"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/ctxlog"
)

const (
	// serviceAccountAnnotationGSAEmail is the key to the GCP Service Account annotation
	// in ServiceAccount objects.
	serviceAccountAnnotationGSAEmail = "iam.gke.io/gcp-service-account"
)

// Verifier verifies if a Kubernetes Service Account (KSA)
// can act as a GCP Service Account (GSA) or not.
type Verifier struct {
	auth        *auth.Client
	saIndexer   cache.Indexer
	verifiedSAs *saMap
	loadGroup   singleflight.Group
}

// NewVerifier creates a new Verifier.
func NewVerifier(saInformer coreinformers.ServiceAccountInformer, auth *auth.Client) *Verifier {
	return &Verifier{
		auth:        auth,
		saIndexer:   saInformer.Informer().GetIndexer(),
		verifiedSAs: newSAMap(),
	}
}

// VerifiedGSA returns the verified GSA for a given KSA if it has been verified and stored in memory.
// Otherwise, it calls Auth server to verify and store the result in memory.
func (v *Verifier) VerifiedGSA(ctx context.Context, ksa ServiceAccount) (GSAEmail, error) {
	gsa, ok := v.verifiedSAs.get(ksa)
	if ok {
		return gsa, nil
	}
	res, err := v.ForceVerify(ctx, ksa)
	if err != nil {
		return "", err
	}
	if res.denied {
		return "", nil
	}
	return res.curGSA, err
}

// ForceVerify verifies a KSA no matter it has been verified or not.
func (v *Verifier) ForceVerify(ctx context.Context, ksa ServiceAccount) (verifyResult, error) {
	resChan := v.loadGroup.DoChan(ksa.Key(), func() (entry any, err error) {
		return v.verify(ctx, ksa)
	})
	select {
	case <-ctx.Done():
		return verifyResult{}, fmt.Errorf("original request context is done: %w", ctx.Err())
	case res := <-resChan:
		return res.Val.(verifyResult), nil
	}
}

func (v *Verifier) verify(ctx context.Context, ksa ServiceAccount) (res verifyResult, err error) {
	if gsa, ok := v.verifiedSAs.get(ksa); ok {
		res.preVerifiedGSA = gsa
	}
	gsa, existed, err := v.getGSA(ctx, ksa)
	if err != nil {
		return res, err
	}
	if !existed || gsa == "" {
		v.verifiedSAs.remove(ksa)
		return res, nil
	}
	permitted, err := v.auth.Authorize(ctx, ksa.Namespace, ksa.Name, string(gsa))
	if err != nil {
		return res, fmt.Errorf("failed to authorize %s:%s; err: %w", ksa, gsa, err)
	}
	if !permitted {
		v.verifiedSAs.remove(ksa)
		res.denied = true
		return res, nil
	}
	v.verifiedSAs.addOrUpdate(ctx, ksa, gsa)
	res.curGSA = gsa
	return res, nil
}

func (v *Verifier) getGSA(ctx context.Context, ksa ServiceAccount) (GSAEmail, bool, error) {
	o, exists, err := v.saIndexer.GetByKey(ksa.Key())
	if err != nil {
		return "", false, fmt.Errorf("failed to get ServiceAccount %v: %w", ksa, err)
	}
	if !exists {
		return "", false, nil
	}
	sa, ok := o.(*core.ServiceAccount)
	if !ok {
		return "", false, fmt.Errorf("invalid object for service account %v: %#v", ksa, o)
	}

	ann, found := sa.ObjectMeta.Annotations[serviceAccountAnnotationGSAEmail]
	return GSAEmail(ann), found, nil
}

// AllVerified returns a full set of verified KSA-GSA pairs.
func (v *Verifier) AllVerified(ctx context.Context) (map[ServiceAccount]GSAEmail, error) {
	m := make(map[ServiceAccount]GSAEmail)
	for _, o := range v.saIndexer.List() {
		sa, ok := o.(*core.ServiceAccount)
		if !ok {
			ctxlog.Warningf(ctx, "Dropping invalid service account: %v", o)
			continue
		}
		ksa := ServiceAccount{
			Name:      sa.Name,
			Namespace: sa.Namespace,
		}
		gsa, err := v.VerifiedGSA(ctx, ksa)
		// Don't let some failures block the whole process.
		// A ksa failure here means that the ksa event is still in processing or
		// in the backlog of the SA event handler. Once the SA is processed successfully,
		// it will send another configmap event.
		if err != nil {
			ctxlog.Warningf(ctx, "Ignore the failure verifying ksa %q: %v", ksa, err)
			continue
		}
		if gsa != "" {
			m[ksa] = gsa
		}
	}
	return m, nil
}
