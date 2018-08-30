load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "iam_client.go",
        "path_funcs.go",
        "policy_methods.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/iam/admin/apiv1",
    importpath = "cloud.google.com/go/iam/admin/apiv1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/cloud.google.com/go/iam:go_default_library",
        "//vendor/cloud.google.com/go/internal/version:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/iterator:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "//vendor/google.golang.org/api/transport:go_default_library",
        "@com_github_googleapis_gax_go//:go_default_library",
        "@org_golang_google_genproto//googleapis/iam/admin/v1:go_default_library",
        "@org_golang_google_genproto//googleapis/iam/v1:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//metadata:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "iam_client_example_test.go",
        "mock_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/github.com/golang/protobuf/ptypes:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/iterator:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "@io_bazel_rules_go//proto/wkt:empty_go_proto",
        "@org_golang_google_genproto//googleapis/iam/admin/v1:go_default_library",
        "@org_golang_google_genproto//googleapis/iam/v1:go_default_library",
        "@org_golang_google_genproto//googleapis/rpc/status:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//metadata:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)
