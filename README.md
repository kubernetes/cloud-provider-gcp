# cloud-provider-gcp

## Publishing gcp-controller-manager image

This command will build and publish
`gcr.io/k8s-image-staging/gcp-controller-manager:latest`:

```
bazel run //cmd/gcp-controller-manager:publish
```

Environment variables `IMAGE_REGISTRY`, `IMAGE_REPO` and `IMAGE_TAG` can be
used to override destination GCR repository and tag.

This command will build and publish
`example.com/my-repo/gcp-controller-manager:v1`:


```
IMAGE_REGISTRY=example.com IMAGE_REPO=my-repo IMAGE_TAG=v1 bazel run //cmd/gcp-controller-manager:publish
```

# Cross-compiling

Selecting the target platform is done with the `--platforms` option with `bazel`.
This command builds release tarballs for Windows:

```
bazel build --platforms=@io_bazel_rules_go//go/toolchain:windows_amd64 //release:release-tars
```

This command explicitly targets Linux as the target platform:

```
bazel build --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 //release:release-tars
```


# Dependency management

Dependencies are managed using [Go modules](https://github.com/golang/go/wiki/Modules) (`go mod` subcommands).

Note that builds are done with Bazel and not the Go tool. Don't follow public
Go module docs, instead use instructions in this readme.

## Working within GOPATH

If you work within `GOPATH`, `go mod` will error out unless you do one of:

- move repo outside of GOPATH (it should "just work")
- set env var `GO111MODULE=on`

## Add a new dependency

```
go get github.com/new/dependency && ./tools/update_vendor.sh
```

## Update an existing dependency

```
go get -u github.com/existing/dependency && ./tools/update_vendor.sh
```

## Update all dependencies

```
go get -u && ./tools/update_vendor.sh
```

Note that this most likely won't work due to cross-dependency issues or repos
not implementing modules correctly.

# Bazel

Bazel is required to build and release cloud-provider-gcp.

To install:

```sh
go get github.com/bazelbuild/bazelisk
alias bazel=bazelisk
```

To re-generate `BUILD` files:

```sh
tools/update_bazel.sh
```
