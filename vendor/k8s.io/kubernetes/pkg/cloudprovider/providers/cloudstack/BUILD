package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "cloudstack.go",
        "cloudstack_instances.go",
        "cloudstack_loadbalancer.go",
        "metadata.go",
        "metadata_linux.go",
        "metadata_other.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/cloudprovider/providers/cloudstack",
    importpath = "k8s.io/kubernetes/pkg/cloudprovider/providers/cloudstack",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/gopkg.in/gcfg.v1:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/cloudprovider:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller:go_default_library",
        "@com_github_d2g_dhcp4//:go_default_library",
        "@com_github_kardianos_osext//:go_default_library",
        "@com_github_xanzy_go_cloudstack//cloudstack:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:android": [
            "@com_github_d2g_dhcp4client//:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:darwin": [
            "@com_github_d2g_dhcp4client//:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:dragonfly": [
            "@com_github_d2g_dhcp4client//:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:freebsd": [
            "@com_github_d2g_dhcp4client//:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "@com_github_d2g_dhcp4client//:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:nacl": [
            "@com_github_d2g_dhcp4client//:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:netbsd": [
            "@com_github_d2g_dhcp4client//:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:openbsd": [
            "@com_github_d2g_dhcp4client//:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:plan9": [
            "@com_github_d2g_dhcp4client//:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:solaris": [
            "@com_github_d2g_dhcp4client//:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:windows": [
            "@com_github_d2g_dhcp4client//:go_default_library",
        ],
        "//conditions:default": [],
    }),
)

go_test(
    name = "go_default_test",
    srcs = ["cloudstack_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
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
