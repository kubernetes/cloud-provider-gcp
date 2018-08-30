load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["client.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/api/transport/bytestream",
    importpath = "google.golang.org/api/transport/bytestream",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "@org_golang_google_genproto//googleapis/bytestream:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "client_test.go",
        "examples_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/transport/bytestream/internal:go_default_library",
        "@org_golang_google_genproto//googleapis/bytestream:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//metadata:go_default_library",
    ],
)
