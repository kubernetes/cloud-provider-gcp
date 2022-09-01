load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "authenticatorfactory",
    srcs = [
        "delegating.go",
        "loopback.go",
        "metrics.go",
        "requestheader.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authentication/authenticatorfactory",
    importpath = "k8s.io/apiserver/pkg/authentication/authenticatorfactory",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/authentication/authenticator",
        "//vendor/k8s.io/apiserver/pkg/authentication/group",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/anonymous",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/bearertoken",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/headerrequest",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/union",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/websocket",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/x509",
        "//vendor/k8s.io/apiserver/pkg/authentication/token/cache",
        "//vendor/k8s.io/apiserver/pkg/authentication/token/tokenfile",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/apiserver/pkg/server/dynamiccertificates",
        "//vendor/k8s.io/apiserver/plugin/pkg/authenticator/token/webhook",
        "//vendor/k8s.io/client-go/kubernetes/typed/authentication/v1:authentication",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
    ],
)
