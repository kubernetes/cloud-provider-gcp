# cloud-provider-gcp

## Dependency management

Use [dep_ensure.sh](./tools/dep_ensure.sh) script to update package dependencies. 

Run the following command to update bazel build rules automatically.

```shell
bazel run //:gazelle
```

## Publishing gcp-controller-manager image

This command will build and publish
`gcr.io/k8s-image-staging/gcp-controller-manager:latest`:

```
bazel run //cmd/gcp-controller-manager:publish
```

Environment variables `IMAGE_REPO` and `IMAGE_TAG` can be used to override
destination GCR repository and tag.

This command will build and publish
`gcr.io/my-repo/gcp-controller-manager:v1`:


```
IMAGE_REPO=my-repo IMAGE_TAG=v1 bazel run //cmd/gcp-controller-manager:publish
```
