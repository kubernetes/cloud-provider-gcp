load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "metrics.go",
        "proxier.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/proxy/winkernel",
    importpath = "k8s.io/kubernetes/pkg/proxy/winkernel",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/prometheus/client_golang/prometheus:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/github.com/davecgh/go-spew/spew:go_default_library",
            "//vendor/github.com/golang/glog:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
            "//vendor/k8s.io/client-go/tools/record:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/api/service:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/apis/core:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/apis/core/helper:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/proxy/healthcheck:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/async:go_default_library",
            "@com_github_microsoft_hcsshim//:go_default_library",
        ],
        "//conditions:default": [],
    }),
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
    visibility = ["//visibility:public"],
)
