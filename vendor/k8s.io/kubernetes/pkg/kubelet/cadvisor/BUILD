package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "cadvisor_linux.go",
        "cadvisor_unsupported.go",
        "cadvisor_windows.go",
        "doc.go",
        "helpers_linux.go",
        "helpers_unsupported.go",
        "types.go",
        "util.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/cadvisor",
    importpath = "k8s.io/kubernetes/pkg/kubelet/cadvisor",
    deps = [
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/api/resource:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core/v1/helper:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/features:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/types:go_default_library",
        "@com_github_google_cadvisor//events:go_default_library",
        "@com_github_google_cadvisor//info/v1:go_default_library",
        "@com_github_google_cadvisor//info/v2:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/github.com/golang/glog:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/runtime:go_default_library",
            "@com_github_google_cadvisor//cache/memory:go_default_library",
            "@com_github_google_cadvisor//container:go_default_library",
            "@com_github_google_cadvisor//fs:go_default_library",
            "@com_github_google_cadvisor//http:go_default_library",
            "@com_github_google_cadvisor//manager:go_default_library",
            "@com_github_google_cadvisor//metrics:go_default_library",
            "@com_github_google_cadvisor//utils/sysfs:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/winstats:go_default_library",
        ],
        "//conditions:default": [],
    }),
)

go_test(
    name = "go_default_test",
    srcs = [
        "cadvisor_linux_test.go",
        "util_test.go",
    ],
    embed = [":go_default_library"],
    deps = select({
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/k8s.io/apimachinery/pkg/api/resource:go_default_library",
            "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/apis/core/v1/helper:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/features:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/types:go_default_library",
            "@com_github_google_cadvisor//info/v1:go_default_library",
            "@com_github_google_cadvisor//metrics:go_default_library",
            "@com_github_stretchr_testify//assert:go_default_library",
        ],
        "//conditions:default": [],
    }),
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
        "//pkg/kubelet/cadvisor/testing:all-srcs",
    ],
    tags = ["automanaged"],
)
