/*
Copyright 2019 The Kubernetes Authors.

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
	"encoding/json"
	"fmt"
	"sync"

	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/ctxlog"
)

// GSAEmail identifies a GCP service account in email format.
type GSAEmail string

// ServiceAccount identifies a K8s service account object by its namespace and name.  Empty
// Namespace indicates the corresponding Kubernetes object was created in the "default" namespace.
type ServiceAccount struct {
	Namespace, Name string
}

// MarshalText implements the encoding.TextMarshaler interface.
func (sa ServiceAccount) MarshalText() ([]byte, error) {
	return []byte(sa.String()), nil
}

// String returns sa in a string as "<namespace>/<name>" or "default/<name>" if sa.Namespace is
// empty.
func (sa ServiceAccount) String() string {
	if sa.Namespace == "" {
		return fmt.Sprintf("default/%s", sa.Name)
	}
	return fmt.Sprintf("%s/%s", sa.Namespace, sa.Name)
}

// SAMap is a Mutax protected map of GSAEmail keyed by ServiceAccount.
type SAMap struct {
	sync.RWMutex
	ma map[ServiceAccount]GSAEmail
}

// Key generates the key with the format Namespace/Name.
func (sa ServiceAccount) Key() string {
	return fmt.Sprintf("%s/%s", sa.Namespace, sa.Name)
}

// Serialize returns m in its JSON encoded format or error if serialization had failed.
func (m *SAMap) Serialize() ([]byte, error) {
	m.RLock()
	defer m.RUnlock()
	return json.Marshal(m.ma)
}

// saMap is a Mutax protected map of GSAEmail keyed by ServiceAccount.
type saMap struct {
	sync.RWMutex
	ma map[ServiceAccount]GSAEmail
}

// NewSAMap creates an empty SAMap
func newSAMap() *saMap {
	t := make(map[ServiceAccount]GSAEmail)
	return &saMap{
		ma: t,
	}
}

func (m *saMap) addOrUpdate(ctx context.Context, sa ServiceAccount, gsa GSAEmail) {
	m.Lock()
	defer m.Unlock()
	lastGSA := m.ma[sa]
	if string(gsa) == string(lastGSA) {
		ctxlog.Infof(ctx, "ksa %v is re-verified to act as gsa %q", sa, gsa)
	} else {
		ctxlog.Infof(ctx, "ksa %v can act as gsa %q instead of %q", sa, gsa, lastGSA)
	}
	m.ma[sa] = gsa
}

func (m *saMap) remove(sa ServiceAccount) {
	m.Lock()
	defer m.Unlock()
	delete(m.ma, sa)
}

// get looks up sa from m and returns its gsa if sa exists.
func (m *saMap) get(sa ServiceAccount) (GSAEmail, bool) {
	m.RLock()
	defer m.RUnlock()
	gsa, ok := m.ma[sa]
	return gsa, ok
}

type verifyResult struct {
	preVerifiedGSA GSAEmail
	curGSA         GSAEmail
	denied         bool
}
