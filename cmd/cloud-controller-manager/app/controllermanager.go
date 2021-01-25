package app

import (
	"fmt"
	"github.com/spf13/pflag"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/options"
	"k8s.io/component-base/cli/flag"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/term"
	"k8s.io/component-base/version/verflag"
	genericcontrollermanager "k8s.io/controller-manager/app"
	"k8s.io/klog/v2"
	nodeipamcontrolleroptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	nodeipamconfig "k8s.io/kubernetes/pkg/controller/nodeipam/config"
)

const cloudProviderName = "gce"

// NewCloudControllerManagerCommand creates a cobra command object with default parameters
func NewCloudControllerManagerCommand() *cobra.Command {
	s, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	cmd := &cobra.Command{
		Use:  "gcp-cloud-controller-manager",
		Long: `gcp-cloud-controller-manager manages gcp cloud resources for a Kubernetes cluster.`,
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()

			cloudProviderFlag := cmd.Flags().Lookup("cloud-provider")
			if cloudProviderFlag.Value.String() == "" {
				cloudProviderFlag.Value.Set(cloudProviderName)
			}
			cliflag.PrintFlags(cmd.Flags())

			c, err := s.Config(knownControllers(), app.ControllersDisabledByDefault.List())
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			cloud := initializeCloudProvider(cloudProviderFlag.Value.String(), c)
			controllerInitializers := app.DefaultControllerInitializers(c.Complete(), cloud)

			fs := pflag.NewFlagSet("fs", pflag.ContinueOnError)
			var nodeIPAMControllerOptions nodeipamcontrolleroptions.NodeIPAMControllerOptions
			nodeIPAMControllerOptions.AddFlags(fs)
			errors := nodeIPAMControllerOptions.Validate()
			if len(errors) > 0 {
				klog.Fatal("NodeIPAM controller values are not properly.")
			}
			var nodeipamconfig nodeipamconfig.NodeIPAMControllerConfiguration
			nodeIPAMControllerOptions.ApplyTo(&nodeipamconfig)

			controllerInitializers["nodeipam"] = startNodeIpamControllerWrapper(c.Complete(), nodeipamconfig, cloud)

			if err := app.Run(c.Complete(), controllerInitializers, wait.NeverStop); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}
	namedFlagSets := s.Flags(knownControllers(), app.ControllersDisabledByDefault.List())
	setupAdditionalFlags(cmd, namedFlagSets)

	return cmd
}

func setupAdditionalFlags(command *cobra.Command, namedFlagSets flag.NamedFlagSets) {
	fs := command.Flags()
	verflag.AddFlags(namedFlagSets.FlagSet("global"))
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), command.Name())

	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}
	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.TerminalSize(command.OutOrStdout())
	command.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), namedFlagSets, cols)
		return nil
	})
	command.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), namedFlagSets, cols)
	})
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

func startNodeIpamControllerWrapper(ccmconfig *config.CompletedConfig, nodeipamconfig nodeipamconfig.NodeIPAMControllerConfiguration, cloud cloudprovider.Interface) func(ctx genericcontrollermanager.ControllerContext) (http.Handler, bool, error) {
	return func(ctx genericcontrollermanager.ControllerContext) (http.Handler, bool, error) {
		return startNodeIpamController(ccmconfig, nodeipamconfig, ctx, cloud)
	}
}
