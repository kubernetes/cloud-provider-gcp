load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "intstr",
    srcs = [
        "generated.pb.go",
        "instr_fuzz.go",
        "intstr.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/util/intstr",
    importpath = "k8s.io/apimachinery/pkg/util/intstr",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/gogo/protobuf/proto",
        "//vendor/github.com/google/gofuzz",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer/cbor/direct",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
