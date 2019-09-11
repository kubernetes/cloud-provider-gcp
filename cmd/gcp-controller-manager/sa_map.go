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

package main

import (
	"fmt"
	"sync"
)

// gsaEmail identifies a GCP service account in email format.
type gsaEmail string

// serviceAccount identifies a K8s service account object by its namespace and name.
type serviceAccount struct {
	namespace, name string
}

// String returns sa in string in the format of "<namespace>/<name>".
func (sa *serviceAccount) String() string {
	return fmt.Sprintf("%s/%s", sa.namespace, sa.name)
}

// saMap is a Mutax protected map of gsaEmail keyed by serviceAccount.  It contains fields to
// support (lazy) encoding of the map to a serialized form:
type saMap struct {
	sync.RWMutex
	ma map[serviceAccount]gsaEmail
}

func newSAMap() *saMap {
	return &saMap{
		ma: make(map[serviceAccount]gsaEmail),
	}
}

// Add stores the mapping from sa to gsa to m and returns the previous gsa if sa already existed.
func (m *saMap) add(sa serviceAccount, gsa gsaEmail) (gsaEmail, bool) {
	m.Lock()
	defer m.Unlock()
	lastGSA, found := m.ma[sa]
	if !found || lastGSA != gsa {
		m.ma[sa] = gsa
	}
	return lastGSA, found
}

// Remove removes the entry keyed by sa in m and returns its gsa if sa existed.
func (m *saMap) remove(sa serviceAccount) (gsaEmail, bool) {
	m.Lock()
	defer m.Unlock()
	removedGSA, found := m.ma[sa]
	if found {
		delete(m.ma, sa)
	}
	return removedGSA, found
}

// Get looks up sa from m and returns its gsa if sa exists.
func (m *saMap) get(sa serviceAccount) (gsaEmail, bool) {
	m.RLock()
	defer m.RUnlock()
	gsa, ok := m.ma[sa]
	return gsa, ok
}

// Stringmap returns all entries in m as a string map.
func (m *saMap) stringMap() map[string]string {
	m.RLock()
	defer m.RUnlock()
	n := make(map[string]string, len(m.ma))
	for sa, gsa := range m.ma {
		n[sa.String()] = string(gsa)
	}
	return n
}

// Len returns the number of entries in m.
func (m *saMap) len() int {
	m.RLock()
	defer m.RUnlock()
	return len(m.ma)
}
