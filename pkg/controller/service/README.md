# Service Controller

This service controller is a fork of the upstream service controller from `k8s.io/cloud-provider/controllers/service`.

## Modifications

The primary modification in this fork is to the `WantsLoadBalancer` function in `controller.go`. This change allows the controller to manage services that have a `loadBalancerClass` set to one of the following GKE-specific values:

- `networking.gke.io/l4-regional-external-legacy`
- `networking.gke.io/l4-regional-internal-legacy`

The upstream controller ignores any service with a non-nil `loadBalancerClass`. This fork extends that logic to claim these specific classes for the GKE cloud provider.
