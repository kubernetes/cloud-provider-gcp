package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)
load("//defs:version.bzl", "version_x_defs")

go_library(
    name = "provider",
    srcs = ["provider.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/provider",
    deps = [
        "//pkg/credentialconfig",
        "//pkg/gcpcredential",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/kubelet/pkg/apis/credentialprovider/v1alpha1",
    ],
)

go_test(
    name = "provider_test",
    srcs = ["provider_test.go"],
    embed = [":provider"],
    deps = [
        "//pkg/gcpcredential",
        "//vendor/k8s.io/apimachinery/pkg/util/net",
        "//vendor/k8s.io/kubelet/pkg/apis/credentialprovider/v1alpha1",
    ],
)
