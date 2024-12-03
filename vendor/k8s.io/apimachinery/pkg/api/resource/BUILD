load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "resource",
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
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/api/resource",
    importpath = "k8s.io/apimachinery/pkg/api/resource",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/gogo/protobuf/proto",
        "//vendor/gopkg.in/inf.v0:inf_v0",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer/cbor/direct",
    ],
)
