package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "cni.go",
        "cni_others.go",
        "cni_windows.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network/cni",
    importpath = "k8s.io/kubernetes/pkg/kubelet/dockershim/network/cni",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
        "//vendor/k8s.io/utils/exec:go_default_library",
        "@com_github_containernetworking_cni//libcni:go_default_library",
        "@com_github_containernetworking_cni//pkg/types:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:windows": [
            "@com_github_containernetworking_cni//pkg/types/020:go_default_library",
        ],
        "//conditions:default": [],
    }),
)

go_test(
    name = "go_default_test",
    srcs = ["cni_test.go"],
    embed = [":go_default_library"],
    deps = select({
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/k8s.io/api/core/v1:go_default_library",
            "//vendor/k8s.io/client-go/kubernetes:go_default_library",
            "//vendor/k8s.io/client-go/util/testing:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/container/testing:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network/cni/testing:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network/hostport:go_default_library",
            "//vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network/testing:go_default_library",
            "//vendor/k8s.io/utils/exec:go_default_library",
            "//vendor/k8s.io/utils/exec/testing:go_default_library",
            "@com_github_containernetworking_cni//pkg/types/020:go_default_library",
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
    srcs = [
        ":package-srcs",
        "//pkg/kubelet/dockershim/network/cni/testing:all-srcs",
    ],
    tags = ["automanaged"],
)
