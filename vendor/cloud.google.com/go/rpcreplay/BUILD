load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "rpcreplay.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/rpcreplay",
    importpath = "cloud.google.com/go/rpcreplay",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/cloud.google.com/go/rpcreplay/proto/rpcreplay:go_default_library",
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/github.com/golang/protobuf/ptypes:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "@io_bazel_rules_go//proto/wkt:any_go_proto",
        "@org_golang_google_genproto//googleapis/rpc/status:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//metadata:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "example_test.go",
        "fake_test.go",
        "rpcreplay_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "//vendor/cloud.google.com/go/rpcreplay/proto/intstore:go_default_library",
        "//vendor/cloud.google.com/go/rpcreplay/proto/rpcreplay:go_default_library",
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)
