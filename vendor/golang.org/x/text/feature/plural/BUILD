load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "plural",
    srcs = [
        "common.go",
        "message.go",
        "plural.go",
        "tables.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/text/feature/plural",
    importpath = "golang.org/x/text/feature/plural",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/text/internal/catmsg",
        "//vendor/golang.org/x/text/internal/language/compact",
        "//vendor/golang.org/x/text/internal/number",
        "//vendor/golang.org/x/text/language",
        "//vendor/golang.org/x/text/message/catalog",
    ],
)
