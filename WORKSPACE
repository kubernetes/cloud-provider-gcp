workspace(name = "io_k8s_cloud_provider_gcp")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "b2038e2de2cace18f032249cb4bb0048abf583a36369fa98f687af1b3f880b26",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.48.1/rules_go-v0.48.1.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.48.1/rules_go-v0.48.1.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    integrity = "sha256-MpOL2hbmcABjA1R5Bj2dJMYO2o15/Uc5Vj9Q0zHLMgk=",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.35.0/bazel-gazelle-v0.35.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.35.0/bazel-gazelle-v0.35.0.tar.gz",
    ],
)


http_archive(
    name = "bazel_skylib",
    sha256 = "f7be3474d42aae265405a592bb7da8e171919d74c16f082a5457840f06054728",
    type = "tar.gz",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.2.1/bazel-skylib-1.2.1.tar.gz",
        "https://github.com/bazelbuild/bazel-skylib/releases/download/1.2.1/bazel-skylib-1.2.1.tar.gz",
    ],
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "b1e80761a8a8243d03ebca8845e9cc1ba6c82ce7c5179ce2b295cd36f7e394bf",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.25.0/rules_docker-v0.25.0.tar.gz"],
)

load("@bazel_skylib//lib:versions.bzl", "versions")

versions.check(minimum_bazel_version = "5.3.0")

load("@io_bazel_rules_go//go:deps.bzl", "go_download_sdk", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_download_sdk(
    name = "go_sdk",
    version = "1.21.11",
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

container_pull(
    name = "go-runner",
    # this digest is actually go-runner-amd64
    digest = "sha256:6c91ef6e31219fa98fd6e564de848795b6693cb26f6dcf0b0f6d438a0058deb4",
    registry = "registry.k8s.io",
    repository = "build-image/go-runner",
    # 'tag' is also supported, but digest is encouraged for reproducibility.
    tag = "v2.3.1-go1.21.11-bullseye.0",
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

git_repository(
    name = "io_k8s_repo_infra",
    commit = "df02ded38f9506e5bbcbf21702034b4fef815f2f",
    remote = "https://github.com/kubernetes/repo-infra.git",
)

load("//defs:repo_rules.bzl", "fetch_kube_release")

fetch_kube_release(
    name = "io_k8s_release",
    archives = {
        "kubernetes-node-linux-amd64.tar.gz": "3750568d8aea8a1ac25ad934d09425c47007f44a36a84caefaf6f0533fd10c79",
        "kubernetes-manifests.tar.gz": "5470af626aa852585c8ee99d9bbcacf34338e1c0ca417d7363f7d83c9787424e",
        "kubernetes-server-linux-amd64.tar.gz": "bd2dc7d2f936a1d98d9d31d8dfef42fa284be744311c13d77edbfe576649f86c",
        "kubernetes-node-windows-amd64.tar.gz": "114ce72a5a3cb24c429d5610c46f3591ad7daeaf2e68e6e49d5355eb1ef4743b",
    },
    version = "v1.30.1",
)
