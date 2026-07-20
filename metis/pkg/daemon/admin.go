package daemon

import (
	"context"
	
	"k8s.io/metis/api/admin/v1"
)

func (s *adaptiveIpamServer) ListCIDRBlocks(ctx context.Context, req *adminv1.ListCIDRBlocksRequest) (*adminv1.ListCIDRBlocksResponse, error) {
	blocks, err := s.store.ListCIDRBlocks(ctx)
	if err != nil {
		return nil, err
	}
	
	var res adminv1.ListCIDRBlocksResponse
	for _, b := range blocks {
		res.CidrBlocks = append(res.CidrBlocks, &adminv1.CIDRBlock{
			Id:           b.ID,
			TotalIps:     int32(b.TotalIPs),
			AllocatedIps: int32(b.AllocatedIPs),
			Cidr:         b.CIDR,
			Network:      b.Network,
			IpFamily:     b.IpFamily,
			State:        b.State,
		})
	}
	return &res, nil
}

func (s *adaptiveIpamServer) ListIPAddresses(ctx context.Context, req *adminv1.ListIPAddressesRequest) (*adminv1.ListIPAddressesResponse, error) {
	ips, err := s.store.ListIPAddresses(ctx)
	if err != nil {
		return nil, err
	}
	
	var res adminv1.ListIPAddressesResponse
	for _, ip := range ips {
		res.IpAddresses = append(res.IpAddresses, &adminv1.IPAddress{
			Id:            ip.ID,
			Address:       ip.Address,
			CidrBlockId:   ip.CIDRBlockID,
			ContainerId:   ip.ContainerID,
			PodName:       ip.PodName,
			PodNamespace:  ip.PodNamespace,
			InterfaceName: ip.InterfaceName,
			IsAllocated:   ip.IsAllocated,
		})
	}
	return &res, nil
}
