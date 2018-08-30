load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "apparmor.go",
        "events.go",
        "framework.go",
        "kubelet.go",
        "kubelet_perf.go",
        "mount_propagation.go",
        "pod_gc.go",
        "pods.go",
        "pre_stop.go",
        "security_context.go",
        "ssh.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/test/e2e/node",
    importpath = "k8s.io/kubernetes/test/e2e/node",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/api/resource:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/fields:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/labels:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/watch:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1:go_default_library",
        "//vendor/k8s.io/kubernetes/test/e2e/common:go_default_library",
        "//vendor/k8s.io/kubernetes/test/e2e/framework:go_default_library",
        "//vendor/k8s.io/kubernetes/test/utils:go_default_library",
        "//vendor/k8s.io/kubernetes/test/utils/image:go_default_library",
        "@com_github_onsi_ginkgo//:go_default_library",
        "@com_github_onsi_gomega//:go_default_library",
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
