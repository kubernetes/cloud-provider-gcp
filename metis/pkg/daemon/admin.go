package daemon

import (
	"context"

	adminv1 "k8s.io/metis/api/admin/v1"
)

func (s *adaptiveIpamServer) ListCIDRBlocks(ctx context.Context, _ *adminv1.ListCIDRBlocksRequest) (*adminv1.AdminTableDumpResponse, error) {
	headers, results, err := s.store.QueryTable(ctx, "cidr_blocks")
	if err != nil {
		return nil, err
	}

	return formatAdminTableDumpResponse(headers, results), nil
}

func (s *adaptiveIpamServer) ListIPAddresses(ctx context.Context, _ *adminv1.ListIPAddressesRequest) (*adminv1.AdminTableDumpResponse, error) {
	headers, results, err := s.store.QueryTable(ctx, "ip_addresses")
	if err != nil {
		return nil, err
	}

	return formatAdminTableDumpResponse(headers, results), nil
}

func (s *adaptiveIpamServer) GetCIDRBlock(ctx context.Context, req *adminv1.GetCIDRBlockRequest) (*adminv1.AdminTableDumpResponse, error) {
	headers, results, err := s.store.QueryTableByID(ctx, "cidr_blocks", req.Id)
	if err != nil {
		return nil, err
	}

	return formatAdminTableDumpResponse(headers, results), nil
}

func (s *adaptiveIpamServer) GetIPAddress(ctx context.Context, req *adminv1.GetIPAddressRequest) (*adminv1.AdminTableDumpResponse, error) {
	headers, results, err := s.store.QueryTableByID(ctx, "ip_addresses", req.Id)
	if err != nil {
		return nil, err
	}

	return formatAdminTableDumpResponse(headers, results), nil
}

func formatAdminTableDumpResponse(headers []string, results [][]string) *adminv1.AdminTableDumpResponse {
	var parsedRows []*adminv1.Row
	for _, rawRow := range results {
		parsedRows = append(parsedRows, &adminv1.Row{
			Values: rawRow,
		})
	}
	return &adminv1.AdminTableDumpResponse{
		Headers: headers,
		Rows:    parsedRows,
	}
}
