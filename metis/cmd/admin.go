package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	adminv1 "k8s.io/metis/api/admin/v1"
	"k8s.io/metis/pkg"
)

func newAdminCommand() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:    "admin",
		Short:  "Admin CLI",
		Hidden: true,
	}

	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "json", "Output format (json or table)")
	cmd.PersistentFlags().MarkHidden("output")

	printDumpResponse := func(res *adminv1.AdminTableDumpResponse) {
		if outputFormat == "table" {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			// Print Headers
			fmt.Fprintln(w, strings.ToUpper(strings.Join(res.Headers, "\t")))
			// Print Rows
			for _, row := range res.Rows {
				fmt.Fprintln(w, strings.Join(row.Values, "\t"))
			}
			w.Flush()
		} else {
			var jsonPayload []map[string]interface{}
			for _, row := range res.Rows {
				rowMap := make(map[string]interface{})
				for i, header := range res.Headers {
					rowMap[header] = row.Values[i]
				}
				jsonPayload = append(jsonPayload, rowMap)
			}
			b, _ := json.MarshalIndent(jsonPayload, "", "  ")
			fmt.Println(string(b))
		}
	}

	cidrCmd := &cobra.Command{
		Use:    "cidr-blocks",
		Short:  "Manage CIDR blocks",
		Hidden: true,
	}
	cidrCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List CIDR blocks",
		Run: func(cmd *cobra.Command, args []string) {
			client, conn, err := getAdminClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()
			res, err := client.ListCIDRBlocks(context.Background(), &adminv1.ListCIDRBlocksRequest{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to query: %v\n", err)
				os.Exit(1)
			}
			printDumpResponse(res)
		},
	})
	cidrCmd.AddCommand(&cobra.Command{
		Use:   "get [id]",
		Short: "Get CIDR block by ID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, conn, err := getAdminClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()
			res, err := client.GetCIDRBlock(context.Background(), &adminv1.GetCIDRBlockRequest{Id: args[0]})
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to query: %v\n", err)
				os.Exit(1)
			}
			printDumpResponse(res)
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
		Run: func(cmd *cobra.Command, args []string) {
			client, conn, err := getAdminClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()
			res, err := client.ListIPAddresses(context.Background(), &adminv1.ListIPAddressesRequest{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to query: %v\n", err)
				os.Exit(1)
			}
			printDumpResponse(res)
		},
	})
	ipCmd.AddCommand(&cobra.Command{
		Use:   "get [id]",
		Short: "Get IP address by ID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, conn, err := getAdminClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()
			res, err := client.GetIPAddress(context.Background(), &adminv1.GetIPAddressRequest{Id: args[0]})
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to query: %v\n", err)
				os.Exit(1)
			}
			printDumpResponse(res)
		},
	})

	cmd.AddCommand(cidrCmd)
	cmd.AddCommand(ipCmd)

	return cmd
}

func getAdminClient() (adminv1.AdminClient, *grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		pkg.DefaultSockPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", addr)
		}),
	)
	if err != nil {
		return nil, nil, err
	}
	return adminv1.NewAdminClient(conn), conn, nil
}
