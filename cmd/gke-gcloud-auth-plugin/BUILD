package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)
load("//defs:version.bzl", "version_x_defs")

go_binary(
    name = "gke-gcloud-auth-plugin",
    embed = [":go_default_library"],
    pure = "on",
    x_defs = version_x_defs(),
)

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin",
    deps = [
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/golang.org/x/oauth2/google:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/client-go/pkg/apis/clientauthentication/v1beta1:go_default_library",
        "//vendor/k8s.io/component-base/version/verflag:go_default_library",
    ],
)
