package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
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

	cidrCmd := &cobra.Command{
		Use:   "cidrblocks",
		Short: "List CIDR blocks",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			client, conn, err := getAdminClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()
			res, err := client.ListCIDRBlocks(context.Background(), &adminv1.ListCIDRBlocksRequest{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to list cidrblocks: %v\n", err)
				os.Exit(1)
			}
			if outputFormat == "table" {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tCIDR\tNETWORK\tIP_FAMILY\tSTATE\tTOTAL_IPS\tALLOCATED_IPS")
				for _, c := range res.CidrBlocks {
					fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%d\t%d\n", c.Id, c.Cidr, c.Network, c.IpFamily, c.State, c.TotalIps, c.AllocatedIps)
				}
				w.Flush()
			} else {
				b, _ := json.MarshalIndent(res, "", "  ")
				fmt.Println(string(b))
			}
		},
	}

	ipCmd := &cobra.Command{
		Use:   "ip_addresses",
		Short: "List IP addresses",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			client, conn, err := getAdminClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()
			res, err := client.ListIPAddresses(context.Background(), &adminv1.ListIPAddressesRequest{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to list ip_addresses: %v\n", err)
				os.Exit(1)
			}
			if outputFormat == "table" {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tADDRESS\tCIDR_BLOCK_ID\tCONTAINER_ID\tPOD_NAME\tPOD_NAMESPACE\tINTERFACE_NAME\tIS_ALLOCATED")
				for _, ip := range res.IpAddresses {
					fmt.Fprintf(w, "%d\t%s\t%d\t%s\t%s\t%s\t%s\t%t\n", ip.Id, ip.Address, ip.CidrBlockId, ip.ContainerId, ip.PodName, ip.PodNamespace, ip.InterfaceName, ip.IsAllocated)
				}
				w.Flush()
			} else {
				b, _ := json.MarshalIndent(res, "", "  ")
				fmt.Println(string(b))
			}
		},
	}

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
