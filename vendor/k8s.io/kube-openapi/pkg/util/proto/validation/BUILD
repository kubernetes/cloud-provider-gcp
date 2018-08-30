load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "errors.go",
        "types.go",
        "validation.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kube-openapi/pkg/util/proto/validation",
    importpath = "k8s.io/kube-openapi/pkg/util/proto/validation",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/kube-openapi/pkg/util/proto:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "validation_suite_test.go",
        "validation_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/ghodss/yaml:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/util/proto:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/util/proto/testing:go_default_library",
        "@com_github_onsi_ginkgo//:go_default_library",
        "@com_github_onsi_ginkgo//config:go_default_library",
        "@com_github_onsi_ginkgo//types:go_default_library",
        "@com_github_onsi_gomega//:go_default_library",
    ],
)
