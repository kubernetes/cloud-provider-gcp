load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["azure_auth.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/cloudprovider/providers/azure/auth",
    importpath = "k8s.io/kubernetes/pkg/cloudprovider/providers/azure/auth",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/golang.org/x/crypto/pkcs12:go_default_library",
        "@com_github_azure_go_autorest//autorest/adal:go_default_library",
        "@com_github_azure_go_autorest//autorest/azure:go_default_library",
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
