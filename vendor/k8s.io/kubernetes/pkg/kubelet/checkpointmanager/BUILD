package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = ["checkpoint_manager.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/checkpointmanager",
    importpath = "k8s.io/kubernetes/pkg/kubelet/checkpointmanager",
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/kubelet/checkpointmanager/errors:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/util/store:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/filesystem:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["checkpoint_manager_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/kubelet/checkpointmanager/checksum:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/checkpointmanager/testing:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/checkpointmanager/testing/example_checkpoint_formats/v1:go_default_library",
        "@com_github_stretchr_testify//assert:go_default_library",
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
    srcs = [
        ":package-srcs",
        "//pkg/kubelet/checkpointmanager/checksum:all-srcs",
        "//pkg/kubelet/checkpointmanager/errors:all-srcs",
        "//pkg/kubelet/checkpointmanager/testing:all-srcs",
    ],
    tags = ["automanaged"],
)
