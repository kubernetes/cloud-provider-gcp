load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["utils.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/cmd/kubeadm/app/util/audit",
    importpath = "k8s.io/kubernetes/cmd/kubeadm/app/util/audit",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/apis/audit/install:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/apis/audit/v1beta1:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/util:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["utils_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/apis/audit/install:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/apis/audit/v1beta1:go_default_library",
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
