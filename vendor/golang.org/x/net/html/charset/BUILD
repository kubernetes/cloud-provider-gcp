load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "charset",
    srcs = ["charset.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/net/html/charset",
    importpath = "golang.org/x/net/html/charset",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/net/html",
        "//vendor/golang.org/x/text/encoding",
        "//vendor/golang.org/x/text/encoding/charmap",
        "//vendor/golang.org/x/text/encoding/htmlindex",
        "//vendor/golang.org/x/text/transform",
    ],
)
