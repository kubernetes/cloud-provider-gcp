load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "etcd3.go",
        "factory.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/storagebackend/factory",
    importpath = "k8s.io/apiserver/pkg/storage/storagebackend/factory",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/grpc-ecosystem/go-grpc-prometheus:go_default_library",
        "//vendor/go.etcd.io/etcd/client/pkg/v3/transport:go_default_library",
        "//vendor/go.etcd.io/etcd/client/v3:go_default_library",
        "//vendor/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc:go_default_library",
        "//vendor/google.golang.org/grpc:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/features:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/server/egressselector:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/etcd3:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/etcd3/metrics:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/storagebackend:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/value:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
        "//vendor/k8s.io/component-base/metrics/legacyregistry:go_default_library",
        "//vendor/k8s.io/component-base/traces:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
    ],
)
