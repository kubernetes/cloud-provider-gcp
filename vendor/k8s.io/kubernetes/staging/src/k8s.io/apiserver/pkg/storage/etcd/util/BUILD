package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_test(
    name = "go_default_test",
    srcs = ["etcd_util_test.go"],
    embed = [":go_default_library"],
    deps = [
        "@com_github_coreos_etcd//client:go_default_library",
        "@com_github_stretchr_testify//assert:go_default_library",
    ],
)

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "etcd_util.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/storage/etcd/util",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/storage/etcd/util",
    deps = ["@com_github_coreos_etcd//client:go_default_library"],
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
