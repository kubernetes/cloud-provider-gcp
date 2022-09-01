load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "gcpcredential",
    srcs = [
        "credentialutil.go",
        "gcpcredential.go",
        "registry_marshal.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/providers/gce/gcpcredential",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/cloud-provider/credentialconfig",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

go_test(
    name = "gcpcredential_test",
    srcs = ["registry_marshal_test.go"],
    embed = [":gcpcredential"],
    deps = ["//vendor/k8s.io/cloud-provider/credentialconfig"],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
