package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "fake_iptables.go",
        "hostport.go",
        "hostport_manager.go",
        "hostport_syncer.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/dockershim/network/hostport",
    importpath = "k8s.io/kubernetes/pkg/kubelet/dockershim/network/hostport",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/errors:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/proxy/iptables:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/conntrack:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/net:go_default_library",
        "//vendor/k8s.io/utils/exec:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "fake_iptables_test.go",
        "hostport_manager_test.go",
        "hostport_syncer_test.go",
        "hostport_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/iptables:go_default_library",
        "//vendor/k8s.io/utils/exec:go_default_library",
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
        "//pkg/kubelet/dockershim/network/hostport/testing:all-srcs",
    ],
    tags = ["automanaged"],
)
