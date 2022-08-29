package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)
load("//defs:version.bzl", "version_x_defs")

go_library(
    name = "go_default_library",
    srcs = ["gcp.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/clientauthplugin/gcp",
    deps = [
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/golang.org/x/oauth2/google:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/yaml:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/client-go/util/jsonpath:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["gcp_test.go"],
    embed = [":go_default_library"],
    deps = ["//vendor/golang.org/x/oauth2:go_default_library"],
)
