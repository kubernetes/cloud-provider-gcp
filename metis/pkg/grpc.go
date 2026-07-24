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

package pkg

import (
	"fmt"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/local"
)

// NewLocalGrpcConnection creates a unified grpc.ClientConn to a local unix socket.
func NewLocalGrpcConnection(socketPath string) (*grpc.ClientConn, error) {
	absPath, err := filepath.Abs(socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for socket %s: %v", socketPath, err)
	}

	conn, err := grpc.NewClient(fmt.Sprintf("unix://%s", absPath), grpc.WithTransportCredentials(local.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %v", err)
	}

	return conn, nil
}
