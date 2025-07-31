load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "httputil",
    srcs = ["negotiate.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/prometheus/client_golang/internal/github.com/golang/gddo/httputil",
    importpath = "github.com/prometheus/client_golang/internal/github.com/golang/gddo/httputil",
    visibility = ["//vendor/github.com/prometheus/client_golang:__subpackages__"],
    deps = ["//vendor/github.com/prometheus/client_golang/internal/github.com/golang/gddo/httputil/header"],
)
