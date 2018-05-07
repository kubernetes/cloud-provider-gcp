load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "generated.pb.go",
        "intstr.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apimachinery/pkg/util/intstr",
    importpath = "k8s.io/apimachinery/pkg/util/intstr",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/go-openapi/spec:go_default_library",
        "//vendor/github.com/gogo/protobuf/proto:go_default_library",
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/github.com/google/gofuzz:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/common:go_default_library",
    ],
)
