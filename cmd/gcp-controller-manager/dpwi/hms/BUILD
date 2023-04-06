load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "hms",
    srcs = ["hms.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/hms",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer",
        "//vendor/k8s.io/apiserver/pkg/server/options",
        "//vendor/k8s.io/apiserver/pkg/util/webhook",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/tools/clientcmd/api",
    ],
)
