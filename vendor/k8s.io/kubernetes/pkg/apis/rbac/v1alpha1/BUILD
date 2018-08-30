package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "conversion.go",
        "defaults.go",
        "doc.go",
        "helpers.go",
        "register.go",
        "zz_generated.conversion.go",
        "zz_generated.defaults.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/apis/rbac/v1alpha1",
    importpath = "k8s.io/kubernetes/pkg/apis/rbac/v1alpha1",
    deps = [
        "//vendor/k8s.io/api/rbac/v1alpha1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/conversion:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/rbac:go_default_library",
    ],
)

go_test(
    name = "go_default_xtest",
    srcs = ["conversion_test.go"],
    deps = [
        "//pkg/api/legacyscheme:go_default_library",
        "//pkg/apis/rbac:go_default_library",
        "//pkg/apis/rbac/install:go_default_library",
        "//vendor/k8s.io/api/rbac/v1alpha1:go_default_library",
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

go_test(
    name = "go_default_test",
    srcs = ["conversion_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/api/rbac/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/api/legacyscheme:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/rbac:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/rbac/install:go_default_library",
    ],
)
