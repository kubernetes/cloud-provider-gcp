load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "openapi",
    srcs = [
        "client.go",
        "groupversion.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/openapi",
    importpath = "k8s.io/client-go/openapi",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/gnostic/openapiv3",
        "//vendor/google.golang.org/protobuf/proto",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/kube-openapi/pkg/handler3",
    ],
)
