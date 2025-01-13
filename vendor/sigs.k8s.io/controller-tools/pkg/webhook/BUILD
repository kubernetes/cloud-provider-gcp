load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "webhook",
    srcs = [
        "parser.go",
        "zz_generated.markerhelp.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/controller-tools/pkg/webhook",
    importpath = "sigs.k8s.io/controller-tools/pkg/webhook",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/admissionregistration/v1:admissionregistration",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/sigs.k8s.io/controller-tools/pkg/genall",
        "//vendor/sigs.k8s.io/controller-tools/pkg/markers",
    ],
)
