package app

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/options"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/term"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
)

const cloudProviderName = "gce"

// NewCloudControllerManagerCommand creates a cobra command object with default parameters
func NewCloudControllerManagerCommand() *cobra.Command {
	s, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	cmd := &cobra.Command{
		Use: "cloud-controller-manager",
		Long: `The Cloud controller manager is a daemon that embeds
the cloud specific control loops shipped with Kubernetes.`,
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()
			cliflag.PrintFlags(cmd.Flags())

			c, err := s.Config(knownControllers(), app.ControllersDisabledByDefault.List())
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			cloud := initializeCloudProvider(c.ComponentConfig.KubeCloudShared.CloudProvider.Name, c)
			controllerInitializers := app.DefaultControllerInitializers(c.Complete(), cloud)
			if err := app.Run(c.Complete(), controllerInitializers, wait.NeverStop); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

		},
	}

	fs := cmd.Flags()
	namedFlagSets := s.Flags(knownControllers(), app.ControllersDisabledByDefault.List())
	verflag.AddFlags(namedFlagSets.FlagSet("global"))
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), cmd.Name())

	if flag.CommandLine.Lookup("cloud-provider-gce-lb-src-cidrs") != nil {
		// hoist this flag from the global flagset to preserve the commandline until
		// the gce cloudprovider is removed.
		globalflag.Register(namedFlagSets.FlagSet("generic"), "cloud-provider-gce-lb-src-cidrs")
	}
	if flag.CommandLine.Lookup("cloud-provider-gce-l7lb-src-cidrs") != nil {
		globalflag.Register(namedFlagSets.FlagSet("generic"), "cloud-provider-gce-l7lb-src-cidrs")
	}
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}
	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), namedFlagSets, cols)
		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), namedFlagSets, cols)
	})

	return cmd
}

func initializeCloudProvider(name string, config *config.Config) cloudprovider.Interface {
	cloudConfigFile := config.ComponentConfig.KubeCloudShared.CloudProvider.CloudConfigFile
	// initialize cloud provider with the cloud provider name and config file provided
	cloud, err := cloudprovider.InitCloudProvider(name, cloudConfigFile)
	if err != nil {
		klog.Fatalf("Cloud provider could not be initialized: %v", err)
	}
	if cloud == nil {
		klog.Fatalf("Cloud provider is nil")
	}

	if !cloud.HasClusterID() {
		if config.ComponentConfig.KubeCloudShared.AllowUntaggedCloud {
			klog.Warning("detected a cluster without a ClusterID.  A ClusterID will be required in the future.  Please tag your cluster to avoid any future issues")
		} else {
			klog.Fatalf("no ClusterID found.  A ClusterID is required for the cloud provider to function properly.  This check can be bypassed by setting the allow-untagged-cloud option")
		}
	}

	// Initialize the cloud provider with a reference to the clientBuilder
	cloud.Initialize(config.ClientBuilder, make(chan struct{}))
	// Set the informer on the user cloud object
	if informerUserCloud, ok := cloud.(cloudprovider.InformerUser); ok {
		informerUserCloud.SetInformers(config.SharedInformers)
	}

	return cloud
}

func knownControllers() []string {
	return []string{"cloud-node", "cloud-node-lifecycle", "service", "route"}
}
