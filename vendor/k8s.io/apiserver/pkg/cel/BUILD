load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cel",
    srcs = [
        "cidr.go",
        "errors.go",
        "escaping.go",
        "format.go",
        "ip.go",
        "limits.go",
        "quantity.go",
        "semver.go",
        "types.go",
        "url.go",
        "value.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/cel",
    importpath = "k8s.io/apiserver/pkg/cel",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/blang/semver/v4:semver",
        "//vendor/github.com/google/cel-go/cel",
        "//vendor/github.com/google/cel-go/checker/decls",
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/github.com/google/cel-go/common/types/traits",
        "//vendor/google.golang.org/genproto/googleapis/api/expr/v1alpha1",
        "//vendor/k8s.io/apimachinery/pkg/api/resource",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apiserver/pkg/apis/cel",
    ],
)
