load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "video_intelligence_client.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/videointelligence/apiv1",
    importpath = "cloud.google.com/go/videointelligence/apiv1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/cloud.google.com/go/internal/version:go_default_library",
        "//vendor/cloud.google.com/go/longrunning:go_default_library",
        "//vendor/cloud.google.com/go/longrunning/autogen:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "//vendor/google.golang.org/api/transport:go_default_library",
        "@com_github_googleapis_gax_go//:go_default_library",
        "@org_golang_google_genproto//googleapis/cloud/videointelligence/v1:go_default_library",
        "@org_golang_google_genproto//googleapis/longrunning:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//metadata:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "mock_test.go",
        "video_intelligence_client_example_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/github.com/golang/protobuf/ptypes:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "@org_golang_google_genproto//googleapis/cloud/videointelligence/v1:go_default_library",
        "@org_golang_google_genproto//googleapis/longrunning:go_default_library",
        "@org_golang_google_genproto//googleapis/rpc/status:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//metadata:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)
