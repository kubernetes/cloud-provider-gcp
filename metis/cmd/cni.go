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
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "k8s.io/metis/api/adaptiveipam/v1"
)

// NetConf extends standard CNI network configuration.
type NetConf struct {
	types.NetConf
	DaemonSocket   string `json:"daemon_socket"`
	InitialPodCIDR string `json:"initial_pod_cidr"`
}

// K8sArgs contains the standard Kubernetes CNI arguments.
type K8sArgs struct {
	types.CommonArgs
	K8S_POD_NAME      types.UnmarshallableString `json:"K8S_POD_NAME"`
	K8S_POD_NAMESPACE types.UnmarshallableString `json:"K8S_POD_NAMESPACE"`
}

const defaultDaemonSocket = "/var/run/metis.sock"

var clientFactory = getGrpcClient

func loadNetConf(bytes []byte) (*NetConf, error) {
	conf := &NetConf{
		DaemonSocket: defaultDaemonSocket,
	}
	if err := json.Unmarshal(bytes, conf); err != nil {
		return nil, fmt.Errorf("failed to parse network configuration: %v", err)
	}
	return conf, nil
}

func loadK8sArgs(args string) (*K8sArgs, error) {
	k8sArgs := &K8sArgs{}
	if err := types.LoadArgs(args, k8sArgs); err != nil {
		return nil, fmt.Errorf("failed to parse CNI args: %v", err)
	}
	return k8sArgs, nil
}

func getGrpcClient(socketPath string) (pb.AdaptiveIpamClient, *grpc.ClientConn, error) {
	dialOption := grpc.WithTransportCredentials(insecure.NewCredentials())
	
	absPath, err := filepath.Abs(socketPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path for socket %s: %v", socketPath, err)
	}
	dialTarget := fmt.Sprintf("unix://%s", absPath)
	
	conn, err := grpc.NewClient(dialTarget, dialOption)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to daemon: %v", err)
	}
	
	return pb.NewAdaptiveIpamClient(conn), conn, nil
}

func cmdAdd(args *skel.CmdArgs) error {
	conf, err := loadNetConf(args.StdinData)
	if err != nil {
		return err
	}

	k8sArgs, err := loadK8sArgs(args.Args)
	if err != nil {
		return err
	}

	client, conn, err := clientFactory(conf.DaemonSocket)
	if err != nil {
		return err
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.AllocatePodIPRequest{
		Network: conf.Name,
		Ipv4Config: &pb.IPConfig{
			InterfaceName:  args.IfName,
			ContainerId:    args.ContainerID,
			InitialPodCidr: conf.InitialPodCIDR,
		},
		PodName:      string(k8sArgs.K8S_POD_NAME),
		PodNamespace: string(k8sArgs.K8S_POD_NAMESPACE),
	}

	resp, err := client.AllocatePodIP(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to allocate IP via daemon: %v", err)
	}

	result := &current.Result{
		CNIVersion: conf.CNIVersion,
	}

	if resp.Ipv4 != nil {
		_, ipNet, err := net.ParseCIDR(resp.Ipv4.Cidr)
		if err != nil {
			return fmt.Errorf("failed to parse allocated CIDR: %v", err)
		}
		ip := net.ParseIP(resp.Ipv4.IpAddress)
		
		result.IPs = append(result.IPs, &current.IPConfig{
			Address: net.IPNet{IP: ip, Mask: ipNet.Mask},
		})
	}

	if resp.Ipv6 != nil {
		return fmt.Errorf("IPv6 allocation is not implemented yet")
	}

	return types.PrintResult(result, conf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	conf, err := loadNetConf(args.StdinData)
	if err != nil {
		return err
	}

	k8sArgs, err := loadK8sArgs(args.Args)
	if err != nil {
		return err
	}

	client, conn, err := clientFactory(conf.DaemonSocket)
	if err != nil {
		return err
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.DeallocatePodIPRequest{
		Network:       conf.Name,
		InterfaceName: args.IfName,
		ContainerId:   args.ContainerID,
		PodName:       string(k8sArgs.K8S_POD_NAME),
		PodNamespace:  string(k8sArgs.K8S_POD_NAMESPACE),
	}

	_, err = client.DeallocatePodIP(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to deallocate IP via daemon: %v", err)
	}

	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	conf, err := loadNetConf(args.StdinData)
	if err != nil {
		return err
	}

	client, conn, err := clientFactory(conf.DaemonSocket)
	if err != nil {
		return err
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.CheckPodIPRequest{
		Network:       conf.Name,
		InterfaceName: args.IfName,
		ContainerId:   args.ContainerID,
	}

	_, err = client.CheckPodIP(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to check IP allocation via daemon: %v", err)
	}

	return nil
}

func RunCni() {
	skel.PluginMainFuncs(
		skel.CNIFuncs{
			Add:   cmdAdd,
			Check: cmdCheck,
			Del:   cmdDel,
		},
		version.All,
		"metis CNI plugin",
	)
}
