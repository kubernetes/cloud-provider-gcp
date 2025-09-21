# Service Controller

This service controller is a fork of the upstream service controller from `k8s.io/cloud-provider/controllers/service`.

## Modifications

The primary modification in this fork is to the `WantsLoadBalancer` function in `controller.go`. This change allows the controller to manage services that have a `loadBalancerClass` set to one of the following GKE-specific values:

- `networking.gke.io/l4-regional-external-legacy`
- `networking.gke.io/l4-regional-internal-legacy`

The upstream controller ignores any service with a non-nil `loadBalancerClass`. This fork extends that logic to claim these specific classes for the GKE cloud provider. This ensures that this controller will only process services with a valid `loadBalancerClass`, while the default service controller handles all other services without a `LoadBalancerClass`.

When updating this forked controller from upstream, it is critical that the custom logic in the `WantsLoadBalancer` function is preserved. The purpose of this modification is to allow the controller to manage services that have a `loadBalancerClass` set to one of the GKE-specific values mentioned above.

## Controller Startup

This controller is started using a wrapper function, `startGkeServiceControllerWrapper`, located in `@cmd/cloud-controller-manager/gkeservicecontroller.go`. This wrapper initializes and runs the GKE-specific service controller when the `--controllers` flag includes `gke-service`.

## Configuration

The configuration for this controller, defined in `@vendor/k8s.io/cloud-provider/controllers/service/config/**`, is intentionally not forked. This approach allows the controller to reuse the existing command-line flags and configuration parameters from the upstream service controller, ensuring backward compatibility and simplifying maintenance.
