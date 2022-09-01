load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cloud",
    srcs = [
        "constants.go",
        "context.go",
        "doc.go",
        "gce_projects.go",
        "gen.go",
        "op.go",
        "project.go",
        "ratelimit.go",
        "service.go",
        "utils.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud",
    importpath = "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/filter",
        "//vendor/github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta",
        "//vendor/google.golang.org/api/compute/v0.alpha:v0_alpha",
        "//vendor/google.golang.org/api/compute/v0.beta:v0_beta",
        "//vendor/google.golang.org/api/compute/v1:compute",
        "//vendor/google.golang.org/api/googleapi",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
