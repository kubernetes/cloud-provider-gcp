load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "registry.go",
        "root.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/registry",
    importpath = "github.com/docker/distribution/registry",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution/configuration:go_default_library",
        "//vendor/github.com/docker/distribution/context:go_default_library",
        "//vendor/github.com/docker/distribution/health:go_default_library",
        "//vendor/github.com/docker/distribution/registry/handlers:go_default_library",
        "//vendor/github.com/docker/distribution/registry/listener:go_default_library",
        "//vendor/github.com/docker/distribution/registry/storage:go_default_library",
        "//vendor/github.com/docker/distribution/registry/storage/driver/factory:go_default_library",
        "//vendor/github.com/docker/distribution/uuid:go_default_library",
        "//vendor/github.com/docker/distribution/version:go_default_library",
        "//vendor/github.com/spf13/cobra:go_default_library",
        "@com_github_bshuster_repo_logrus_logstash_hook//:go_default_library",
        "@com_github_bugsnag_bugsnag_go//:go_default_library",
        "@com_github_docker_libtrust//:go_default_library",
        "@com_github_gorilla_handlers//:go_default_library",
        "@com_github_sirupsen_logrus//:go_default_library",
        "@com_github_yvasiyarov_gorelic//:go_default_library",
        "@io_rsc_letsencrypt//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["registry_test.go"],
    embed = [":go_default_library"],
    deps = ["//vendor/github.com/docker/distribution/configuration:go_default_library"],
)
