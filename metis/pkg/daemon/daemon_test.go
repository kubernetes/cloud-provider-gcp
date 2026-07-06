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

package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestDaemon_Run(t *testing.T) {
	tests := []struct {
		name        string
		setupDaemon func(t *testing.T, d *Daemon)
		wantErr     bool
		errContains string
	}{
		{
			name: "successful run",
			setupDaemon: func(t *testing.T, d *Daemon) {
				d.NNCClient = nncfake.NewSimpleClientset(&nncv1.NodeNetworkConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
					},
				})
				d.KubeClient = kubefake.NewSimpleClientset(&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
					},
				})
			},
			wantErr: false,
		},
		{
			name: "successful run falling back to os.Hostname",
			setupDaemon: func(t *testing.T, d *Daemon) {
				t.Setenv("NODE_NAME", "")
				hostname, err := os.Hostname()
				if err != nil {
					t.Fatalf("Failed to get hostname: %v", err)
				}
				d.NNCClient = nncfake.NewSimpleClientset(&nncv1.NodeNetworkConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: hostname,
					},
				})
				d.KubeClient = kubefake.NewSimpleClientset(&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: hostname,
					},
				})
			},
			wantErr: false,
		},
		{
			name:        "both clients nil (initClients fails)",
			setupDaemon: func(t *testing.T, d *Daemon) {},
			wantErr:     true,
			errContains: "failed to initialize clients",
		},
		{
			name: "only NNCClient set (initClients fails)",
			setupDaemon: func(t *testing.T, d *Daemon) {
				d.NNCClient = nncfake.NewSimpleClientset()
			},
			wantErr:     true,
			errContains: "failed to initialize clients",
		},
		{
			name: "only KubeClient set (initClients fails)",
			setupDaemon: func(t *testing.T, d *Daemon) {
				d.KubeClient = kubefake.NewSimpleClientset()
			},
			wantErr:     true,
			errContains: "failed to initialize clients",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			dbPath := filepath.Join(tempDir, "metis_daemon_test.sqlite")
			sockPath := filepath.Join(tempDir, "metis_test.sock")

			cfg := Config{
				MonitorInterval: 5 * time.Second,
				ReleaseCooldown: 1 * time.Minute,
				DBPath:          dbPath,
				SocketPath:      sockPath,
			}

			t.Setenv("NODE_NAME", "test-node")

			d := NewDaemon(cfg)
			tc.setupDaemon(t, d)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			errCh := make(chan error, 1)
			go func() {
				errCh <- d.Run(ctx)
			}()

			if !tc.wantErr {
				// Wait for server to start and create socket
				time.Sleep(500 * time.Millisecond)

				if _, err := os.Stat(sockPath); os.IsNotExist(err) {
					t.Errorf("Expected socket to be created at %s, but doesn't exist", sockPath)
				}

				if _, err := os.Stat(dbPath); os.IsNotExist(err) {
					t.Errorf("Expected database to be created at %s, but doesn't exist", dbPath)
				}

				// Trigger clean shutdown
				cancel()
			}

			select {
			case err := <-errCh:
				if tc.wantErr {
					if err == nil {
						t.Fatal("Expected error, got nil")
					}
					if !strings.Contains(err.Error(), tc.errContains) {
						t.Errorf("Expected error to contain %q, got: %v", tc.errContains, err)
					}
				} else {
					if err != nil {
						t.Errorf("Daemon exited with error: %v", err)
					}
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("Daemon failed to complete run or shut down within timeout")
			}
		})
	}
}

func TestEnsureNodeNetworkConfig(t *testing.T) {
	nodeName := "test-node"
	nodeUID := types.UID("test-node-uid")

	tests := []struct {
		desc          string
		initNode      *corev1.Node
		initNNC       *nncv1.NodeNetworkConfig
		createReactor func(action clienttesting.Action) (handled bool, ret runtime.Object, err error)
		wantErr       bool
		verifyNNC     func(t *testing.T, nncClient nncclientset.Interface)
	}{
		{
			desc: "Successful creation",
			initNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
					UID:  nodeUID,
				},
			},
			verifyNNC: func(t *testing.T, nncClient nncclientset.Interface) {
				nnc, err := nncClient.NetworkingV1().NodeNetworkConfigs().Get(context.Background(), nodeName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Failed to get NodeNetworkConfig: %v", err)
				}
				if nnc.Name != nodeName {
					t.Errorf("Expected NNC name %s, got %s", nodeName, nnc.Name)
				}
				if len(nnc.OwnerReferences) != 1 {
					t.Fatalf("Expected 1 owner reference, got %d", len(nnc.OwnerReferences))
				}
				owner := nnc.OwnerReferences[0]
				if owner.Kind != "Node" || owner.Name != nodeName || owner.UID != nodeUID || owner.Controller == nil || !*owner.Controller {
					t.Errorf("Unexpected owner reference: %+v", owner)
				}
			},
		},
		{
			desc: "Already exists initially",
			initNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
					UID:  nodeUID,
				},
			},
			initNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
			},
			verifyNNC: func(t *testing.T, nncClient nncclientset.Interface) {
				nnc, err := nncClient.NetworkingV1().NodeNetworkConfigs().Get(context.Background(), nodeName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Failed to get NodeNetworkConfig: %v", err)
				}
				if len(nnc.OwnerReferences) != 0 {
					t.Errorf("Expected owner reference to remain empty, got %+v", nnc.OwnerReferences)
				}
			},
		},
		{
			desc: "Concurrent creation (AlreadyExists on Create)",
			initNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
					UID:  nodeUID,
				},
			},
			createReactor: func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, apierrors.NewAlreadyExists(schema.GroupResource{Group: "networking.gke.io", Resource: "nodenetworkconfigs"}, nodeName)
			},
		},
		{
			desc:     "Node lookup failure",
			initNode: nil, // Node doesn't exist
			wantErr:  true,
		},
		{
			desc: "Create returns general error",
			initNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
					UID:  nodeUID,
				},
			},
			createReactor: func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("general API error")
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var kubeObjects []runtime.Object
			if tc.initNode != nil {
				kubeObjects = append(kubeObjects, tc.initNode)
			}
			kubeClient := kubefake.NewSimpleClientset(kubeObjects...)

			var nncObjects []runtime.Object
			if tc.initNNC != nil {
				nncObjects = append(nncObjects, tc.initNNC)
			}
			nncClient := nncfake.NewSimpleClientset(nncObjects...)

			if tc.createReactor != nil {
				nncClient.PrependReactor("create", "nodenetworkconfigs", tc.createReactor)
			}

			logger := logr.Discard()

			d := &Daemon{
				NNCClient:  nncClient,
				KubeClient: kubeClient,
			}
			err := d.ensureNodeNetworkConfig(context.Background(), nodeName, logger)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ensureNodeNetworkConfig() error = %v, wantErr = %v", err, tc.wantErr)
			}

			if tc.verifyNNC != nil {
				tc.verifyNNC(t, nncClient)
			}
		})
	}
}
