load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "authorizerfactory",
    srcs = [
        "builtin.go",
        "delegating.go",
        "metrics.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authorization/authorizerfactory",
    importpath = "k8s.io/apiserver/pkg/authorization/authorizerfactory",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer",
        "//vendor/k8s.io/apiserver/pkg/authorization/cel",
        "//vendor/k8s.io/apiserver/plugin/pkg/authorizer/webhook",
        "//vendor/k8s.io/apiserver/plugin/pkg/authorizer/webhook/metrics",
        "//vendor/k8s.io/client-go/kubernetes/typed/authorization/v1:authorization",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
    ],
)
