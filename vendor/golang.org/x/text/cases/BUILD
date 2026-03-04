load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cases",
    srcs = [
        "cases.go",
        "context.go",
        "fold.go",
        "info.go",
        "map.go",
        "tables15.0.0.go",
        "tables17.0.0.go",
        "trieval.go",
    ],
    cgo = True,
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/text/cases",
    importpath = "golang.org/x/text/cases",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/text/internal",
        "//vendor/golang.org/x/text/language",
        "//vendor/golang.org/x/text/transform",
        "//vendor/golang.org/x/text/unicode/norm",
    ],
)
