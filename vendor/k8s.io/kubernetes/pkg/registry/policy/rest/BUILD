package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["storage_policy.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/registry/policy/rest",
    importpath = "k8s.io/kubernetes/pkg/registry/policy/rest",
    deps = [
        "//vendor/k8s.io/api/policy/v1beta1:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/registry/generic:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/registry/rest:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/server:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/server/storage:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/api/legacyscheme:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/policy:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/registry/policy/poddisruptionbudget/storage:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/registry/policy/podsecuritypolicy/storage:go_default_library",
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
