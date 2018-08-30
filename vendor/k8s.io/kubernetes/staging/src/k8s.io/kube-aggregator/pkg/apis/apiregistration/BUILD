package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "helpers.go",
        "register.go",
        "types.go",
        "zz_generated.deepcopy.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator/pkg/apis/apiregistration",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator/pkg/apis/apiregistration",
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/version:go_default_library",
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
        "//staging/src/k8s.io/kube-aggregator/pkg/apis/apiregistration/install:all-srcs",
        "//staging/src/k8s.io/kube-aggregator/pkg/apis/apiregistration/v1:all-srcs",
        "//staging/src/k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1:all-srcs",
        "//staging/src/k8s.io/kube-aggregator/pkg/apis/apiregistration/validation:all-srcs",
    ],
    tags = ["automanaged"],
)

go_test(
    name = "go_default_test",
    srcs = ["helpers_test.go"],
    embed = [":go_default_library"],
)
