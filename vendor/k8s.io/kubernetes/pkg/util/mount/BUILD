load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "exec.go",
        "exec_mount.go",
        "exec_mount_unsupported.go",
        "fake.go",
        "mount.go",
        "mount_linux.go",
        "mount_unsupported.go",
        "mount_windows.go",
        "nsenter_mount.go",
        "nsenter_mount_unsupported.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/util/mount",
    importpath = "k8s.io/kubernetes/pkg/util/mount",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/utils/exec:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:android": [
            "//vendor/k8s.io/kubernetes/pkg/util/nsenter:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:darwin": [
            "//vendor/k8s.io/kubernetes/pkg/util/nsenter:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:dragonfly": [
            "//vendor/k8s.io/kubernetes/pkg/util/nsenter:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:freebsd": [
            "//vendor/k8s.io/kubernetes/pkg/util/nsenter:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/golang.org/x/sys/unix:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/file:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/io:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/nsenter:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:nacl": [
            "//vendor/k8s.io/kubernetes/pkg/util/nsenter:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:netbsd": [
            "//vendor/k8s.io/kubernetes/pkg/util/nsenter:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:openbsd": [
            "//vendor/k8s.io/kubernetes/pkg/util/nsenter:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:plan9": [
            "//vendor/k8s.io/kubernetes/pkg/util/nsenter:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:solaris": [
            "//vendor/k8s.io/kubernetes/pkg/util/nsenter:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/k8s.io/kubernetes/pkg/util/file:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/nsenter:go_default_library",
        ],
        "//conditions:default": [],
    }),
)

go_test(
    name = "go_default_test",
    srcs = [
        "exec_mount_test.go",
        "mount_linux_test.go",
        "mount_windows_test.go",
        "nsenter_mount_test.go",
        "safe_format_and_mount_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/utils/exec/testing:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/github.com/golang/glog:go_default_library",
            "//vendor/golang.org/x/sys/unix:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/nsenter:go_default_library",
            "//vendor/k8s.io/utils/exec:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:windows": [
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
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
