# GCP CCM with kOps Quickstart

This guide provides a quickstart for building and deploying the GCP Cloud Controller Manager (CCM) to a self-managed Kubernetes cluster provisioned with kOps.

## Prerequisites

A Google Cloud Platform project with billing enabled.

## Deployment

The `make kops-up` target is an end-to-end workflow that automatically:
- Provisions a Kubernetes cluster using kOps.
- Builds the CCM image locally.
- Pushes the image to your Artifact Registry.
- Deploys the CCM (along with required RBAC) to the cluster.

Run the following commands to get started:

```sh
# Enable required GCP APIs
gcloud services enable compute.googleapis.com
gcloud services enable artifactregistry.googleapis.com

# Set environment variables
export GCP_PROJECT=$(gcloud config get-value project)
export GCP_LOCATION=us-central1
export GCP_ZONES=${GCP_LOCATION}-a
export KOPS_CLUSTER_NAME=kops.k8s.local
export KOPS_STATE_STORE=gs://${GCP_PROJECT}-kops-state

# Create the state store bucket if it doesn't already exist
gcloud storage buckets create ${KOPS_STATE_STORE} --location=${GCP_LOCATION} || true

# Run the cluster creation target, may take several minutes
make kops-up
```

## Verification

To verify that the Cloud Controller Manager is running successfully:

1.  **Check the Pod Status**: Verify the pod is `Running` in the `kube-system` namespace.
```sh
kubectl get pods -n kube-system -l component=cloud-controller-manager
```

2.  **Check Pod Logs**: Look for any errors or access and authentication issues with the GCP API.
```sh
kubectl logs -n kube-system -l component=cloud-controller-manager
```

3.  **Check Node Initialization**: The CCM should remove the `node.cloudprovider.kubernetes.io/uninitialized` taint once it successfully fetches the node's properties from the GCP API.
```sh
# Ensure no nodes have the uninitialized taint, output should be empty.
kubectl get nodes -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints | grep uninitialized
```

4.  **Verify ProviderID**: Check if your nodes are correctly populated with GCP-specific data (e.g., `ProviderID` in the format `gce://...`).
```sh
kubectl describe nodes | grep "ProviderID:"
```

## Teardown

To tear down the cluster and clean up resources:

```sh
make kops-down
```
