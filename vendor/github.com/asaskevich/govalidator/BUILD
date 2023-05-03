load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "govalidator",
    srcs = [
        "arrays.go",
        "converter.go",
        "error.go",
        "numerics.go",
        "patterns.go",
        "types.go",
        "utils.go",
        "validator.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/asaskevich/govalidator",
    importpath = "github.com/asaskevich/govalidator",
    visibility = ["//visibility:public"],
)
