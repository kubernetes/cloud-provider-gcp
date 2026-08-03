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
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	adminv1 "k8s.io/metis/api/admin/v1"
	"k8s.io/metis/pkg"
)

func newAdminCommand() *cobra.Command {
	var outputFormat string
	var filter string

	cmd := &cobra.Command{
		Use:    "admin",
		Short:  "Admin CLI",
		Hidden: true,
	}

	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format (json or table)")
	cmd.PersistentFlags().MarkHidden("output")
	cmd.PersistentFlags().StringVarP(&filter, "filter", "f", "", "Filter rows in list output")
	cmd.PersistentFlags().MarkHidden("filter")

	cidrCmd := &cobra.Command{
		Use:    "cidr-blocks",
		Short:  "Manage CIDR blocks",
		Hidden: true,
	}
	cidrCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List CIDR blocks",
		Long:  "List CIDR blocks.",
		Example: `  # List all CIDR blocks
  metis admin cidr-blocks list
  # List a specific CIDR block by ID
  metis admin cidr-blocks list --filter "id = '1'"
  # List all Ready CIDR blocks
  metis admin cidr-blocks list --filter "state = 'Ready'"`,
		Run: func(_ *cobra.Command, _ []string) {
			executeAdminListCommand(outputFormat, func(ctx context.Context, client adminv1.AdminClient) (*adminv1.AdminTableDumpResponse, error) {
				return client.ListCIDRBlocks(ctx, &adminv1.ListCIDRBlocksRequest{Filter: filter})
			})
		},
	})

	ipCmd := &cobra.Command{
		Use:    "ip-addresses",
		Short:  "Manage IP addresses",
		Hidden: true,
	}
	ipCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List IP addresses",
		Long:  "List IP addresses.",
		Example: `  # List all IP addresses
  metis admin ip-addresses list
  # List a specific IP address by ID
  metis admin ip-addresses list --filter "id = '1'"
  # List IP addresses that are allocated in a specific namespace
  metis admin ip-addresses list --filter "pod_namespace = 'default' AND is_allocated = '1'"`,
		Run: func(_ *cobra.Command, _ []string) {
			executeAdminListCommand(outputFormat, func(ctx context.Context, client adminv1.AdminClient) (*adminv1.AdminTableDumpResponse, error) {
				return client.ListIPAddresses(ctx, &adminv1.ListIPAddressesRequest{Filter: filter})
			})
		},
	})

	cmd.AddCommand(cidrCmd)
	cmd.AddCommand(ipCmd)

	return cmd
}

func executeAdminListCommand(outputFormat string, queryFunc func(context.Context, adminv1.AdminClient) (*adminv1.AdminTableDumpResponse, error)) {
	client, conn, err := getAdminClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	res, err := queryFunc(context.Background(), client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to query: %v\n", err)
		os.Exit(1)
	}
	printDumpResponse(res, outputFormat)
}

func printDumpResponse(res *adminv1.AdminTableDumpResponse, outputFormat string) {
	if outputFormat == "table" {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		// Print Headers
		_ , _ = fmt.Fprintln(w, strings.ToUpper(strings.Join(res.Headers, "\t")))
		// Print Rows
		for _, row := range res.Rows {
			_, _ = fmt.Fprintln(w, strings.Join(row.Values, "\t"))
		}
		_ = w.Flush()
	} else {
		var jsonPayload []map[string]any
		for _, row := range res.Rows {
			rowMap := map[string]any{}
			for i, header := range res.Headers {
				rowMap[header] = row.Values[i]
			}
			jsonPayload = append(jsonPayload, rowMap)
		}
		b, _ := json.MarshalIndent(jsonPayload, "", "  ")
		fmt.Println(string(b))
	}
}

func getAdminClient() (adminv1.AdminClient, *grpc.ClientConn, error) {
	conn, err := pkg.NewLocalGrpcConnection(pkg.DefaultSockPath)
	if err != nil {
		return nil, nil, err
	}
	return adminv1.NewAdminClient(conn), conn, nil
}
