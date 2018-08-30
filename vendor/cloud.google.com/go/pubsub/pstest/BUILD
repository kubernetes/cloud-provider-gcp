load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["fake.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/pubsub/pstest",
    importpath = "cloud.google.com/go/pubsub/pstest",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "//vendor/github.com/golang/protobuf/ptypes:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "@io_bazel_rules_go//proto/wkt:duration_go_proto",
        "@io_bazel_rules_go//proto/wkt:empty_go_proto",
        "@org_golang_google_genproto//googleapis/pubsub/v1:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "examples_test.go",
        "fake_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "//vendor/cloud.google.com/go/pubsub:go_default_library",
        "//vendor/github.com/golang/protobuf/ptypes:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "@org_golang_google_genproto//googleapis/pubsub/v1:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)
