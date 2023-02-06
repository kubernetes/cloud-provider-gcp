load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1beta1",
    srcs = [
        "expansion_generated.go",
        "mutatingwebhookconfiguration.go",
        "validatingwebhookconfiguration.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/listers/admissionregistration/v1beta1",
    importpath = "k8s.io/client-go/listers/admissionregistration/v1beta1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/admissionregistration/v1beta1",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
