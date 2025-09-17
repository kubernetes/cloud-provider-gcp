load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "certificates",
    srcs = [
        "conversion.go",
        "defaults.go",
        "doc.go",
        "helpers.go",
        "register.go",
        "zz_generated.conversion.go",
        "zz_generated.defaults.go",
        "zz_generated.validations.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/apis/certificates/v1",
    importpath = "k8s.io/kubernetes/pkg/apis/certificates/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/certificates/v1:certificates",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/equality",
        "//vendor/k8s.io/apimachinery/pkg/api/operation",
        "//vendor/k8s.io/apimachinery/pkg/api/safe",
        "//vendor/k8s.io/apimachinery/pkg/api/validate",
        "//vendor/k8s.io/apimachinery/pkg/conversion",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field",
        "//vendor/k8s.io/kubernetes/pkg/apis/certificates",
        "//vendor/k8s.io/kubernetes/pkg/apis/core",
    ],
)
