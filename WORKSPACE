workspace(name = "io_k8s_cloud_provider_gcp")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "4b2c61795ac2eefcb28f3eb8e1cb2d8fb3c2eafa0f6712473bc5f93728f38758",
    urls = ["https://github.com/bazelbuild/rules_go/releases/download/0.10.2/rules_go-0.10.2.tar.gz"],
)

http_archive(
    name = "bazel_skylib",
    sha256 = "bbccf674aa441c266df9894182d80de104cabd19be98be002f6d478aaa31574d",
    strip_prefix = "bazel-skylib-2169ae1c374aab4a09aa90e65efe1a3aad4e279b",
    urls = ["https://github.com/bazelbuild/bazel-skylib/archive/2169ae1c374aab4a09aa90e65efe1a3aad4e279b.tar.gz"],
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "481ab09ce5fb40b57cfa962d211510b47accdd05a8168723da47307ca15d4725",
    strip_prefix = "rules_docker-452878d665648ada0aaf816931611fdd9c683a97",
    urls = ["https://github.com/bazelbuild/rules_docker/archive/452878d665648ada0aaf816931611fdd9c683a97.tar.gz"],
)

load("@bazel_skylib//:lib.bzl", "versions")

versions.check(minimum_bazel_version = "0.10.0")

load("@io_bazel_rules_go//go:def.bzl", "go_rules_dependencies", "go_register_toolchains", "go_download_sdk")

go_rules_dependencies()

go_download_sdk(
    name = "go_sdk",
    sdks = {
        "linux_amd64": ("go1.9.4b4.linux-amd64.tar.gz", "c595338b66c4ce2c2085309987c43949be3dbf60109fb535029f0f625c79ef2e"),
    },
    urls = ["https://storage.googleapis.com/go-boringcrypto/{}"],
)

go_register_toolchains(
    go_version = "1.10.1",
)

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
    container_repositories = "repositories",
)

container_repositories()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()
