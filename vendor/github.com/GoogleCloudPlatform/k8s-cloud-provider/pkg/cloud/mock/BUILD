load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "mock",
    srcs = ["mock.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/mock",
    importpath = "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/mock",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud",
        "//vendor/github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/filter",
        "//vendor/github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta",
        "//vendor/google.golang.org/api/compute/v0.alpha:v0_alpha",
        "//vendor/google.golang.org/api/compute/v0.beta:v0_beta",
        "//vendor/google.golang.org/api/compute/v1:compute",
        "//vendor/google.golang.org/api/googleapi",
    ],
)
