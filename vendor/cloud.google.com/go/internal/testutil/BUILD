load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "cmp.go",
        "context.go",
        "go18.go",
        "server.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/internal/testutil",
    importpath = "cloud.google.com/go/internal/testutil",
    visibility = ["//vendor/cloud.google.com/go:__subpackages__"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/golang.org/x/oauth2/google:go_default_library",
        "//vendor/golang.org/x/oauth2/jwt:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@io_opencensus_go//plugin/ocgrpc:go_default_library",
        "@io_opencensus_go//stats/view:go_default_library",
        "@io_opencensus_go//trace:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["server_test.go"],
    embed = [":go_default_library"],
    deps = [
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
    ],
)
