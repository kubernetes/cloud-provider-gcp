package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["config.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubeapiserver/authenticator",
    importpath = "k8s.io/kubernetes/pkg/kubeapiserver/authenticator",
    deps = [
        "//vendor/k8s.io/apiserver/pkg/authentication/authenticator:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/authenticatorfactory:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/group:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/anonymous:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/bearertoken:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/headerrequest:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/union:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/websocket:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/request/x509:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/token/cache:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/token/tokenfile:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/token/union:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
        "//vendor/k8s.io/apiserver/plugin/pkg/authenticator/password/passwordfile:go_default_library",
        "//vendor/k8s.io/apiserver/plugin/pkg/authenticator/request/basicauth:go_default_library",
        "//vendor/k8s.io/apiserver/plugin/pkg/authenticator/token/oidc:go_default_library",
        "//vendor/k8s.io/apiserver/plugin/pkg/authenticator/token/webhook:go_default_library",
        "//vendor/k8s.io/client-go/plugin/pkg/client/auth:go_default_library",
        "//vendor/k8s.io/client-go/util/cert:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/features:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/serviceaccount:go_default_library",
        "@com_github_go_openapi_spec//:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
)
