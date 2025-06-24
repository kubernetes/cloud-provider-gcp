load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "storagebackend",
    srcs = ["config.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/storagebackend",
    importpath = "k8s.io/apiserver/pkg/storage/storagebackend",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/trace",
        "//vendor/go.opentelemetry.io/otel/trace/noop",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apiserver/pkg/server/egressselector",
        "//vendor/k8s.io/apiserver/pkg/storage/etcd3",
        "//vendor/k8s.io/apiserver/pkg/storage/value",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/request",
    ],
)
