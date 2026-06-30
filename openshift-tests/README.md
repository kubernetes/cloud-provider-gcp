# openshift-tests

This is an OTE (OpenShift Tests Extension) suite for the GCP cloud controller manager. It runs the upstream GCP e2es from the parent suite and adds downstream OpenShift-specific tests.

## Run locally

Build the binary, then run one of the suites against a GCP OpenShift cluster with `KUBECONFIG` set and Google credentials available through `GOOGLE_APPLICATION_CREDENTIALS`:

```sh
make build
KUBECONFIG=/path/to/kubeconfig \
GOOGLE_APPLICATION_CREDENTIALS=/path/to/gce.json \
./bin/cloud-controller-manager-gcp-tests-ext run-suite ccm/gcp/conformance/parallel
```

Use `ccm/gcp/conformance/serial` for the serial suite.

If you have access to a CI or Cluster Bot Prow job for the target cluster, `gce.json` can be obtained from `/var/run/secrets/ci.openshift.io/cluster-profile/..data/gce.json`.
