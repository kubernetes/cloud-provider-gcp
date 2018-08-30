package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "kubenet.go",
        "kubenet_linux.go",
        "kubenet_unsupported.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network/kubenet",
    importpath = "k8s.io/kubernetes/pkg/kubelet/dockershim/network/kubenet",
    deps = select({
        "@io_bazel_rules_go//go/platform:android": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:darwin": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:dragonfly": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:freebsd": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/github.com/golang/glog:go_default_library",
            "//vendor/golang.org/x/sys/unix:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/errors:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/net:go_default_library",
            "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network/hostport:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/bandwidth:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/dbus:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/ebtables:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/sysctl:go_default_library",
            "//vendor/k8s.io/utils/exec:go_default_library",
            "@com_github_containernetworking_cni//libcni:go_default_library",
            "@com_github_containernetworking_cni//pkg/types:go_default_library",
            "@com_github_containernetworking_cni//pkg/types/020:go_default_library",
            "@com_github_vishvananda_netlink//:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:nacl": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:netbsd": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:openbsd": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:plan9": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:solaris": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
        ],
        "//conditions:default": [],
    }),
)

go_test(
    name = "go_default_test",
    srcs = ["kubenet_linux_test.go"],
    embed = [":go_default_library"],
    deps = select({
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network/cni/testing:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network/hostport/testing:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network/testing:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/bandwidth:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/iptables/testing:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/util/sysctl/testing:go_default_library",
            "//vendor/k8s.io/utils/exec:go_default_library",
            "//vendor/k8s.io/utils/exec/testing:go_default_library",
            "@com_github_stretchr_testify//assert:go_default_library",
            "@com_github_stretchr_testify//mock:go_default_library",
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
)
