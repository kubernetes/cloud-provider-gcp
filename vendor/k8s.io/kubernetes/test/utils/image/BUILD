load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "image",
    srcs = [
        "csi_manifest.go",
        "manifest.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/test/utils/image",
    importpath = "k8s.io/kubernetes/test/utils/image",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.yaml.in/yaml/v2:yaml",
        "//vendor/k8s.io/apimachinery/pkg/util/yaml",
        "//vendor/k8s.io/kubernetes/test/e2e/testing-manifests",
    ],
)
