load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "network",
    srcs = [
        "annotations.go",
        "doc.go",
        "network.go",
        "network_types.go",
        "network_validation_interface.go",
        "networkinterface_types.go",
        "zz_generated.deepcopy.go",
        "zz_generated.register.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/crd/apis/network/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
    ],
)

go_test(
    name = "network_test",
    srcs = ["annotations_test.go"],
    embed = [":network"],
    deps = ["//vendor/github.com/google/go-cmp/cmp"],
)
