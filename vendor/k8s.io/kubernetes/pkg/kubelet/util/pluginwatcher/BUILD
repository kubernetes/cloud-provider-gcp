package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "example_plugin.go",
        "plugin_watcher.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/util/pluginwatcher",
    importpath = "k8s.io/kubernetes/pkg/kubelet/util/pluginwatcher",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/pluginregistration/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/util/pluginwatcher/example_plugin_apis/v1beta1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/util/pluginwatcher/example_plugin_apis/v1beta2:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/filesystem:go_default_library",
        "@com_github_fsnotify_fsnotify//:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
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
        "//pkg/kubelet/util/pluginwatcher/example_plugin_apis/v1beta1:all-srcs",
        "//pkg/kubelet/util/pluginwatcher/example_plugin_apis/v1beta2:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["plugin_watcher_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/pluginregistration/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/util/pluginwatcher/example_plugin_apis/v1beta1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/util/pluginwatcher/example_plugin_apis/v1beta2:go_default_library",
        "@com_github_stretchr_testify//require:go_default_library",
    ],
)
