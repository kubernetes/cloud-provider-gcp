load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "meta",
    srcs = [
        "doc.go",
        "key.go",
        "meta.go",
        "method.go",
        "service.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta",
    importpath = "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/api/compute/v0.alpha:v0_alpha",
        "//vendor/google.golang.org/api/compute/v0.beta:v0_beta",
        "//vendor/google.golang.org/api/compute/v1:compute",
    ],
)
