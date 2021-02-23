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
    sha256 = "4521794f0fba2e20f3bf15846ab5e01d5332e587e9ce81629c7f96c793bb7036",
    strip_prefix = "rules_docker-0.14.4",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.14.4/rules_docker-v0.14.4.tar.gz"],
)

load("@bazel_skylib//lib:versions.bzl", "versions")

versions.check(minimum_bazel_version = "0.20.0")

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains", "go_download_sdk")

go_rules_dependencies()

go_download_sdk(
    name = "go_sdk",
    sdks = {
        "linux_amd64": ("go1.15.10b5.linux-amd64.tar.gz", "7533b0307fd995deb9ef68d67899582c336a3c62387d19d03d10202129e9fad3"),
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

load("@io_bazel_rules_docker//repositories:pip_repositories.bzl", "pip_deps")

pip_deps()

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
        #"kubernetes-server-linux-amd64.tar.gz": "853904a632b3adbabcbc61e5d563447ed75b8408c0297515f65c6a3d2b46be42",
        "kubernetes-server-linux-amd64.tar.gz": "1772c20e7765a2288bd335f754b9cf5cb2e2e0b5b11f80db0d9b582c1016c663",
        #"kubernetes-manifests.tar.gz": "38946246fb192d6e877d0d04a7b0645980b983b8b81ca2a259c50a035ced815b7",
        "kubernetes-manifests.tar.gz": "bf99bd768afdda829f617038b10aafff7f8cd07071bd0e8b585f118bfbff9df7",
        # we do not currently make modifications to these release tars below
<<<<<<< HEAD
        #"kubernetes-node-linux-amd64.tar.gz": "7128a3c647c93181b7b52b668eb3030b8beee025b8b4614f14f159874e47dc34",
        "kubernetes-node-linux-amd64.tar.gz": "5baa2b45bdca2dd09d03b9e2a4fb17e105c47972b600f7c2dcbdbaec2a956ae1",
=======
        #"kubernetes-node-linux-amd64.tar.gz": "d32e568a78230ee25de25ca5ba0d9fc9b5b783d0e41fadb983f318b338a70357",
        "kubernetes-node-linux-amd64.tar.gz": "7128a3c647c93181b7b52b668eb3030b8beee025b8b4614f14f159874e47dc34",
        "kubernetes-node-windows-amd64.tar.gz": "3e2a0560a3af45add14290f4dddc6e5720b34851b49b3a7f1a4144f1e35a0dcb",
>>>>>>> a71f515f (Add initial Windows crossbuild support.)
    },
    version = "v1.20.0",
)
