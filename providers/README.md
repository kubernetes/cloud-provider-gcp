# cloud-provider-gcp/providers

Currently feature development is no longer accepted in `k8s.io/kubernetes/staging/src/k8s.io/legacy-cloud-providers/<provider>`. 
The `cloud-provider-gcp/providers` directory is to support further development for in-tree cloud provider gce.

`cloud-provider-gcp/providers/gce` will contain the files moved from `k8s.io/kubernetes/staging/src/k8s.io/legacy-cloud-providers/gce`. And `cloud-provider-gcp/providers` will
be released separately.
Later, k8s.io/kubernetes will switch the dependency from `k8s.io/legacy-cloud-providers/gce` to `cloud-provider-gcp/providers`. `k8s.io/legacy-cloud-providers/gce` will be deleted.

After the dependency switch completed, the feature development will continue in `cloud-provider-gcp/providers`.