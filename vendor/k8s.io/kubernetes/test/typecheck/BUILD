package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
    "go_test",
)

go_binary(
    name = "list",
    embed = [":go_default_library"],
)

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/test/typecheck",
    importpath = "k8s.io/kubernetes/test/typecheck",
    deps = [
        "//vendor/golang.org/x/crypto/ssh/terminal:go_default_library",
        "//vendor/k8s.io/kubernetes/test/typecheck/srcimporter:go_default_library",
        "//vendor/k8s.io/kubernetes/third_party/forked/golang/go/types:go_default_library",
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
        "//test/typecheck/srcimporter:all-srcs",
    ],
    tags = ["automanaged"],
)

go_binary(
    name = "typecheck",
    embed = [":go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["main_test.go"],
    embed = [":go_default_library"],
)
