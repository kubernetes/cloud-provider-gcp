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
        "netsh.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/util/netsh",
    importpath = "k8s.io/kubernetes/pkg/util/netsh",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/utils/exec:go_default_library",
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
        "//pkg/util/netsh/testing:all-srcs",
    ],
    tags = ["automanaged"],
)

go_test(
    name = "go_default_test",
    srcs = ["netsh_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/utils/exec:go_default_library",
        "//vendor/k8s.io/utils/exec/testing:go_default_library",
        "@com_github_pkg_errors//:go_default_library",
        "@com_github_stretchr_testify//assert:go_default_library",
    ],
)
