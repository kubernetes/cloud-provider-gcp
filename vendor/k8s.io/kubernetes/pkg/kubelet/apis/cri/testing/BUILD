package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "fake_image_service.go",
        "fake_runtime_service.go",
        "utils.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/apis/cri/testing",
    importpath = "k8s.io/kubernetes/pkg/kubelet/apis/cri/testing",
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/util/sliceutils:go_default_library",
        "@com_github_stretchr_testify//assert:go_default_library",
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
