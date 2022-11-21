load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "v1",
    srcs = [
        "annotations.go",
        "doc.go",
        "gkenetworkparams_types.go",
        "network.go",
        "network_types.go",
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
    name = "v1_test",
    srcs = ["annotations_test.go"],
    embed = [":v1"],
    deps = [
        "//vendor/github.com/google/go-cmp/cmp",
    ],
)
