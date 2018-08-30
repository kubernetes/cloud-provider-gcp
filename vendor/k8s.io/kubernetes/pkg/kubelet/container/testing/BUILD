load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "fake_cache.go",
        "fake_runtime.go",
        "fake_runtime_helper.go",
        "mock_runtime_cache.go",
        "mockfileinfo.go",
        "os.go",
        "runtime_mock.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/container/testing",
    importpath = "k8s.io/kubernetes/pkg/kubelet/container/testing",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/client-go/tools/remotecommand:go_default_library",
        "//vendor/k8s.io/client-go/util/flowcontrol:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/volume:go_default_library",
        "@com_github_golang_mock//gomock:go_default_library",
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
