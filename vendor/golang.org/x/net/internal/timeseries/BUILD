load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "timeseries",
    srcs = ["timeseries.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/net/internal/timeseries",
    importpath = "golang.org/x/net/internal/timeseries",
    visibility = ["//vendor/golang.org/x/net:__subpackages__"],
)
