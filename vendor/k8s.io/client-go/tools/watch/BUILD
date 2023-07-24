load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "watch",
    srcs = [
        "informerwatcher.go",
        "retrywatcher.go",
        "until.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/tools/watch",
    importpath = "k8s.io/client-go/tools/watch",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/davecgh/go-spew/spew",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/net",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
