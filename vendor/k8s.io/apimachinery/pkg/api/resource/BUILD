package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "amount.go",
        "generated.pb.go",
        "math.go",
        "quantity.go",
        "quantity_proto.go",
        "scale_int.go",
        "suffix.go",
        "zz_generated.deepcopy.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apimachinery/pkg/api/resource",
    importpath = "k8s.io/apimachinery/pkg/api/resource",
    deps = [
        "//vendor/github.com/go-openapi/spec:go_default_library",
        "//vendor/github.com/gogo/protobuf/proto:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/gopkg.in/inf.v0:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/common:go_default_library",
    ],
)

go_test(
    name = "go_default_xtest",
    srcs = ["quantity_example_test.go"],
    deps = ["//vendor/k8s.io/apimachinery/pkg/api/resource:go_default_library"],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
)

filegroup(
    name = "go_default_library_protos",
    srcs = ["generated.proto"],
    visibility = ["//visibility:public"],
)
