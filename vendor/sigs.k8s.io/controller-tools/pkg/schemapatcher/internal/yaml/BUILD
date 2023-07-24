load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "yaml",
    srcs = [
        "convert.go",
        "nested.go",
        "set.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/controller-tools/pkg/schemapatcher/internal/yaml",
    importpath = "sigs.k8s.io/controller-tools/pkg/schemapatcher/internal/yaml",
    visibility = ["//vendor/sigs.k8s.io/controller-tools/pkg/schemapatcher:__subpackages__"],
    deps = ["//vendor/gopkg.in/yaml.v3:yaml_v3"],
)
