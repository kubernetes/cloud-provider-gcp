load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "fs_resource_analyzer.go",
        "handler.go",
        "prometheus_resource_metrics.go",
        "resource_analyzer.go",
        "summary.go",
        "summary_sys_containers.go",
        "summary_sys_containers_windows.go",
        "volume_stat_calculator.go",
    ],
    importpath = "k8s.io/kubernetes/pkg/kubelet/server/stats",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/util:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/util/format:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/volume:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/component-base/metrics:go_default_library",
        "//vendor/github.com/emicklei/go-restful:go_default_library",
        "//vendor/github.com/google/cadvisor/info/v1:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "prometheus_resource_metrics_test.go",
        "summary_test.go",
        "summary_windows_test.go",
        "volume_stat_calculator_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/server/stats/testing:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/volume:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/api/resource:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/component-base/metrics:go_default_library",
        "//vendor/k8s.io/component-base/metrics/testutil:go_default_library",
        "//vendor/github.com/stretchr/testify/assert:go_default_library",
        "//vendor/github.com/stretchr/testify/mock:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:android": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
            "//vendor/github.com/google/gofuzz:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:darwin": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
            "//vendor/github.com/google/gofuzz:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:dragonfly": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
            "//vendor/github.com/google/gofuzz:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:freebsd": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
            "//vendor/github.com/google/gofuzz:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:ios": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
            "//vendor/github.com/google/gofuzz:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
            "//vendor/github.com/google/gofuzz:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:nacl": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
            "//vendor/github.com/google/gofuzz:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:netbsd": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
            "//vendor/github.com/google/gofuzz:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:openbsd": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
            "//vendor/github.com/google/gofuzz:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:plan9": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
            "//vendor/github.com/google/gofuzz:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:solaris": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
            "//vendor/github.com/google/gofuzz:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/cm:go_default_library",
            "//vendor/github.com/google/gofuzz:go_default_library",
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
        "//vendor/k8s.io/kubernetes/pkg/kubelet/server/stats/testing:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
