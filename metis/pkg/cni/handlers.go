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

func (p *Plugin) CmdAdd(args *skel.CmdArgs) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("metis cni: %w", err)
		}
	}()

	session, err := p.prepare(args, "ADD")
	if err != nil {
		return err
	}
	defer session.close()

	if len(session.conf.IPAM.Ranges) == 0 {
		return fmt.Errorf("no IPAM ranges specified in config")
	}

	req := &pb.AllocatePodIPRequest{
		Network: session.conf.Name,
	}

	for _, rangeSet := range session.conf.IPAM.Ranges {
		if len(rangeSet) == 0 {
			return fmt.Errorf("invalid IPAM range: empty range set in config")
		}
		if rangeSet[0].Subnet.IP == nil {
			return fmt.Errorf("invalid IPAM range: subnet IP is nil")
		}

		ipNet := (net.IPNet)(rangeSet[0].Subnet)
		config := &pb.IPConfig{
			InterfaceName:  args.IfName,
			ContainerId:    args.ContainerID,
			InitialPodCidr: ipNet.String(),
		}

		if rangeSet[0].Subnet.IP.To4() != nil {
			req.Ipv4Config = config
		} else {
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
		return fmt.Errorf("failed to allocate IP via daemon: %v", err)
	}
	session.logger.Info("AllocatePodIP response", "resp", resp)

	result, err := toCNIResult(resp, session.conf, args)
	if err != nil {
		return err
	}

	session.logger.Info("Returning CNI Result", "result", result)
	return types.PrintResult(result, session.conf.CNIVersion)
}

func (p *Plugin) CmdDel(args *skel.CmdArgs) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("metis cni: %w", err)
		}
	}()

	session, err := p.prepare(args, "DEL")
	if err != nil {
		return err
	}
	defer session.close()

	ctx, cancel := context.WithTimeout(context.Background(), defaultRPCTimeout)
	defer cancel()

	req := &pb.DeallocatePodIPRequest{
		Network:       session.conf.Name,
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
		return fmt.Errorf("failed to deallocate IP via daemon: %v", err)
	}

	session.logger.Info("Successfully deallocated IP")
	return nil
}

func (p *Plugin) CmdCheck(args *skel.CmdArgs) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("metis cni: %w", err)
		}
	}()

	session, err := p.prepare(args, "CHECK")
	if err != nil {
		return err
	}
	defer session.close()

	ctx, cancel := context.WithTimeout(context.Background(), defaultRPCTimeout)
	defer cancel()

	req := &pb.CheckPodIPRequest{
		Network:       session.conf.Name,
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
		return fmt.Errorf("failed to check IP allocation via daemon: %v", err)
	}

	session.logger.Info("Successfully checked IP")
	return nil
}

// toCNIResult translates the daemon IP allocation response into a CNI Result.
func toCNIResult(resp *pb.AllocatePodIPResponse, conf *NetConf, args *skel.CmdArgs) (*current.Result, error) {
	result := &current.Result{
		CNIVersion: conf.CNIVersion,
	}

	if resp.Ipv4 != nil || resp.Ipv6 != nil {
		result.Interfaces = append(result.Interfaces, &current.Interface{
			Name:    args.IfName,
			Sandbox: args.Netns,
		})
	}

	var v4GW net.IP
	var v6GW net.IP

	if resp.Ipv4 != nil {
		_, ipNet, err := net.ParseCIDR(resp.Ipv4.Cidr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse allocated CIDR: %v", err)
		}
		ip := net.ParseIP(resp.Ipv4.IpAddress)

		ifIndex := 0
		v4GW = getGatewayIP(ipNet)
		result.IPs = append(result.IPs, &current.IPConfig{
			Address:   net.IPNet{IP: ip, Mask: ipNet.Mask},
			Interface: &ifIndex,
			Gateway:   v4GW,
		})
	}

	if resp.Ipv6 != nil {
		_, ipNet, err := net.ParseCIDR(resp.Ipv6.Cidr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse allocated IPv6 CIDR: %v", err)
		}
		ip := net.ParseIP(resp.Ipv6.IpAddress)

		ifIndex := 0
		v6GW = getGatewayIP(ipNet)
		result.IPs = append(result.IPs, &current.IPConfig{
			Address:   net.IPNet{IP: ip, Mask: ipNet.Mask},
			Interface: &ifIndex,
			Gateway:   v6GW,
		})
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
