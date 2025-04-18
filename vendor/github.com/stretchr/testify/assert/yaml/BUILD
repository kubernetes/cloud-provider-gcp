load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "yaml",
    srcs = ["yaml_default.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/stretchr/testify/assert/yaml",
    importpath = "github.com/stretchr/testify/assert/yaml",
    visibility = ["//visibility:public"],
    deps = ["//vendor/gopkg.in/yaml.v3:yaml_v3"],
)
