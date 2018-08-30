load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "equal.go",
        "fake.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/logging/internal/testing",
    importpath = "cloud.google.com/go/logging/internal/testing",
    visibility = ["//vendor/cloud.google.com/go/logging:__subpackages__"],
    deps = [
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "@io_bazel_rules_go//proto/wkt:empty_go_proto",
        "@io_bazel_rules_go//proto/wkt:timestamp_go_proto",
        "@org_golang_google_genproto//googleapis/api/label:go_default_library",
        "@org_golang_google_genproto//googleapis/api/monitoredres:go_default_library",
        "@org_golang_google_genproto//googleapis/logging/v2:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["fake_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "@io_bazel_rules_go//proto/wkt:timestamp_go_proto",
        "@org_golang_google_genproto//googleapis/logging/v2:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)
