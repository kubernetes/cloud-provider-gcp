workspace(name = "io_k8s_cloud_provider_gcp")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "078f2a9569fa9ed846e60805fb5fb167d6f6c4ece48e6d409bf5fb2154eaf0d8",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/rules_go/releases/download/v0.20.0/rules_go-v0.20.0.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.20.0/rules_go-v0.20.0.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "41bff2a0b32b02f20c227d234aa25ef3783998e5453f7eade929704dcff7cd4b",
    urls = ["https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.19.0/bazel-gazelle-v0.19.0.tar.gz"],
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
    sha256 = "480daa8737bf4370c1a05bfced903827e75046fea3123bd8c81389923d968f55",
    strip_prefix = "rules_docker-0.11.0",
    urls = ["https://github.com/bazelbuild/rules_docker/archive/v0.11.0.tar.gz"],
)

load("@bazel_skylib//lib:versions.bzl", "versions")

versions.check(minimum_bazel_version = "0.20.0")

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains", "go_download_sdk")

go_rules_dependencies()

go_download_sdk(
    name = "go_sdk",
    sdks = {
        "linux_amd64": ("go1.13.1b4.linux-amd64.tar.gz", "70be1bae05feb67d0560f39767e80707343d96554c5a611fbb93b04ce5913693"),
    },
    urls = ["https://storage.googleapis.com/go-boringcrypto/{}"],
)

go_register_toolchains()

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)
load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
)

container_repositories()

load("@io_bazel_rules_docker//repositories:deps.bzl", container_deps = "deps")

container_deps()

container_pull(
    name = "distroless",
    digest = "sha256:c6d5981545ce1406d33e61434c61e9452dad93ecd8397c41e89036ef977a88f4",
    registry = "gcr.io",
    repository = "distroless/static",
    tag = "b54513ef989c81d68cb27d9c7958697e2fedd2c4",
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
