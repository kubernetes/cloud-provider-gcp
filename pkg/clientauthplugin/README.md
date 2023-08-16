Client Auth Plugin library

* Introduction

Client auth library is a copy of the gcp plugin library that existed in Client-Go until k8s 1.25. Going ahead, this library is not supported and is expected to be replaced by [GKE-GCLOUD-AUTH-PLUGIN](https://cloud.google.com/blog/products/containers-kubernetes/kubectl-auth-changes-in-gke). The code here is kept as a reference, in case anyone needs to refer to it. 

* Usage

This library can be reference from code by importing it, similar to the library that existed in client-go.

```
import _ pkg/clientauthplugin/gcp/gcp
```

* How to build kube config to use this library?

To use this library, the ~/.kube/config has to be edited. This can be done by running

1. Set `export USE_GKE_GCLOUD_AUTH_PLUGIN=False` in ~/.bashrc
2. Run `source ~/.bashrc`
3. Run `gcloud container clusters get-credentials CLUSTER_NAME`
