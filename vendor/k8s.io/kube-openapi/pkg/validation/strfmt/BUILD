load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "strfmt",
    srcs = [
        "bson.go",
        "date.go",
        "default.go",
        "doc.go",
        "duration.go",
        "format.go",
        "kubernetes-extensions.go",
        "time.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/validation/strfmt",
    importpath = "k8s.io/kube-openapi/pkg/validation/strfmt",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/kube-openapi/pkg/internal/third_party/govalidator",
        "//vendor/k8s.io/kube-openapi/pkg/validation/errors",
        "//vendor/k8s.io/kube-openapi/pkg/validation/strfmt/bson",
        "//vendor/k8s.io/utils/net",
    ],
)
