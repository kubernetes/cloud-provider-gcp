load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["server.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/api/transport/bytestream/internal",
    importpath = "google.golang.org/api/transport/bytestream/internal",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "@org_golang_google_genproto//googleapis/bytestream:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "examples_test.go",
        "server_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "@org_golang_google_genproto//googleapis/bytestream:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
    ],
)
