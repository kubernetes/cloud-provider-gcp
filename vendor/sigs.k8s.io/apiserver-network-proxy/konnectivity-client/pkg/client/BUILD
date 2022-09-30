load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "client",
    srcs = [
        "client.go",
        "conn.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client",
    importpath = "sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/sigs.k8s.io/apiserver-network-proxy/konnectivity-client/proto/client",
    ],
)
