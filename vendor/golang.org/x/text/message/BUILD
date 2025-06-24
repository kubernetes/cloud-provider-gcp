load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "message",
    srcs = [
        "catalog.go",
        "doc.go",
        "format.go",
        "message.go",
        "print.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/text/message",
    importpath = "golang.org/x/text/message",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/text/feature/plural",
        "//vendor/golang.org/x/text/internal/format",
        "//vendor/golang.org/x/text/internal/number",
        "//vendor/golang.org/x/text/language",
        "//vendor/golang.org/x/text/message/catalog",
    ],
)
