load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go-grpc-prometheus",
    srcs = [
        "client.go",
        "client_metrics.go",
        "client_reporter.go",
        "metric_options.go",
        "server.go",
        "server_metrics.go",
        "server_reporter.go",
        "util.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/grpc-ecosystem/go-grpc-prometheus",
    importpath = "github.com/grpc-ecosystem/go-grpc-prometheus",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/prometheus/client_golang/prometheus",
        "//vendor/golang.org/x/net/context",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/status",
    ],
)
