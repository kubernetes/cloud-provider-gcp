load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "apiserver",
    srcs = [
        "defaults.go",
        "doc.go",
        "register.go",
        "types.go",
        "types_encryption.go",
        "zz_generated.conversion.go",
        "zz_generated.deepcopy.go",
        "zz_generated.defaults.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/apis/apiserver/v1",
    importpath = "k8s.io/apiserver/pkg/apis/apiserver/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/conversion",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apiserver/pkg/apis/apiserver",
        "//vendor/k8s.io/component-base/tracing/api/v1:api",
        "//vendor/k8s.io/utils/ptr",
    ],
)
