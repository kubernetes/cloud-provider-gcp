load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["conformance.pb.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/golang/protobuf/conformance/internal/conformance_proto",
    importpath = "github.com/golang/protobuf/conformance/internal/conformance_proto",
    visibility = ["//vendor/github.com/golang/protobuf/conformance:__subpackages__"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "@io_bazel_rules_go//proto/wkt:any_go_proto",
        "@io_bazel_rules_go//proto/wkt:duration_go_proto",
        "@io_bazel_rules_go//proto/wkt:struct_go_proto",
        "@io_bazel_rules_go//proto/wkt:timestamp_go_proto",
        "@io_bazel_rules_go//proto/wkt:wrappers_go_proto",
        "@org_golang_google_genproto//protobuf/field_mask:go_default_library",
    ],
)
