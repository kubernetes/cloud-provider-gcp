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

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"k8s.io/metis/pkg"
)

func newHealthzCommand() *cobra.Command {
	var socketPath string

	cmd := &cobra.Command{
		Use:   "healthz",
		Short: "Run a gRPC health check against the Metis daemon UDS",
		Run: func(cmd *cobra.Command, args []string) {
			os.Exit(RunGRPCHealthCheck(socketPath))
		},
	}

	cmd.Flags().StringVar(&socketPath, "socket-path", pkg.DefaultSockPath, "Path to the Metis daemon Unix Domain Socket")

	return cmd
}

// RunGRPCHealthCheck executes a standard gRPC health check over UDS.
func RunGRPCHealthCheck(socketPath string) int {
	if socketPath == "" {
		socketPath = pkg.DefaultSockPath
	}

	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		return net.Dial("unix", addr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithBlock(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unhealthy: failed to connect to Metis UDS at %s: %v\n", socketPath, err)
		return 1
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: ""})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unhealthy: gRPC health check RPC failed: %v\n", err)
		return 1
	}

	status := resp.GetStatus()
	if status != grpc_health_v1.HealthCheckResponse_SERVING {
		fmt.Fprintf(os.Stderr, "Unhealthy: Metis daemon status is %s (expected SERVING)\n", status)
		return 1
	}

	fmt.Println("Healthy: Metis daemon is SERVING")
	return 0
}
