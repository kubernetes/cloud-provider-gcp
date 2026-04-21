/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kops

const defaultTemplate = `apiVersion: kops.k8s.io/v1alpha2
kind: Cluster
metadata:
  name: {{ .clusterName }}
spec:
  api:
    loadBalancer:
      type: Public
  authorization:
    rbac: {}
  channel: stable
  cloudConfig:
    gceServiceAccount: default
  cloudLabels:
    group: sig-cluster-lifecycle
    subproject: kops
  cloudProvider: gce
  configBase: {{ .stateStore }}/{{ .clusterName }}
  containerd:
    configAdditions:
      plugins."io.containerd.grpc.v1.cri".containerd.runtimes.test-handler.runtime_type: io.containerd.runc.v2
  etcdClusters:
  - cpuRequest: 200m
    etcdMembers:
{{- range $i, $zone := (splitList "," "${GCP_ZONES}") }}
    - instanceGroup: control-plane-{{ $zone }}
      name: {{ index (splitList "-" $zone) 2 }}
{{- end }}
    manager:
      backupRetentionDays: 90
    memoryRequest: 100Mi
    name: main
  - cpuRequest: 100m
    etcdMembers:
{{- range $i, $zone := (splitList "," "${GCP_ZONES}") }}
    - instanceGroup: control-plane-{{ $zone }}
      name: {{ index (splitList "-" $zone) 2 }}
{{- end }}
    manager:
      backupRetentionDays: 90
    memoryRequest: 100Mi
    name: events
  iam:
    allowContainerRegistry: true
    legacy: false
  kubelet:
    anonymousAuth: false
  kubernetesApiAccess:
  - 0.0.0.0/0
  kubernetesVersion: ${K8S_VERSION}
  networking:
    gce: {}
  nodePortAccess:
  - 0.0.0.0/0
  nonMasqueradeCIDR: 10.0.0.0/8
  podCIDR: 10.4.0.0/14
  project: ${GCP_PROJECT}
  serviceClusterIPRange: 10.1.0.0/16
  sshAccess:
  - 0.0.0.0/0
  subnets:
  - cidr: 10.0.32.0/19
    name: ${GCP_LOCATION}
    region: ${GCP_LOCATION}
    type: Public
  topology:
    dns:
      type: None
  externalCloudControllerManager:
    useServiceAccountCredentials: false
---
apiVersion: kops.k8s.io/v1alpha2
kind: SSHCredential
metadata:
  name: admin
  labels:
    kops.k8s.io/cluster: {{ .clusterName }}
spec:
  publicKey: {{ .sshPublicKey }}
{{- range $i, $zone := (splitList "," "${GCP_ZONES}") }}
---
apiVersion: kops.k8s.io/v1alpha2
kind: InstanceGroup
metadata:
  labels:
    kops.k8s.io/cluster: {{ $.clusterName }}
  name: control-plane-{{ $zone }}
spec:
  machineType: ${CONTROL_PLANE_MACHINE_TYPE}
  role: Master
  subnets:
  - ${GCP_LOCATION}
  zones:
  - {{ $zone }}
{{- end }}
---
apiVersion: kops.k8s.io/v1alpha2
kind: InstanceGroup
metadata:
  labels:
    kops.k8s.io/cluster: {{ .clusterName }}
  name: nodes-{{ (index (splitList "," "${GCP_ZONES}") 0) }}
spec:
  machineType: ${NODE_MACHINE_TYPE}
  minSize: ${NODE_COUNT}
  maxSize: ${NODE_COUNT}
  role: Node
  subnets:
  - ${GCP_LOCATION}
  zones:
{{- range $i, $zone := (splitList "," "${GCP_ZONES}") }}
  - {{ $zone }}
{{- end }}
`
