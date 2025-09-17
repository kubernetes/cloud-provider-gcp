load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1alpha3",
    srcs = [
        "celdeviceselector.go",
        "deviceselector.go",
        "devicetaint.go",
        "devicetaintrule.go",
        "devicetaintrulespec.go",
        "devicetaintselector.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/applyconfigurations/resource/v1alpha3",
    importpath = "k8s.io/client-go/applyconfigurations/resource/v1alpha3",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/resource/v1alpha3",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/managedfields",
        "//vendor/k8s.io/client-go/applyconfigurations/internal",
        "//vendor/k8s.io/client-go/applyconfigurations/meta/v1:meta",
    ],
)
