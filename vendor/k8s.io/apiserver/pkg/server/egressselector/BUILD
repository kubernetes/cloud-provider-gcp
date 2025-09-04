load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "egressselector",
    srcs = [
        "config.go",
        "egress_selector.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/server/egressselector",
    importpath = "k8s.io/apiserver/pkg/server/egressselector",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/credentials/insecure",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer",
        "//vendor/k8s.io/apimachinery/pkg/util/net",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field",
        "//vendor/k8s.io/apiserver/pkg/apis/apiserver",
        "//vendor/k8s.io/apiserver/pkg/apis/apiserver/install",
        "//vendor/k8s.io/apiserver/pkg/server/egressselector/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
        "//vendor/k8s.io/component-base/tracing",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/path",
        "//vendor/sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client",
    ],
)
