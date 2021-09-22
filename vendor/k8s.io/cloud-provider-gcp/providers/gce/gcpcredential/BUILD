load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "credentialutil.go",
        "gcpcredential.go",
        "registry_marshal.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/providers/gce/gcpcredential",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/cloud-provider/credentialconfig:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["registry_marshal_test.go"],
    embed = [":go_default_library"],
    deps = ["//vendor/k8s.io/cloud-provider/credentialconfig:go_default_library"],
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
