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

	adminv1 "k8s.io/metis/api/admin/v1"
)

// ListCIDRBlocks implements AdminServer.ListCIDRBlocks
func (s *adaptiveIpamServer) ListCIDRBlocks(ctx context.Context, req *adminv1.ListCIDRBlocksRequest) (*adminv1.AdminTableDumpResponse, error) {
	headers, results, err := s.store.AdminListCIDRBlocks(ctx, req.Filter)
	if err != nil {
		return nil, err
	}
	return buildAdminTableDumpResponse(headers, results), nil
}

// ListIPAddresses implements AdminServer.ListIPAddresses
func (s *adaptiveIpamServer) ListIPAddresses(ctx context.Context, req *adminv1.ListIPAddressesRequest) (*adminv1.AdminTableDumpResponse, error) {
	headers, results, err := s.store.AdminListIPAddresses(ctx, req.Filter)
	if err != nil {
		return nil, err
	}
	return buildAdminTableDumpResponse(headers, results), nil
}

func buildAdminTableDumpResponse(headers []string, results [][]string) *adminv1.AdminTableDumpResponse {
	resp := &adminv1.AdminTableDumpResponse{
		Headers: headers,
	}
	for _, rowValue := range results {
		resp.Rows = append(resp.Rows, &adminv1.Row{
			Values: rowValue,
		})
	}
	return resp
}
