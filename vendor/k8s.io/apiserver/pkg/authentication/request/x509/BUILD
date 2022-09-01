load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "x509",
    srcs = [
        "doc.go",
        "verify_options.go",
        "x509.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authentication/request/x509",
    importpath = "k8s.io/apiserver/pkg/authentication/request/x509",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/errors",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apiserver/pkg/authentication/authenticator",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/client-go/util/cert",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
    ],
)
