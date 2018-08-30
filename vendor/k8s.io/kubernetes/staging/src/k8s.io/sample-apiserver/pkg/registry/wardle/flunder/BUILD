package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "etcd.go",
        "strategy.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/sample-apiserver/pkg/registry/wardle/flunder",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/sample-apiserver/pkg/registry/wardle/flunder",
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/fields:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/labels:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/registry/generic:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/registry/generic/registry:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/names:go_default_library",
        "@io_k8s_sample_apiserver//pkg/apis/wardle:go_default_library",
        "@io_k8s_sample_apiserver//pkg/apis/wardle/validation:go_default_library",
        "@io_k8s_sample_apiserver//pkg/registry:go_default_library",
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
