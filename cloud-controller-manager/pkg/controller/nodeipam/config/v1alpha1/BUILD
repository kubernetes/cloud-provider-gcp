load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1alpha1",
    srcs = [
        "conversion.go",
        "defaults.go",
        "doc.go",
        "register.go",
        "zz_generated.conversion.go",
        "zz_generated.deepcopy.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/cloud-controller-manager/pkg/controller/nodeipam/config/v1alpha1",
    visibility = ["//visibility:public"],
    deps = [
        "//cloud-controller-manager/pkg/controller/nodeipam/config",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/conversion",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/kube-controller-manager/config/v1alpha1",
    ],
)
