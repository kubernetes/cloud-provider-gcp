load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "spec3",
    srcs = [
        "component.go",
        "encoding.go",
        "example.go",
        "external_documentation.go",
        "fuzz.go",
        "header.go",
        "media_type.go",
        "operation.go",
        "parameter.go",
        "path.go",
        "request_body.go",
        "response.go",
        "security_scheme.go",
        "server.go",
        "spec.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/spec3",
    importpath = "k8s.io/kube-openapi/pkg/spec3",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/go-openapi/swag",
        "//vendor/k8s.io/kube-openapi/pkg/internal",
        "//vendor/k8s.io/kube-openapi/pkg/internal/third_party/go-json-experiment/json",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
        "//vendor/sigs.k8s.io/randfill",
    ],
)
