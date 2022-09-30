load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cache",
    srcs = [
        "cache_simple.go",
        "cache_striped.go",
        "cached_token_authenticator.go",
        "stats.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authentication/token/cache",
    importpath = "k8s.io/apiserver/pkg/authentication/token/cache",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/sync/singleflight",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/util/cache",
        "//vendor/k8s.io/apiserver/pkg/apis/audit",
        "//vendor/k8s.io/apiserver/pkg/audit",
        "//vendor/k8s.io/apiserver/pkg/authentication/authenticator",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/clock",
    ],
)
