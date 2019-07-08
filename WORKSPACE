workspace(name = "io_k8s_cloud_provider_gcp")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "f04d2373bcaf8aa09bccb08a98a57e721306c8f6043a2a0ee610fd6853dcde3d",
    urls = ["https://github.com/bazelbuild/rules_go/releases/download/0.18.6/rules_go-0.18.6.tar.gz"],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "3c681998538231a2d24d0c07ed5a7658cb72bfb5fd4bf9911157c0e9ac6a2687",
    urls = ["https://github.com/bazelbuild/bazel-gazelle/releases/download/0.17.0/bazel-gazelle-0.17.0.tar.gz"],
)

skylib_version = "0.8.0"

http_archive(
    name = "bazel_skylib",
    sha256 = "2ef429f5d7ce7111263289644d233707dba35e39696377ebab8b0bc701f7818e",
    type = "tar.gz",
    url = "https://github.com/bazelbuild/bazel-skylib/releases/download/{}/bazel-skylib.{}.tar.gz".format(skylib_version, skylib_version),
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "aed1c249d4ec8f703edddf35cbe9dfaca0b5f5ea6e4cd9e83e99f3b0d1136c3d",
    strip_prefix = "rules_docker-0.7.0",
    urls = ["https://github.com/bazelbuild/rules_docker/archive/v0.7.0.tar.gz"],
)

load("@bazel_skylib//lib:versions.bzl", "versions")

versions.check(minimum_bazel_version = "0.20.0")

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains", "go_download_sdk")

go_rules_dependencies()

go_download_sdk(
    name = "go_sdk",
    sdks = {
        "linux_amd64": ("go1.11.5b4.linux-amd64.tar.gz", "9b5b2972b452da9ba6bba65bab18fb9e8fbda31b5c489275710e5429d76f568c"),
    },
    urls = ["https://storage.googleapis.com/go-boringcrypto/{}"],
)

go_register_toolchains(
    go_version = "1.11.5",
)

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)
load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
)

container_repositories()

container_pull(
    name = "distroless",
    digest = "sha256:de63da39d0477a9994276cb1de6cec710d9e293ca667ef01ef189b6c87b554e9",
    registry = "gcr.io",
    repository = "distroless/static",
    # tag = "latest",
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

git_repository(
    name = "bazel_skylib",
    remote = "https://github.com/bazelbuild/bazel-skylib.git",
    tag = "0.6.0",
)

git_repository(
    name = "io_k8s_repo_infra",
    commit = "df02ded38f9506e5bbcbf21702034b4fef815f2f",
    remote = "https://github.com/kubernetes/repo-infra.git",
)

load("//defs:repo_rules.bzl", "fetch_kube_release")

fetch_kube_release(
    name = "io_k8s_release",
    archives = {
        "kubernetes-server-linux-amd64.tar.gz": "fdc13919936ca9c698c96038b033ff9e14a9724871e08300386d52a397e0f432",
        "kubernetes-manifests.tar.gz": "b1ada46f36337b378f080ff076cc20ec2eb8fd9cc5d2f444d7e6e14583ec2429",
        # we do not currently make modifications to these release tars below
        "kubernetes-node-linux-amd64.tar.gz": "0672c41e66af76225f3cf5cd12b3e63ff26ea42bcb617d1a94e1a3a528face5a",
    },
    version = "v1.14.0-beta.2",
)
