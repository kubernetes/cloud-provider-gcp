load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "filestore.go",
        "store.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/util/store",
    importpath = "k8s.io/kubernetes/pkg/kubelet/util/store",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/kubernetes/pkg/util/filesystem:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "filestore_test.go",
        "store_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/util/filesystem:go_default_library",
        "@com_github_stretchr_testify//assert:go_default_library",
        "@com_github_stretchr_testify//require:go_default_library",
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
