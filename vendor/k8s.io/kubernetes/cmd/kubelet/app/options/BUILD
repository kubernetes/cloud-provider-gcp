package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "container_runtime.go",
        "globalflags.go",
        "globalflags_linux.go",
        "globalflags_other.go",
        "options.go",
        "osflags_others.go",
        "osflags_windows.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/cmd/kubelet/app/options",
    importpath = "k8s.io/kubernetes/cmd/kubelet/app/options",
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/validation:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flag:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/logs:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/componentconfig:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/core:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/credentialprovider/azure:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/credentialprovider/gcp:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/features:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig/scheme:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig/v1beta1:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig/validation:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/config:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/types:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/master/ports:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/taints:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/version/verflag:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:linux": [
            "@com_github_google_cadvisor//container/common:go_default_library",
            "@com_github_google_cadvisor//container/containerd:go_default_library",
            "@com_github_google_cadvisor//container/docker:go_default_library",
            "@com_github_google_cadvisor//container/raw:go_default_library",
            "@com_github_google_cadvisor//machine:go_default_library",
            "@com_github_google_cadvisor//manager:go_default_library",
            "@com_github_google_cadvisor//storage:go_default_library",
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

go_test(
    name = "go_default_test",
    srcs = ["options_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/diff:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flag:go_default_library",
    ],
)
