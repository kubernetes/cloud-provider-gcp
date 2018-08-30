load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["checkpoint.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/checkpoint",
    importpath = "k8s.io/kubernetes/pkg/kubelet/checkpoint",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/checkpointmanager:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/checkpointmanager/checksum:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["checkpoint_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/checkpointmanager:go_default_library",
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
