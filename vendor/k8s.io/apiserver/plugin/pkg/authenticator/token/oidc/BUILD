load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["oidc.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apiserver/plugin/pkg/authenticator/token/oidc",
    importpath = "k8s.io/apiserver/plugin/pkg/authenticator/token/oidc",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/user:go_default_library",
        "//vendor/k8s.io/client-go/util/cert:go_default_library",
        "@com_github_coreos_go_oidc//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["oidc_test.go"],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/gopkg.in/square/go-jose.v2:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/authentication/user:go_default_library",
        "@com_github_coreos_go_oidc//:go_default_library",
    ],
)
