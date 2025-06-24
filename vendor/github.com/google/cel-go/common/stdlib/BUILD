load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "stdlib",
    srcs = ["standard.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/common/stdlib",
    importpath = "github.com/google/cel-go/common/stdlib",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/common/decls",
        "//vendor/github.com/google/cel-go/common/functions",
        "//vendor/github.com/google/cel-go/common/operators",
        "//vendor/github.com/google/cel-go/common/overloads",
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/github.com/google/cel-go/common/types/traits",
    ],
)
