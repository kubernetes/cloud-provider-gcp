package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_test(
    name = "go_default_test",
    srcs = [
        "ipset_test.go",
        "proxier_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/intstr:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/ipvs/testing:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/util:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/util/testing:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/ipset:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/ipset/testing:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/iptables/testing:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/ipvs:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/ipvs/testing:go_default_library",
        "//vendor/k8s.io/utils/exec:go_default_library",
        "//vendor/k8s.io/utils/exec/testing:go_default_library",
    ],
)

go_library(
    name = "go_default_library",
    srcs = [
        "ipset.go",
        "netlink.go",
        "netlink_linux.go",
        "netlink_unsupported.go",
        "proxier.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/proxy/ipvs",
    importpath = "k8s.io/kubernetes/pkg/proxy/ipvs",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/client-go/tools/record:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/healthcheck:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/metrics:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/util:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/async:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/conntrack:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/ipset:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/ipvs:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/net:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/sysctl:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/version:go_default_library",
        "//vendor/k8s.io/utils/exec:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/golang.org/x/sys/unix:go_default_library",
            "@com_github_vishvananda_netlink//:go_default_library",
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
        "//pkg/proxy/ipvs/testing:all-srcs",
    ],
    tags = ["automanaged"],
)
