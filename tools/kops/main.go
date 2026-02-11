package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cloud-provider-gcp/tools/kops/pkg/kops"
)

var (
	config *kops.Config
)

func main() {
	var err error
	config, err = kops.NewConfigFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
		os.Exit(1)
	}

	rootCmd := &cobra.Command{
		Use:   "kops",
		Short: "kOps cluster lifecycle management tool",
	}

	// Define flags for all config fields
	rootCmd.PersistentFlags().StringVar(&config.ClusterName, "cluster-name", config.ClusterName, "The name of the cluster")
	rootCmd.PersistentFlags().StringVar(&config.GCPProject, "gcp-project", config.GCPProject, "The GCP project")
	rootCmd.PersistentFlags().StringVar(&config.GCPLocation, "gcp-location", config.GCPLocation, "The GCP location (region)")
	rootCmd.PersistentFlags().StringVar(&config.GCPZones, "gcp-zones", config.GCPZones, "The GCP zones (comma separated)")
	rootCmd.PersistentFlags().StringVar(&config.StateStore, "state-store", config.StateStore, "The kOps state store (GCS bucket)")
	rootCmd.PersistentFlags().StringVar(&config.KopsBin, "kops-binary-path", config.KopsBin, "Path to kops binary")
	rootCmd.PersistentFlags().StringVar(&config.KopsBaseURL, "kops-base-url", config.KopsBaseURL, "The kOps base URL for CI artifacts")
	rootCmd.PersistentFlags().StringVar(&config.K8sVersion, "kubernetes-version", config.K8sVersion, "Kubernetes version")
	rootCmd.PersistentFlags().StringVar(&config.TemplateSrc, "template-src", config.TemplateSrc, "Path to kOps cluster template source")
	rootCmd.PersistentFlags().StringVar(&config.TemplatePath, "template-path", config.TemplatePath, "Path where hydrated template will be saved")
	rootCmd.PersistentFlags().StringVar(&config.AdminAccess, "admin-access", config.AdminAccess, "Admin access CIDR")
	rootCmd.PersistentFlags().StringVar(&config.ControlPlaneMachineType, "control-plane-machine-type", config.ControlPlaneMachineType, "Control plane machine type")
	rootCmd.PersistentFlags().StringVar(&config.NodeMachineType, "node-machine-type", config.NodeMachineType, "Node machine type")
	rootCmd.PersistentFlags().IntVar(&config.NodeCount, "node-count", config.NodeCount, "Number of nodes")
	rootCmd.PersistentFlags().StringVar(&config.SSHPrivateKey, "ssh-private-key", config.SSHPrivateKey, "Path to SSH private key")
	rootCmd.PersistentFlags().StringVar(&config.NewCCMSpec, "new-ccm-spec", config.NewCCMSpec, "New CCM spec for template hydration")
	rootCmd.PersistentFlags().StringVar(&config.ImageRepo, "image-repo", config.ImageRepo, "Image repository for local CCM injection")
	rootCmd.PersistentFlags().StringVar(&config.ImageTag, "image-tag", config.ImageTag, "Image tag for local CCM injection")
	rootCmd.PersistentFlags().DurationVar(&config.ValidationWait, "validation-wait", config.ValidationWait, "Time to wait for cluster validation")

	upCmd := &cobra.Command{
		Use:   "up",
		Short: "Provision the kOps cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Update derived SSH public key if private key was changed via flag
			config.SSHPublicKey = config.SSHPrivateKey + ".pub"
			if err := kops.HydrateTemplate(config); err != nil {
				return err
			}
			return kops.Up(config)
		},
	}

	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Tear down the kOps cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			return kops.Down(config)
		},
	}

	prepareCmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare the kOps cluster (hydrate template and ensure state store)",
		RunE: func(cmd *cobra.Command, args []string) error {
			config.SSHPublicKey = config.SSHPrivateKey + ".pub"
			if err := kops.HydrateTemplate(config); err != nil {
				return err
			}
			return kops.EnsureStateStore(config)
		},
	}

	rootCmd.AddCommand(upCmd, downCmd, prepareCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
