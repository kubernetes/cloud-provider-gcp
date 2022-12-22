load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "functions",
    srcs = [
        "functions.go",
        "standard.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/interpreter/functions",
    importpath = "github.com/google/cel-go/interpreter/functions",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/common/operators",
        "//vendor/github.com/google/cel-go/common/overloads",
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/github.com/google/cel-go/common/types/traits",
    ],
)
