load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "etcd_util.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apiserver/pkg/storage/etcd/util",
    importpath = "k8s.io/apiserver/pkg/storage/etcd/util",
    visibility = ["//visibility:public"],
    deps = ["@com_github_coreos_etcd//client:go_default_library"],
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
