load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "topology.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/topology",
    importpath = "k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/topology",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/cm/cpuset:go_default_library",
        "@com_github_google_cadvisor//info/v1:go_default_library",
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
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["topology_test.go"],
    embed = [":go_default_library"],
    deps = ["@com_github_google_cadvisor//info/v1:go_default_library"],
)
