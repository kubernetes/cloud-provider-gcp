load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["protostruct.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/internal/protostruct",
    importpath = "cloud.google.com/go/internal/protostruct",
    visibility = ["//vendor/cloud.google.com/go:__subpackages__"],
    deps = ["@io_bazel_rules_go//proto/wkt:struct_go_proto"],
)

go_test(
    name = "go_default_test",
    srcs = ["protostruct_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "@io_bazel_rules_go//proto/wkt:struct_go_proto",
    ],
)
