# cloud-provider-gcp

## Publishing cloud-controller-manager image

Create an [Artifact Registry repository](https://docs.cloud.google.com/artifact-registry/docs/docker/store-docker-container-images) for the CCM image.

Then use `make publish` to build and push the `cloud-controller-manager` Docker image. For example, the following command will build and push the image to `us-central1-docker.pkg.dev/my-project/my-repo/cloud-controller-manager:v0`.
Change the location, project, and repo names to match yours.

```sh
LOCATION=us-central1 PROJECT=my-project REPO=my-repo
gcloud auth configure-docker ${LOCATION}-docker.pkg.dev
IMAGE_REPO=${LOCATION}-docker.pkg.dev/${PROJECT}/${REPO} IMAGE_TAG=v0 make publish
```

If `IMAGE_REPO` is not set, the script will exit with an error. If `IMAGE_TAG` is not set, it defaults to a unique value combining the current git commit SHA and the build date.

### Docker Commands

**Note:** To push images to Google Artifact Registry, you must first authenticate Docker by running the following command:
`gcloud auth configure-docker ${LOCATION}-docker.pkg.dev`

*   **`make publish`**: Builds the `cloud-controller-manager` Docker image (including multi-architecture support) and pushes it to the container registry specified by the `IMAGE_REPO` environment variable.

*   **`make bundle`**: Builds the `cloud-controller-manager` Docker image and saves it as a `.tar` file locally, along with creating a `.docker_tag` file. This is useful for offline distribution or loading.

*   **`make clean-builder`**: Removes the `docker buildx` builder used for multi-platform Docker builds. This command is useful to reset the builder environment if the builder encounters an error or becomes corrupted. It can also be used to free up resources when the builder is no longer needed.


# Cross-compiling

Selecting the target platform is done with the `--platforms` option with `bazel`.
This command builds release tarballs for Windows:

```sh
bazel build --platforms=@io_bazel_rules_go//go/toolchain:windows_amd64 //release:release-tars
```

This command explicitly targets Linux as the target platform:

```sh
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

```sh
go get github.com/new/dependency && make update-vendor
```

## Update an existing dependency

```sh
go get -u github.com/existing/dependency && make update-vendor
```

## Update all dependencies

```sh
go get -u && make update-vendor
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
