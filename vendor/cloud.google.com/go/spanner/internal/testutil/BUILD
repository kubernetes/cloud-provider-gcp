load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "mockclient.go",
        "mockserver.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/spanner/internal/testutil",
    importpath = "cloud.google.com/go/spanner/internal/testutil",
    visibility = ["//vendor/cloud.google.com/go/spanner:__subpackages__"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "@io_bazel_rules_go//proto/wkt:empty_go_proto",
        "@io_bazel_rules_go//proto/wkt:struct_go_proto",
        "@io_bazel_rules_go//proto/wkt:timestamp_go_proto",
        "@org_golang_google_genproto//googleapis/spanner/v1:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)
