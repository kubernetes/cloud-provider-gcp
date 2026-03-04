load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "bidirule",
    srcs = ["bidirule.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/text/secure/bidirule",
    importpath = "golang.org/x/text/secure/bidirule",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/text/transform",
        "//vendor/golang.org/x/text/unicode/bidi",
    ],
)
