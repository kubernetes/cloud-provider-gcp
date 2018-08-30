package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "cadvisor_fake.go",
        "cadvisor_mock.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/cadvisor/testing",
    importpath = "k8s.io/kubernetes/pkg/kubelet/cadvisor/testing",
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/kubelet/cadvisor:go_default_library",
        "@com_github_google_cadvisor//events:go_default_library",
        "@com_github_google_cadvisor//info/v1:go_default_library",
        "@com_github_google_cadvisor//info/v2:go_default_library",
        "@com_github_stretchr_testify//mock:go_default_library",
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
)
