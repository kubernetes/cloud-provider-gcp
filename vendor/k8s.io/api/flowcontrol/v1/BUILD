load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "flowcontrol",
    srcs = [
        "doc.go",
        "generated.pb.go",
        "register.go",
        "types.go",
        "types_swagger_doc_generated.go",
        "zz_generated.deepcopy.go",
        "zz_generated.prerelease-lifecycle.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/api/flowcontrol/v1",
    importpath = "k8s.io/api/flowcontrol/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/gogo/protobuf/proto",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
    ],
)
