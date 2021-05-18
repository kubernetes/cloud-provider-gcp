# Update cloud-provider-gcp with Kubernetes release

## Overview

k8s.io/cloud-provider-gcp provides a sample distribution of how to run Kubernetes on the Google Cloud.
This project allows a Kubernetes cluster to provision, monitor and add/remove GCP resources necessary for operation of the cluster. 
It has to be updated together with Kubernetes releases. 
This document describes the steps to update cloud-provider-gcp with a Kubernetes release version.

## Workflow

1. Update library to the desired version.
`https://github.com/kubernetes/cloud-provider-gcp/blob/master/go.mod` describes the required libraries. 
   Update the version of each dependency to the desired Kubernetes release version.


2. Update library to the desired version in //providers package.
   `https://github.com/kubernetes/cloud-provider-gcp/blob/master/providers/go.mod` describes the required libraries for `providers` package.
   Update the version of each dependency to the desired Kubernetes release version.


3. In `https://github.com/kubernetes/cloud-provider-gcp/blob/master/WORKSPACE`, update kube-release sha and version to the [Kubernetes desired release version](https://kubernetes.io/releases/). 
   Note: The current Kubernetes release is using sha512 hash while cloud-provider-gcp is using sha256. Re-sha with command `sha256sum` if needed.


4. Update KUBE_GIT_VERSION in `https://github.com/kubernetes/cloud-provider-gcp/blob/master/tools/version.sh#L77` with the right tag.


5. Refer to [update an existing dependency](https://github.com/kubernetes/cloud-provider-gcp/blob/master/README.md#update-an-existing-dependency)
   and [clean up unused dependency](https://github.com/kubernetes/cloud-provider-gcp/blob/master/README.md#clean-up-unused-dependencies) for the dependencies operations if needed.


## Build cloud-provider-gcp

This command will build a deploy-able cloud-provider-gcp package/release

```
bazel run //release:release-tars
```

##Validation

A cluster could be brought up for testing with either
```
https://github.com/kubernetes/cloud-provider-gcp/blob/master/cluster/kube-up.sh 
```

OR

```
kubetest2 gce -v 2 --repo-root $REPO_ROOT --build --up
```

Currently we have conformance test run periodically. The command to run conformance test locally:
```
kubetest2 gce -v 2 --repo-root $REPO_ROOT --build --up --down --test=ginkgo -- --test-package-version=[your version] --focus-regex='\[Conformance\]'
```
