# GCP Cloud Controller Manager (CCM) Manual Setup Guide

This guide provides instructions for building and deploying the GCP Cloud Controller Manager (CCM) to a self-managed Kubernetes cluster.

## Prerequisites

1.  **Kubernetes Cluster**: A Kubernetes cluster running on Google Cloud Platform. 
    *   The cluster's components (`kube-apiserver`, `kube-controller-manager`, and `kubelet`) must have the `--cloud-provider=external` flag.
    *   For an example of how to create GCE instances and initialize such a cluster manually using `kubeadm`, see **[Manual Kubernetes Cluster on GCE](manual-cluster-gce.md)**.
2.  **GCP Service Account**: The nodes (or the CCM pod itself) must have access to a GCP IAM Service Account with sufficient permissions to manage compute resources (e.g. instances, load balancers, and routes).
3.  **Docker & gcloud CLI**: Authorized and configured for pushing images to GCP Artifact Registry.


### [Optional] Build and Push the CCM Image (Manual Clusters)

> [!NOTE]
> This step is only necessary if you intend to use a locally built image of the CCM. Otherwise, use an official release image as specified in Step 1.

If you are using a manually provisioned cluster (e.g. `kubeadm`), build the `cloud-controller-manager` Docker image and push it to your registry:

```sh
# Google Cloud Project ID, registry location, and repository name.
GCP_PROJECT=$(gcloud config get-value project)
GCP_LOCATION=us-central1
REPO=my-repo

# Create an Artifact Registry repository (if it doesn't already exist)
gcloud artifacts repositories create ${REPO} \
    --project=${GCP_PROJECT} \
    --repository-format=docker \
    --location=${GCP_LOCATION} \
    --description="Docker repository for CCM"
```

Replace `NODE_NAME` and `NODE_ZONE` with the name and zone of your master node.
```sh
# Or with kubectl: NODE_NAME=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
# Grant the cluster nodes permission to pull images from Artifact Registry.
# This automatically extracts your GCE node's service account using kubectl and gcloud.
NODE_NAME=k8s-master
NODE_ZONE=us-central1-a
NODE_SA=$(gcloud compute instances describe $NODE_NAME \
    --zone=$NODE_ZONE --project=${GCP_PROJECT} \
    --format="value(serviceAccounts[0].email)")
echo $NODE_SA
gcloud artifacts repositories add-iam-policy-binding ${REPO} \
    --project=${GCP_PROJECT} \
    --location=${GCP_LOCATION} \
    --member="serviceAccount:${NODE_SA}" \
    --role="roles/artifactregistry.reader"
# Configure docker to authenticate with Artifact Registry
gcloud auth configure-docker ${GCP_LOCATION}-docker.pkg.dev

# Build and Push
IMAGE_REPO=${GCP_LOCATION}-docker.pkg.dev/${GCP_PROJECT}/${REPO} IMAGE_TAG=v0 make publish
```

*Note: If `IMAGE_TAG` is omitted, the Makefile will use a combination of the current Git commit SHA and the build date.*

## Step 1: Deploy the CCM to your Cluster (Manual Clusters)

For native Kubernetes clusters, avoid the legacy `deploy/cloud-controller-manager.manifest` (which is a template used by legacy script `kube-up`). Instead, use the kustomize-ready DaemonSet which correctly includes the RBAC roles and deployment.

Update the image to the official release:
```sh
# Replace with desired version vX.Y.Z
(cd deploy/packages/default && kustomize edit set image k8scloudprovidergcp/cloud-controller-manager=registry.k8s.io/cloud-provider-gcp/cloud-controller-manager:v30.0.0)
```

**For development with a locally built image**
If you built the image locally in the optional prerequisite step above, instead run this command to use your newly pushed tag:
```sh
(cd deploy/packages/default && kustomize edit set image k8scloudprovidergcp/cloud-controller-manager=$IMAGE_REPO:$IMAGE_TAG)
```

### Update CCM Manifest

The `manifest.yaml` DaemonSet intentionally has empty command-line arguments (`args: []`). You must provide the necessary arguments to the `cloud-controller-manager` container. For a typical Kops or GCE cluster, you can supply these arguments by creating a Kustomize patch.

> [!NOTE]
> Modify `--cluster-cidr` and `--cluster-name` below to match your cluster. Note that GCP resource names cannot contain dots (`.`), so if your cluster name is `my.cluster.net`, you must use a sanitized format like `my-cluster-net`. If you don't have a cluster name, just use any name (e.g., node name).

```sh
cat << EOF > deploy/packages/default/args-patch.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
    name: cloud-controller-manager
    namespace: kube-system
spec:
    template:
        spec:
            volumes:
            - name: host-kubeconfig
              hostPath:
                path: /etc/kubernetes/admin.conf
            containers:
            - name: cloud-controller-manager
              command: ["/usr/local/bin/cloud-controller-manager"]
              volumeMounts:
              - name: host-kubeconfig
                mountPath: /etc/kubernetes/admin.conf
                readOnly: true
              args:
              - --kubeconfig=/etc/kubernetes/admin.conf
              - --authentication-kubeconfig=/etc/kubernetes/admin.conf
              - --authorization-kubeconfig=/etc/kubernetes/admin.conf
              - --cloud-provider=gce
              - --allocate-node-cidrs=true
              - --cluster-cidr=10.4.0.0/14
              - --cluster-name=kops-k8s-local
              - --configure-cloud-routes=true
              - --leader-elect=true
              - --use-service-account-credentials=true
              - --v=2
EOF
(cd deploy/packages/default && kustomize edit add patch --path args-patch.yaml)

# Deploy the configured package (this applies the DaemonSet and its required roles):
kubectl apply -k deploy/packages/default
```

If you encounter a `NotFound` pod error for the container command path, try `command: ["/cloud-controller-manager"]` instead.


### Alternative: Apply Standalone RBAC Roles

If you prefer to deploy the RBAC rules independently from the base daemonset package, you can apply them directly:

```sh
kubectl apply -f deploy/cloud-node-controller-role.yaml
kubectl apply -f deploy/cloud-node-controller-binding.yaml
kubectl apply -f deploy/pvl-controller-role.yaml
```

## Step 3: Verification

To verify that the Cloud Controller Manager is running successfully:

1.  **Check the Pod Status**: Verify the pod is `Running` in the `kube-system` namespace.
```sh
kubectl get pods -n kube-system -l component=cloud-controller-manager
```

2.  **Check Pod Logs**: Look for any errors or access and authentication issues with the GCP API.
```sh
kubectl describe pod -n kube-system -l component=cloud-controller-manager
kubectl logs -n kube-system -l component=cloud-controller-manager
```

3.  **Check Node Initialization**: The `kubelet` initially applies a `node.cloudprovider.kubernetes.io/uninitialized` taint when bound to an external cloud provider. The CCM should remove this taint once it successfully fetches the node's properties from the GCP API.
```sh
# Ensure no nodes have the uninitialized taint, output should be empty.
kubectl get nodes -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints | grep uninitialized
```

4.  **Verify External IPs and ProviderID**: Check if your nodes are correctly populated with GCP-specific data.
```sh
kubectl describe nodes | grep "ProviderID:"
# Expect output `ProviderID:    gce://...`
```

## Teardown

If you used the default CCM package, you can clean up the local patch file and reset all changes to kustomization.yaml:
```sh
rm deploy/packages/default/args-patch.yaml
git checkout deploy/packages/default/kustomization.yaml
```

If you followed the [manual cluster setup guide](manual-cluster-gce.md), you may follow the [teardown steps](manual-cluster-gce.md#teardown) to clean up your GCP resources.