load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["fake_client.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/volume/csi/fake",
    importpath = "k8s.io/kubernetes/pkg/volume/csi/fake",
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_container_storage_interface_spec//lib/go/csi/v0:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
