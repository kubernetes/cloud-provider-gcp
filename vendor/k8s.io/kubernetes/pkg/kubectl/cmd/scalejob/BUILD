load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "scalejob.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubectl/cmd/scalejob",
    importpath = "k8s.io/kubernetes/pkg/kubectl/cmd/scalejob",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/batch:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/batch/internalversion:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["scalejob_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/client-go/testing:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/batch:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/batch/internalversion:go_default_library",
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
