# Manual Kubernetes Cluster on GCE

This guide provides an example of how to create GCE instances and initialize a Kubernetes cluster manually using `kubeadm`, configured to use an external Cloud Controller Manager.

## Step 1: Create GCE Instances

```sh
ZONE=us-central1-a
MACHINE_TYPE=e2-medium  # Minimum recommended for K8s control plane

# Control pane instance
gcloud compute instances create k8s-master \
    --zone=$ZONE \
    --machine-type=$MACHINE_TYPE \
    --can-ip-forward \
    --scopes=cloud-platform \
    --tags=k8s-master

# Worker instance (optional)
gcloud compute instances create k8s-worker \
    --zone=$ZONE \
    --machine-type=$MACHINE_TYPE \
    --can-ip-forward \
    --scopes=cloud-platform \
    --tags=k8s-worker
```

## Step 2: Access and Configure Nodes (Master and Worker)

SSH into master before the rest of Step 2:
```sh
gcloud compute ssh k8s-master --zone=us-central1-a
```

### 2.1 Install Container Runtime
```sh
# Update package list
sudo apt-get update

# Install containerd
sudo apt-get install -y containerd

# Configure containerd (Generate default config)
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml

# update SystemdCgroup to true (Recommended for systemd integration)
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml

# Restart containerd
sudo systemctl restart containerd
```

### 2.2 Install Kubeadm, Kubelet, and Kubectl
```sh
# Install dependencies
sudo apt-get update
sudo apt-get install -y apt-transport-https ca-certificates curl gpg

# Download the public signing key
sudo mkdir -p -m 755 /etc/apt/keyrings
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.30/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

# Add the Kubernetes apt repository
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.30/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list

# Install tools and lock versions
sudo apt-get update
sudo apt-get install -y kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl
```

### 2.3 Configure Kubelet for External Cloud Provider
```sh
# Add --cloud-provider=external to the KUBELET_KUBEADM_ARGS
echo 'KUBELET_EXTRA_ARGS="--cloud-provider=external"' | sudo tee /etc/default/kubelet

# Restart kubelet
sudo systemctl daemon-reload
sudo systemctl restart kubelet
```

### 2.4 Enable Kernel Modules and IP Forwarding
```sh
# Load required kernel modules
sudo modprobe overlay
sudo modprobe br_netfilter

# Persist modules across boots
cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

# Set sysctl parameters
cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

# Apply sysctl parameters
sudo sysctl --system
```

[Optional] To set up the Worker Node, SSH into worker:
```sh
gcloud compute ssh k8s-worker --zone=us-central1-a
```
And run the same steps as with master (2.1 through 2.4).

## Step 3: Initialize Cluster

To find the correct version string to use in the configuration below, check the version of `kubelet` installed on your node:
```sh
kubelet --version
```

Create the kubeadm config (modfy `kubernetesVersion`):
```sh
cat <<EOF > kubeadm-config.yaml
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
kubernetesVersion: v1.30.14 # Match the version you installed
apiServer:
  extraArgs:
    cloud-provider: external
controllerManager:
  extraArgs:
    cloud-provider: external
EOF

sudo kubeadm init --config=kubeadm-config.yaml
```

## Step 3.1: [Optional] Add Worker Node

After `kubeadm init` completes, it will output a `kubeadm join` command with a token. Copy that command.

If you need to generate a new join command later (e.g., because the token expired),
run this on the master node (`gcloud compute ssh k8s-master --zone=us-central1-a`):
```sh
kubeadm token create --print-join-command
```

Then after exiting master:

1.  **SSH into the worker node**:
```sh
gcloud compute ssh k8s-worker --zone=us-central1-a
```

2.  **Enable IP Forwarding** (Required by kubeadm):
```sh
sudo sysctl -w net.ipv4.ip_forward=1
```

3.  **Run the join command** with `sudo` (paste the generated command):
```sh
sudo [kubeadm join ...] # kubeadm command
```


## Step 4: Configure Kubectl for Admin Access
```sh
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
```

## Step 5: Verify Node Initialization
```sh
kubectl get nodes
kubectl describe node k8s-master | grep Taints
# Output should show: node.cloudprovider.kubernetes.io/uninitialized:NoSchedule
```

## Step 6: Configure Kubectl for External Access

If following the CCM manual setup guide, you should configure kubectl for external access.

Exit the master node:
```sh
exit
```

Prepare the kubeconfig on your local machine using an **SSH Tunnel** (recommended to bypass firewall restrictions):

1. **Extract Kubeconfig**:
```sh
gcloud compute scp k8s-master:~/.kube/config /tmp/manual-kubeconfig --zone=us-central1-a
```

2. **Patch config for external use (Skip TLS Verify)**:
```sh
# Delete CA data and append skip flag
sed -i "/certificate-authority-data:/d" /tmp/manual-kubeconfig
sed -i '/server:/a \    insecure-skip-tls-verify: true' /tmp/manual-kubeconfig
```

3. **Start SSH Tunnel** (Run this in a **separate background terminal**):
```sh
# Forward local port 6443 to Master port 6443
gcloud compute ssh k8s-master --zone=us-central1-a -- -L 6443:localhost:6443 -N
```

4. **Update config to target the tunnel** (Run this back in your working terminal):
```sh
sed -i "s|server: https://.*|server: https://localhost:6443|g" /tmp/manual-kubeconfig

export KUBECONFIG=/tmp/manual-kubeconfig
kubectl get nodes
```

You may now proceed with the [CCM manual setup guide](ccm-manual.md).

## Teardown

To tear down the manual Kubernetes cluster and release all GCE resources:

1. **Delete GCE Instances**:
   ```sh
   gcloud compute instances delete k8s-master k8s-worker --zone=us-central1-a --quiet
   ```

2. **Clean up Local Kubeconfig**:
   ```sh
   rm /tmp/manual-kubeconfig
   ```

3. **Stop SSH Tunnel**:
   Terminate the background `gcloud compute ssh` tunnel command running in your secondary terminal window (e.g. via `Ctrl + C`).

4. **Unset local KUBECONFIG**:
   Restore default `kubectl` context behavior:
   ```sh
   unset KUBECONFIG
   ```

