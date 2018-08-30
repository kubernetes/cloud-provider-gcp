package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "mirror_client.go",
        "pod_manager.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/pod",
    importpath = "k8s.io/kubernetes/pkg/kubelet/pod",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/api/errors:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/checkpoint:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/checkpointmanager:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/configmap:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/secret:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/types:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "mirror_client_test.go",
        "pod_manager_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/configmap:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/pod/testing:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/secret:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/types:go_default_library",
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
    srcs = [
        ":package-srcs",
        "//pkg/kubelet/pod/testing:all-srcs",
    ],
    tags = ["automanaged"],
)
