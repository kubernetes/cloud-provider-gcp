load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["mock_stats_provider.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/server/stats/testing",
    importpath = "k8s.io/kubernetes/pkg/kubelet/server/stats/testing",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/volume:go_default_library",
        "@com_github_google_cadvisor//info/v1:go_default_library",
        "@com_github_stretchr_testify//mock:go_default_library",
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
    visibility = ["//visibility:public"],
)
