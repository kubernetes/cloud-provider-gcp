package(default_visibility = ["//visibility:public"])

licenses(["notice"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "endpoint.go",
        "endpoint_windows.go",
        "fake_image_service.go",
        "fake_runtime.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/remote/fake",
    importpath = "k8s.io/kubernetes/pkg/kubelet/remote/fake",
    tags = ["automanaged"],
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/cri/testing:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/util:go_default_library",
        "//vendor/k8s.io/utils/exec:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
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
