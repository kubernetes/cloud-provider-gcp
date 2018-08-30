package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_test(
    name = "go_default_test",
    srcs = ["metrics_test.go"],
    embed = [":go_default_library"],
)

go_library(
    name = "go_default_library",
    srcs = ["metrics.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/endpoints/metrics",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/endpoints/metrics",
    deps = [
        "//vendor/github.com/prometheus/client_golang/prometheus:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request:go_default_library",
        "@com_github_emicklei_go_restful//:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
)
