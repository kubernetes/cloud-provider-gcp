package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)
load("//defs:version.bzl", "version_x_defs")

go_binary(
    name = "kubectl-gke-exec-auth-plugin",
    embed = [":go_default_library"],
)

go_library(
    name = "go_default_library",
    srcs = [
        "main.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/cmd/kubectl-gke-exec-auth-plugin",
    deps = [
        "//cmd/kubectl-gke-exec-auth-plugin/provider:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/golang.org/x/oauth2/google:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/client-go/pkg/apis/clientauthentication:go_default_library",
        "//vendor/k8s.io/client-go/pkg/apis/clientauthentication/v1beta1:go_default_library",
    ],
)
