load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "proto",
    srcs = [
        "doc.go",
        "document.go",
        "document_v3.go",
        "openapi.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/util/proto",
    importpath = "k8s.io/kube-openapi/pkg/util/proto",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/gnostic-models/openapiv2",
        "//vendor/github.com/google/gnostic-models/openapiv3",
        "//vendor/go.yaml.in/yaml/v2:yaml",
        "//vendor/go.yaml.in/yaml/v3:yaml",
    ],
)
