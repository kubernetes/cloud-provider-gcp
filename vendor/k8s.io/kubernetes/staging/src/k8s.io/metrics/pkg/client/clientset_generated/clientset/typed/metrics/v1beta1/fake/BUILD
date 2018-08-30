load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "fake_metrics_client.go",
        "fake_nodemetrics.go",
        "fake_podmetrics.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/metrics/pkg/client/clientset_generated/clientset/typed/metrics/v1beta1/fake",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/metrics/pkg/client/clientset_generated/clientset/typed/metrics/v1beta1/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/labels:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/watch:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/client-go/testing:go_default_library",
        "@io_k8s_metrics//pkg/apis/metrics/v1beta1:go_default_library",
        "@io_k8s_metrics//pkg/client/clientset_generated/clientset/typed/metrics/v1beta1:go_default_library",
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
