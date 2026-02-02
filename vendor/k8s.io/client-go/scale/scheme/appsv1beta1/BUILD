load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "appsv1beta1",
    srcs = [
        "conversion.go",
        "doc.go",
        "register.go",
        "zz_generated.conversion.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/scale/scheme/appsv1beta1",
    importpath = "k8s.io/client-go/scale/scheme/appsv1beta1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/apps/v1beta1",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/conversion",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/client-go/scale/scheme",
    ],
)
