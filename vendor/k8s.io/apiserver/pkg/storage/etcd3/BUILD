load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "etcd3",
    srcs = [
        "compact.go",
        "errors.go",
        "event.go",
        "healthcheck.go",
        "latency_tracker.go",
        "lease_manager.go",
        "logger.go",
        "store.go",
        "watcher.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/etcd3",
    importpath = "k8s.io/apiserver/pkg/storage/etcd3",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.etcd.io/etcd/api/v3/mvccpb",
        "//vendor/go.etcd.io/etcd/api/v3/v3rpc/rpctypes",
        "//vendor/go.etcd.io/etcd/client/v3:client",
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/grpclog",
        "//vendor/google.golang.org/grpc/status",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured",
        "//vendor/k8s.io/apimachinery/pkg/conversion",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/apiserver/pkg/audit",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request",
        "//vendor/k8s.io/apiserver/pkg/features",
        "//vendor/k8s.io/apiserver/pkg/storage",
        "//vendor/k8s.io/apiserver/pkg/storage/etcd3/metrics",
        "//vendor/k8s.io/apiserver/pkg/storage/feature",
        "//vendor/k8s.io/apiserver/pkg/storage/value",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol",
        "//vendor/k8s.io/component-base/tracing",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
