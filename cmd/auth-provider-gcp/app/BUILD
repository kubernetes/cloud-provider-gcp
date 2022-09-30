package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)
load("//defs:version.bzl", "version_x_defs")

go_library(
    name = "app",
    srcs = ["getcredentials.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/app",
    deps = [
        "//cmd/auth-provider-gcp/provider",
        "//pkg/credentialconfig",
        "//vendor/github.com/spf13/cobra",
        "//vendor/k8s.io/apimachinery/pkg/util/net",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/kubelet/pkg/apis/credentialprovider/v1alpha1",
    ],
)

go_test(
    name = "app_test",
    srcs = ["getcredentials_test.go"],
    embed = [":app"],
)
