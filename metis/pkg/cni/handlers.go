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

package cni

import (
	"context"
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"

	pb "k8s.io/metis/api/adaptiveipam/v1"
)

func (p *Plugin) CmdAdd(args *skel.CmdArgs) error {
	result, err := p.cmdAdd(args)
	if err != nil {
		return err
	}
	return types.PrintResult(result, result.CNIVersion)
}

func (p *Plugin) cmdAdd(args *skel.CmdArgs) (*current.Result, error) {
	session, err := p.prepare(args, "ADD")
	if err != nil {
		return nil, fmt.Errorf("metis cni add: prepare failed: %w", err)
	}
	defer session.close()

	if len(session.pluginConf.IPAM.Ranges) == 0 {
		return nil, fmt.Errorf("metis cni add: no IPAM ranges specified in config")
	}

	req := &pb.AllocatePodIPRequest{
		Network: session.pluginConf.Name,
	}

	// The Metis Daemon server only requires the initial block allocation range
	// configured per IP family for bootstrapping. Hence we grab the first usable
	// blocks.
	for _, rangeSet := range session.pluginConf.IPAM.Ranges {
		if len(rangeSet) == 0 || rangeSet[0].Subnet.IP == nil {
			continue
		}

		ipNet := (net.IPNet)(rangeSet[0].Subnet)
		config := &pb.IPConfig{
			InterfaceName:  args.IfName,
			ContainerId:    args.ContainerID,
			InitialPodCidr: ipNet.String(),
		}

		if rangeSet[0].Subnet.IP.To4() != nil && req.Ipv4Config == nil {
			req.Ipv4Config = config
		} else if rangeSet[0].Subnet.IP.To4() == nil && req.Ipv6Config == nil {
			req.Ipv6Config = config
		}
	}

	if session.k8sArgs != nil {
		req.PodName = string(session.k8sArgs.K8S_POD_NAME)
		req.PodNamespace = string(session.k8sArgs.K8S_POD_NAMESPACE)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRPCTimeout)
	defer cancel()

	session.logger.Info("AllocatePodIP request", "req", req)
	resp, err := session.client.AllocatePodIP(ctx, req)
	if err != nil {
		session.logger.Info("AllocatePodIP failed", "err", err)
		return nil, fmt.Errorf("metis cni add: allocation via daemon failed: %w", err)
	}
	session.logger.Info("AllocatePodIP response", "resp", resp)

	result, err := toCNIResult(resp, session.pluginConf, args)
	if err != nil {
		return nil, fmt.Errorf("metis cni add: build CNI result failed: %w", err)
	}

	session.logger.Info("Returning CNI Result", "result", result)
	return result, nil
}

func (p *Plugin) CmdDel(args *skel.CmdArgs) error {
	return p.cmdDel(args)
}

func (p *Plugin) cmdDel(args *skel.CmdArgs) error {
	session, err := p.prepare(args, "DEL")
	if err != nil {
		return fmt.Errorf("metis cni delete: prepare failed: %w", err)
	}
	defer session.close()

	ctx, cancel := context.WithTimeout(context.Background(), defaultRPCTimeout)
	defer cancel()

	req := &pb.DeallocatePodIPRequest{
		Network:       session.pluginConf.Name,
		InterfaceName: args.IfName,
		ContainerId:   args.ContainerID,
	}
	if session.k8sArgs != nil {
		req.PodName = string(session.k8sArgs.K8S_POD_NAME)
		req.PodNamespace = string(session.k8sArgs.K8S_POD_NAMESPACE)
	}

	session.logger.Info("DeallocatePodIP request", "req", req)
	_, err = session.client.DeallocatePodIP(ctx, req)
	if err != nil {
		session.logger.Info("DeallocatePodIP failed", "err", err)
		return fmt.Errorf("metis cni delete: deallocation via daemon failed: %w", err)
	}

	session.logger.Info("Successfully deallocated IP")
	return nil
}

func (p *Plugin) CmdCheck(args *skel.CmdArgs) error {
	return p.cmdCheck(args)
}

func (p *Plugin) cmdCheck(args *skel.CmdArgs) error {
	session, err := p.prepare(args, "CHECK")
	if err != nil {
		return fmt.Errorf("metis cni check: prepare failed: %w", err)
	}
	defer session.close()

	ctx, cancel := context.WithTimeout(context.Background(), defaultRPCTimeout)
	defer cancel()

	req := &pb.CheckPodIPRequest{
		Network:       session.pluginConf.Name,
		InterfaceName: args.IfName,
		ContainerId:   args.ContainerID,
	}
	if session.k8sArgs != nil {
		req.PodName = string(session.k8sArgs.K8S_POD_NAME)
		req.PodNamespace = string(session.k8sArgs.K8S_POD_NAMESPACE)
	}

	session.logger.Info("CheckPodIP request", "req", req)
	_, err = session.client.CheckPodIP(ctx, req)
	if err != nil {
		session.logger.Info("CheckPodIP failed", "err", err)
		return fmt.Errorf("metis cni check: check via daemon failed: %w", err)
	}

	session.logger.Info("Successfully checked IP")
	return nil
}

func buildIPConfig(ipConfig *pb.PodIP) (*current.IPConfig, net.IP, error) {
	if ipConfig == nil {
		return nil, nil, nil
	}
	_, ipNet, err := net.ParseCIDR(ipConfig.Cidr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse allocated CIDR: %v", err)
	}
	ip := net.ParseIP(ipConfig.IpAddress)
	gw := getGatewayIP(ipNet)
	
	return &current.IPConfig{
		Address: net.IPNet{IP: ip, Mask: ipNet.Mask},
		Gateway: gw,
	}, gw, nil
}

// toCNIResult translates the daemon IP allocation response into a CNI Result.
func toCNIResult(resp *pb.AllocatePodIPResponse, conf *PluginConf, args *skel.CmdArgs) (*current.Result, error) {
	result := &current.Result{
		CNIVersion: conf.CNIVersion,
	}

	if resp.Ipv4 != nil || resp.Ipv6 != nil {
		result.Interfaces = append(result.Interfaces, &current.Interface{
			Name:    args.IfName,
			Sandbox: args.Netns,
		})
	}

	v4IPConfig, v4GW, err := buildIPConfig(resp.Ipv4)
	if err != nil {
		return nil, err
	}
	if v4IPConfig != nil {
		result.IPs = append(result.IPs, v4IPConfig)
	}

	v6IPConfig, v6GW, err := buildIPConfig(resp.Ipv6)
	if err != nil {
		return nil, err
	}
	if v6IPConfig != nil {
		result.IPs = append(result.IPs, v6IPConfig)
	}

	for _, route := range conf.IPAM.Routes {
		_, routeNet, err := net.ParseCIDR(route.Dst)
		if err != nil {
			return nil, fmt.Errorf("failed to parse route dst %s: %v", route.Dst, err)
		}
		gw := v4GW
		if routeNet.IP.To4() == nil {
			gw = v6GW
		}
		result.Routes = append(result.Routes, &types.Route{
			Dst: *routeNet,
			GW:  gw,
		})
	}

	return result, nil
}
