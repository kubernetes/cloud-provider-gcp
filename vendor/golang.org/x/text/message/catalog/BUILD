load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "catalog",
    srcs = [
        "catalog.go",
        "dict.go",
        "go19.go",
        "gopre19.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/text/message/catalog",
    importpath = "golang.org/x/text/message/catalog",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/text/internal",
        "//vendor/golang.org/x/text/internal/catmsg",
        "//vendor/golang.org/x/text/language",
    ],
)
