load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "container",
    srcs = ["container-gen.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/api/container/v1",
    importpath = "google.golang.org/api/container/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/api/googleapi",
        "//vendor/google.golang.org/api/internal",
        "//vendor/google.golang.org/api/internal/gensupport",
        "//vendor/google.golang.org/api/option",
        "//vendor/google.golang.org/api/option/internaloption",
        "//vendor/google.golang.org/api/transport/http",
    ],
)
