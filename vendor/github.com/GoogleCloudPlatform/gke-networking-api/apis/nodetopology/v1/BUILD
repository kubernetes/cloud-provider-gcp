load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "nodetopology",
    srcs = [
        "doc.go",
        "types.go",
        "zz_generated.deepcopy.go",
        "zz_generated.register.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/gke-networking-api/apis/nodetopology/v1",
    importpath = "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodetopology/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
    ],
)
