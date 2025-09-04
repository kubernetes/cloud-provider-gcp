load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "common",
    srcs = [
        "adaptor.go",
        "equality.go",
        "maplist.go",
        "schemas.go",
        "typeprovider.go",
        "values.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/cel/common",
    importpath = "k8s.io/apiserver/pkg/cel/common",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/cel",
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/github.com/google/cel-go/common/types/traits",
        "//vendor/k8s.io/apimachinery/pkg/api/equality",
        "//vendor/k8s.io/apiserver/pkg/cel",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
        "//vendor/k8s.io/kube-openapi/pkg/validation/strfmt",
    ],
)
