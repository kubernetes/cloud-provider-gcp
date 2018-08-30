load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "logging.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/logging",
    importpath = "cloud.google.com/go/logging",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/cloud.google.com/go/compute/metadata:go_default_library",
        "//vendor/cloud.google.com/go/internal/version:go_default_library",
        "//vendor/cloud.google.com/go/logging/apiv2:go_default_library",
        "//vendor/cloud.google.com/go/logging/internal:go_default_library",
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/github.com/golang/protobuf/ptypes:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "//vendor/google.golang.org/api/support/bundler:go_default_library",
        "@io_bazel_rules_go//proto/wkt:struct_go_proto",
        "@io_bazel_rules_go//proto/wkt:timestamp_go_proto",
        "@org_golang_google_genproto//googleapis/api/monitoredres:go_default_library",
        "@org_golang_google_genproto//googleapis/logging/type:go_default_library",
        "@org_golang_google_genproto//googleapis/logging/v2:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "examples_test.go",
        "logging_test.go",
        "logging_unexported_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/internal:go_default_library",
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "//vendor/cloud.google.com/go/internal/uid:go_default_library",
        "//vendor/cloud.google.com/go/logging/internal/testing:go_default_library",
        "//vendor/cloud.google.com/go/logging/logadmin:go_default_library",
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/google.golang.org/api/iterator:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "//vendor/google.golang.org/api/support/bundler:go_default_library",
        "@com_github_googleapis_gax_go//:go_default_library",
        "@io_bazel_rules_go//proto/wkt:duration_go_proto",
        "@io_bazel_rules_go//proto/wkt:struct_go_proto",
        "@org_golang_google_genproto//googleapis/api/monitoredres:go_default_library",
        "@org_golang_google_genproto//googleapis/logging/type:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)
